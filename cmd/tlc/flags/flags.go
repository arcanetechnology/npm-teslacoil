// Package flags provides functionality for managing flags for tlc
package flags

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
)

var log = build.AddSubLogger("FLAG")

// Concat concatenates the given list of flags, without mutating them
func Concat(first []cli.Flag, rest ...[]cli.Flag) []cli.Flag {
	var copied = make([]cli.Flag, len(first))
	_ = copy(copied, first)
	for _, r := range rest {
		copied = append(copied, r...)
	}
	return copied
}

// CommonFlags is a set of flags that all commands take
var CommonFlags = Concat([]cli.Flag{
	cli.StringFlag{
		Name:  "network",
		Usage: "the network lnd is running on e.g. mainnet, testnet, etc.",
		Value: "regtest",
	},
}, logging)

// ReadDbConf reads the approriate flags for connecting to the DB
func ReadDbConf(c *cli.Context) db.DatabaseConfig {
	conf := db.DatabaseConfig{
		User:           c.String("db.user"),
		Password:       c.String("db.password"),
		Host:           c.String("db.host"),
		Port:           c.Int("db.port"),
		Name:           c.String("db.name"),
		MigrationsPath: c.String("db.migrationspath"),
	}

	// if no scheme was supplied to migrations path, default to file:
	parsedPath, err := url.Parse(conf.MigrationsPath)
	if err != nil {
		panic(fmt.Errorf("could not parse migrations path into URL: %w", err))
	}
	if len(parsedPath.Scheme) == 0 {
		conf.MigrationsPath = path.Join("file:", conf.MigrationsPath)
	}

	// how flags work in urfave/cli can be a bit confusing. flags belongs to a
	// context, and I haven't been able to find a natural way of scoping flags
	// correctly. so one issue that kept popping up was that DB flags were passed
	// in, but weren't picked up, because we did c.String instead of c.GlobalString.
	// however, doing c.GlobalString (or Int, or whatever) everywhere doesn't work
	// either. therefore, we recurse here until we find a context where the flags
	// are defined
	if conf.User == "" {
		parent := c.Parent()
		if parent == nil {
			panic("Reached root CLI context without hitting valid DB credentials!")
		}
		return ReadDbConf(parent)
	}
	return conf
}

// readNetwork reads the network flag, erroring if an invalid value is passed
func readNetwork(c *cli.Context) (chaincfg.Params, error) {
	var network chaincfg.Params
	networkString := c.GlobalString("network")
	switch networkString {
	case "mainnet":
		network = chaincfg.MainNetParams
	case "testnet", "testnet3":
		network = chaincfg.TestNet3Params
	case "regtest", "":
		network = chaincfg.RegressionNetParams
	default:
		return chaincfg.Params{}, fmt.Errorf("unknown network: %s. Valid: mainnet, testnet, regtest", networkString)
	}
	return network, nil
}

// ReadLnConf reads the approriate flags for constructing a LND configuration
func ReadLnConf(c *cli.Context) (ln.LightningConfig, error) {
	network, err := readNetwork(c)
	if err != nil {
		return ln.LightningConfig{}, err
	}

	return ln.LightningConfig{
		LndDir:       c.String("lnd.dir"),
		TLSCertPath:  c.String("lnd.certpath"),
		MacaroonPath: c.String("lnd.macaroonpath"),
		Network:      network,
		RPCHost:      c.String("lnd.rpchost"),
		RPCPort:      c.Int("lnd.rpcport"),
	}, nil
}

// ReadBitcoindConf reads the approriate flags for constructing a bitcoind configuration
func ReadBitcoindConf(c *cli.Context) (bitcoind.Config, error) {

	network, err := readNetwork(c)
	if err != nil {
		return bitcoind.Config{}, nil
	}

	host := c.String("bitcoind.rpchost")
	conf := bitcoind.Config{
		ZmqPubRawTx:    fmt.Sprintf("%s:%d", host, c.Int("bitcoind.zmqpubrawtx")),
		ZmqPubRawBlock: fmt.Sprintf("%s:%d", host, c.Int("bitcoind.zmqpubrawblock")),
		RpcPort:        c.Int("bitcoind.rpcport"),
		Password:       c.String("bitcoind.rpcpassword"),
		User:           c.String("bitcoind.rpcuser"),
		Network:        network,
		RpcHost:        host,
	}

	if conf.RpcPort == 0 {
		log.Debug("bitcoind.rpcport flag is not set, falling back to network default")
		port, err := bitcoind.DefaultRpcPort(network)
		if err != nil {
			return bitcoind.Config{}, err
		}
		conf.RpcPort = port
	}

	return conf, nil
}

