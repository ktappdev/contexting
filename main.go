package main

import (
	"os"
)

var version = "0.0.1"

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		logErrorf("%v", err)
		os.Exit(1)
	}
}
