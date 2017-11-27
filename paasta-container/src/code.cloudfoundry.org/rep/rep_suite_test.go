package rep_test

import (
	"time"

	"code.cloudfoundry.org/cfhttp"
	executorfakes "code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/rep"
	"code.cloudfoundry.org/rep/evacuation/evacuation_context/fake_evacuation_context"
	"code.cloudfoundry.org/rep/repfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	cfHttpTimeout time.Duration
	auctionRep    *repfakes.FakeClient
	factory       rep.ClientFactory

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
	auctionRep = &repfakes.FakeClient{}
	fakeExecutorClient = &executorfakes.FakeClient{}
	fakeEvacuatable = &fake_evacuation_context.FakeEvacuatable{}

	var err error
	httpClient := cfhttp.NewClient()
	factory, err = rep.NewClientFactory(httpClient, httpClient, nil)
	Expect(err).NotTo(HaveOccurred())
})
