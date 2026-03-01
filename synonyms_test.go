package main

import "testing"

func TestLexicalSynonymsSplitsIdentifiers(t *testing.T) {
	syns := lexicalSynonyms("localStore_config.go")
	want := map[string]bool{"local": true, "store": true, "config": true}
	for token := range want {
		found := false
		for _, syn := range syns {
			if syn == token {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected token %q in lexical synonyms: %v", token, syns)
		}
	}
}

func TestSanitizeSynonymsFiltersGenericAndCaps(t *testing.T) {
	input := []string{"file", "Data", "local", "cache", "local", "x", "storage", "folder", "state"}
	out := sanitizeSynonyms(input, 3)

	if len(out) != 3 {
		t.Fatalf("expected capped output of 3, got %d (%v)", len(out), out)
	}
	for _, value := range out {
		if value == "file" || value == "folder" || value == "data" || len(value) <= 1 {
			t.Fatalf("unexpected unfiltered value %q in output %v", value, out)
		}
	}
}
