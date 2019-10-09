package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	_ "github.com/lib/pq" // Import postgres
	"github.com/lightningnetwork/lnd/lnrpc"
	pkgerrors "github.com/pkg/errors"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/asyncutil"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/util"
	"gopkg.in/urfave/cli.v1"
)

var (
	log = build.Log

	// DatabaseName is the database being used to run the API
	DatabaseName string
	// DatabaseUser is the user being used to run the API
	DatabaseUser string
	// DatabaseHost is the host we connect to to run the API
	DatabaseHost string
	// DatabasePassword is the password we use while running
	// the API
	DatabasePassword string
	// GinMode is the mode server is in, either debug or release
	// Set to debug by default
	GinMode string

	// DatabasePort is the port we use to connect to the database
	DatabasePort = util.GetDatabasePort()

	// SendgridApiKey is the API key we use to interact with Sendgrid's servers
	SendgridApiKey string

	// lnConfig is the configuration we use to connect to LND, read from CLI
	// parameters
	lnConfig ln.LightningConfig

	// bitcoindConfig is the configuration we use to connect to bitcoind, read
	// from CLI parameters
	bitcoindConfig bitcoind.Config

	// network is the network our application runs on
	network chaincfg.Params
)

type realHttpSender struct{}

func (s realHttpSender) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	return http.Post(url, contentType, reader)
}

func init() {

	DatabaseUser = util.GetEnvOrFail("DATABASE_USER")
	DatabasePassword = util.GetEnvOrFail("DATABASE_PASSWORD")
	DatabaseName = util.GetEnvOrFail("DATABASE_NAME")
	DatabaseHost = util.GetEnvOrElse("DATABASE_HOST", "localhost")
	GinMode = util.GetEnvOrElse("GIN_MODE", "debug")
	SendgridApiKey = util.GetEnvOrFail("SENDGRID_API_KEY")

	databaseConfig = db.DatabaseConfig{
		User:     DatabaseUser,
		Password: DatabasePassword,
		Host:     DatabaseHost,
		Port:     DatabasePort,
		Name:     DatabaseName,
	}

}

const (
	rpcAwaitAttempts = 5
	rpcAwaitDuration = time.Second
)

// awaitBitcoind tries to get a RPC response from bitcoind, returning an error
// if that isn't possible within a set of attempts
func awaitBitcoind(btc *bitcoind.Conn) error {
	retry := func() bool {
		_, err := btc.Btcctl().GetBlockChainInfo()
		return err == nil
	}
	return asyncutil.Await(rpcAwaitAttempts, rpcAwaitDuration, retry, "couldn't reach bitcoind")
}

// awaitLnd tries to get a RPC response from lnd, returning an error
// if that isn't possible within a set of attempts
func awaitLnd(lncli lnrpc.LightningClient) error {
	retry := func() bool {
		_, err := lncli.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		return err == nil
	}
	return asyncutil.Await(rpcAwaitAttempts, rpcAwaitDuration, retry, "couldn't reach lnd")
}

// awaitLndMacaroonFile waits for the creation of the macaroon file in the given
// configuration
func awaitLndMacaroonFile(config ln.LightningConfig) error {
	macaroon := config.MacaroonPath
	if macaroon == "" {
		macaroon = path.Join(config.LndDir,
			ln.DefaultRelativeMacaroonPath(config.Network))
	}
	retry := func() bool {
		_, err := os.Stat(macaroon)
		return err == nil
	}
	return asyncutil.Await(rpcAwaitAttempts, rpcAwaitDuration,
		retry, fmt.Sprintf("couldn't read macaroon file %q", macaroon))
}

// checkBitcoindConfig verifies that the given configuration has at least the
// fields that we need to connect
func checkBitcoindConfig(conf bitcoind.Config) error {
	if conf.Password == "" {
		return errors.New("config.Bitcoind.Password is not set")
	}
	if conf.User == "" {
		return errors.New("config.Bitcoind.User is not set")
	}
	if conf.RpcPort == 0 {
		return errors.New("config.Bitcoind.RpcPort is not set")
	}
	if conf.ZmqPubRawTx == "" {
		return errors.New("config.Bitcoind.ZmqPubRawTx is not set")
	}
	if conf.ZmqPubRawBlock == "" {
		return errors.New("config.Bitcoind.ZmqPubRawBlock is not set")
	}
	return nil
}

