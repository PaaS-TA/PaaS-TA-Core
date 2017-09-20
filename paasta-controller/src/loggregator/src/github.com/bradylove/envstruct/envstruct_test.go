package envstruct_test

import (
	"errors"
	"os"

	"github.com/bradylove/envstruct"

	"fmt"
	"time"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("envstruct", func() {
	Describe("Load()", func() {
		var (
			ts        LargeTestStruct
			loadError error
			envVars   map[string]string
		)

		BeforeEach(func() {
			ts = LargeTestStruct{}
			ts.UnmarshallerPointer = newMockUnmarshaller()
			ts.UnmarshallerPointer.UnmarshalEnvOutput.Ret0 <- nil
			um := newMockUnmarshaller()
			ts.UnmarshallerValue = *um
			ts.UnmarshallerValue.UnmarshalEnvOutput.Ret0 <- nil

			envVars = make(map[string]string)
			for k, v := range baseEnvVars {
				envVars[k] = v
			}
		})

		JustBeforeEach(func() {
			for k, v := range envVars {
				os.Setenv(k, v)
			}
		})

		Context("when load is successful", func() {
			JustBeforeEach(func() {
				loadError = envstruct.Load(&ts)
			})

			AfterEach(func() {
				for k := range envVars {
					os.Setenv(k, "")
				}
			})

			It("does not return an error", func() {
				Expect(loadError).ToNot(HaveOccurred())
			})

			Context("with unmarshallers", func() {
				It("passes the value to the pointer field", func() {
					Expect(ts.UnmarshallerPointer.UnmarshalEnvInput).To(BeCalled(
						With("pointer"),
					))
				})

				It("passes the value to the value field's address", func() {
					Expect(ts.UnmarshallerValue.UnmarshalEnvInput).To(BeCalled(
						With("value"),
					))
				})
			})

			Context("with strings", func() {
				It("populates the string thing", func() {
					Expect(ts.StringThing).To(Equal("stringy thingy"))
				})
			})

			Context("with bools", func() {
				Context("with 'true'", func() {
					It("is true", func() {
						Expect(ts.BoolThing).To(BeTrue())
					})
				})

				Context("with 'false'", func() {
					BeforeEach(func() {
						envVars["BOOL_THING"] = "false"
					})

					It("is true", func() {
						Expect(ts.BoolThing).To(BeFalse())
					})
				})

				Context("with '1'", func() {
					BeforeEach(func() {
						envVars["BOOL_THING"] = "1"
					})

					It("is true", func() {
						Expect(ts.BoolThing).To(BeTrue())
					})
				})

				Context("with '0'", func() {
					BeforeEach(func() {
						envVars["BOOL_THING"] = "0"
					})

					It("is false", func() {
						Expect(ts.BoolThing).To(BeFalse())
					})
				})
			})

			Context("with ints", func() {
				It("populates the int thing", func() {
					Expect(ts.IntThing).To(Equal(100))
				})

				It("populates the int 8 thing", func() {
					Expect(ts.Int8Thing).To(Equal(int8(20)))
				})

				It("populates the int 16 thing", func() {
					Expect(ts.Int16Thing).To(Equal(int16(2000)))
				})

				It("populates the int 32 thing", func() {
					Expect(ts.Int32Thing).To(Equal(int32(200000)))
				})

				It("populates the int 64 thing", func() {
					Expect(ts.Int64Thing).To(Equal(int64(200000000)))
				})
			})

			Context("with uints", func() {
				It("populates the uint thing", func() {
					Expect(ts.UintThing).To(Equal(uint(100)))
				})

				It("populates the uint 8 thing", func() {
					Expect(ts.Uint8Thing).To(Equal(uint8(20)))
				})

				It("populates the uint 16 thing", func() {
					Expect(ts.Uint16Thing).To(Equal(uint16(2000)))
				})

				It("populates the uint 32 thing", func() {
					Expect(ts.Uint32Thing).To(Equal(uint32(200000)))
				})

				It("populates the uint 64 thing", func() {
					Expect(ts.Uint64Thing).To(Equal(uint64(200000000)))
				})
			})

			Context("with comma separated strings", func() {
				Context("slice of strings", func() {
					It("populates a slice of strings", func() {
						Expect(ts.StringSliceThing).To(Equal([]string{"one", "two", "three"}))
					})

					Context("with leading and trailing spaces", func() {
						BeforeEach(func() {
							envVars["STRING_SLICE_THING"] = "one , two , three"
						})

						It("populates a slice of strings", func() {
							Expect(ts.StringSliceThing).To(Equal([]string{"one", "two", "three"}))
						})
					})
				})

				Context("slice of ints", func() {
					It("populates a slice of ints", func() {
						Expect(ts.IntSliceThing).To(Equal([]int{1, 2, 3}))
					})
				})
			})

			Context("with structs", func() {
				It("parses the duration string", func() {
					Expect(ts.DurationThing).To(Equal(2 * time.Second))
				})

				It("parses the url string", func() {
					Expect(ts.URLThing.Scheme).To(Equal("http"))
					Expect(ts.URLThing.Host).To(Equal("github.com"))
					Expect(ts.URLThing.Path).To(Equal("/some/path"))
				})
			})
		})

		Context("with defaults", func() {
			It("honors default values if env var is empty", func() {
				ts.DefaultThing = "Default Value"

				Expect(envstruct.Load(&ts)).To(Succeed())
				Expect(ts.DefaultThing).To(Equal("Default Value"))
			})
		})

		Context("when load is unsuccessfull", func() {
			Context("when a required environment variable is not given", func() {
				BeforeEach(func() {
					envVars["REQUIRED_THING"] = ""
				})

				It("returns a validation error", func() {
					loadError = envstruct.Load(&ts)

					Expect(loadError).To(MatchError(fmt.Errorf("REQUIRED_THING is required but was empty")))
				})
			})

			Context("with an invalid int", func() {
				BeforeEach(func() {
					envVars["INT_THING"] = "Hello!"
				})

				It("returns an error", func() {
					Expect(envstruct.Load(&ts)).ToNot(Succeed())
				})
			})

			Context("with an invalid uint", func() {
				BeforeEach(func() {
					envVars["UINT_THING"] = "Hello!"
				})

				It("returns an error", func() {
					Expect(envstruct.Load(&ts)).ToNot(Succeed())
				})
			})

			Context("with a failing unmarshaller pointer", func() {
				BeforeEach(func() {
					ts.UnmarshallerPointer.UnmarshalEnvOutput.Ret0 = make(chan error, 100)
					ts.UnmarshallerPointer.UnmarshalEnvOutput.Ret0 <- errors.New("failed to unmarshal")
				})

				It("returns an error", func() {
					Expect(envstruct.Load(&ts)).ToNot(Succeed())
				})
			})

			Context("with a failing unmarshaller value", func() {
				BeforeEach(func() {
					ts.UnmarshallerValue.UnmarshalEnvOutput.Ret0 = make(chan error, 100)
					ts.UnmarshallerValue.UnmarshalEnvOutput.Ret0 <- errors.New("failed to unmarshal")
				})

				It("returns an error", func() {
					Expect(envstruct.Load(&ts)).ToNot(Succeed())
				})
			})
		})
	})
})
