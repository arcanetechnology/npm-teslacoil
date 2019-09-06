package testutil

/*
This file has several handy files for logging tests with pretty-printed
output.
*/

import (
	"fmt"
	"strings"
	"testing"
)

const (
	///////////////////////////
	// ASCII control characters
	green = "\u001b[32m"
	red   = "\u001b[31m"
	reset = "\u001b[0m"
	// resets coloring
	///////////////////////////

	checkmark = "✔️"
	cross     = "❌"
)

// FatalMsg fails the test immedetialy, printing a red
// error message containing the given test message
func FatalMsg(t *testing.T, message interface{}) {
	t.Helper()
	var msg string

	switch message := message.(type) {
	case error:
		msg = message.Error()
	case fmt.Stringer:
		msg = message.String()
	case string:
		msg = message
	}

	FatalMsgf(t, msg)
}

// FatalMsgf fails the test immediately, printing a red error message containing
// the given format string interpolated with the given args
func FatalMsgf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	message := fmt.Sprintf(format, args...)
	t.Fatalf("\t%s%s\t error: %s%s", red, cross, message, reset)
}

func FailMsg(t *testing.T, message string) {
	t.Helper()
	FailMsgf(t, message)
}

func FailMsgf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	message := fmt.Sprintf(format, args...)
	t.Errorf("\t%s%s\t error: %s%s", red, cross, message, reset)
	t.Fail()
}

// DescribeTest logs the name of test with a green checkmark
func DescribeTest(t *testing.T) {
	t.Helper()
	strippedName := strings.TrimLeft(t.Name(), "Test")
	t.Logf("\t%s\t should be able to %s%s", green, strippedName, reset)
}

// Log logs the given message with a green checkmark
func Succeed(t *testing.T, message string) {
	t.Helper()
	Succeedf(t, message)
}

// Logf logs the given format message and arguments with a green checkmark
func Succeedf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	message := fmt.Sprintf(format, args...)
	t.Logf("\t%s%s\t%s%s", green, checkmark, message, reset)
}