var (
	databaseConfig db.DatabaseConfig
	serveCommand   = cli.Command{
		Name:  "serve",
		Usage: "Starts the lightning payment processing api",
		Before: func(c *cli.Context) error {

			bitcoindConfig = bitcoind.Config{
				ZmqPubRawTx:    c.GlobalString("zmqpubrawtx"),
				ZmqPubRawBlock: c.GlobalString("zmqpubrawblock"),
				RpcPort:        c.GlobalInt("bitcoind.rpcport"),
				Password:       c.GlobalString("bitcoind.rpcpassword"),
				User:           c.GlobalString("bitcoind.rpcuser"),
				Network:        network,
				RpcHost:        c.GlobalString("bitcoind.rpchost"),
			}

			if bitcoindConfig.RpcPort == 0 {
				log.Debug("bitcoind.rpcport flag is not set, falling back to network default")
				port, err := bitcoind.DefaultRpcPort(network)
				if err != nil {
					return err
				}
				bitcoindConfig.RpcPort = port
			}

			if bitcoindConfig.ZmqPubRawTx == "" {
				return errors.New("zmqpubrawtx flag is not set")
			}

			if bitcoindConfig.ZmqPubRawBlock == "" {
				return errors.New("zmqpubrawblock flag is not set")
			}
			return nil
		},
		Action: func(c *cli.Context) error {

			database, err := db.Open(databaseConfig)
			if err != nil {
				return err
			}

			defer func() { err = database.Close() }()

			if err := checkBitcoindConfig(bitcoindConfig); err != nil {
				return err
			}

			bitcoindConn, err := bitcoind.NewConn(bitcoindConfig, 1*time.Second)
			if err != nil {
				return err
			}

			log.Info("opened connection to bitcoind")
			if err := awaitBitcoind(bitcoindConn); err != nil {
				return err
			}
			log.Info("bitcoind is properly started")

			if err := awaitLndMacaroonFile(lnConfig); err != nil {
				return err
			}

			lncli, err := ln.NewLNDClient(lnConfig)
			if err != nil {
				return err
			}
			if err := awaitLnd(lncli); err != nil {
				return err
			}
			log.Info("lnd is properly started")

			sendGridClient := sendgrid.NewSendClient(SendgridApiKey)

			config := api.Config{
				LogLevel: build.Log.Level,
				Network:  network,
			}
			a, err := api.NewApp(database, lncli, sendGridClient,
				bitcoindConn, realHttpSender{}, config)
			if err != nil {
				return err
			}

			log.Info("opened connection to bitcoind")

			address := ":" + c.String("port")
			if GinMode == "release" {
				// Set up to run TLS on lightningspin using certs
				// generated by certbot
				err = a.Router.RunTLS(address,
					"/etc/letsencrypt/live/api.teslacoil.io/fullchain.pem",
					"/etc/letsencrypt/live/api.teslacoil.io/privkey.pem")
			} else {
				err = a.Router.Run(address)
			}

			return err
		},

		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "port",
				Value: "5000",
				Usage: "Port number to listen on",
			},
		},
	}
	dbCommand = cli.Command{
		Name:    "db",
		Aliases: []string{"db"},
		Usage:   "Database related commands",
		Subcommands: []cli.Command{
			{
				Name:    "down",
				Aliases: []string{"md"},
				Usage:   "down x, migrates the database down x number of steps",
				Action: func(c *cli.Context) error {
					if c.NArg() != 1 {
						return cli.NewExitError(
							"You need to specify a number of steps to migrate down",
							22,
						)
					}
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()
					steps, err := strconv.Atoi(c.Args().First())
					if err != nil {
						return err
					}
					err = database.MigrateDown(
						path.Join("file://", db.MigrationsPath), steps)

					return err
				},
			},
			{
				Name:    "up",
				Aliases: []string{"mu"},
				Usage:   "migrates the database up",
				Action: func(c *cli.Context) (err error) {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()

					err = database.MigrateUp(
						path.Join("file://", db.MigrationsPath))

					return
				},
			}, {
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "check migrations status and version number",
				Action: func(c *cli.Context) error {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()

					err = database.MigrationStatus(
						path.Join("file://", db.MigrationsPath))
					return err
				},
			}, {
				Name:    "newmigration",
				Aliases: []string{"nm"},
				Usage:   "newmigration `NAME`, creates new migration file",
				Action: func(c *cli.Context) error {

					migrationText := c.Args().First() // get the filename
					if migrationText == "" {
						// What's the best way of handling this error? This way
						// doesn't lead to pretty console output
						return errors.New("you must provide a file name for the migration")
					}

					return db.CreateMigration(db.MigrationsPath, migrationText)
				},
			}, {
				Name:    "drop",
				Aliases: []string{"dr"},
				Usage:   "drops the entire database.",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "force",
						Usage: "Don't ask for confirmation before dropping the DB",
					},
				},
				Action: func(c *cli.Context) (err error) {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()

					force := c.Bool("force")
					if !force {
						fmt.Println(
							"Are you sure you want to drop the entire database? y/n")
						if !askForConfirmation() {
							log.Debug("Not dropping DB")
							return nil
						}
					}
					err = database.Drop(
						path.Join("file://", db.MigrationsPath))
					if err != nil {
						log.WithError(err).Error("Could not drop DB")
						return err
					}

					log.Info("Dropped DB")
					return
				},
			},
			{
				Name:    "dummy",
				Aliases: []string{"dd"},
				Usage:   "fills the database with dummy data",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "force",
						Usage: "Don't ask for confirmation before populating the DB",
					},
					cli.BoolFlag{
						Name:  "only-once",
						Usage: "Only fill with dummy data if DB is empty",
					},
				},
				Action: func(c *cli.Context) error {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()
					force := c.Bool("force")
					if !force {
						fmt.Println("Are you sure you want to fill dummy data? y/n")
						if !askForConfirmation() {
							log.Info("Not populating DB with dummy data")
							return nil
						}
					}

					lncli, err := ln.NewLNDClient(lnConfig)
					if err != nil {
						return pkgerrors.Wrap(err, "could not connect to lnd")
					}
					return FillWithDummyData(database, lncli, c.Bool("only-once"))
				},
			},
		},
	}
)

