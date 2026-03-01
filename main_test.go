package main

import "testing"

func TestRootCommandHasExpectedSubcommands(t *testing.T) {
	root := NewRootCommand()
	if root == nil {
		t.Fatalf("expected root command")
	}

	names := map[string]bool{}
	for _, cmd := range root.Commands() {
		names[cmd.Name()] = true
	}

	for _, expected := range []string{"init", "watch", "search-hints", "eval", "doctor", "config"} {
		if !names[expected] {
			t.Fatalf("expected subcommand %q", expected)
		}
	}
}
