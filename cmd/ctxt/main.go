package main

import (
	"os"
	"github.com/ktappdev/contexting"
)

func main() {
	if err := contexting.NewRootCommand().Execute(); err != nil {
		contexting.LogErrorf("%v", err)
		os.Exit(1)
	}
}
