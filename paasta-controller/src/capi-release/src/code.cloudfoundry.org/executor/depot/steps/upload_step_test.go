package steps_test

import (
	"archive/tar"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"time"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/garden"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	Compressor "code.cloudfoundry.org/archiver/compressor"
	"code.cloudfoundry.org/executor/depot/log_streamer/fake_log_streamer"
	"code.cloudfoundry.org/executor/depot/steps"
	Uploader "code.cloudfoundry.org/executor/depot/uploader"
	"code.cloudfoundry.org/executor/depot/uploader/fake_uploader"
	"code.cloudfoundry.org/executor/fakes"
	"code.cloudfoundry.org/lager/lagertest"
)

type fakeUploader struct {
	ready   chan<- struct{}
	barrier <-chan struct{}
}

func (u *fakeUploader) Upload(fileLocation string, destinationUrl *url.URL, cancel <-chan struct{}) (int64, error) {
	u.ready <- struct{}{}
	<-u.barrier
	return 0, nil
}

func newFakeStreamer() *fake_log_streamer.FakeLogStreamer {
	fakeStreamer := new(fake_log_streamer.FakeLogStreamer)

	stdoutBuffer := gbytes.NewBuffer()
	stderrBuffer := gbytes.NewBuffer()
	fakeStreamer.StdoutReturns(stdoutBuffer)
	fakeStreamer.StderrReturns(stderrBuffer)

	return fakeStreamer
}

