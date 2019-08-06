package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"

	_ "github.com/lib/pq" // Import postgres
	"gitlab.com/arcanecrypto/lpp/cmd/lpp/api"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gopkg.in/urfave/cli.v1"
)

const (
	defaultLoggingLevel = "trace"
)

var (
	defaultLppDir      = fmt.Sprintf("%s/src/gitlab.com/arcanecrypto/lpp/logs/", os.Getenv("GOPATH"))
	defaultLogFilename = "lpp.log"
)

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

// You might want to put the following two functions in a separate utility package.

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

var (
	dbCommand = cli.Command{
		Name:    "db",
		Aliases: []string{"db"},
		Usage:   "Database related commands",
		Subcommands: []cli.Command{
			{
				Name:    "down",
				Aliases: []string{"md"},
				Usage:   "down x, Migrates the database down x number of steps",
				Action: func(c *cli.Context) error {
					if c.NArg() > 0 {
						database, err := db.OpenDatabase()
						if err != nil {
							return err
						}
						defer database.Close()
						steps, err := strconv.Atoi(c.Args().First())
						if err != nil {
							return err
						}
						_, filename, _, ok := runtime.Caller(0)
						if ok == false {
							return cli.NewExitError("Cannot find migrations folder", 22)
						}

						migrationsPath := path.Join("file://", path.Dir(filename), "/migrations")
						return MigrateDown(migrationsPath, database, steps)
					}
					return cli.NewExitError(
						"You need to specify a number of steps to migrate down",
						22,
					)
				},
			},
			{
				Name:    "up",
				Aliases: []string{"mu"},
				Usage:   "Migrates the database up",
				Action: func(c *cli.Context) error {
					database, err := db.OpenDatabase()
					if err != nil {
						return err
					}
					defer database.Close()
					_, filename, _, ok := runtime.Caller(0)
					if ok == false {
						return cli.NewExitError("Cannot find migrations folder", 22)
					}

					migrationsPath := path.Join("file://", path.Dir(filename), "/migrations")

					return MigrateUp(migrationsPath, database)
				},
			}, {
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "Check migrations status and version number",
				Action: func(c *cli.Context) error {
					database, err := db.OpenDatabase()
					if err != nil {
						return err
					}
					defer database.Close()
					_, filename, _, ok := runtime.Caller(0)
					if ok == false {
						return cli.NewExitError("Cannot find migrations folder", 22)
					}

					migrationsPath := path.Join("file://", path.Dir(filename), "/migrations")

					return MigrationStatus(migrationsPath, database)
				},
			}, {
				Name:    "newmigration",
				Aliases: []string{"nm"},
				Usage:   "Creates a new migration file",
				Action: func(c *cli.Context) error {

					_, filename, _, ok := runtime.Caller(0)
					if ok == false {
						return cli.NewExitError("Cannot find migrations folder", 22)
					}

					migrationText := c.Args().First() // get the filename
					migrationsPath := path.Join(path.Dir(filename), "/migrations")

					return CreateMigration(migrationsPath, migrationText)
				},
			}, {
				Name:    "drop",
				Aliases: []string{"dr"},
				Usage:   "Drops the entire database.",
				Action: func(c *cli.Context) error {
					database, err := db.OpenDatabase()
					if err != nil {
						return err
					}
					defer database.Close()
					_, filename, _, ok := runtime.Caller(0)
					if ok == false {
						return cli.NewExitError("Cannot find migrations folder", 22)
					}
					migrationsPath := path.Join("file://", path.Dir(filename), "/migrations")

					fmt.Println("Are you sure you want to drop the entire database? y/n")
					if askForConfirmation() {
						if err = DropDatabase(migrationsPath, database); err != nil {
							return err
						}
					}

					return nil
				},
			},
			{
				Name:    "dummy",
				Aliases: []string{"dd"},
				Usage:   "Fills the database with dummy data",
				Action: func(c *cli.Context) error {
					database, err := db.OpenDatabase()
					if err != nil {
						return err
					}
					defer database.Close()
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
							return err
						}
						return FillWithDummyData(database, lncli)
					}
					return nil
				},
			},
		},
	}
)

func main() {

	initLogRotator(ln.CleanAndExpandPath(path.Join(defaultLppDir, defaultLogFilename)), 10, 3)
	setLogLevels("info")

	app := cli.NewApp()
	app.Name = "lpp"
	app.Usage = "Managing helper for developing lightning payment processor"
	app.EnableBashCompletion = true
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
			Name:  "debuglevel",
			Value: defaultLoggingLevel,
			Usage: "Logging level for all subsystems {trace, debug, info, warn, error, critical}",
		},
	}

	app.Commands = []cli.Command{
		cli.Command{
			Name:  "serve",
			Usage: "Starts the lightning payment processing api",
			Action: func(c *cli.Context) error {
				database, err := db.OpenDatabase()
				if err != nil {
					log.Fatal(err)
					return err
				}

				config := api.Config{
					LightningConfig: ln.LightningConfig{
						LndDir:       c.GlobalString("lnddir"),
						TLSCertPath:  c.GlobalString("tlscertpath"),
						MacaroonPath: c.GlobalString("macaroonpath"),
						Network:      c.GlobalString("network"),
						RPCServer:    c.GlobalString("lndrpcserver"),
					},

					DebugLevel: c.GlobalString("debuglevel"),
				}
				defer database.Close()
				a, err := api.NewApp(database, config)
				if err != nil {
					log.Fatal(err)
					return err
				}

				address := ":" + c.String("port")
				a.Router.Run(address)
				return nil
			},

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "port",
					Value: "8080",
					Usage: "Port number to listen on",
				},
			},
		},
		dbCommand,
	}

	sort.Sort(cli.CommandsByName(app.Commands))
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
