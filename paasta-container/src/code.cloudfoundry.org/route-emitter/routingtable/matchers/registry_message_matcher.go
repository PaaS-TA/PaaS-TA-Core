package matchers

import (
	"fmt"
	"reflect"
	"sort"

	"code.cloudfoundry.org/route-emitter/routingtable"
	"github.com/onsi/gomega/format"
)

func MatchRegistryMessage(message routingtable.RegistryMessage) *registryMessageMatcher {
	return &registryMessageMatcher{
		expected: message,
	}
}

type registryMessageMatcher struct {
	expected routingtable.RegistryMessage
}

func (m *registryMessageMatcher) Match(a interface{}) (success bool, err error) {
	actual, ok := a.(routingtable.RegistryMessage)
	if !ok {
		return false, fmt.Errorf("%s is not a routingtable.RegistryMessage", format.Object(actual, 1))
	}

	sort.Sort(sort.StringSlice(m.expected.URIs))
	sort.Sort(sort.StringSlice(actual.URIs))
	return reflect.DeepEqual(actual, m.expected), nil
}

func (m *registryMessageMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to match", m.expected)
}

func (m *registryMessageMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to match", m.expected)
}
