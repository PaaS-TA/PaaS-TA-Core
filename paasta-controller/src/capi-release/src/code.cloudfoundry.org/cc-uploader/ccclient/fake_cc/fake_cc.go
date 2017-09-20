package fake_cc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sync"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/tedsuo/ifrit/http_server"
)

const (
	CC_USERNAME          = "bob"
	CC_PASSWORD          = "password"
	finishedResponseBody = `
        {
            "metadata":{
                "guid": "inigo-job-guid",
                "url": "/v2/jobs/inigo-job-guid"
            },
            "entity": {
                "status": "finished"
            }
        }
    `
)

type FakeCC struct {
	address string

	UploadedDroplets             map[string][]byte
	UploadedBuildArtifactsCaches map[string][]byte
	stagingGuids                 []string
	stagingResponses             []cc_messages.StagingResponseForCC
	stagingResponseStatusCode    int
	stagingResponseBody          string
	lock                         *sync.RWMutex
}

func New(address string) *FakeCC {
	return &FakeCC{
		address: address,

		UploadedDroplets:             map[string][]byte{},
		UploadedBuildArtifactsCaches: map[string][]byte{},
		stagingGuids:                 []string{},
		stagingResponses:             []cc_messages.StagingResponseForCC{},
		stagingResponseStatusCode:    http.StatusOK,
		stagingResponseBody:          "{}",
		lock:                         new(sync.RWMutex),
	}
}

func (f *FakeCC) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	err := http_server.New(f.address, f).Run(signals, ready)

	f.Reset()

	return err
}

func (f *FakeCC) Address() string {
	return "http://" + f.address
}

func (f *FakeCC) Username() string {
	return CC_USERNAME
}

func (f *FakeCC) Password() string {
	return CC_PASSWORD
}

func (f *FakeCC) Reset() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.UploadedDroplets = map[string][]byte{}
	f.UploadedBuildArtifactsCaches = map[string][]byte{}
	f.stagingGuids = []string{}
	f.stagingResponses = []cc_messages.StagingResponseForCC{}
	f.stagingResponseStatusCode = http.StatusOK
	f.stagingResponseBody = "{}"
}

func (f *FakeCC) SetStagingResponseStatusCode(statusCode int) {
	f.stagingResponseStatusCode = statusCode
}

func (f *FakeCC) SetStagingResponseBody(body string) {
	f.stagingResponseBody = body
}

func (f *FakeCC) StagingGuids() []string {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.stagingGuids
}

func (f *FakeCC) StagingResponses() []cc_messages.StagingResponseForCC {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.stagingResponses
}

func (f *FakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "[FAKE CC] Handling request: %s\n", r.URL.Path)

	endpoints := map[string]func(http.ResponseWriter, *http.Request){
		"/staging/droplets/.*/upload": f.handleDropletUploadRequest,
	}

	for pattern, handler := range endpoints {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(r.URL.Path)
		if matches != nil {
			handler(w, r)
			return
		}
	}

	ginkgo.Fail(fmt.Sprintf("[FAKE CC] No matching endpoint handler for %s", r.URL.Path))
}

func (f *FakeCC) handleDropletUploadRequest(w http.ResponseWriter, r *http.Request) {
	basicAuthVerifier := ghttp.VerifyBasicAuth(CC_USERNAME, CC_PASSWORD)
	basicAuthVerifier(w, r)

	key := getFileUploadKey(r)
	file, _, err := r.FormFile(key)
	Expect(err).NotTo(HaveOccurred())

	uploadedBytes, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())

	re := regexp.MustCompile("/staging/droplets/(.*)/upload")
	appGuid := re.FindStringSubmatch(r.URL.Path)[1]

	f.UploadedDroplets[appGuid] = uploadedBytes
	fmt.Fprintf(ginkgo.GinkgoWriter, "[FAKE CC] Received %d bytes for droplet for app-guid %s\n", len(uploadedBytes), appGuid)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(finishedResponseBody))
}

func getFileUploadKey(r *http.Request) string {
	err := r.ParseMultipartForm(1024)
	Expect(err).NotTo(HaveOccurred())

	Expect(r.MultipartForm.File).To(HaveLen(1))
	var key string
	for k, _ := range r.MultipartForm.File {
		key = k
	}
	Expect(key).NotTo(BeEmpty())
	return key
}
