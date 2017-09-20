package gomegamatchers

import (
	"fmt"
	"reflect"

	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func MatchYAML(expected interface{}) types.GomegaMatcher {
	return &MatchYAMLMatcher{expected}
}

type MatchYAMLMatcher struct {
	YAMLToMatch interface{}
}

func (matcher *MatchYAMLMatcher) Match(actual interface{}) (success bool, err error) {
	actualString, err := matcher.prettyPrint(actual)
	if err != nil {
		return false, err
	}

	expectedString, err := matcher.prettyPrint(matcher.YAMLToMatch)
	if err != nil {
		return false, err
	}

	var aval interface{}
	var eval interface{}

	// this is guarded by prettyPrint
	candiedyaml.Unmarshal([]byte(actualString), &aval)
	candiedyaml.Unmarshal([]byte(expectedString), &eval)

	return reflect.DeepEqual(aval, eval), nil
}

func (matcher *MatchYAMLMatcher) FailureMessage(actual interface{}) (message string) {
	actualString, _ := matcher.prettyPrint(actual)
	expectedString, _ := matcher.prettyPrint(matcher.YAMLToMatch)
	return format.Message(actualString, "to match YAML of", expectedString)
}

func (matcher *MatchYAMLMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	actualString, _ := matcher.prettyPrint(actual)
	expectedString, _ := matcher.prettyPrint(matcher.YAMLToMatch)
	return format.Message(actualString, "not to match YAML of", expectedString)
}

func (matcher *MatchYAMLMatcher) prettyPrint(input interface{}) (formatted string, err error) {
	inputString, ok := toString(input)
	if !ok {
		return "", fmt.Errorf("MatchYAMLMatcher matcher requires a string or stringer.  Got:\n%s", format.Object(input, 1))
	}

	var data interface{}
	if err := candiedyaml.Unmarshal([]byte(inputString), &data); err != nil {
		return "", err
	}
	buf, _ := candiedyaml.Marshal(data)

	return string(buf), nil
}

func toString(a interface{}) (string, bool) {
	aString, isString := a.(string)
	if isString {
		return aString, true
	}

	aBytes, isBytes := a.([]byte)
	if isBytes {
		return string(aBytes), true
	}

	aStringer, isStringer := a.(fmt.Stringer)
	if isStringer {
		return aStringer.String(), true
	}

	return "", false
}
