package vizzini_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The container environment", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = "http://" + RouteForGuid(guid) + "/env?json=true"
		lrp = DesiredLRPWithGuid(guid)
		lrp.Ports = []uint32{8080, 5000}
	})

	getEnvs := func(url string) [][]string {
		response, err := http.Get(url)
		Expect(err).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		envs := [][]string{}
		err = json.NewDecoder(response.Body).Decode(&envs)
		Expect(err).NotTo(HaveOccurred())
		response.Body.Close()
		return envs
	}

	Describe("InstanceGuid and InstanceIndex", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("matches the ActualLRP's index and instance guid", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())

			envs := getEnvs(url)

			Expect(envs).To(ContainElement([]string{"INSTANCE_INDEX", "0"}))
			Expect(envs).To(ContainElement([]string{"INSTANCE_GUID", actualLRP.InstanceGuid}))

		})
	})

	//{LOCAL} because: Instance IP and PORT are not injected by default.  One needs to opt-into this feature.
	Describe("{LOCAL} Instance IP and PORT", func() {
		BeforeEach(func() {
			Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
			Eventually(EndpointCurler(url), 40).Should(Equal(http.StatusOK))
		})

		It("matches the ActualLRP's index and instance guid", func() {
			actualLRP, err := ActualLRPByProcessGuidAndIndex(logger, guid, 0)
			Expect(err).NotTo(HaveOccurred())

			type cfPortMapping struct {
				External uint32 `json:"external"`
				Internal uint32 `json:"internal"`
			}

			cfPortMappingPayload, err := json.Marshal([]cfPortMapping{
				{External: actualLRP.Ports[0].HostPort, Internal: actualLRP.Ports[0].ContainerPort},
				{External: actualLRP.Ports[1].HostPort, Internal: actualLRP.Ports[1].ContainerPort},
			})
			Expect(err).NotTo(HaveOccurred())

			envs := getEnvs(url)
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_IP", actualLRP.Address}), "If this fails, then your executor may not be configured to expose ip:port to the container")
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_PORT", fmt.Sprintf("%d", actualLRP.Ports[0].HostPort)}))
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_ADDR", fmt.Sprintf("%s:%d", actualLRP.Address, actualLRP.Ports[0].HostPort)}))
			Expect(envs).To(ContainElement([]string{"CF_INSTANCE_PORTS", string(cfPortMappingPayload)}))
		})
	})
})
