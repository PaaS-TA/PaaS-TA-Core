package testhelpers_test

import (
	"github.com/apoydence/eachers"
	"github.com/apoydence/eachers/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("AlwaysReturn", func() {
	var (
		args []interface{}
	)

	Context("given a struct with channels", func() {
		type sampleStruct struct {
			A chan int32
			B chan string
		}

		type invalidStruct struct {
			A chan int
			B string
		}

		var (
			receiver sampleStruct
			invalid  invalidStruct
		)

		BeforeEach(func() {
			receiver = sampleStruct{
				A: make(chan int32, 100),
				B: make(chan string, 100),
			}

			invalid = invalidStruct{
				A: make(chan int, 100),
			}
		})

		Context("proper args", func() {
			BeforeEach(func() {
				args = eachers.With(99, "some-arg")
			})

			It("keeps all the channels populated with the given arguments", func() {
				testhelpers.AlwaysReturn(receiver, args...)

				Eventually(receiver.A).Should(HaveLen(cap(receiver.A)))
				Eventually(receiver.B).Should(HaveLen(cap(receiver.B)))
			})

			It("sends the expected argument", func() {
				testhelpers.AlwaysReturn(receiver, args...)

				Eventually(receiver.A).Should(Receive(BeEquivalentTo(args[0])))
				Eventually(receiver.B).Should(Receive(Equal(args[1])))
			})
		})

		DescribeTable("invalid args", func(rx interface{}, args ...interface{}) {
			f := func() {
				testhelpers.AlwaysReturn(rx, args...)
			}
			Expect(f).To(Panic())
		},
			Entry("not enough arguments", receiver, 99),
			Entry("wrong argument type", receiver, 99, invalid),
			Entry("receiver doesn't have channel type", invalid, 0, ""),
		)
	})

	Context("given a channel", func() {
		var (
			channel chan int
		)

		BeforeEach(func() {
			channel = make(chan int, 100)
		})

		Context("with arguments", func() {

			BeforeEach(func() {
				args = eachers.With(99)
			})

			It("keeps the channel populated with the given argument", func() {
				testhelpers.AlwaysReturn(channel, args...)

				Eventually(channel).Should(HaveLen(cap(channel)))
			})

			It("sends the expected argument", func() {
				testhelpers.AlwaysReturn(channel, args...)

				Eventually(channel).Should(Receive(Equal(args[0])))
			})

			Context("channel with a type that has bit width", func() {
				var (
					bitWidthChannel chan int32
				)

				BeforeEach(func() {
					bitWidthChannel = make(chan int32, 100)
				})

				It("accepts a convertable type", func() {
					f := func() {
						testhelpers.AlwaysReturn(bitWidthChannel, args...)
					}

					f()
					Expect(f).ToNot(Panic())
				})

				It("sends the expected argument", func() {
					testhelpers.AlwaysReturn(channel, args...)

					Eventually(channel).Should(Receive(Equal(args[0])))
				})
			})
		})

		Context("invalid arguments", func() {
			It("panics for no arguments", func() {
				f := func() {
					testhelpers.AlwaysReturn(channel)
				}
				Expect(f).To(Panic())
			})

			It("panics for more than one argument", func() {
				f := func() {
					testhelpers.AlwaysReturn(channel, 1, 2)
				}
				Expect(f).To(Panic())
			})

			It("panics if the argument type and channel type don't match", func() {
				f := func() {
					testhelpers.AlwaysReturn(channel, "invalid")
				}
				Expect(f).To(Panic())
			})
		})
	})

	Context("given something other than a channel or struct full of channels", func() {
		BeforeEach(func() {
			args = eachers.With(1, 2, 3)
		})

		It("panics for non channel or struct", func() {
			f := func() {
				testhelpers.AlwaysReturn(8, args)
			}

			Expect(f).To(Panic())
		})

		It("panics for read only channel", func() {
			var invalid <-chan int
			f := func() {
				testhelpers.AlwaysReturn(invalid, args)
			}

			Expect(f).To(Panic())
		})
	})
})
