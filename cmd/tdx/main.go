package main

import (
	"fmt"
	"os"

	"github.com/ipm/tdx/internal/cli"
)

// version is overridden at build time with -ldflags "-X main.version=..."
var version = "0.1.0-dev"

func main() {
	root := cli.NewRootCmd(version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tdx:", err)
		os.Exit(1)
	}
}
