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

func TestParsePersistMode(t *testing.T) {
	mode, err := parsePersistMode("shutdown")
	if err != nil || mode != PersistShutdown {
		t.Fatalf("expected shutdown mode, got mode=%q err=%v", mode, err)
	}

	mode, err = parsePersistMode("interval")
	if err != nil || mode != PersistInterval {
		t.Fatalf("expected interval mode, got mode=%q err=%v", mode, err)
	}

	mode, err = parsePersistMode("change")
	if err != nil || mode != PersistChange {
		t.Fatalf("expected change mode, got mode=%q err=%v", mode, err)
	}

	if _, err := parsePersistMode("bad-mode"); err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}
