package main

import (
	"os"

	"github.com/yhotamos/cli2http/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
