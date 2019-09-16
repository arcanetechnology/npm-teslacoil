package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"

	_ "github.com/lib/pq" // Import postgres
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/util"
	"gopkg.in/urfave/cli.v1"
)

const (
	defaultLoggingLevel = "trace"
)

var (
	// DatabaseName is the database being used to run the API
	DatabaseName string
	// DatabaseUser is the user being used to run the API
	DatabaseUser string
	// DatabaseHost is the host we connect to to run the API
	DatabaseHost string
	// DatabasePassword is the password we use while running
	// the API
	DatabasePassword string

	// DatabasePort is the port we use to connect to the database
	DatabasePort = util.GetDatabasePort()
)

func init() {
	log = logrus.New().WithFields(logrus.Fields{
		"package": "main",
	})

	DatabaseUser = util.GetEnvOrFail("DATABASE_USER")
	DatabasePassword = util.GetEnvOrFail("DATABASE_PASSWORD")
	DatabaseHost = util.GetEnvOrElse("DATABASE_HOST", "localhost")

	databaseConfig = db.DatabaseConfig{
		User:     DatabaseUser,
		Password: DatabasePassword,
		Host:     DatabaseHost,
		Port:     DatabasePort,
		Name:     DatabaseName,
	}
}

var (
	log            *logrus.Entry
	databaseConfig db.DatabaseConfig
	serveCommand   = cli.Command{
		Name:  "serve",
		Usage: "Starts the lightning payment processing api",
		Action: func(c *cli.Context) error {
			database, err := db.Open(databaseConfig)
			if err != nil {
				log.Fatal(err)
				return err
			}

			defer func() { err = database.Close() }()

			lnConfig := ln.LightningConfig{
				LndDir:       c.GlobalString("lnddir"),
				TLSCertPath:  c.GlobalString("tlscertpath"),
				MacaroonPath: c.GlobalString("macaroonpath"),
				Network:      c.GlobalString("network"),
				RPCServer:    c.GlobalString("lndrpcserver"),
			}

			config := api.Config{
				LogLevel: build.Log.Level,
			}

			lncli, err := ln.NewLNDClient(lnConfig)
			if err != nil {
				log.Fatal(err)
				return err
			}
			a, err := api.NewApp(database, lncli, config)
			if err != nil {
				log.Fatal(err)
				return err
			}

			address := ":" + c.String("port")
			err = a.Router.Run(address)

			return err
		},

		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "port",
				Value: "8080",
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
				Action: func(c *cli.Context) error {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()

					err = database.MigrateUp(
						path.Join("file://", db.MigrationsPath))
					return err
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
				Action: func(c *cli.Context) error {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()

					fmt.Println(
						"Are you sure you want to drop the entire database? y/n")
					if askForConfirmation() {
						err = database.Drop(
							path.Join("file://", db.MigrationsPath))
					}

					return err
				},
			},
			{
				Name:    "dummy",
				Aliases: []string{"dd"},
				Usage:   "fills the database with dummy data",
				Action: func(c *cli.Context) error {
					database, err := db.Open(databaseConfig)
					if err != nil {
						return err
					}
					defer func() { err = database.Close() }()
					fmt.Println("Are you sure you want to fill dummy data? y/n")
					if askForConfirmation() {
						lnConfig := ln.LightningConfig{
							LndDir:       c.GlobalString("lnddir"),
							TLSCertPath:  c.GlobalString("tlscertpath"),
							MacaroonPath: c.GlobalString("macaroonpath"),
							Network:      c.GlobalString("network"),
							RPCServer:    c.GlobalString("lndrpcserver"),
						}
						lncli, err := ln.NewLNDClient(lnConfig)
						if err != nil {
							log.Fatalf("Could not connect to LND. ln.NewLNDClient(%+v): %+v", lnConfig, err)
						}
						return FillWithDummyData(database, lncli)
					}
					return err
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
			log.Fatal(err)
			return err
		}
		build.SetLogLevel(level)
		return nil
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "lnddir",
			Value: ln.DefaultLndDir,
			Usage: "path to lnd's base directory",
		},
		cli.StringFlag{
			Name:  "tlscertpath",
			Value: ln.DefaultTLSCertPath,
			Usage: "path to TLS ceritiface(tls.cert)",
		},
		cli.StringFlag{
			Name:  "macaroonpath",
			Value: ln.DefaultMacaroonPath,
			Usage: "path to macaroon folder",
		},
		cli.StringFlag{
			Name:  "network",
			Value: ln.DefaultNetwork,
			Usage: "the network lnd is running on e.g. mainnet, testnet, etc.",
		},
		cli.StringFlag{
			Name:  "lndrpcserver",
			Value: ln.DefaultRPCHostPort,
			Usage: "host:port of ln daemon",
		},
		cli.StringFlag{
			Name:  "loglevel",
			Value: defaultLoggingLevel,
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
