package fake_cc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"

	"code.cloudfoundry.org/runtimeschema/cc_messages"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
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
	requestCount                 int
}

func New() *FakeCC {
	return &FakeCC{
		UploadedDroplets:             map[string][]byte{},
		UploadedBuildArtifactsCaches: map[string][]byte{},
		stagingGuids:                 []string{},
		stagingResponses:             []cc_messages.StagingResponseForCC{},
		stagingResponseStatusCode:    http.StatusOK,
		stagingResponseBody:          "{}",
	}
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

func (f *FakeCC) RequestCount() int {
	return f.requestCount
}

func (f *FakeCC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "[FAKE CC] Handling request: %s\n", r.URL.Path)

	endpoints := map[string]func(http.ResponseWriter, *http.Request){
		".*": f.handleDropletUploadRequest,
		"/staging/droplets/.*/upload": f.handleDropletUploadRequest,
	}

	f.requestCount = f.requestCount + 1

	for pattern, handler := range endpoints {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(r.URL.Path)
		if matches != nil {
			ginkgo.GinkgoWriter.Write([]byte("FOUND A MATCH"))
			handler(w, r)
			return
		}
	}

	ginkgo.Fail(fmt.Sprintf("[FAKE CC] No matching endpoint handler for %s", r.URL.Path))
}

func (f *FakeCC) handleDefaultRequest(w http.ResponseWriter, r *http.Request) {
	ginkgo.GinkgoWriter.Write([]byte("GOT A REQUEST FOR DEFAULT"))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (f *FakeCC) handleDropletUploadRequest(w http.ResponseWriter, r *http.Request) {
	ginkgo.GinkgoWriter.Write([]byte("GOT A REQUEST FOR DROPLET"))

	key := getFileUploadKey(r)
	file, _, err := r.FormFile(key)
	Expect(err).NotTo(HaveOccurred())

	uploadedBytes, err := ioutil.ReadAll(file)
	Expect(err).NotTo(HaveOccurred())

	re := regexp.MustCompile("/staging/droplets/(.*)/upload")
	fmt.Fprintf(ginkgo.GinkgoWriter, "Request URL: %s\n", r.URL.String())
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
