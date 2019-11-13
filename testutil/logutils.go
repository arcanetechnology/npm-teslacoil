package testutil

/*
This file has several handy files for logging tests with pretty-printed
output.
*/

import (
	"regexp"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/sirupsen/logrus"
)

var log = build.AddSubLogger("TESTUTIL")

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
