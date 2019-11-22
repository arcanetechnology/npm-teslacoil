package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type tunableLogger interface {
	setLevel(level logrus.Level)
	setDir(dir string) error
}

type hook struct {
	console     *consoleLogHook
	jsonFile    *jsonFileHook
	regularFile *humanReadableFileHook
}

var _ tunableLogger = &hook{}

func (h *hook) setDir(dir string) error {
	jsonFile, err := openFileForAppend(filepath.Join(dir, "teslacoil.log.json"))
	if err != nil {
		return fmt.Errorf("could not open JSON log file: %w", err)
	}
	h.jsonFile.file = jsonFile

	regularFile, err := openFileForAppend(filepath.Join(dir, "teslacoil.log"))
	if err != nil {
		return fmt.Errorf("could not open regular log file: %w", err)
	}
	h.regularFile.file = regularFile
	return nil
}

func (h *hook) setLevel(level logrus.Level) {
	h.console.setLevel(level)
	h.jsonFile.setLevel(level)
	h.regularFile.setLevel(level)
}

var logConfigLock sync.Mutex
var subsystemHooks = map[string]tunableLogger{}

func SetLogLevel(subsystem string, level logrus.Level) {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	hook, ok := subsystemHooks[subsystem]
	if !ok {
		return
	}
	hook.setLevel(level)
}

func SetLogLevels(level logrus.Level) {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	for _, hook := range subsystemHooks {
		hook.setLevel(level)
	}
}

// AddSubLogger creates a new logger with a standard format
func AddSubLogger(subsystem string) *logrus.Logger {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	logger := logrus.New()
	logger.SetOutput(ioutil.Discard) // send all logs to nowhere by default

	jsonHook := &jsonFileHook{
		subsystem: subsystem,
	}
	fileHook := &humanReadableFileHook{
		subsystem: subsystem,
	}
	consoleHook := &consoleLogHook{
		subsystem: subsystem,
	}
	logger.AddHook(jsonHook)    // write logs to JSON formatted file
	logger.AddHook(fileHook)    // write non-colored with precise timestamp logs to human readable file
	logger.AddHook(consoleHook) // write colored logs with imprecise timestamp to console
	trio := &hook{
		console:     consoleHook,
		jsonFile:    jsonHook,
		regularFile: fileHook,
	}
	subsystemHooks[subsystem] = trio

	return logger
}

func openFileForAppend(file string) (*os.File, error) {
	return os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
}

// SetLogDir sets logrus to write to the given directory
func SetLogDir(dir string) error {
	logConfigLock.Lock()
	defer logConfigLock.Unlock()

	for _, hook := range subsystemHooks {
		if err := hook.setDir(dir); err != nil {
			return err
		}
	}
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
func GinLoggingMiddleWare(logger *logrus.Logger, blacklist []string) gin.HandlerFunc {
	blackListMap := make(map[string]struct{})
	for _, elem := range blacklist {
		blackListMap[elem] = struct{}{}
	}

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
		var bodyBytes []byte
		// don't read the body if the path is blacklisted
		if _, found := blackListMap[path]; !found {
			// we don't check the error here, as we later check for 0 length anyways
			bodyBytes, _ = ioutil.ReadAll(c.Request.Body)
			// restore the original buffer so it can be read later
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		} else {
			bodyBytes = []byte("not logged")
		}

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

type consoleLogHook struct {
	hasLevel
	subsystem string
}

var _ logrus.Hook = &consoleLogHook{}
var consoleFormat = logrus.TextFormatter{
	TimestampFormat: "15:04:05",
	ForceColors:     true,
	FullTimestamp:   true,
}

func (c *consoleLogHook) Fire(entry *logrus.Entry) error {
	if c.level < entry.Level {
		return nil
	}
	if entry == nil {
		return nil
	}

	// append subsystem to log message
	copied := *entry
	copied.Message = fmt.Sprintf("%s %s", c.subsystem, entry.Message)

	formatted, err := consoleFormat.Format(&copied)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(formatted)
	return err
}

type humanReadableFileHook struct {
	hasLevel
	file      *os.File
	subsystem string
}

var _ logrus.Hook = &humanReadableFileHook{}
var fileHookFormat = logrus.TextFormatter{
	// see comment below on coloring and formatting
	ForceColors:     true,
	TimestampFormat: time.RFC3339,
	FullTimestamp:   true,
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegex = regexp.MustCompile(ansi)

func (h humanReadableFileHook) Fire(entry *logrus.Entry) error {
	// don't write anything if file isn't set
	if h.file == nil {
		return nil
	}

	if h.level < entry.Level {
		return nil
	}

	if entry == nil {
		return nil
	}

	// append subsystem to log message
	copied := *entry
	copied.Message = fmt.Sprintf("%s %s", h.subsystem, entry.Message)
	formatted, err := fileHookFormat.Format(&copied)
	if err != nil {
		return err
	}

	// for some reason whether or not you log with color affect the default
	// output of logrus... we wan't the formats written to file and console
	// to be more or less identical, so we have to log _with_ color and then
	// strip out the ANSI codes afterwards...
	stripped := ansiRegex.ReplaceAll(formatted, nil)
	_, err = h.file.Write(stripped)
	return err
}

type jsonFileHook struct {
	hasLevel
	file      *os.File
	subsystem string
}

var _ logrus.Hook = &jsonFileHook{}
var jsonHookFormat = logrus.JSONFormatter{
	TimestampFormat: time.RFC3339,
}

func (j jsonFileHook) Fire(entry *logrus.Entry) error {
	// don't write anything if file isn't set
	if j.file == nil {
		return nil
	}

	if j.level < entry.Level {
		return nil
	}
	if entry == nil {
		return nil
	}

	// this is a bit awkward, but we want to add a field to the entry without
	// modifying the underlying entry map. this is because this entry map is
	// shared with other loggers. `WithField` doesn't copy over _everything_,
	// for some reason. so we copy message and level manually
	withSubsystem := entry.WithField("subsystem", j.subsystem)
	withSubsystem.Message = entry.Message
	withSubsystem.Level = entry.Level
	formatted, err := jsonHookFormat.Format(withSubsystem)
	if err != nil {
		return err
	}

	_, err = j.file.Write(formatted)
	return err
}

type hasLevel struct {
	level logrus.Level
}

// Levels is here to satisfy the Hook interface.
// confusingly, this method appears to do exactly nothing... it's only needed
// to satisfy an interface, and the field was added over 6 years ago
// https://github.com/sirupsen/logrus/blob/93a1736895ca25a01a739e0394bf7f672299a27d/hooks.go#L9
// I've not been able to identify any places in the code except for tests where
// that value is actually read
func (h *hasLevel) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *hasLevel) setLevel(level logrus.Level) {
	h.level = level
}
