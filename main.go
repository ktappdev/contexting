package main

import (
	"os"
)

var version = "dev"

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		logErrorf("%v", err)
		os.Exit(1)
	}
}
