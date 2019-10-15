package build

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Log is the logger for the whole application
var Log = logrus.New()

func getFormatter() *logrus.TextFormatter {
	return &logrus.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
		// This uses an absolutely ridicoulous format:
		// https://stackoverflow.com/a/20234207/10359642
		TimestampFormat: "15:04:05",
	}
}

func init() {
	Log.SetLevel(logrus.TraceLevel)
	Log.SetFormatter(getFormatter())
}

// SetLogLevel sets the log level for the whole application
func SetLogLevel(logLevel logrus.Level) {
	Log.SetLevel(logLevel)
}

// DisableColors forces logrus to log without colors
func DisableColors() {
	formatter := getFormatter()
	formatter.DisableColors = true
	Log.SetFormatter(formatter)
}

// SetLogFile sets logrus to write to the given file
func SetLogFile(file string) error {
	logFile, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrap(err, "could not open logfile")
	}
	writer := io.MultiWriter(os.Stdout, logFile)
	Log.SetOutput(writer)
	return nil
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

type loggerEntryWithFields interface {
	WithFields(fields logrus.Fields) *logrus.Entry
}

// GinLoggingMiddleWare returns  a middleware that logs incoming requests with Logrus.
// It is based on the discontinued Ginrus middleware: https://github.com/gin-gonic/contrib/blob/master/ginrus/ginrus.go
func GinLoggingMiddleWare(logger loggerEntryWithFields, level logrus.Level) gin.HandlerFunc {
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

		withFields.WithField("latency", latency).Log(level, "Gin request")
	}
}
