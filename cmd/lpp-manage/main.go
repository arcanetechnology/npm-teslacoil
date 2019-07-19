package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	_ "github.com/lib/pq" // Import postgres
	"gopkg.in/urfave/cli.v1"
)

func askForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
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
	app.Name = "lpp-manage"
	app.Usage = "Manageing helper for developing lightning payment processor"
	app.EnableBashCompletion = true
	app.Commands = []cli.Command{
		{
			Name:    "db",
			Aliases: []string{"db"},
			Usage:   "Database related commands",
			Subcommands: []cli.Command{
				{
					Name:    "dummy",
					Aliases: []string{"d"},
					Usage:   "Fills the database with dummy data",
					Action: func(c *cli.Context) error {
						fmt.Println("Just filled the database with dummy data")
						return nil
					},
				},
				{
					Name:    "reset",
					Aliases: []string{"r"},
					Usage:   "Resets the database",
					Action:  ResetDatabase,
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
