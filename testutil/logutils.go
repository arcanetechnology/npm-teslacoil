package testutil

/*
This file has several handy files for logging tests with pretty-printed
output.
*/

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
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

var log = logrus.New()

type LogWriter struct {
	Label string
	Level logrus.Level
}

var lndDateRegex = regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d\d\d`)

func (p LogWriter) Write(data []byte) (n int, err error) {
	logLine := string(data)
	if lndDateRegex.MatchString(logLine) {
		logLine = logLine[24:]
	}

	log.Logf(p.Level, "[%s] %s", p.Label, logLine)
	return len(data), nil
}

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

func FailMsg(t *testing.T, message interface{}) {
	t.Helper()
	FailMsgf(t, "%v", message)
}

func FailMsgf(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	message := fmt.Sprintf(format, args...)
	t.Errorf("\t%s%s\t error: %s%s", red, cross, message, reset)
	t.Fail()
}

func FailError(t *testing.T, message string, err error) {
	t.Helper()
	t.Errorf("\t%s%s\t %s: \terr: %s%s", red, cross, message, err, reset)
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
