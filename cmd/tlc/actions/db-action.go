// Package actions provides actions that the Teslacoil CLI can execute
package actions

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/urfave/cli"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/flags"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

var log = build.AddSubLogger("ACTN")

// Db returns commands for handling DB access and migrations
func Db() cli.Command {
	return cli.Command{
		Name:  "db",
		Usage: "Database related commands",
		Flags: flags.Db,
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
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()
					steps, err := strconv.Atoi(c.Args().First())
					if err != nil {
						return err
					}
					err = database.MigrateDown(steps)

					return err
				},
			},
			{
				Name:    "listversions",
				Aliases: []string{"lv"},
				Usage:   "listversion lists all the migration versions with their description",
				Action: func(c *cli.Context) error {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()
					files := database.ListVersions()

					var s []string
					for _, file := range files {
						s = append(s, fmt.Sprintf("version: %d - %s", file.Version, file.Description))
					}

					fmt.Println(strings.Join(s, "\n"))
					return nil
				},
			},
			{
				Name:    "forceversion",
				Aliases: []string{"fv"},
				Usage:   "forceversion forces the database version, and resets the dirty state to false",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:     "version",
						Required: true,
						Usage:    "version number you know the database is currently at",
					},
				},
				Action: func(c *cli.Context) error {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()
					version := c.Int("version")

					err = database.ForceVersion(version)
					if err != nil {
						return err
					}
					log.WithField("version", version).Info("forced database version")
					return nil
				},
			},
			{
				Name:    "migrateto",
				Aliases: []string{"mt"},
				Usage:   "migrateto looks at the currently active migration version, then migrates either up or down to the specified version",
				Flags: []cli.Flag{
					cli.IntFlag{
						Name:     "version",
						Required: true,
						Usage:    "version to migrate to",
					},
				},
				Action: func(c *cli.Context) error {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()
					version := c.Uint("version")

					err = database.MigrateToVersion(version)
					if err != nil {
						return err
					}
					log.WithField("version", version).Info("forced database version")
					return nil
				},
			},
			{
				Name:    "up",
				Aliases: []string{"mu"},
				Usage:   "migrates the database up",
				Action: func(c *cli.Context) (err error) {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()

					err = database.MigrateUp()

					return
				},
			},
			{
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "check migrations status and version number",
				Action: func(c *cli.Context) error {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()

					status, err := database.MigrationStatus()
					if err != nil {
						return err
					}

					fmt.Printf("migration version: %d dirty: %t\n", status.Version, status.Dirty)
					return nil
				},
			},
			{
				Name:    "newmigration",
				Aliases: []string{"nm"},
				Usage:   "newmigration `NAME`, creates new migration file",
				Action: func(c *cli.Context) (err error) {
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()
					migrationText := c.Args().First() // get the filename
					if migrationText == "" {
						// What's the best way of handling this error? This way
						// doesn't lead to pretty console output
						return errors.New("you must provide a file name for the migration")
					}

					migration, err := database.CreateMigration(migrationText)
					if err != nil {
						return err
					}
					fmt.Printf("created migration %s\n", migration)
					return nil
				},
			},
			{
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
					conf := flags.ReadDbConf(c)
					database, err := db.Open(conf)
					if err != nil {
						return err
					}
					defer func() {
						if dbErr := database.Close(); dbErr != nil {
							err = dbErr
						}
					}()

					force := c.Bool("force")
					if !force {
						fmt.Println(
							"Are you sure you want to drop the entire database? y/n")
						if !askForConfirmation() {
							log.Debug("Not dropping DB")
							return nil
						}
					}
					err = database.Drop()
					if err != nil {
						log.WithError(err).Error("Could not drop DB")
						return err
					}

					log.Info("Dropped DB")
					return
				},
			},
		}}
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

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

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
