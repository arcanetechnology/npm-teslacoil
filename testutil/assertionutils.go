package testutil

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func isNilValue(i interface{}) bool {
	switch t := i.(type) {
	case nil:
		return true
	case int:
		return t == 0
	case string:
		return t == ""
	case float32:
	case float64:
		return t == 0
	case bool:
		return !t

	}
	return reflect.ValueOf(i).IsNil()
}

// AssertEqual asserts that the given expected and actual values are equal
// Does not work with structs, use AssertStructEquals if you want to compare
// structs
func AssertEqual(t *testing.T, expected interface{}, actual interface{}) {
	t.Helper()
	if reflect.ValueOf(expected).Kind() == reflect.Struct && reflect.ValueOf(actual).Kind() == reflect.Struct {
		if !reflect.DeepEqual(expected, actual) {
			FatalMsgf(t, "expected structs to be equal: %s", cmp.Diff(expected, actual))
		}
		return
	}

	bothAreNil := isNilValue(expected) && isNilValue(actual)
	if !bothAreNil && expected != actual {
		FatalMsgf(t, "Expected (%+v) is not equal to actual (%+v)!", expected, actual)
	}
}

//AssertMsg asserts that the given condition holds, failing with the given
// message if it doesn't
func AssertMsg(t *testing.T, cond bool, message string) {
	t.Helper()
	if !cond {
		FatalMsgf(t, "Assertion error: %s", message)
	}
}

// AssertMsgf assert that the given condition holds, failing with the given
// format string and args if it doesn't
func AssertMsgf(t *testing.T, cond bool, format string, args ...interface{}) {
	t.Helper()
	AssertMsg(t, cond, fmt.Sprintf(format, args...))
}

// AssertMapEquals asserts that the `actual` map has all the keys with the
// same values as `expected`
func AssertMapEquals(t *testing.T,
	expected, actual map[string]interface{}) {
	t.Helper()
	for key := range expected {
		actualVal, ok := actual[key]
		if !ok {
			FatalMsgf(t, "Expected map contains key %s, actual map does not!",
				key)
		}
		expectedVal := expected[key]
		if actualVal != expectedVal {
			FatalMsgf(t, "Expected[%s] (%+v) is not equal to actual[%s] (%+v)!",
				key, expectedVal, key, actualVal)

		}
	}
}
