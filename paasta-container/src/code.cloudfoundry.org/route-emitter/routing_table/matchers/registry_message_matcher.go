package matchers

import (
	"fmt"
	"reflect"
	"sort"

	"code.cloudfoundry.org/route-emitter/routing_table"
	"github.com/onsi/gomega/format"
)

func MatchRegistryMessage(message routing_table.RegistryMessage) *registryMessageMatcher {
	return &registryMessageMatcher{
		expected: message,
	}
}

type registryMessageMatcher struct {
	expected routing_table.RegistryMessage
}

func (m *registryMessageMatcher) Match(a interface{}) (success bool, err error) {
	actual, ok := a.(routing_table.RegistryMessage)
	if !ok {
		return false, fmt.Errorf("%s is not a routing_table.RegistryMessage", format.Object(actual, 1))
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
