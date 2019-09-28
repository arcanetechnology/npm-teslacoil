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
	if reflect.ValueOf(expected).Kind() == reflect.Struct {
		panic(`argument "expected" can not be a struct. To compare structs use the function AssertStructEqual`)
	}
	if reflect.ValueOf(actual).Kind() == reflect.Struct {
		panic(`argument "actual" can not be a struct. To compare structs use the function AssertStructEqual`)
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

// AssertStructEquals assert that the two structs are deeply equal.
// AssertStructEquals always compares values, never pointers.
// All pointers are expanded, and their values are compared
func AssertStructEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if !cmp.Equal(expected, actual) {
		FatalMsgf(t, "expected struct %+v to equal struct %+v, however it does not", expected, actual)
	}
}
