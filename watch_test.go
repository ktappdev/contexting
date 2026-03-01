package main

import "testing"

func TestShouldSkipInternalOutput(t *testing.T) {
	out := "/tmp/project/context.json"
	cache := "/tmp/project/.contexting_synonyms_cache.json"

	if !shouldSkipInternalOutput(out, out, cache) {
		t.Fatalf("expected output path to be skipped")
	}
	if !shouldSkipInternalOutput(cache, out, cache) {
		t.Fatalf("expected cache path to be skipped")
	}
	if !shouldSkipInternalOutput("/tmp/project/.tmp-123.json", out, cache) {
		t.Fatalf("expected temp json path to be skipped")
	}
	if shouldSkipInternalOutput("/tmp/project/src/main.go", out, cache) {
		t.Fatalf("did not expect normal file path to be skipped")
	}
}
