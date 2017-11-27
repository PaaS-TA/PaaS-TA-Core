package handlers_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/cloudfoundry-incubator/etcd-release/src/acceptance-tests/testing/iptables_agent/handlers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type fakeIPTables struct {
	runCalls []runCall
	returns  struct {
		output string
		err    error
	}
}

type runCall struct {
	Receives struct {
		Args []string
	}
}

func (fakeIPTables *fakeIPTables) Run(args []string) (string, error) {
	runCall := runCall{}
	runCall.Receives.Args = args
	fakeIPTables.runCalls = append(fakeIPTables.runCalls, runCall)
	return fakeIPTables.returns.output, fakeIPTables.returns.err
}

var _ = Describe("DropHandler", func() {

	var (
		dropHandler handlers.DropHandler
		ipTables    *fakeIPTables
	)

	BeforeEach(func() {
		ipTables = &fakeIPTables{}
		buffer := bytes.NewBuffer([]byte{})
		dropHandler = handlers.NewDropHandler(ipTables, buffer)
	})

	DescribeTable("required params", func(requestParams, expectedErrMsg string) {
		url := fmt.Sprintf("/drop?%s", requestParams)
		request, err := http.NewRequest("PUT", url, strings.NewReader(""))
		Expect(err).NotTo(HaveOccurred())

		recorder := httptest.NewRecorder()

		dropHandler.ServeHTTP(recorder, request)

		Expect(recorder.Code).To(Equal(http.StatusBadRequest))

		respContents, err := ioutil.ReadAll(recorder.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(respContents)).To(Equal(expectedErrMsg))
	},
		Entry("missing addr", "", "must provide addr param"),
	)

	Context("when adding a drop rule with PUT request", func() {
		It("calls iptables with the correct arguments", func() {
			request, err := http.NewRequest("PUT", "/drop?addr=some-ip-addr", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			dropHandler.ServeHTTP(recorder, request)

			Expect(ipTables.runCalls).To(HaveLen(2))
			Expect(ipTables.runCalls[0].Receives.Args).To(Equal([]string{
				"-A", "INPUT",
				"-s", "some-ip-addr",
				"-j", "DROP",
			}))
			Expect(ipTables.runCalls[1].Receives.Args).To(Equal([]string{
				"-A", "OUTPUT",
				"-d", "some-ip-addr",
				"-j", "DROP",
			}))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})
	})

	Context("when removing a drop rule with DELETE request", func() {
		It("calls iptables with the correct arguments", func() {
			request, err := http.NewRequest("DELETE", "/drop?addr=some-ip-addr", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			dropHandler.ServeHTTP(recorder, request)
			Expect(ipTables.runCalls[0].Receives.Args).To(Equal([]string{
				"-D", "INPUT",
				"-s", "some-ip-addr",
				"-j", "DROP",
			}))
			Expect(ipTables.runCalls[1].Receives.Args).To(Equal([]string{
				"-D", "OUTPUT",
				"-d", "some-ip-addr",
				"-j", "DROP",
			}))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})
	})

	Context("failure cases", func() {
		It("returns a bad request when the request is not a PUT or a DELETE", func() {
			request, err := http.NewRequest("GET", "/drop", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			dropHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("returns an error when iptables command fails", func() {
			failingIPTables := &fakeIPTables{}
			failingIPTables.returns.err = errors.New("failed iptables")
			buffer := bytes.NewBuffer([]byte{})
			dropHandler = handlers.NewDropHandler(failingIPTables, buffer)

			request, err := http.NewRequest("PUT", "/drop?addr=some-ip-addr", strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()

			dropHandler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusInternalServerError))

			respContents, err := ioutil.ReadAll(recorder.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respContents)).To(ContainSubstring("failed iptables"))
		})
	})
})
