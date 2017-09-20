package initializer_test

import (
	"net/http"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/executor/initializer"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/lagertest"
	fake_metric "github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Initializer", func() {
	var initialTime time.Time
	var sender *fake_metric.FakeMetricSender
	var fakeGarden *ghttp.Server
	var fakeClock *fakeclock.FakeClock
	var errCh chan error
	var done chan struct{}
	var config initializer.Configuration

	BeforeEach(func() {
		initialTime = time.Now()
		sender = fake_metric.NewFakeMetricSender()
		metrics.Initialize(sender, nil)
		fakeGarden = ghttp.NewUnstartedServer()
		fakeClock = fakeclock.NewFakeClock(initialTime)
		errCh = make(chan error, 1)
		done = make(chan struct{})

		fakeGarden.RouteToHandler("GET", "/ping", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/containers", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		fakeGarden.RouteToHandler("GET", "/capacity", ghttp.RespondWithJSONEncoded(http.StatusOK,
			garden.Capacity{MemoryInBytes: 1024 * 1024 * 1024, DiskInBytes: 2048 * 1024 * 1024, MaxContainers: 4}))
		fakeGarden.RouteToHandler("GET", "/containers/bulk_info", ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{}))
		config = initializer.DefaultConfiguration
	})

	AfterEach(func() {
		Eventually(done).Should(BeClosed())
		fakeGarden.Close()
	})

	JustBeforeEach(func() {
		fakeGarden.Start()
		config.GardenAddr = fakeGarden.HTTPTestServer.Listener.Addr().String()
		config.GardenNetwork = "tcp"
		go func() {
			_, _, err := initializer.Initialize(lagertest.NewTestLogger("test"), config, fakeClock)
			errCh <- err
			close(done)
		}()
	})

	checkStalledMetric := func() float64 {
		return sender.GetValue("StalledGardenDuration").Value
	}

	Context("when garden doesn't respond", func() {
		var waitChan chan struct{}

		BeforeEach(func() {
			waitChan = make(chan struct{})
			fakeGarden.RouteToHandler("GET", "/ping", func(w http.ResponseWriter, req *http.Request) {
				<-waitChan
				ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, req)
			})
		})

		AfterEach(func() {
			close(waitChan)
		})

		It("emits metrics when garden doesn't respond", func() {
			Consistently(checkStalledMetric, 10*time.Millisecond).Should(BeEquivalentTo(0))
			fakeClock.WaitForWatcherAndIncrement(initializer.StalledMetricHeartbeatInterval)
			Eventually(checkStalledMetric).Should(BeNumerically("~", fakeClock.Since(initialTime)))
		})
	})

	Context("when garden responds", func() {
		It("emits 0", func() {
			Eventually(func() bool { return sender.HasValue("StalledGardenDuration") }).Should(BeTrue())
			Expect(checkStalledMetric()).To(BeEquivalentTo(0))
		})
	})

	Context("when garden responds with an error", func() {
		var retried chan struct{}

		BeforeEach(func() {
			callCount := 0
			retried = make(chan struct{})
			fakeGarden.RouteToHandler("GET", "/ping", func(w http.ResponseWriter, req *http.Request) {
				callCount++
				if callCount == 1 {
					ghttp.RespondWith(http.StatusInternalServerError, "")(w, req)
				} else if callCount == 2 {
					ghttp.RespondWithJSONEncoded(http.StatusOK, struct{}{})(w, req)
					close(retried)
				}
			})
		})

		It("retries on a timer until it succeeds", func() {
			Consistently(retried).ShouldNot(BeClosed())
			fakeClock.Increment(initializer.PingGardenInterval)
			Eventually(retried).Should(BeClosed())
		})

		It("emits zero once it succeeds", func() {
			Consistently(func() bool { return sender.HasValue("StalledGardenDuration") }).Should(BeFalse())
			fakeClock.Increment(initializer.PingGardenInterval)
			Eventually(func() bool { return sender.HasValue("StalledGardenDuration") }).Should(BeTrue())
			Expect(checkStalledMetric()).To(BeEquivalentTo(0))
		})

		Context("when the error is unrecoverable", func() {
			BeforeEach(func() {
				fakeGarden.RouteToHandler(
					"GET",
					"/ping",
					ghttp.RespondWith(http.StatusGatewayTimeout, `{ "Type": "UnrecoverableError" , "Message": "Extra Special Error Message"}`),
				)
			})

			It("returns an error", func() {
				Eventually(errCh).Should(Receive(HaveOccurred()))
			})
		})
	})

	Context("when the post setup hook is invalid", func() {
		BeforeEach(func() {
			config.PostSetupHook = "unescaped quote\\"
		})

		It("fails fast", func() {
			Eventually(errCh).Should(Receive(HaveOccurred()))
		})
	})

	Describe("configuring trusted CA bundle", func() {
		Context("when valid", func() {
			BeforeEach(func() {
				config.CACertsForDownloads = []byte(`-----BEGIN CERTIFICATE-----
MIIBdzCCASOgAwIBAgIBADALBgkqhkiG9w0BAQUwEjEQMA4GA1UEChMHQWNtZSBD
bzAeFw03MDAxMDEwMDAwMDBaFw00OTEyMzEyMzU5NTlaMBIxEDAOBgNVBAoTB0Fj
bWUgQ28wWjALBgkqhkiG9w0BAQEDSwAwSAJBAN55NcYKZeInyTuhcCwFMhDHCmwa
IUSdtXdcbItRB/yfXGBhiex00IaLXQnSU+QZPRZWYqeTEbFSgihqi1PUDy8CAwEA
AaNoMGYwDgYDVR0PAQH/BAQDAgCkMBMGA1UdJQQMMAoGCCsGAQUFBwMBMA8GA1Ud
EwEB/wQFMAMBAf8wLgYDVR0RBCcwJYILZXhhbXBsZS5jb22HBH8AAAGHEAAAAAAA
AAAAAAAAAAAAAAEwCwYJKoZIhvcNAQEFA0EAAoQn/ytgqpiLcZu9XKbCJsJcvkgk
Se6AbGXgSlq+ZCEVo0qIwSgeBqmsJxUu7NCSOwVJLYNEBO2DtIxoYVk+MA==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIFATCCAuugAwIBAgIBATALBgkqhkiG9w0BAQswEjEQMA4GA1UEAxMHZGllZ29D
QTAeFw0xNjAyMTYyMTU1MzNaFw0yNjAyMTYyMTU1NDZaMBIxEDAOBgNVBAMTB2Rp
ZWdvQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC7N7lGx7QGqkMd
wjqgkr09CPoV3HW+GL+YOPajf//CCo15t3mLu9Npv7O7ecb+g/6DxEOtHFpQBSbQ
igzHZkdlBJEGknwH2bsZ4wcVT2vcv2XPAIMDrnT7VuF1S2XD7BJK3n6BeXkFsVPA
OUjC/v0pM/rCFRId5CwtRD/0IHFC/qgEtFQx+zejXXEn1AJMzvNNJ3B0bd8VQGEX
ppemZXS1QvTP7/j2h7fJjosyoL6+76k4mcoScmWFNJHKcG4qcAh8rdnDlw+hJ+5S
z73CadYI2BTnlZ/fxEcsZ/kcteFSf0mFpMYX6vs9/us/rgGwjUNzg+JlzvF43TYY
VQ+TRkFUYHhDv3xwuRHnPNe0Nm30esKpqvbSXtoS6jcnpHn9tMOU0+4NW4aEdy9s
7l4lcGyih4qZfHbYTsRDk1Nrq5EzQbhlZSPC3nxMrLxXri7j22rVCY/Rj9IgAxwC
R3KcCdADGJeNOw44bK/BsRrB+Hxs9yNpXc2V2dez+w3hKNuzyk7WydC3fgXxX6x8
66xnlhFGor7fvM0OSMtGUBD16igh4ySdDiEMNUljqQ1DuMglT1eGdg+Kh+1YYWpz
v3JkNTX96C80IivbZyunZ2CczFhW2HlGWZLwNKeuM0hxt6AmiEa+KJQkx73dfg3L
tkDWWp9TXERPI/6Y2696INi0wElBUQIDAQABo2YwZDAOBgNVHQ8BAf8EBAMCAAYw
EgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU5xGtUKEzsfGmk/Siqo4fgAMs
TBwwHwYDVR0jBBgwFoAU5xGtUKEzsfGmk/Siqo4fgAMsTBwwCwYJKoZIhvcNAQEL
A4ICAQBkWgWl2t5fd4PZ1abpSQNAtsb2lfkkpxcKw+Osn9MeGpcrZjP8XoVTxtUs
GMpeVn2dUYY1sxkVgUZ0Epsgl7eZDK1jn6QfWIjltlHvDtJMh0OrxmdJUuHTGIHc
lsI9NGQRUtbyFHmy6jwIF7q925OmPQ/A6Xgkb45VUJDGNwOMUL5I9LbdBXcjmx6F
ZifEON3wxDBVMIAoS/mZYjP4zy2k1qE2FHoitwDccnCG5Wya+AHdZv/ZlfJcuMtU
U82oyHOctH29BPwASs3E1HUKof6uxJI+Y1M2kBDeuDS7DWiTt3JIVCjewIIhyYYw
uTPbQglqhqHr1RWohliDmKSroIil68s42An0fv9sUr0Btf4itKS1gTb4rNiKTZC/
8sLKs+CA5MB+F8lCllGGFfv1RFiUZBQs9+YEE+ru+yJw39lHeZQsEUgHbLjbVHs1
WFqiKTO8VKl1/eGwG0l9dI26qisIAa/I7kLjlqboKycGDmAAarsmcJBLPzS+ytiu
hoxA/fLhSWJvPXbdGemXLWQGf5DLN/8QGB63Rjp9WC3HhwSoU0NvmNmHoh+AdRRT
dYbCU/DMZjsv+Pt9flhj7ELLo+WKHyI767hJSq9A7IT3GzFt8iGiEAt1qj2yS0DX
36hwbfc1Gh/8nKgFeLmPOlBfKncjTjL2FvBNap6a8tVHXO9FvQ==
-----END CERTIFICATE-----`)
			})

			It("uses it for the cached downloader", func() {
				// not really an easy way to check this at this layer -- inigo
				// let's just check that our validation passes
				Consistently(errCh).ShouldNot(Receive(HaveOccurred()))
			})
		})

		Context("when invalid", func() {
			BeforeEach(func() {
				config.CACertsForDownloads = []byte("some-stuff")
			})

			It("fails", func() {
				Eventually(errCh).Should(Receive(HaveOccurred()))
			})
		})
	})
})
