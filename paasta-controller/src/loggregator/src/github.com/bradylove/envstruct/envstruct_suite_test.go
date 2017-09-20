//go:generate hel

package envstruct_test

import (
	"net/url"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	baseEnvVars = map[string]string{
		"STRING_THING":         "stringy thingy",
		"REQUIRED_THING":       "im so required",
		"BOOL_THING":           "true",
		"INT_THING":            "100",
		"INT8_THING":           "20",
		"INT16_THING":          "2000",
		"INT32_THING":          "200000",
		"INT64_THING":          "200000000",
		"UINT_THING":           "100",
		"UINT8_THING":          "20",
		"UINT16_THING":         "2000",
		"UINT32_THING":         "200000",
		"UINT64_THING":         "200000000",
		"STRING_SLICE_THING":   "one,two,three",
		"INT_SLICE_THING":      "1,2,3",
		"DURATION_THING":       "2s",
		"URL_THING":            "http://github.com/some/path",
		"UNMARSHALLER_POINTER": "pointer",
		"UNMARSHALLER_VALUE":   "value",
	}
)

type LargeTestStruct struct {
	NonEnvThing   string
	DefaultThing  string `env:"default_thing"`
	StringThing   string `env:"string_thing"`
	RequiredThing string `env:"required_thing,noreport,required"`

	BoolThing bool `env:"bool_thing"`

	IntThing    int    `env:"int_thing"`
	Int8Thing   int8   `env:"int8_thing"`
	Int16Thing  int16  `env:"int16_thing"`
	Int32Thing  int32  `env:"int32_thing"`
	Int64Thing  int64  `env:"int64_thing"`
	UintThing   uint   `env:"uint_thing"`
	Uint8Thing  uint8  `env:"uint8_thing"`
	Uint16Thing uint16 `env:"uint16_thing"`
	Uint32Thing uint32 `env:"uint32_thing"`
	Uint64Thing uint64 `env:"uint64_thing"`

	StringSliceThing []string `env:"string_slice_thing"`
	IntSliceThing    []int    `env:"int_slice_thing"`

	DurationThing time.Duration `env:"duration_thing"`
	URLThing      *url.URL      `env:"url_thing"`

	UnmarshallerPointer *mockUnmarshaller `env:"unmarshaller_pointer"`
	UnmarshallerValue   mockUnmarshaller  `env:"unmarshaller_value"`
}

type SmallTestStruct struct {
	HiddenThing      string   `env:"hidden_thing,noreport"`
	StringThing      string   `env:"string_thing"`
	BoolThing        bool     `env:"bool_thing"`
	IntThing         int      `env:"int_thing"`
	URLThing         *url.URL `env:"url_thing"`
	StringSliceThing []string `env:"string_slice_thing"`
}

func TestEnvstruct(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Envstruct Suite")
}
