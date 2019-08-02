package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/btcsuite/btclog"
	"github.com/jrick/logrotate/rotator"
	"github.com/lightningnetwork/lnd/build"
	"gitlab.com/arcanecrypto/lpp/cmd/lpp/api"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

var (
	logWriter = &build.LogWriter{}

	backendLog = btclog.NewBackend(logWriter)

	logRotator *rotator.Rotator

	apiLog      = build.NewSubLogger("API", backendLog.Logger)
	cliLog      = build.NewSubLogger("CLI", backendLog.Logger)
	paymentsLog = build.NewSubLogger("PAYMENTS", backendLog.Logger)
	usersLog    = build.NewSubLogger("USERS", backendLog.Logger)
	dbLog       = build.NewSubLogger("DB", backendLog.Logger)
	lnLog       = build.NewSubLogger("LN", backendLog.Logger)
)

func init() {
	api.UseLogger(apiLog)
	users.UseLogger(usersLog)
	payments.UseLogger(paymentsLog)
	db.UseLogger(dbLog)
	ln.UseLogger(lnLog)
}

// subsystemLoggers maps each subsystem identifier to its associated logger.
var subsystemLoggers = map[string]btclog.Logger{
	"API":      apiLog,
	"USERS":    usersLog,
	"PAYMENTS": paymentsLog,
	"DB":       dbLog,
	"LN":       lnLog,
}

// initLogRotator initializes the logging rotator to write logs to logFile and
// create roll files in the same directory.  It must be called before the
// package-global log rotator variables are used.
func initLogRotator(logFile string, MaxLogFileSize int, MaxLogFiles int) {
	fmt.Printf("logFile %s\n", logFile)
	logDir, _ := filepath.Split(logFile)
	fmt.Printf("logDir %s\n", logDir)
	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create log directory: %v\n", err)
		os.Exit(1)
	}
	r, err := rotator.New(logFile, int64(MaxLogFileSize*1024), false, MaxLogFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create file rotator: %v\n", err)
		os.Exit(1)
	}

	pr, pw := io.Pipe()
	go r.Run(pr)

	logWriter.RotatorPipe = pw
	logRotator = r

	setLogLevels("INFO")
}

// setLogLevel sets the logging level for provided subsystem.  Invalid
// subsystems are ignored.  Uninitialized subsystems are dynamically created as
// needed.
func setLogLevel(subsystemID string, logLevel string) {
	// Ignore invalid subsystems.
	logger, ok := subsystemLoggers[subsystemID]
	if !ok {
		return
	}

	// Defaults to info if the log level is invalid.
	level, _ := btclog.LevelFromString(logLevel)
	logger.SetLevel(level)
}

// setLogLevels sets the log level for all subsystem loggers to the passed
// level. It also dynamically creates the subsystem loggers as needed, so it
// can be used to initialize the logging system.
func setLogLevels(logLevel string) {
	// Configure all sub-systems with the new logging level.  Dynamically
	// create loggers as needed.
	for subsystemID := range subsystemLoggers {
		setLogLevel(subsystemID, logLevel)
	}
}

// logClosure is used to provide a closure over expensive logging operations so
// don't have to be performed when the logging level doesn't warrant it.
type logClosure func() string

// String invokes the underlying function and returns the result.
func (c logClosure) String() string {
	return c()
}

// newLogClosure returns a new closure over a function that returns a string
// which itself provides a Stringer interface so that it can be used with the
// logging system.
func newLogClosure(c func() string) logClosure {
	return logClosure(c)
}
