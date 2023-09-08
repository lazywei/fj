package main

import (
	"os"

	"github.com/lazywei/fj/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
