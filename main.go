package main

import (
	"github.com/cpacia/atomicswap/cmd"
	"github.com/jessevdk/go-flags"
	"os"
)

func main() {
	parser := flags.NewParser(nil, flags.Default)
	parser.AddCommand("start",
		"start the app",
		"The start command starts the app and connects to the p2p network",
		&cmd.Start{})
	if _, err := parser.Parse(); err != nil {
		os.Exit(1)
	}
}
