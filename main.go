package main

import (
	"os"

	"github.com/brunoribeiro127/gobin/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
