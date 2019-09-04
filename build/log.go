package build

import (
	"github.com/sirupsen/logrus"
)

// Log is the logger for the whole application
var Log = logrus.New()

func init() {
	Log.SetLevel(logrus.TraceLevel)
	formatter := logrus.TextFormatter{
		ForceColors: true,
	}
	Log.SetFormatter(&formatter)
}

// SetLogLevel sets the log level for the whole application
func SetLogLevel(logLevel logrus.Level) {
	Log.SetLevel(logLevel)
}
