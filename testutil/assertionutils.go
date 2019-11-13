package testutil

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func AssertEqualErr(t *testing.T, expected error, actual error, msgs ...string) bool {
	t.Helper()
	if expected.Error() == actual.Error() {
		return true
	}

	if !errors.Is(expected, actual) {
		assert.Fail(t, fmt.Sprintf("Errors were not equal. Expected: %v, actual: %v.", expected, actual), msgs)
		return false
	}
	return true
}
