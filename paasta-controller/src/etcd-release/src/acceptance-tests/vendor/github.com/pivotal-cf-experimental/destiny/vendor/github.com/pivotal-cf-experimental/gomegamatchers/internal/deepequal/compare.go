package deepequal

import (
	"reflect"

	"github.com/pivotal-cf-experimental/gomegamatchers/internal/diff"
)

func Compare(expected interface{}, actual interface{}) (bool, diff.Difference) {
	expectedValue := reflect.ValueOf(expected)
	actualValue := reflect.ValueOf(actual)

	if expectedValue.Type() != actualValue.Type() {
		return false, diff.PrimitiveTypeMismatch{
			ExpectedType: expectedValue.Type(),
			ActualValue:  actualValue.Interface(),
		}
	}

	switch actualValue.Kind() {
	case reflect.Slice:
		return Slice(expectedValue, actualValue)

	case reflect.Map:
		return Map(expectedValue, actualValue)

	default:
		return Primitive(expected, actual)
	}
}
