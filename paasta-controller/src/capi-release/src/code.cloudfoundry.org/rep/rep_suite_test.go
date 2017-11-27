package rep_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/cfhttp"
	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/handlers"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/rata"

	"testing"
)

var (
	cfHttpTimeout    time.Duration
	auctionRep       *repfakes.FakeClient
	server           *httptest.Server
	serverThatErrors *ghttp.Server
	factory          rep.ClientFactory

	client, clientForServerThatErrors rep.Client

	fakeExecutorClient *executorfakes.FakeClient
	fakeEvacuatable    *fake_evacuation_context.FakeEvacuatable
)

func TestRep(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rep Suite")
}

var _ = BeforeSuite(func() {
	cfHttpTimeout = 1 * time.Second
	cfhttp.Initialize(cfHttpTimeout)
})

var _ = BeforeEach(func() {
	logger := lagertest.NewTestLogger("test")

	auctionRep = &repfakes.FakeClient{}
	fakeExecutorClient = &executorfakes.FakeClient{}
	fakeEvacuatable = &fake_evacuation_context.FakeEvacuatable{}

	handler, err := rata.NewRouter(rep.Routes, handlers.NewLegacy(auctionRep, fakeExecutorClient, fakeEvacuatable, logger))
	Expect(err).NotTo(HaveOccurred())
	server = httptest.NewServer(handler)

	httpClient := cfhttp.NewClient()
	factory, err = rep.NewClientFactory(httpClient, httpClient, nil)
	Expect(err).NotTo(HaveOccurred())

	client, err = factory.CreateClient(server.URL, "")
	Expect(err).NotTo(HaveOccurred())

	serverThatErrors = ghttp.NewServer()
	erroringHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		serverThatErrors.CloseClientConnections()
	})
	//5 erroringHandlers should be more than enough: none of the individual tests should make more than 5 requests to this server
	serverThatErrors.AppendHandlers(erroringHandler, erroringHandler, erroringHandler, erroringHandler, erroringHandler)

	clientForServerThatErrors, err = factory.CreateClient(serverThatErrors.URL(), "")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	server.Close()
	serverThatErrors.Close()
})
