package main

import (
	"os"
)

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		logErrorf("%v", err)
		os.Exit(1)
	}
}
