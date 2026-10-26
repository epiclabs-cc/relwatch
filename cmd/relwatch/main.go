package main

import (
	"fmt"
	"os"

	"relwatch/internal/cli"
)

var version = "dev"

func main() {
	cmd := cli.NewRootCmd()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
