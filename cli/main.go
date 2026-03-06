package main

import (
	"os"

	"github.com/workhubonline-soft/aistack/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
