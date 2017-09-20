package main_test

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"code.cloudfoundry.org/cc-uploader"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"code.cloudfoundry.org/urljoiner"
	"github.com/hashicorp/consul/api"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type ByteEmitter struct {
	written int
	length  int
}

func NewEmitter(length int) *ByteEmitter {
	return &ByteEmitter{
		length:  length,
		written: 0,
	}
}

func (emitter *ByteEmitter) Read(p []byte) (n int, err error) {
	if emitter.written >= emitter.length {
		return 0, io.EOF
	}
	time.Sleep(time.Millisecond)
	p[0] = 0xF1
	emitter.written++
	return 1, nil
}

var _ = Describe("CC Uploader", func() {
	var (
		port    int
		address string
		session *gexec.Session
		err     error
		appGuid = "app-guid"
	)

	dropletUploadRequest := func(appGuid string, body io.Reader, contentLength int) *http.Request {
		ccUrl, err := url.Parse(fakeCC.Address())
		Expect(err).NotTo(HaveOccurred())
		ccUrl.User = url.UserPassword(fakeCC.Username(), fakeCC.Password())
		ccUrl.Path = urljoiner.Join("staging", "droplets", appGuid, "upload")
		v := url.Values{"async": []string{"true"}}
		ccUrl.RawQuery = v.Encode()

		route, ok := ccuploader.Routes.FindRouteByName(ccuploader.UploadDropletRoute)
		Expect(ok).To(BeTrue())

		path, err := route.CreatePath(map[string]string{"guid": appGuid})
		Expect(err).NotTo(HaveOccurred())

		u, err := url.Parse(urljoiner.Join(address, path))
		Expect(err).NotTo(HaveOccurred())
		v = url.Values{cc_messages.CcDropletUploadUriKey: []string{ccUrl.String()}}
		u.RawQuery = v.Encode()

		postRequest, err := http.NewRequest("POST", u.String(), body)
		Expect(err).NotTo(HaveOccurred())
		postRequest.ContentLength = int64(contentLength)
		postRequest.Header.Set("Content-Type", "application/octet-stream")

		return postRequest
	}

	BeforeEach(func() {
		Expect(err).NotTo(HaveOccurred())
		port = 8182 + config.GinkgoConfig.ParallelNode
		address = fmt.Sprintf("http://localhost:%d", port)

		args := []string{
			"-consulCluster", consulRunner.URL(),
			"-address", fmt.Sprintf("localhost:%d", port),
			"-skipCertVerify",
		}

		session, err = gexec.Start(exec.Command(ccUploaderBinary, args...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session).Should(gbytes.Say("cc-uploader.ready"))
	})

	AfterEach(func() {
		session.Kill().Wait()
	})

	Describe("uploading a file", func() {
		var contentLength = 100

		It("should upload the file...", func() {
			emitter := NewEmitter(contentLength)
			postRequest := dropletUploadRequest(appGuid, emitter, contentLength)
			resp, err := http.DefaultClient.Do(postRequest)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusCreated))
			Expect(len(fakeCC.UploadedDroplets[appGuid])).To(Equal(contentLength))
		})
	})

	Describe("Initialization", func() {
		It("registers itself with consul", func() {
			services, err := consulRunner.NewClient().Agent().Services()
			Expect(err).NotTo(HaveOccurred())
			Expect(services).Should(HaveKeyWithValue("cc-uploader",
				&api.AgentService{
					Service: "cc-uploader",
					ID:      "cc-uploader",
					Port:    port,
				}))
		})

		It("registers a TTL healthcheck", func() {
			checks, err := consulRunner.NewClient().Agent().Checks()
			Expect(err).NotTo(HaveOccurred())
			Expect(checks).Should(HaveKeyWithValue("service:cc-uploader",
				&api.AgentCheck{
					Node:        "0",
					CheckID:     "service:cc-uploader",
					Name:        "Service 'cc-uploader' check",
					Status:      "passing",
					ServiceID:   "cc-uploader",
					ServiceName: "cc-uploader",
				}))
		})
	})
})
