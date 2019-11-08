package main

import (
	"fmt"
	"os"

	"gitlab.com/arcanecrypto/teslacoil/build/teslalog"

	_ "github.com/lib/pq" // Import postgres
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/actions"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/flags"
)

var log = logrus.New()

func main() { //nolint:deadcode,unused
	app := cli.NewApp()
	app.Name = "tlc"
	app.Usage = "Managing helper for developing lightning payment processor"
	app.EnableBashCompletion = true
	// have log levels be set for all commands/subcommands
	app.Before = func(c *cli.Context) error {
		if c.GlobalBool("logging.disablecolors") {
			build.DisableColors()
		}

		level, err := teslalog.ToLogLevel(c.GlobalString("logging.level"))
		if err != nil {
			return err
		}
		existingLevel := log.Level
		if existingLevel != level {
			build.SetLogLevels(level)
		}

		logToFile := c.GlobalBool("logging.writetofile")
		if logToFile {
			logFile := c.GlobalString("logging.file")
			if err = build.SetLogFile(logFile); err != nil {
				return err
			}
			log.Info("Logging to file")
		}
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

	err := app.Run(os.Args)
	if err != nil {
		// only print error if something was supplied to tlc, help
		// message is printed anyways
		if len(os.Args) > 1 {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

}
