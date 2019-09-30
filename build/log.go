package build

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// Log is the logger for the whole application
var Log = logrus.New()

func init() {
	Log.SetLevel(logrus.TraceLevel)
	formatter := logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		// This uses an absolutely ridicoulous format:
		// https://stackoverflow.com/a/20234207/10359642
		TimestampFormat: "15:04:05",
	}
	Log.SetFormatter(&formatter)
}

// SetLogLevel sets the log level for the whole application
func SetLogLevel(logLevel logrus.Level) {
	Log.SetLevel(logLevel)
}

// ToLogLevel takes in a string and converts it to a Logrus log level
func ToLogLevel(s string) (logrus.Level, error) {
	switch strings.ToLower(s) {
	case "trace":
		return logrus.TraceLevel, nil
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal":
		return logrus.FatalLevel, nil
	case "panic":
		return logrus.FatalLevel, nil
	default:
		return logrus.InfoLevel, fmt.Errorf("%s is not a valid log level", s)
	}
}
