package api

// log is a logger that is initialized with no output filters.  This
// means the package will not perform any logging by default until the caller
// requests it.

// DisableLog disables all library log output.  Logging output is disabled
// by default until either UseLogger or SetLogWriter are called.
// func DisableLog() {
	// UseLogger(btclog.Disabled)
// }

// UseLogger uses a specified Logger to output package logging info.
// This should be used in preference to SetLogWriter if the caller is also
// using btclog.

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.New()

func init() {
	log.WithFields(logrus.Fields{
		"package": "db",
	})
}