// You might want to put the following two functions in a separate utility
// package.
// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

func askForConfirmation() bool {
	var response string
	_, err := fmt.Scan(&response)
	if err != nil {
		log.Fatal(err)
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return askForConfirmation()
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "lpp"
	app.Usage = "Managing helper for developing lightning payment processor"
	app.EnableBashCompletion = true
	// have log levels be set for all commands/subcommands
	app.Before = func(c *cli.Context) error {
		level, err := build.ToLogLevel(c.GlobalString("loglevel"))
		if err != nil {
			return err
		}
		build.SetLogLevel(level)
		log.Info("Setting log level to " + level.String())

		networkString := c.GlobalString("network")
		switch networkString {
		case "mainnet":
			network = chaincfg.MainNetParams
		case "testnet", "testnet3":
			network = chaincfg.TestNet3Params
		case "regtest", "":
			network = chaincfg.RegressionNetParams
		default:
			return fmt.Errorf("unknown network: %s. Valid: mainnet, testnet, regtest", networkString)
		}

		lnConfig = ln.LightningConfig{
			LndDir:       c.GlobalString("lnddir"),
			TLSCertPath:  c.GlobalString("tlscertpath"),
			MacaroonPath: c.GlobalString("macaroonpath"),
			Network:      network,
			RPCServer:    c.GlobalString("lndrpcserver"),
		}

		return nil
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "lnddir",
			Value: ln.DefaultLndDir,
			Usage: "path to lnd's base directory",
		},
		cli.StringFlag{
			Name:  "zmqpubrawblock",
			Usage: "The address listening for ZMQ connections to deliver raw block notifications",
		},
		cli.StringFlag{
			Name:  "zmqpubrawtx",
			Usage: "The address listening for ZMQ connections to deliver raw transaction notifications",
		},
		cli.StringFlag{
			Name:  "bitcoind.rpcuser",
			Usage: "The bitcoind RPC username",
		},
		cli.StringFlag{
			Name:  "bitcoind.rpcpassword",
			Usage: "The bitcoind RPC password",
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
		cli.StringFlag{
			Name:  "tlscertpath",
			Usage: "path to TLS ceritiface(tls.cert)",
		},
		cli.StringFlag{
			Name:  "macaroonpath",
			Usage: "path to macaroon folder",
		},
		cli.StringFlag{
			Name:  "network",
			Usage: "the network lnd is running on e.g. mainnet, testnet, etc.",
		},
		cli.StringFlag{
			Name:  "lndrpcserver",
			Value: ln.DefaultRpcServer,
			Usage: "host:port of ln daemon",
		},
		cli.StringFlag{
			Name:  "loglevel",
			Value: logrus.InfoLevel.String(),
			Usage: "Logging level for all subsystems {trace, debug, info, warn, error, fatal, panic}",
		},
	}

	app.Commands = []cli.Command{
		serveCommand,
		dbCommand,
	}

	sort.Sort(cli.CommandsByName(app.Commands))
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
