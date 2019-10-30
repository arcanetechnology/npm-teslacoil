package main

import (
	"fmt"
	"os"

	tlc "gitlab.com/arcanecrypto/teslacoil"
)

func main() {
	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	if err := tlc.Main(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