var _ = Describe("UploadStep", func() {
	var (
		step steps.Step

		uploadAction    *models.UploadAction
		uploader        Uploader.Uploader
		tempDir         string
		gardenClient    *fakes.FakeGardenClient
		logger          *lagertest.TestLogger
		compressor      Compressor.Compressor
		fakeStreamer    *fake_log_streamer.FakeLogStreamer
		uploadTarget    *httptest.Server
		uploadedPayload []byte
	)

	BeforeEach(func() {
		var err error

		uploadTarget = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var err error

			uploadedPayload, err = ioutil.ReadAll(req.Body)
			Expect(err).NotTo(HaveOccurred())

			w.WriteHeader(http.StatusOK)
		}))

		uploadAction = &models.UploadAction{
			To:   uploadTarget.URL,
			From: "./expected-src.txt",
			User: "notroot",
		}

		tempDir, err = ioutil.TempDir("", "upload-step-tmpdir")
		Expect(err).NotTo(HaveOccurred())

		gardenClient = fakes.NewGardenClient()

		logger = lagertest.NewTestLogger("test")

		compressor = Compressor.NewTgz()
		uploader = Uploader.New(logger, 5*time.Second, nil)

		fakeStreamer = newFakeStreamer()

		_, err = user.Current()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
		uploadTarget.Close()
	})

	handle := "some-container-handle"

	JustBeforeEach(func() {
		gardenClient.Connection.CreateReturns(handle, nil)

		container, err := gardenClient.Create(garden.ContainerSpec{})
		Expect(err).NotTo(HaveOccurred())

		step = steps.NewUpload(
			container,
			*uploadAction,
			uploader,
			compressor,
			tempDir,
			fakeStreamer,
			make(chan struct{}, 1),
			logger,
		)
	})

	Describe("Perform", func() {
		Context("when streaming out works", func() {
			var buffer *gbytes.Buffer

			BeforeEach(func() {
				buffer = gbytes.NewBuffer()

				gardenClient.Connection.StreamOutStub = func(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
					Expect(spec.Path).To(Equal("./expected-src.txt"))
					Expect(spec.User).To(Equal("notroot"))
					Expect(handle).To(Equal("some-container-handle"))

					tarWriter := tar.NewWriter(buffer)

					dropletContents := "expected-contents"

					err := tarWriter.WriteHeader(&tar.Header{
						Name: "./expected-src.txt",
						Size: int64(len(dropletContents)),
					})
					Expect(err).NotTo(HaveOccurred())

					_, err = tarWriter.Write([]byte(dropletContents))
					Expect(err).NotTo(HaveOccurred())

					err = tarWriter.Flush()
					Expect(err).NotTo(HaveOccurred())

					return buffer, nil
				}
			})

			It("uploads the specified file to the destination", func() {
				err := step.Perform()
				Expect(err).NotTo(HaveOccurred())

				Expect(uploadedPayload).NotTo(BeZero())

				Expect(buffer.Closed()).To(BeTrue())

				Expect(string(uploadedPayload)).To(Equal("expected-contents"))
			})

			It("logs the step", func() {
				err := step.Perform()
				Expect(err).NotTo(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.URLUploader.uploading",
					"test.URLUploader.succeeded-uploading",
					"test.upload-step.upload-successful",
				}))
			})

			Describe("Cancel", func() {
				cancelledErr := errors.New("upload cancelled")

				var fakeUploader *fake_uploader.FakeUploader

				BeforeEach(func() {
					fakeUploader = new(fake_uploader.FakeUploader)

					fakeUploader.UploadStub = func(from string, dest *url.URL, cancel <-chan struct{}) (int64, error) {
						<-cancel
						return 0, cancelledErr
					}

					uploader = fakeUploader
				})

				It("cancels any in-flight upload", func() {
					errs := make(chan error)

					go func() {
						errs <- step.Perform()
					}()

					Eventually(fakeUploader.UploadCallCount).Should(Equal(1))

					Consistently(errs).ShouldNot(Receive())

					step.Cancel()

					Eventually(errs).Should(Receive(Equal(steps.ErrCancelled)))
				})
			})

			Describe("streaming logs for uploads", func() {
				BeforeEach(func() {
					fakeUploader := new(fake_uploader.FakeUploader)
					fakeUploader.UploadReturns(1024, nil)
					uploader = fakeUploader
				})

				Context("when an artifact is specified", func() {
					BeforeEach(func() {
						uploadAction.Artifact = "artifact"
					})

					It("streams the upload filesize", func() {
						err := step.Perform()
						Expect(err).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(ContainSubstring("Uploaded artifact (1K)"))
					})
				})

				Context("when an artifact is not specified", func() {
					It("does not stream the upload information", func() {
						err := step.Perform()
						Expect(err).NotTo(HaveOccurred())

						stdout := fakeStreamer.Stdout().(*gbytes.Buffer)
						Expect(stdout.Contents()).To(BeEmpty())
					})
				})

				It("does not stream an error", func() {
					err := step.Perform()
					Expect(err).NotTo(HaveOccurred())

					stderr := fakeStreamer.Stderr().(*gbytes.Buffer)
					Expect(stderr.Contents()).To(BeEmpty())
				})
			})

			Context("when there is an error uploading", func() {
				errUploadFailed := errors.New("Upload failed!")

				BeforeEach(func() {
					fakeUploader := new(fake_uploader.FakeUploader)
					fakeUploader.UploadReturns(0, errUploadFailed)
					uploader = fakeUploader
				})

				It("returns the appropriate error", func() {
					err := step.Perform()
					Expect(err).To(MatchError(errUploadFailed))
				})

				It("logs the step", func() {
					err := step.Perform()
					Expect(err).To(HaveOccurred())
					Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
						"test.upload-step.upload-starting",
						"test.upload-step.failed-to-upload",
					}))

				})
			})
		})

		Context("when there is an error parsing the upload url", func() {
			BeforeEach(func() {
				uploadAction.To = "foo/bar"
			})

			It("returns the appropriate error", func() {
				err := step.Perform()
				Expect(err).To(BeAssignableToTypeOf(&url.Error{}))
			})

			It("logs the step", func() {
				err := step.Perform()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-parse-url",
				}))

			})
		})

		Context("when there is an error initiating the stream", func() {
			errStream := errors.New("stream error")

			BeforeEach(func() {
				gardenClient.Connection.StreamOutReturns(nil, errStream)
			})

			It("returns the appropriate error", func() {
				err := step.Perform()
				Expect(err).To(MatchError(steps.NewEmittableError(errStream, steps.ErrEstablishStream)))
			})

			It("logs the step", func() {
				err := step.Perform()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-stream-out",
				}))

			})
		})

		Context("when there is an error in reading the data from the stream", func() {
			errStream := errors.New("stream error")

			BeforeEach(func() {
				gardenClient.Connection.StreamOutReturns(&errorReader{err: errStream}, nil)
			})

			It("returns the appropriate error", func() {
				err := step.Perform()
				Expect(err).To(MatchError(steps.NewEmittableError(errStream, steps.ErrReadTar)))
			})

			It("logs the step", func() {
				err := step.Perform()
				Expect(err).To(HaveOccurred())
				Expect(logger.TestSink.LogMessages()).To(ConsistOf([]string{
					"test.upload-step.upload-starting",
					"test.upload-step.failed-to-read-stream",
				}))

			})
		})
	})

	Describe("the uploads are rate limited", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gardenClient.Create(garden.ContainerSpec{
				Handle: handle,
			})
			Expect(err).NotTo(HaveOccurred())

			gardenClient.Connection.StreamOutStub = func(handle string, spec garden.StreamOutSpec) (io.ReadCloser, error) {
				buffer := gbytes.NewBuffer()
				tarWriter := tar.NewWriter(buffer)

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "./does-not-matter.txt",
					Size: int64(0),
				})
				Expect(err).NotTo(HaveOccurred())

				return buffer, nil
			}
		})

		It("allows only N concurrent uploads", func() {
			rateLimiter := make(chan struct{}, 2)

			ready := make(chan struct{}, 3)
			barrier := make(chan struct{})
			uploader := &fakeUploader{
				ready:   ready,
				barrier: barrier,
			}

			uploadAction1 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo1.txt",
			}

			step1 := steps.NewUpload(
				container,
				uploadAction1,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			uploadAction2 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo2.txt",
			}

			step2 := steps.NewUpload(
				container,
				uploadAction2,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			uploadAction3 := models.UploadAction{
				To:   "http://mybucket.mf",
				From: "./foo3.txt",
			}

			step3 := steps.NewUpload(
				container,
				uploadAction3,
				uploader,
				compressor,
				tempDir,
				newFakeStreamer(),
				rateLimiter,
				logger,
			)

			go func() {
				defer GinkgoRecover()

				err := step1.Perform()
				Expect(err).NotTo(HaveOccurred())
			}()
			go func() {
				defer GinkgoRecover()

				err := step2.Perform()
				Expect(err).NotTo(HaveOccurred())
			}()
			go func() {
				defer GinkgoRecover()

				err := step3.Perform()
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(ready).Should(Receive())
			Eventually(ready).Should(Receive())
			Consistently(ready).ShouldNot(Receive())

			barrier <- struct{}{}

			Eventually(ready).Should(Receive())

			close(barrier)
		})
	})
})

type errorReader struct {
	err error
}

func (r *errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (r *errorReader) Close() error {
	return nil
}
