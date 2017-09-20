package services_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/cf-test-helpers/services"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
)

type apiRequestInputs struct {
	method, endpoint string
	timeout          time.Duration
	data             []string
}

var _ = Describe("ConfiguredContext", func() {

	var (
		FakeCfCalls             [][]string
		FakeCfCallback          map[string]func(args ...string) *exec.Cmd
		FakeAsUserCalls         []cf.UserContext
		FakeApiRequestCalls     []apiRequestInputs
		FakeApiRequestCallbacks []func(method, endpoint string, response interface{}, data ...string)
	)

	var NoOpCmd = func() *exec.Cmd {
		return exec.Command("echo", "OK")
	}

	var FakeCf = func(args ...string) *gexec.Session {
		FakeCfCalls = append(FakeCfCalls, args)
		var cmd *exec.Cmd
		if callback, exists := FakeCfCallback[args[0]]; exists {
			cmd = callback(args...)
		} else {
			cmd = NoOpCmd()
		}
		session, _ := gexec.Start(cmd, nil, nil)
		return session
	}

	var FakeAsUser = func(userContext cf.UserContext, timeout time.Duration, actions func()) {
		FakeAsUserCalls = append(FakeAsUserCalls, userContext)
		actions()
	}

	var FakeApiRequest = func(method, endpoint string, response interface{}, timeout time.Duration, data ...string) {
		FakeApiRequestCalls = append(FakeApiRequestCalls, apiRequestInputs{
			method:   method,
			endpoint: endpoint,
			timeout:  timeout,
			data:     data,
		})

		if len(FakeApiRequestCallbacks) > 0 {
			callback := FakeApiRequestCallbacks[0]
			FakeApiRequestCallbacks = FakeApiRequestCallbacks[1:]
			callback(method, endpoint, response, data...)
		}
	}

	BeforeEach(func() {
		FakeCfCalls = [][]string{}
		FakeCfCallback = map[string]func(args ...string) *exec.Cmd{}
		FakeAsUserCalls = []cf.UserContext{}
		FakeApiRequestCalls = []apiRequestInputs{}
		FakeApiRequestCallbacks = []func(method, endpoint string, response interface{}, data ...string){}

		cf.Cf = FakeCf
		cf.AsUser = FakeAsUser
		cf.ApiRequest = FakeApiRequest

		//TODO: TimeoutScale = float64(0.1)
	})

	Describe("Setup", func() {
		var (
			config services.Config
			prefix = "fake-prefix"

			context services.Context
		)

		Context("without OrgName", func() {

			BeforeEach(func() {
				config = services.Config{
					AppsDomain:        "fake-domain",
					ApiEndpoint:       "fake-endpoint",
					AdminUser:         "fake-admin-user",
					AdminPassword:     "fake-admin-password",
					SkipSSLValidation: true,
					TimeoutScale:      1,
				}
				context = services.NewContext(config, prefix)
			})

			It("executes commands as the admin user", func() {
				context.Setup()

				Expect(FakeAsUserCalls).To(Equal([]cf.UserContext{
					{
						ApiUrl:            "fake-endpoint",
						Username:          "fake-admin-user",
						Password:          "fake-admin-password",
						SkipSSLValidation: true,
					},
				}))
			})

			It("creates a new user with a unique name", func() {
				context.Setup()

				createUserCall := FakeCfCalls[0]
				Expect(createUserCall[0]).To(Equal("create-user"))
				Expect(createUserCall[1]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
				Expect(createUserCall[2]).To(Equal("meow")) //why meow??
			})

			It("creates a new quota with a unique name", func() {
				FakeApiRequestCallbacks = append(FakeApiRequestCallbacks, func(method, endpoint string, response interface{}, data ...string) {
					genericResponse, ok := response.(*cf.GenericResource)
					if !ok {
						Fail(fmt.Sprintf("Expected response to be of type *cf.GenericResource: %#v", response))
					}
					genericResponse.Metadata.Guid = "fake-guid"
				})

				context.Setup()

				createQuotaCall := FakeApiRequestCalls[0]
				Expect(createQuotaCall.method).To(Equal("POST"))
				Expect(createQuotaCall.endpoint).To(Equal("/v2/quota_definitions"))
				Expect(createQuotaCall.data).To(HaveLen(1))

				definitionJSON := createQuotaCall.data[0]
				definition := &services.QuotaDefinition{}
				err := json.Unmarshal([]byte(definitionJSON), definition)
				Expect(err).ToNot(HaveOccurred())

				Expect(definition.Name).To(MatchRegexp("fake-prefix-QUOTA-\\d+-.*"))
				Expect(definition.TotalServices).To(Equal(100))
				Expect(definition.TotalRoutes).To(Equal(1000))
				Expect(definition.MemoryLimit).To(Equal(10240))
				Expect(definition.NonBasicServicesAllowed).To(BeTrue())
			})

			It("creates a new org with a unique name", func() {
				context.Setup()

				createOrgCall := FakeCfCalls[1]
				Expect(createOrgCall).To(HaveLen(2))
				Expect(createOrgCall[0]).To(Equal("create-org"))
				Expect(createOrgCall[1]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))
			})

			It("sets the quota on the new org", func() {
				context.Setup()

				setQuotaCall := FakeCfCalls[2]
				Expect(setQuotaCall).To(HaveLen(3))
				Expect(setQuotaCall[0]).To(Equal("set-quota"))
				Expect(setQuotaCall[1]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))
				Expect(setQuotaCall[2]).To(MatchRegexp("fake-prefix-QUOTA-\\d+-.*"))
			})

			It("creates a new space with a unique name", func() {
				context.Setup()

				createSpaceCall := FakeCfCalls[3]
				Expect(createSpaceCall).To(HaveLen(4))
				Expect(createSpaceCall[0]).To(Equal("create-space"))
				Expect(createSpaceCall[1]).To(Equal("-o"))
				Expect(createSpaceCall[2]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))
				Expect(createSpaceCall[3]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
			})

			It("binds the user to the new space", func() {

				expectedRoles := []string{
					"SpaceManager",
					"SpaceDeveloper",
					"SpaceAuditor",
				}

				context.Setup()

				previousCfCallCount := 4
				for i, role := range expectedRoles {
					setRoleCall := FakeCfCalls[previousCfCallCount+i]
					Expect(setRoleCall).To(HaveLen(5))
					Expect(setRoleCall[0]).To(Equal("set-space-role"))
					Expect(setRoleCall[1]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
					Expect(setRoleCall[2]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))
					Expect(setRoleCall[3]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
					Expect(setRoleCall[4]).To(Equal(role))
				}
			})

			Context("when create-user fails", func() {
				BeforeEach(func() {
					FakeCfCallback["create-user"] = func(args ...string) *exec.Cmd {
						return exec.Command("false")
					}
				})

				It("causes gomega matcher failure with stdout & stderr", func() {
					failures := InterceptGomegaFailures(func() {
						context.Setup()
					})
					Expect(failures[0]).To(MatchRegexp(
						"Failed executing command \\(exit 1\\):\nCommand: %s\n\n\\[stdout\\]:\n%s\n\n\\[stderr\\]:\n%s",
						"false",
						"",
						"",
					))
				})
			})

			Context("when create-org fails", func() {
				BeforeEach(func() {
					FakeCfCallback["create-org"] = func(args ...string) *exec.Cmd {
						return exec.Command("false")
					}
				})

				It("causes gomega matcher failure with stdout & stderr", func() {
					failures := InterceptGomegaFailures(func() {
						context.Setup()
					})
					Expect(failures[0]).To(MatchRegexp(
						"Failed executing command \\(exit 1\\):\nCommand: %s\n\n\\[stdout\\]:\n%s\n\n\\[stderr\\]:\n%s",
						"false",
						"",
						"",
					))
				})
			})

			Context("when set-quota fails", func() {
				BeforeEach(func() {
					FakeCfCallback["set-quota"] = func(args ...string) *exec.Cmd {
						return exec.Command("false")
					}
				})

				It("causes gomega matcher failure with stdout & stderr", func() {
					failures := InterceptGomegaFailures(func() {
						context.Setup()
					})
					Expect(failures[0]).To(MatchRegexp(
						"Failed executing command \\(exit 1\\):\nCommand: %s\n\n\\[stdout\\]:\n%s\n\n\\[stderr\\]:\n%s",
						"false",
						"",
						"",
					))
				})
			})
		})

		Context("With OrgName", func() {
			BeforeEach(func() {
				config = services.Config{
					AppsDomain:        "fake-domain",
					ApiEndpoint:       "fake-endpoint",
					AdminUser:         "fake-admin-user",
					AdminPassword:     "fake-admin-password",
					SkipSSLValidation: true,
					TimeoutScale:      1,
					OrgName:           "fake-existing-org",
				}
				context = services.NewContext(config, prefix)
			})

			It("executes commands as the admin user", func() {
				context.Setup()

				Expect(FakeAsUserCalls).To(Equal([]cf.UserContext{
					{
						ApiUrl:            "fake-endpoint",
						Username:          "fake-admin-user",
						Password:          "fake-admin-password",
						SkipSSLValidation: true,
					},
				}))
			})

			It("creates a new user with a unique name", func() {
				context.Setup()

				createUserCall := FakeCfCalls[0]
				Expect(createUserCall[0]).To(Equal("create-user"))
				Expect(createUserCall[1]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
				Expect(createUserCall[2]).To(Equal("meow")) //why meow??
			})

			It("does not create a new quota", func() {
				context.Setup()

				Expect(FakeApiRequestCalls).To(BeEmpty())
			})

			It("does not create a new org", func() {

				createOrgCallCount := 0
				FakeCfCallback["create-org"] = func(args ...string) *exec.Cmd {
					createOrgCallCount += 1
					return NoOpCmd()
				}

				context.Setup()

				Expect(createOrgCallCount).To(BeZero(), "Should not have called cf create-org")
			})

			It("does not set the quota on the existing org", func() {
				context.Setup()

				setQuotaCallCount := 0
				FakeCfCallback["set-quota"] = func(args ...string) *exec.Cmd {
					setQuotaCallCount += 1
					return NoOpCmd()
				}

				Expect(setQuotaCallCount).To(BeZero(), "should not set quota on an existing org")
			})

			It("creates a new space with a unique name", func() {
				context.Setup()

				createSpaceCall := FakeCfCalls[1]
				Expect(createSpaceCall).To(HaveLen(4))
				Expect(createSpaceCall[0]).To(Equal("create-space"))
				Expect(createSpaceCall[1]).To(Equal("-o"))
				Expect(createSpaceCall[2]).To(MatchRegexp(config.OrgName))
				Expect(createSpaceCall[3]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
			})

			It("binds the user to the new space", func() {

				expectedRoles := []string{
					"SpaceManager",
					"SpaceDeveloper",
					"SpaceAuditor",
				}

				context.Setup()

				previousCfCallCount := 2
				for i, role := range expectedRoles {
					setRoleCall := FakeCfCalls[previousCfCallCount+i]
					Expect(setRoleCall).To(HaveLen(5))
					Expect(setRoleCall[0]).To(Equal("set-space-role"))
					Expect(setRoleCall[1]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
					Expect(setRoleCall[2]).To(MatchRegexp(config.OrgName))
					Expect(setRoleCall[3]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
					Expect(setRoleCall[4]).To(Equal(role))
				}
			})
		})
	})

	Describe("Teardown", func() {
		var (
			config services.Config
			prefix = "fake-prefix"

			context services.Context
		)

		Context("without an org name", func() {

			BeforeEach(func() {
				config = services.Config{
					AppsDomain:        "fake-domain",
					ApiEndpoint:       "fake-endpoint",
					AdminUser:         "fake-admin-user",
					AdminPassword:     "fake-admin-password",
					SkipSSLValidation: true,
					TimeoutScale:      1,
				}
				context = services.NewContext(config, prefix)

				FakeApiRequestCallbacks = append(FakeApiRequestCallbacks, func(method, endpoint string, response interface{}, data ...string) {
					genericResponse, ok := response.(*cf.GenericResource)
					if !ok {
						Fail(fmt.Sprintf("Expected response to be of type *cf.GenericResource: %#v", response))
					}
					genericResponse.Metadata.Guid = "fake-guid"
				})

				context.Setup()

				// ignore calls made by setup
				FakeCfCalls = [][]string{}
				FakeAsUserCalls = []cf.UserContext{}
				FakeApiRequestCalls = []apiRequestInputs{}
			})

			It("executes commands as the admin user", func() {
				context.Teardown()

				Expect(FakeAsUserCalls).To(Equal([]cf.UserContext{
					{
						ApiUrl:            "fake-endpoint",
						Username:          "fake-admin-user",
						Password:          "fake-admin-password",
						SkipSSLValidation: true,
					},
				}))
			})

			It("logs out to reset CF_HOME", func() {
				context.Teardown()

				logoutCall := FakeCfCalls[0]
				Expect(logoutCall[0]).To(Equal("logout"))
			})

			It("deletes the user created by Setup()", func() {
				context.Teardown()

				deleteUserCall := FakeCfCalls[1]
				Expect(deleteUserCall[0]).To(Equal("delete-user"))
				Expect(deleteUserCall[1]).To(Equal("-f"))
				Expect(deleteUserCall[2]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
			})

			It("deletes the space created by Setup()", func() {
				context.Teardown()

				//cf delete-space does not provide an org flag, so we need to target to org first
				targetCall := FakeCfCalls[2]
				Expect(targetCall[0]).To(Equal("target"))
				Expect(targetCall[1]).To(Equal("-o"))
				Expect(targetCall[2]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))

				deleteSpaceCall := FakeCfCalls[3]
				Expect(deleteSpaceCall[0]).To(Equal("delete-space"))
				Expect(deleteSpaceCall[1]).To(Equal("-f"))
				Expect(deleteSpaceCall[2]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
			})

			It("deletes the org created by Setup()", func() {
				context.Teardown()

				deleteOrgCall := FakeCfCalls[4]
				Expect(deleteOrgCall[0]).To(Equal("delete-org"))
				Expect(deleteOrgCall[1]).To(Equal("-f"))
				Expect(deleteOrgCall[2]).To(MatchRegexp("fake-prefix-ORG-\\d+-.*"))
			})

			It("deletes the quota created by Setup()", func() {
				context.Teardown()

				deleteQuotaCall := FakeApiRequestCalls[0]
				Expect(deleteQuotaCall.method).To(Equal("DELETE"))
				Expect(deleteQuotaCall.endpoint).To(Equal("/v2/quota_definitions/fake-guid?recursive=true"))
			})

			Context("when delete-user fails", func() {
				BeforeEach(func() {
					FakeCfCallback["delete-user"] = func(args ...string) *exec.Cmd {
						return exec.Command("false")
					}
				})

				It("causes gomega matcher failure with stdout & stderr", func() {
					failures := InterceptGomegaFailures(func() {
						context.Teardown()
					})
					Expect(failures[0]).To(MatchRegexp(
						"Failed executing command \\(exit 1\\):\nCommand: %s\n\n\\[stdout\\]:\n%s\n\n\\[stderr\\]:\n%s",
						"false",
						"",
						"",
					))
				})
			})

			Context("when delete-org fails", func() {
				BeforeEach(func() {
					FakeCfCallback["delete-org"] = func(args ...string) *exec.Cmd {
						return exec.Command("false")
					}
				})

				It("causes gomega matcher failure with stdout & stderr", func() {
					failures := InterceptGomegaFailures(func() {
						context.Teardown()
					})
					Expect(failures[0]).To(MatchRegexp(
						"Failed executing command \\(exit 1\\):\nCommand: %s\n\n\\[stdout\\]:\n%s\n\n\\[stderr\\]:\n%s",
						"false",
						"",
						"",
					))
				})
			})
		})

		Context("with an org name", func() {

			BeforeEach(func() {
				config = services.Config{
					AppsDomain:        "fake-domain",
					ApiEndpoint:       "fake-endpoint",
					AdminUser:         "fake-admin-user",
					AdminPassword:     "fake-admin-password",
					OrgName:           "fake-org-name",
					SkipSSLValidation: true,
					TimeoutScale:      1,
				}
				context = services.NewContext(config, prefix)

				FakeApiRequestCallbacks = append(FakeApiRequestCallbacks, func(method, endpoint string, response interface{}, data ...string) {
					genericResponse, ok := response.(*cf.GenericResource)
					if !ok {
						Fail(fmt.Sprintf("Expected response to be of type *cf.GenericResource: %#v", response))
					}
					genericResponse.Metadata.Guid = "fake-guid"
				})

				context.Setup()

				// ignore calls made by setup
				FakeCfCalls = [][]string{}
				FakeAsUserCalls = []cf.UserContext{}
				FakeApiRequestCalls = []apiRequestInputs{}
			})

			It("executes commands as the admin user", func() {
				context.Teardown()

				Expect(FakeAsUserCalls).To(Equal([]cf.UserContext{
					{
						ApiUrl:            "fake-endpoint",
						Username:          "fake-admin-user",
						Password:          "fake-admin-password",
						SkipSSLValidation: true,
					},
				}))
			})

			It("logs out to reset CF_HOME", func() {
				context.Teardown()

				logoutCall := FakeCfCalls[0]
				Expect(logoutCall[0]).To(Equal("logout"))
			})

			It("deletes the user created by Setup()", func() {
				context.Teardown()

				deleteUserCall := FakeCfCalls[1]
				Expect(deleteUserCall[0]).To(Equal("delete-user"))
				Expect(deleteUserCall[1]).To(Equal("-f"))
				Expect(deleteUserCall[2]).To(MatchRegexp("fake-prefix-USER-\\d+-.*"))
			})

			It("deletes the space created by Setup()", func() {
				context.Teardown()

				//cf delete-space does not provide an org flag, so we need to target to org first
				targetCall := FakeCfCalls[2]
				Expect(targetCall[0]).To(Equal("target"))
				Expect(targetCall[1]).To(Equal("-o"))
				Expect(targetCall[2]).To(MatchRegexp(config.OrgName))

				deleteSpaceCall := FakeCfCalls[3]
				Expect(deleteSpaceCall[0]).To(Equal("delete-space"))
				Expect(deleteSpaceCall[1]).To(Equal("-f"))
				Expect(deleteSpaceCall[2]).To(MatchRegexp("fake-prefix-SPACE-\\d+-.*"))
			})

			It("does not delete the existing org", func() {
				context.Teardown()

				deleteOrgCallCount := 0
				FakeCfCallback["delete-org"] = func(args ...string) *exec.Cmd {
					deleteOrgCallCount += 1
					return NoOpCmd()
				}

				Expect(deleteOrgCallCount).To(BeZero(), "should not call delete-org")
			})

			It("should not delete the existing quota", func() {

				receivedDeleteQuotaCall := false
				FakeApiRequestCallbacks = append(FakeApiRequestCallbacks, func(method, endpoint string, response interface{}, data ...string) {
					if method == "DELETE" && strings.HasPrefix(endpoint, "/v2/quota_definitions/") {
						receivedDeleteQuotaCall = true
					}
				})

				context.Teardown()

				Expect(receivedDeleteQuotaCall).To(BeFalse(), "should not delete quota definition")
			})
		})
	})
})
