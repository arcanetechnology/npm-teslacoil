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
	"gopkg.in/urfave/cli.v1"
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

func main() {

	app := cli.NewApp()
	app.Name = "lpp"
	app.Usage = "Manageing helper for developing lightning payment processor"
	app.EnableBashCompletion = true

	app.Commands = []cli.Command{
		{
			Name:  "serve",
			Usage: "Starts the lightning payment processing api",
			Action: func(c *cli.Context) error {
				database, err := db.OpenDatabase()
				if err != nil {
					return err
				}
				defer database.Close()
				a, err := api.NewApp(database)
				if err != nil {
					return err
				}
				a.Run()
				return nil
			},
		},
		{
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
							MigrateDown(migrationsPath, database, steps)
							return nil
						}
						return cli.NewExitError(
							"You need to spesify a number of steps to migrate down",
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
						MigrateUp(migrationsPath, database)
						return nil
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

						MigrationStatus(migrationsPath, database)
						return nil
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

						CreateMigration(migrationsPath, migrationText)
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
						fmt.Println("Are you sure you want to fill dummy data?")
						if askForConfirmation() {

						}
						return FillWithDummyData(database)
					},
				},
			},
		},
	}
	sort.Sort(cli.CommandsByName(app.Commands))
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
