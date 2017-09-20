package turbulence_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/bosh-test/turbulence"
)

const expectedPOSTRequest = `{
	"Tasks": [{
		"Type": "kill"
	}],
	"Deployments": [{
		"Name": "deployment-name",
		"Jobs": [{
			"Name": "job-name",
			"Indices": [0]
		}]
	}]
}`

const successfulPOSTResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "",
	"Events": null
}`

const successfulIncompleteGETResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "",
	"Events": [
		{"Error": ""}
	]
}`

const successfulCompleteGETResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "0001-01-01T00:01:00Z",
	"Events": [
		{"Error": ""}
	]
}`

const invalidGETResponseWithNoEvents = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "0001-01-01T00:01:00Z",
	"Events": []
}`

const failedGETResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "0001-01-01T00:01:00Z",
	"Events": [
		{"Error": ""},
		{"Error": "some-error"}
	]
}`

type fakeTurbulenceServer struct {
	URL string

	receivedPOSTBody []byte
	errorReadingBody error
	POSTResponse     string
	GETResponses     []string
}

func NewFakeTurbulenceServer() *fakeTurbulenceServer {
	fakeServer := new(fakeTurbulenceServer)
	fakeServer.POSTResponse = successfulPOSTResponse
	fakeServer.GETResponses = []string{successfulIncompleteGETResponse, successfulCompleteGETResponse}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/v1/incidents" && request.Method == "POST":
			fakeServer.receivedPOSTBody, fakeServer.errorReadingBody = ioutil.ReadAll(request.Body)
			defer request.Body.Close()
			writer.Write([]byte(fakeServer.POSTResponse))

		case request.URL.Path == "/api/v1/incidents/someID" && request.Method == "GET":
			writer.Write([]byte(fakeServer.GETResponses[0]))
			if len(fakeServer.GETResponses) > 1 {
				fakeServer.GETResponses = fakeServer.GETResponses[1:]
			}
		}
	}))

	fakeServer.URL = server.URL
	return fakeServer
}

var _ = Describe("Client", func() {
	Describe("KillIndices", func() {
		var fakeServer *fakeTurbulenceServer
		var client turbulence.Client

		BeforeEach(func() {
			fakeServer = NewFakeTurbulenceServer()
			client = turbulence.NewClient(fakeServer.URL, 100*time.Millisecond, 40*time.Millisecond)
		})

		It("makes a POST request to create an incident and then polls to wait for completion", func() {
			errorKillingIndices := client.KillIndices("deployment-name", "job-name", []int{0})

			Expect(fakeServer.errorReadingBody).NotTo(HaveOccurred())
			Expect(errorKillingIndices).NotTo(HaveOccurred())
			Expect(string(fakeServer.receivedPOSTBody)).To(MatchJSON(expectedPOSTRequest))
		})

		It("returns a timeout error when execution does not complete", func() {
			fakeServer.GETResponses = []string{successfulIncompleteGETResponse}
			errorKillingIndices := client.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices).NotTo(BeNil())
			Expect(errorKillingIndices.Error()).To(ContainSubstring("Did not finish deleting VM in time"))
		})

		It("returns an error when the turbulence response does not contain any events", func() {
			fakeServer.GETResponses = []string{invalidGETResponseWithNoEvents}
			errorKillingIndices := client.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices).To(MatchError("There should at least be one Event in response from turbulence."))
		})

		It("returns an error when a response event is not an empty string", func() {
			fakeServer.GETResponses = []string{failedGETResponse}
			errorKillingIndices := client.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices).To(MatchError("some-error"))
		})

		It("returns an error when the base URL is malformed", func() {
			clientWithMalformedBaseURL := turbulence.NewClient("%%%%%", 100*time.Millisecond, 40*time.Millisecond)
			errorKillingIndices := clientWithMalformedBaseURL.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices.Error()).To(ContainSubstring("invalid URL escape"))
		})

		It("returns an error when the base url has an unsupported protocol", func() {
			clientWithEmptyBaseURL := turbulence.NewClient("", 100*time.Millisecond, 40*time.Millisecond)
			errorKillingIndices := clientWithEmptyBaseURL.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices.Error()).To(ContainSubstring("unsupported protocol scheme"))
		})

		It("returns an error when turbulence responds with malformed JSON", func() {
			fakeServer.POSTResponse = "some-invalid-json"
			errorKillingIndices := client.KillIndices("deployment-name", "job-name", []int{0})
			Expect(errorKillingIndices.Error()).To(ContainSubstring("Unable to decode turbulence response."))
		})
	})
})
