package main

import (
	"log"
	"os"

	_ "github.com/lib/pq" // Import postgres
	"gitlab.com/arcanecrypto/lpp/cmd/lpp-serve/handlers"
	"gopkg.in/urfave/cli.v1"
)

// var rootCmd = &cobra.Command{
// 	Use:   "lpp",
// 	Short: "Lightning Payment Gateway",
// 	Long:  `Lightning Payment Gateway is responsible for custodial lightning services`,
// 	Run: func(cmd *cobra.Command, args []string) {
// 		fmt.Println("testing is")
// 	},
// }

func main() {
	app := cli.NewApp()
	app.Name = "lpp-serve"
	app.Usage = "Starts the lightning payment processing api"
	app.Action = func(c *cli.Context) error {
		app := handlers.NewApp()
		app.Run()
		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
