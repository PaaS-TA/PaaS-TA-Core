package log_streamer_test

import (
	"fmt"
	"strings"
	"sync"

	"code.cloudfoundry.org/executor/depot/log_streamer"
	mfakes "code.cloudfoundry.org/loggregator_v2/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/sonde-go/events"
)

var _ = Describe("LogStreamer", func() {
	var (
		streamer   log_streamer.LogStreamer
		fakeClient *mfakes.FakeLogSender
	)

	guid := "the-guid"
	sourceName := "the-source-name"
	index := 11

	BeforeEach(func() {
		fakeClient = mfakes.NewFakeLogSender()
		streamer = log_streamer.New(guid, sourceName, index, fakeClient)
	})

	Context("when told to emit", func() {
		Context("when given a message that corresponds to one line", func() {
			BeforeEach(func() {
				fmt.Fprintln(streamer.Stdout(), "this is a log")
				fmt.Fprintln(streamer.Stdout(), "this is another log")
			})

			It("should emit that message", func() {
				logs := fakeClient.Logs()

				Expect(logs).To(HaveLen(2))

				emission := logs[0]
				Expect(emission.AppId).To(Equal(guid))
				Expect(emission.SourceType).To(Equal(sourceName))
				Expect(string(emission.Message)).To(Equal("this is a log"))
				Expect(emission.MessageType).To(Equal(mfakes.OUT))
				Expect(emission.SourceInstance).To(Equal("11"))

				emission = logs[1]
				Expect(emission.AppId).To(Equal(guid))
				Expect(emission.SourceType).To(Equal(sourceName))
				Expect(emission.SourceInstance).To(Equal("11"))
				Expect(string(emission.Message)).To(Equal("this is another log"))
				Expect(emission.MessageType).To(Equal(mfakes.OUT))
			})
		})

		Describe("WithSource", func() {
			Context("when a new log source is provided", func() {
				It("should emit a message with the new log source", func() {
					newSourceName := "new-source-name"
					streamer = streamer.WithSource(newSourceName)
					fmt.Fprintln(streamer.Stdout(), "this is a log")

					logs := fakeClient.Logs()
					Expect(logs).To(HaveLen(1))

					emission := logs[0]
					Expect(emission.SourceType).To(Equal(newSourceName))
				})
			})

			Context("when no log source is provided", func() {
				It("should emit a message with the existing log source", func() {
					streamer = streamer.WithSource("")
					fmt.Fprintln(streamer.Stdout(), "this is a log")

					logs := fakeClient.Logs()
					Expect(logs).To(HaveLen(1))

					emission := logs[0]
					Expect(emission.SourceType).To(Equal(sourceName))
				})
			})
		})

		Context("when given a message with all sorts of fun newline characters", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "A\nB\rC\n\rD\r\nE\n\n\nF\r\r\rG\n\r\r\n\n\n\r")
			})

			It("should do the right thing", func() {
				logs := fakeClient.Logs()
				Expect(logs).To(HaveLen(7))
				for i, expectedString := range []string{"A", "B", "C", "D", "E", "F", "G"} {
					Expect(string(logs[i].Message)).To(Equal(expectedString))
				}
			})
		})

		Context("when given a series of short messages", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "this is a log")
				fmt.Fprintf(streamer.Stdout(), " it is made of wood")
				fmt.Fprintf(streamer.Stdout(), " - and it is longer")
				fmt.Fprintf(streamer.Stdout(), "than it seems\n")
			})

			It("concatenates them, until a new-line is received, and then emits that", func() {
				logs := fakeClient.Logs()
				Expect(logs).To(HaveLen(1))
				emission := fakeClient.Logs()[0]
				Expect(string(emission.Message)).To(Equal("this is a log it is made of wood - and it is longerthan it seems"))
			})
		})

		Context("when given a message with multiple new lines", func() {
			BeforeEach(func() {
				fmt.Fprintf(streamer.Stdout(), "this is a log\nand this is another\nand this one isn't done yet...")
			})

			It("should break the message up into multiple loggings", func() {
				Expect(fakeClient.Logs()).To(HaveLen(2))

				emission := fakeClient.Logs()[0]
				Expect(string(emission.Message)).To(Equal("this is a log"))

				emission = fakeClient.Logs()[1]
				Expect(string(emission.Message)).To(Equal("and this is another"))
			})
		})

		Describe("message limits", func() {
			var message string
			Context("when the message is just at the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)
					Expect([]byte(message)).To(HaveLen(log_streamer.MAX_MESSAGE_SIZE), "Ensure that the byte representation of our message is under the limit")

					fmt.Fprintf(streamer.Stdout(), message+"\n")
				})

				It("should break the message up and send multiple messages", func() {
					Expect(fakeClient.Logs()).To(HaveLen(1))
					emission := fakeClient.Logs()[0]
					Expect(string(emission.Message)).To(Equal(message))
				})
			})

			Context("when the message exceeds the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)
					message += strings.Repeat("8", log_streamer.MAX_MESSAGE_SIZE)
					message += strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE)
					message += "hello\n"
					fmt.Fprintf(streamer.Stdout(), message)
				})

				It("should break the message up and send multiple messages", func() {
					Expect(fakeClient.Logs()).To(HaveLen(4))
					Expect(string(fakeClient.Logs()[0].Message)).To(Equal(strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)))
					Expect(string(fakeClient.Logs()[1].Message)).To(Equal(strings.Repeat("8", log_streamer.MAX_MESSAGE_SIZE)))
					Expect(string(fakeClient.Logs()[2].Message)).To(Equal(strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE)))
					Expect(string(fakeClient.Logs()[3].Message)).To(Equal("hello"))
				})
			})

			Context("when having to deal with byte boundaries and long utf characters", func() {
				BeforeEach(func() {
					message = strings.Repeat("a", log_streamer.MAX_MESSAGE_SIZE-3)
					message += "\U0001F428\n"
				})

				It("should break the message up and send multiple messages without sending error runes", func() {
					fmt.Fprintf(streamer.Stdout(), message)

					Expect(fakeClient.Logs()).To(HaveLen(2))
					Expect(string(fakeClient.Logs()[0].Message)).To(Equal(strings.Repeat("a", log_streamer.MAX_MESSAGE_SIZE-3)))
					Expect(string(fakeClient.Logs()[1].Message)).To(Equal("\U0001F428"))
				})

				Context("with an invalid utf8 character in the message", func() {
					var utfChar string

					BeforeEach(func() {
						message = strings.Repeat("9", log_streamer.MAX_MESSAGE_SIZE-4)
						utfChar = "\U0001F428"
					})

					It("emits both messages correctly", func() {
						fmt.Fprintf(streamer.Stdout(), message+utfChar[0:2])
						fmt.Fprintf(streamer.Stdout(), utfChar+"\n")

						Expect(fakeClient.Logs()).To(HaveLen(2))
						emission := fakeClient.Logs()[0]
						Expect(string(emission.Message)).To(Equal(message + utfChar[0:2]))

						emission = fakeClient.Logs()[1]
						Expect(string(emission.Message)).To(Equal(utfChar))
					})
				})

				Context("when the entire message is invalid utf8 characters", func() {
					var utfChar string

					BeforeEach(func() {
						utfChar = "\U0001F428"
						message = strings.Repeat(utfChar[0:2], log_streamer.MAX_MESSAGE_SIZE/2)
						Expect(len(message)).To(Equal(log_streamer.MAX_MESSAGE_SIZE))
					})

					It("drops the last 3 bytes", func() {
						fmt.Fprintf(streamer.Stdout(), message)

						Expect(fakeClient.Logs()).To(HaveLen(1))
						emission := fakeClient.Logs()[0]
						Expect(string(emission.Message)).To(Equal(message[0 : len(message)-3]))
					})
				})
			})

			Context("while concatenating, if the message exceeds the emittable length", func() {
				BeforeEach(func() {
					message = strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE-2)
					fmt.Fprintf(streamer.Stdout(), message)
					fmt.Fprintf(streamer.Stdout(), "778888\n")
				})

				It("should break the message up and send multiple messages", func() {
					Expect(fakeClient.Logs()).To(HaveLen(2))
					Expect(string(fakeClient.Logs()[0].Message)).To(Equal(strings.Repeat("7", log_streamer.MAX_MESSAGE_SIZE)))
					Expect(string(fakeClient.Logs()[1].Message)).To(Equal("8888"))
				})
			})
		})
	})

	Context("when told to emit stderr", func() {
		It("should handle short messages", func() {
			fmt.Fprintf(streamer.Stderr(), "this is a log\nand this is another\nand this one isn't done yet...")
			Expect(fakeClient.Logs()).To(HaveLen(2))

			emission := fakeClient.Logs()[0]
			Expect(string(emission.Message)).To(Equal("this is a log"))
			Expect(emission.SourceType).To(Equal(sourceName))
			Expect(emission.MessageType).To(Equal(mfakes.ERR))

			emission = fakeClient.Logs()[1]
			Expect(string(emission.Message)).To(Equal("and this is another"))
		})

		It("should handle long messages", func() {
			fmt.Fprintf(streamer.Stderr(), strings.Repeat("e", log_streamer.MAX_MESSAGE_SIZE+1)+"\n")
			Expect(fakeClient.Logs()).To(HaveLen(2))

			emission := fakeClient.Logs()[0]
			Expect(string(emission.Message)).To(Equal(strings.Repeat("e", log_streamer.MAX_MESSAGE_SIZE)))

			emission = fakeClient.Logs()[1]
			Expect(string(emission.Message)).To(Equal("e"))
		})
	})

	Context("when told to flush", func() {
		It("should send whatever log is left in its buffer", func() {
			fmt.Fprintf(streamer.Stdout(), "this is a stdout")
			fmt.Fprintf(streamer.Stderr(), "this is a stderr")

			Expect(fakeClient.Logs()).To(HaveLen(0))

			streamer.Flush()

			Expect(fakeClient.Logs()).To(HaveLen(2))
			Expect(fakeClient.Logs()[0].MessageType).To(Equal(mfakes.OUT))
			Expect(fakeClient.Logs()[1].MessageType).To(Equal(mfakes.ERR))
		})
	})

	Context("when there is no app guid", func() {
		It("does nothing when told to emit or flush", func() {
			streamer = log_streamer.New("", sourceName, index, fakeClient)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Stderr().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.Logs()).To(BeEmpty())
		})
	})

	Context("when there is no log source", func() {
		It("defaults to LOG", func() {
			streamer = log_streamer.New(guid, "", -1, fakeClient)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.Logs()[0].SourceType).To(Equal(log_streamer.DefaultLogSource))

		})
	})

	Context("when there is no source index", func() {
		It("defaults to 0", func() {
			streamer = log_streamer.New(guid, sourceName, -1, fakeClient)

			streamer.Stdout().Write([]byte("hi"))
			streamer.Flush()

			Expect(fakeClient.Logs()[0].SourceInstance).To(Equal("-1"))
		})
	})

	Context("with multiple goroutines emitting simultaneously", func() {
		var waitGroup *sync.WaitGroup

		BeforeEach(func() {
			waitGroup = new(sync.WaitGroup)

			for i := 0; i < 2; i++ {
				waitGroup.Add(1)
				go func() {
					defer waitGroup.Done()
					fmt.Fprintln(streamer.Stdout(), "this is a log")
				}()
			}
		})

		AfterEach(func(done Done) {
			defer close(done)
			waitGroup.Wait()
		})

		It("does not trigger data races", func() {
			Eventually(fakeClient.Logs).Should(HaveLen(2))
		})
	})
})

type FakeLoggregatorEmitter struct {
	emissions []*events.LogMessage
	sync.Mutex
}

func NewFakeLoggregatorEmmitter() *FakeLoggregatorEmitter {
	return &FakeLoggregatorEmitter{}
}

func (e *FakeLoggregatorEmitter) Emit(appid, message string) {
	panic("no no no no")
}

func (e *FakeLoggregatorEmitter) EmitError(appid, message string) {
	panic("no no no no")
}

func (e *FakeLoggregatorEmitter) EmitLogMessage(msg *events.LogMessage) {
	e.Lock()
	defer e.Unlock()
	e.emissions = append(e.emissions, msg)
}

func (e *FakeLoggregatorEmitter) Emissions() []*events.LogMessage {
	e.Lock()
	defer e.Unlock()
	emissionsCopy := make([]*events.LogMessage, len(e.emissions))
	copy(emissionsCopy, e.emissions)
	return emissionsCopy
}
