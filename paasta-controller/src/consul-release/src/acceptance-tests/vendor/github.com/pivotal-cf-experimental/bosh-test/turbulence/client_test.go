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

const expectedKillPOSTRequest = `{
	"Tasks": [{
		"Type": "Kill"
	}],
	"Selector": {
		"ID": {
			"Values": ["some-id"]
		}
	}
}`

const expectedDelayPOSTRequest = `{
	"Tasks": [{
		"Type": "control-net",
		"Timeout": "2000ms",
		"Delay": "1000ms"
	}],
	"Selector": {
		"ID": {
			"Values": ["some-id"]
		}
	}
}`

const successfulPOSTResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "",
	"ExecutionCompletedAt": "",
	"Events": null
}`

const successfulInitialIncompleteGETResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "",
	"ExecutionCompletedAt": "",
	"Events": [
		{"Error": ""}
	]
}`

const successfulIncompleteGETResponse = `{
	"ID": "someID",
	"ExecutionStartedAt": "0001-01-01T00:00:00Z",
	"ExecutionCompletedAt": "",
	"Events": [
		{
			"Type": "ControlNet",
			"ExecutionStartedAt": "0001-01-01T00:00:00Z"
		}
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

	receivedPOSTBody      []byte
	errorReadingBody      error
	POSTResponse          string
	GETResponses          []string
	postIncidentCallCount int
	getIncidentCallCount  int
}

func NewFakeTurbulenceServer() *fakeTurbulenceServer {
	fakeServer := new(fakeTurbulenceServer)
	fakeServer.POSTResponse = successfulPOSTResponse
	fakeServer.GETResponses = []string{successfulInitialIncompleteGETResponse, successfulIncompleteGETResponse, successfulCompleteGETResponse}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/api/v1/incidents" && request.Method == "POST":
			fakeServer.postIncidentCallCount++
			fakeServer.receivedPOSTBody, fakeServer.errorReadingBody = ioutil.ReadAll(request.Body)
			defer request.Body.Close()
			writer.Write([]byte(fakeServer.POSTResponse))

		case request.URL.Path == "/api/v1/incidents/someID" && request.Method == "GET":
			fakeServer.getIncidentCallCount++
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
	Describe("KillIDs", func() {
		var fakeServer *fakeTurbulenceServer
		var client turbulence.Client

		BeforeEach(func() {
			fakeServer = NewFakeTurbulenceServer()
			client = turbulence.NewClient(fakeServer.URL, 100*time.Millisecond, 40*time.Millisecond)
		})

		It("makes a POST request to create an incident and then polls to wait for completion", func() {
			errorKillingIDs := client.KillIDs([]string{"some-id"})

			Expect(fakeServer.errorReadingBody).NotTo(HaveOccurred())
			Expect(errorKillingIDs).NotTo(HaveOccurred())
			Expect(string(fakeServer.receivedPOSTBody)).To(MatchJSON(expectedKillPOSTRequest))
		})

		It("returns a timeout error when execution does not complete", func() {
			fakeServer.GETResponses = []string{successfulIncompleteGETResponse}
			errorKillingIDs := client.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs).NotTo(BeNil())
			Expect(errorKillingIDs.Error()).To(ContainSubstring("Did not finish deleting VM in time"))
		})

		It("returns an error when the turbulence response does not contain any events", func() {
			fakeServer.GETResponses = []string{invalidGETResponseWithNoEvents}
			errorKillingIDs := client.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs).To(MatchError("There should at least be one Event in response from turbulence."))
		})

		It("returns an error when a response event is not an empty string", func() {
			fakeServer.GETResponses = []string{failedGETResponse}
			errorKillingIDs := client.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs).To(MatchError("There was a turbulence event error. Check out the turbulence events (response id: someID) for more information."))
		})

		It("returns an error when the base URL is malformed", func() {
			clientWithMalformedBaseURL := turbulence.NewClient("%%%%%", 100*time.Millisecond, 40*time.Millisecond)
			errorKillingIDs := clientWithMalformedBaseURL.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs.Error()).To(ContainSubstring("invalid URL escape"))
		})

		It("returns an error when the base url has an unsupported protocol", func() {
			clientWithEmptyBaseURL := turbulence.NewClient("", 100*time.Millisecond, 40*time.Millisecond)
			errorKillingIDs := clientWithEmptyBaseURL.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs.Error()).To(ContainSubstring("unsupported protocol scheme"))
		})

		It("returns an error when turbulence responds with malformed JSON", func() {
			fakeServer.POSTResponse = "some-invalid-json"
			errorKillingIDs := client.KillIDs([]string{"some-id"})
			Expect(errorKillingIDs.Error()).To(ContainSubstring("Unable to decode turbulence response."))
		})
	})

	Describe("Delay", func() {
		var fakeServer *fakeTurbulenceServer
		var client turbulence.Client

		BeforeEach(func() {
			fakeServer = NewFakeTurbulenceServer()
			client = turbulence.NewClient(fakeServer.URL, 100*time.Millisecond, 40*time.Millisecond)
		})

		It("makes a POST request to create a control-network delay incident and polls for initial execution", func() {
			resp, err := client.Delay([]string{"some-id"}, time.Second, 2*time.Second)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeServer.postIncidentCallCount).To(Equal(1))
			Expect(fakeServer.errorReadingBody).NotTo(HaveOccurred())
			Expect(string(fakeServer.receivedPOSTBody)).To(MatchJSON(expectedDelayPOSTRequest))
			Expect(resp).To(Equal(turbulence.Response{
				ID:                   "someID",
				ExecutionStartedAt:   "0001-01-01T00:00:00Z",
				ExecutionCompletedAt: "",
				Events: []turbulence.ResponseEvent{
					{
						Type:               "ControlNet",
						ExecutionStartedAt: "0001-01-01T00:00:00Z",
					},
				},
			}))

			Expect(fakeServer.getIncidentCallCount).To(Equal(2))
		})

		It("returns an error when the POST request fails", func() {
			clientWithMalformedBaseURL := turbulence.NewClient("%%%%%", 100*time.Millisecond, 40*time.Millisecond)
			_, errorDelaying := clientWithMalformedBaseURL.Delay([]string{"some-id"}, time.Second, 2*time.Second)
			Expect(errorDelaying.Error()).To(ContainSubstring("invalid URL escape"))
		})

		It("returns a timeout error when the control net event does not start in time", func() {
			fakeServer.GETResponses = []string{successfulInitialIncompleteGETResponse}
			_, errorDelaying := client.Delay([]string{"some-id"}, time.Second, 2*time.Second)
			Expect(errorDelaying.Error()).To(ContainSubstring("Did not start control-net event in time"))
		})
	})
})