// Lnd is a list of flags that apply to functionality that needs LND
var Lnd = []cli.Flag{
	cli.StringFlag{
		Name:     "lnd.dir",
		Usage:    "path to lnd's base directory",
		Required: true,
	},
	cli.StringFlag{
		Name:      "lnd.certpath",
		Usage:     "path to tls.cert",
		TakesFile: true,
	},
	cli.StringFlag{
		Name:      "lnd.macaroonpath",
		Usage:     "path to macaroon file",
		TakesFile: true,
	},
	cli.StringFlag{
		Name:  "lnd.rpchost",
		Value: "localhost",
		Usage: "host of ln daemon",
	},
	cli.IntFlag{
		Name:  "lnd.rpcport",
		Usage: "Port of ln daemon",
		Value: 10009,
	},
}

// Bitcoind is a list of flags that apply to functionality that needs bitcoind
var Bitcoind = []cli.Flag{
	cli.StringFlag{
		Name:     "bitcoind.rpcuser",
		Usage:    "The bitcoind RPC username",
		Required: true,
	},
	cli.StringFlag{
		Name:     "bitcoind.rpcpassword",
		Usage:    "The bitcoind RPC password",
		Required: true,
	},
	cli.IntFlag{
		Name:  "bitcoind.rpcport",
		Usage: "The bitcoind RPC port",
	},
	cli.StringFlag{
		Name:  "bitcoind.rpchost",
		Usage: "The bitcoind RPC host",
		Value: "localhost",
	},
	cli.IntFlag{
		Name:     "bitcoind.zmqpubrawblock",
		Usage:    "The port listening for ZMQ connections to deliver raw block notifications",
		Required: true,
	},
	cli.IntFlag{
		Name:     "bitcoind.zmqpubrawtx",
		Usage:    "The port listening for ZMQ connections to deliver raw transaction notifications",
		Required: true,
	},
}

// Db is a list of flags that apply to functionality that needs Db access
var Db = []cli.Flag{
	cli.StringFlag{
		Name:     "db.user",
		Usage:    "Database user",
		EnvVar:   "DATABASE_USER",
		Required: true,
	},
	cli.StringFlag{
		Name:     "db.password",
		Usage:    "Database password",
		EnvVar:   "DATABASE_PASSWORD",
		Required: true,
	},
	cli.StringFlag{
		Name:   "db.name",
		Usage:  "Database name",
		Value:  "tlc",
		EnvVar: "DATABASE_NAME",
	},
	cli.StringFlag{
		Name:  "db.host",
		Usage: "Database host to connect to",
		Value: "localhost",
	},
	cli.IntFlag{
		Name:   "db.port",
		Usage:  "Database port",
		Value:  5432,
		EnvVar: "DATABASE_PORT",
	},
	cli.StringFlag{
		Name:      "db.migrationspath",
		Usage:     `Path to DB migrations. Needs scheme ("file", etc.) in front of path"`,
		TakesFile: true,
		Value: func() string {
			dir, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			return filepath.Join("file:", dir, "db", "migrations")
		}(),
	},
	cli.BoolFlag{
		Name:  "db.migrateup",
		Usage: "Apply migrations before starting the API",
	},
}

// logging is logging related CLI flags
var logging = []cli.Flag{
	cli.StringFlag{
		Name:  "logging.level",
		Value: logrus.InfoLevel.String(),
		Usage: "Logging level for all subsystems {trace, debug, info, warn, error, fatal, panic}",
	},
	cli.StringFlag{
		Name:  "logging.httplevel",
		Value: logrus.InfoLevel.String(),
		Usage: "Logging level for HTTP requests {trace, debug, info, warn, error, fatal, panic}",
	},
	cli.StringFlag{
		Name:      "logging.directory",
		TakesFile: true,
		Value: func() string {
			dir, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			return filepath.Join(dir, "logs")
		}(),
		Usage: "What directory to write log files to",
	},
}
