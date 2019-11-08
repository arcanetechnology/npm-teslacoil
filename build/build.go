package build

import (
	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/build/teslalog"
)

// Log is the logger for the whole application
var subsystemLoggers = map[string]*teslalog.Logger{}

// AddSubLogger creates a new sublogger that prepends `subsystem`
// to the logs
func AddSubLogger(subsystem string) *teslalog.Logger {

	logger := teslalog.New(subsystem)
	subsystemLoggers[subsystem] = logger

	return logger
}

func SetLogLevel(subsystem string, level logrus.Level) {
	logger, ok := subsystemLoggers[subsystem]
	if !ok {
		return
	}

	logger.SetLevel(level)
}

func SetLogLevels(level logrus.Level) {
	for subsystem := range subsystemLoggers {
		SetLogLevel(subsystem, level)
	}
}

// SubLoggers returns all currently registered subsystem loggers for this log
// writer.
func SubLoggers() map[string]*teslalog.Logger {
	return subsystemLoggers
}

func DisableColors() {
	for subsystem := range subsystemLoggers {
		subsystemLoggers[subsystem].DisableColors()
	}
}

// SetLogFile sets logrus to write to the given file
func SetLogFile(file string) error {
	for subsystem := range subsystemLoggers {
		err := subsystemLoggers[subsystem].SetLogFile(file)
		if err != nil {
			return err
		}
	}

	return nil
}
