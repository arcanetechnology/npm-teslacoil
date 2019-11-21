package main

import (
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq" // Import postgres
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/ztrue/shutdown"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/actions"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/flags"
)

var log = build.AddSubLogger("MAIN")

func main() { //nolint:deadcode,unused

	app := cli.NewApp()
	app.Name = "tlc"
	app.Version = build.Version()
	app.Usage = "Managing helper for developing lightning payment processor"
	app.EnableBashCompletion = true
	// have log levels be set for all commands/subcommands
	app.Before = func(c *cli.Context) error {
		level, err := build.ToLogLevel(c.GlobalString("logging.level"))
		if err != nil {
			return err
		}
		existingLevel := log.Level
		if existingLevel != level {
			build.SetLogLevels(level)
		}

		logFile := c.GlobalString("logging.directory")
		if err = build.SetLogDir(logFile); err != nil {
			return err
		}

		log.WithFields(logrus.Fields{
			"version": app.Version,
		}).Info("starting tlc")
		return nil
	}

	app.Flags = flags.CommonFlags
	app.Commands = []cli.Command{
		actions.Db(),
		actions.Serve(),
		{
			Name:  "fish-completion",
			Usage: "Generate fish shell completion",
			Action: func(c *cli.Context) error {
				// to make this pipeable to `source`, we don't want any other
				// output
				build.SetLogLevels(logrus.FatalLevel)

				completion, err := app.ToFishCompletion()
				if err != nil {
					return err
				}

				// prevent auto complete from suggesting files
				completion = fmt.Sprintf("complete -c %q -f \n", c.App.Name) + completion
				fmt.Println(completion)
				return nil
			},
		},
	}

	shutdown.AddWithParam(func(signal os.Signal) {
		log.WithFields(logrus.Fields{
			"signal": signal.String(),
			"ranFor": time.Since(start),
		}).Info("Shutting down tlc")
	})

	// we start the "real" main method in a goroutine, because the signal listening is a blocking call
	// that needs to be run in main
	go realMain(app)

	shutdown.Listen() // passing in no params means listening for all signal
}

var start time.Time

func realMain(app *cli.App) {
	start = time.Now()
	err := app.Run(os.Args)
	if err != nil {
		// only print error if something was supplied to tlc, help
		// message is printed anyways
		if len(os.Args) > 1 {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
	// we need to explicitly exit, otherwise we'll hang forever waiting for a signal
	os.Exit(0)
}
