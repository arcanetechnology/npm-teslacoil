package build

import (
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/api/apiauth"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/apikeyroutes"
	"gitlab.com/arcanecrypto/teslacoil/api/apitxs"
	"gitlab.com/arcanecrypto/teslacoil/api/apiusers"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/api/validation"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	teslalog "gitlab.com/arcanecrypto/teslacoil/build/teslalog"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/actions"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/flags"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/dummy"
	"gitlab.com/arcanecrypto/teslacoil/email"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

// Log is the logger for the whole application
var subsystemLoggers = map[string]*teslalog.Logger{}

func init() {
	// api loggers
	addSubLogger("APIM", api.UseLogger)
	addSubLogger("APIA", apiauth.UseLogger)
	addSubLogger("APIE", apierr.UseLogger)
	addSubLogger("APIK", apikeyroutes.UseLogger)
	addSubLogger("APIT", apitxs.UseLogger)
	addSubLogger("APIU", apiusers.UseLogger)

	addSubLogger("DB", db.UseLogger)
	addSubLogger("LN", ln.UseLogger)
	addSubLogger("BTCD", bitcoind.UseLogger)

	// models loggers
	addSubLogger("TXNS", transactions.UseLogger)
	addSubLogger("USER", users.UseLogger)
	addSubLogger("KEYS", apikeys.UseLogger)

	addSubLogger("AUTH", auth.UseLogger)
	addSubLogger("VDTN", validation.UseLogger)
	addSubLogger("ACTN", actions.UseLogger)
	addSubLogger("FLAG", flags.UseLogger)
	addSubLogger("DMMY", dummy.UseLogger)
	addSubLogger("EMAL", email.UseLogger)

	// test subloggeres
	addSubLogger("TESTU", testutil.UseLogger)
}

func addSubLogger(subsystem string, useLogger func(*teslalog.Logger)) {
	logger := teslalog.New(subsystem)

	subsystemLoggers[subsystem] = logger
	useLogger(logger)
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
