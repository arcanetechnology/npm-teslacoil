package teslalog

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Logger is our custom logger for the whole application
type Logger struct {
	*logrus.Logger
	Subsystem string
}

var StandardFormatter = &Formatter{}

type Formatter struct {
	TimestampFormat string // default: "2006-01-02 15:04:05"
	DisableColors   bool   // disable colors
	Subsystem       string
}

func (l Logger) getFormatter() *Formatter {
	return &Formatter{
		// This uses an absolutely ridicoulous format:
		// https://stackoverflow.com/a/20234207/10359642
		TimestampFormat: "2006-01-02 15:04:05.000",
		Subsystem:       l.Subsystem,
	}
}

// New creates a new logger with a standard format
func New(subsystem string) *Logger {
	logger := &Logger{logrus.New(), subsystem}
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(logger.getFormatter())

	return logger
}

// DisableColors forces logrus to log without colors
func (l Logger) DisableColors() {
	formatter := l.getFormatter()
	formatter.DisableColors = true
	l.SetFormatter(formatter)
}

// SetLogFile sets logrus to write to the given file
func (l Logger) SetLogFile(file string) error {
	logFile, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return errors.Wrap(err, "could not open logfile")
	}
	writer := io.MultiWriter(os.Stdout, logFile)
	l.SetOutput(writer)
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

// GinLoggingMiddleWare returns  a middleware that logs incoming requests with Logrus.
// It is based on the discontinued Ginrus middleware: https://github.com/gin-gonic/contrib/blob/master/ginrus/ginrus.go
func GinLoggingMiddleWare(logger *Logger) gin.HandlerFunc {
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

// Format formats a long entry
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {

	// output buffer
	b := &bytes.Buffer{}

	// write timestamp
	timestampFormat := f.TimestampFormat
	if timestampFormat == "" {
		timestampFormat = "2006-01-02 15:04:05.000"
	}
	b.WriteString(entry.Time.Format(timestampFormat))

	// write level
	level := strings.ToUpper(entry.Level.String())
	levelColor := getColorByLevel(entry.Level)
	if !f.DisableColors {
		b.WriteString(fmt.Sprintf("\x1b[%dm", levelColor))
	}

	b.WriteString(fmt.Sprintf(" [%s]", level[:4]))
	if !f.DisableColors {
		b.WriteString("\x1b[0m")
	}

	// write subsystem
	b.WriteString(fmt.Sprintf(" %s: ", f.Subsystem))

	// write message
	b.WriteString(entry.Message)
	b.WriteString("\t\t")

	if !f.DisableColors {
		b.WriteString(fmt.Sprintf("\x1b[%dm", levelColor))
	}
	// write fields
	f.writeFields(b, entry)

	if !f.DisableColors {
		b.WriteString("\x1b[0m")
	}
	b.WriteByte('\n')

	return b.Bytes(), nil
}

func (f *Formatter) writeFields(b *bytes.Buffer, entry *logrus.Entry) {
	if len(entry.Data) != 0 {
		fields := make([]string, 0, len(entry.Data))
		for field := range entry.Data {
			fields = append(fields, field)
		}

		sort.Strings(fields)

		for _, field := range fields {
			f.writeField(b, entry, field)
		}
	}
}

func (f *Formatter) writeField(b *bytes.Buffer, entry *logrus.Entry, field string) {
	fmt.Fprintf(b, "%s=%v ", field, entry.Data[field])
}

const (
	colorRed    = 31
	colorYellow = 33
	colorBlue   = 36
	colorGray   = 37
)

func getColorByLevel(level logrus.Level) int {
	switch level {
	case logrus.DebugLevel:
		return colorGray
	case logrus.WarnLevel:
		return colorYellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return colorRed
	default:
		return colorBlue
	}
}
