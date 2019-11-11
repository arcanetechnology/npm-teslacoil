package build

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var logConfigLock sync.Mutex
var baseFormat = formatter{

	TextFormatter: logrus.TextFormatter{
		// This uses an absolutely ridicoulous format:
		// https://stackoverflow.com/a/20234207/10359642
		TimestampFormat: "15:04:05",
		ForceColors:     true,
		FullTimestamp:   true,
	},
	subSystem: "",
}
var _colorsEnabled = true
var _logWriter io.Writer = os.Stdout

func getFormatter(subsystem string) *formatter {
	f := baseFormat
	f.subSystem = subsystem
	return &f
}

type formatter struct {
	logrus.TextFormatter
	subSystem string
}

// Format formats a long entry
func (f *formatter) Format(entry *logrus.Entry) ([]byte, error) {
	entry.Message = fmt.Sprintf("%s %s", f.subSystem, entry.Message)
	return f.TextFormatter.Format(entry)
}

// Log is the logger for the whole application
var subsystemLoggers = map[string]*logrus.Logger{}

func SetLogLevel(subsystem string, level logrus.Level) {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

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

// AddSubLogger creates a new logger with a standard format
func AddSubLogger(subsystem string) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(_logWriter)

	subsystemLoggers[subsystem] = logger

	logger.SetLevel(logrus.TraceLevel)
	f := getFormatter(subsystem)
	if !_colorsEnabled {
		f.DisableColors = true
	}
	logger.SetFormatter(f)
	return logger
}

// SetLogFile sets logrus to write to the given file
func SetLogFile(file string) error {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	logFile, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrap(err, "could not open logfile")
	}
	_logWriter = io.MultiWriter(os.Stdout, logFile)
	for _, logger := range subsystemLoggers {
		logger.SetOutput(_logWriter)
	}
	return nil
}

func DisableColors() {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	_colorsEnabled = false
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

// GinLoggingMiddleWare returns  a middleware that logs incoming requests with Logrus.
// It is based on the discontinued Ginrus middleware: https://github.com/gin-gonic/contrib/blob/master/ginrus/ginrus.go
func GinLoggingMiddleWare(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		withFields := logger.WithFields(logrus.Fields{
			"method":     c.Request.Method,
			"path":       path,
			"ip":         c.ClientIP(),
			"user-agent": c.Request.UserAgent(),
		})

		// read the body so it can be logged
		// we don't check the error here, as we later check for 0 length anyways
		bodyBytes, _ := ioutil.ReadAll(c.Request.Body)
		// restore the original buffer so it can be read later
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

		if c.Request.URL != nil {
			query := c.Request.URL.Query()
			if len(query) > 0 {
				withFields = withFields.WithField("query", query)
			}
		}

		// if the body is non-empty, add it
		if len(bodyBytes) != 0 {
			withFields = withFields.WithField("body", string(bodyBytes))
		}

		// pass the request on to the next handler
		c.Next()

		// set status after error have been handled
		withFields = withFields.WithField("status", c.Writer.Status())

		// public errors are errors that shouldn't be shown to the end user,
		// but might be of relevance for logging purposes
		privateErrors := c.Errors.ByType(gin.ErrorTypePrivate)
		if len(privateErrors) > 0 {
			withFields = withFields.WithField("privateErrors", privateErrors)
		}

		// public errors are errors that can be shown to the end user
		publicErrors := c.Errors.ByType(gin.ErrorTypePublic)
		if len(publicErrors) > 0 {
			withFields = withFields.WithField("publicErrors", publicErrors)
		}

		// binding errors are errors produced during request validation
		bindingErrors := c.Errors.ByType(gin.ErrorTypeBind)
		if len(bindingErrors) > 0 {
			withFields = withFields.WithField("bindingErrors", bindingErrors)
		}

		end := time.Now()
		latency := end.Sub(start)

		withFields = withFields.WithField("latency", latency)
		status := c.Writer.Status()
		requestLevel := logger.Level
		if status >= 300 {
			requestLevel = logrus.ErrorLevel
		}
		withFields.Logf(requestLevel, "HTTP %s %s: %d", c.Request.Method, path, status)
	}
}
