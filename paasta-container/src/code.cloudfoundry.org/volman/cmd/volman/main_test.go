package main_test

import (
	"fmt"
	"os"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/volman"
	"code.cloudfoundry.org/volman/volhttp"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Volman main", func() {
	var (
		args                   []string
		listenAddr, driverAddr string
		driversPath            string
		process                ifrit.Process
		client                 volman.Manager
		testLogger             lager.Logger
	)

	BeforeEach(func() {
		driversPath = fmt.Sprintf("/tmp/testdrivers/%d/", GinkgoParallelNode())
		err := os.MkdirAll(driversPath, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		listenAddr = fmt.Sprintf("0.0.0.0:%d", 8889+GinkgoParallelNode())
		driverAddr = fakeDriver.URL()
		client = volhttp.NewRemoteClient("http://" + listenAddr)
		Expect(err).NotTo(HaveOccurred())

		testLogger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		args = append(args, "--listenAddr", listenAddr)
		args = append(args, "--volmanDriverPaths", driversPath)

		volmanRunner := ginkgomon.New(ginkgomon.Config{
			Name:       "volman",
			Command:    exec.Command(binaryPath, args...),
			StartCheck: "started",
		})
		process = ginkgomon.Invoke(volmanRunner)
	})

	AfterEach(func() {
		ginkgomon.Kill(process)
		err := os.RemoveAll(driversPath)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should listen on the given address", func() {
		_, err := client.ListDrivers(testLogger)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a driverspath with a single spec file", func() {
		BeforeEach(func() {
			err := voldriver.WriteDriverSpec(testLogger, driversPath, "test-driver", "spec", []byte(driverAddr))
			Expect(err).NotTo(HaveOccurred())

			resp, err := json.Marshal(voldriver.ActivateResponse{
				Implements: []string{"VolumeDriver"},
			})
			Expect(err).NotTo(HaveOccurred())
			fakeDriver.RouteToHandler("POST", "/Plugin.Activate",
				ghttp.RespondWith(200, resp),
			)

			resp, err = json.Marshal(voldriver.ListResponse{
				Volumes: []voldriver.VolumeInfo{},
			})
			Expect(err).NotTo(HaveOccurred())

			fakeDriver.RouteToHandler("POST", "/VolumeDriver.List",
				ghttp.RespondWith(200, resp),
			)
		})

		It("should look in that location for driver specs", func() {
			response, err := client.ListDrivers(testLogger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(response.Drivers)).To(Equal(1))
		})
	})
})
