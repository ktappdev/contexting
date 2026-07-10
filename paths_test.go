package main

import (
	"path/filepath"
	"testing"
)

func TestResolveConfigPath_EmptyPath(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", "")
	if got != "" {
		t.Errorf("empty path should return empty, got %q", got)
	}
}

func TestResolveConfigPath_AbsolutePath(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", "/abs/path")
	if got != "/abs/path" {
		t.Errorf("absolute path should return as-is, got %q", got)
	}
}

func TestResolveConfigPath_EmptyConfigFile(t *testing.T) {
	got := resolveConfigPath("", "some/path")
	if got != "some/path" {
		t.Errorf("empty configFile should return path as-is, got %q", got)
	}
}

func TestResolveConfigPath_ConfigNotInDotCtx(t *testing.T) {
	got := resolveConfigPath("/root/config.toml", "sub/path")
	want := filepath.Join("/root", "sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_NoPrefix(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", "sub/path")
	want := filepath.Join("/root/.ctxt", "sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_UnixPrefix(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", ".ctxt/sub/path")
	want := filepath.Join("/root/.ctxt", "sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_BackslashPrefix(t *testing.T) {
	// Backslash paths should work on all platforms, not just Windows.
	// On Unix, filepath.Clean preserves \ as literals, so we must normalize first.
	got := resolveConfigPath("/root/.ctxt/config.toml", ".ctxt\\sub\\path")
	want := filepath.Join("/root/.ctxt", "sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_JustDotCtx(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", ".ctxt")
	want := "/root/.ctxt"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_NestedDotCtx(t *testing.T) {
	got := resolveConfigPath("/root/.ctxt/config.toml", ".ctxt/.ctxt/sub/path")
	want := filepath.Join("/root/.ctxt", ".ctxt/sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPath_ConfigInDotCtx_MixedSeparators(t *testing.T) {
	// Path containing both / and \ separators — must work on all platforms.
	// Use a literal string with both separators: .ctxt\sub/path
	// After normalization, .ctxt prefix is stripped, yielding sub/path joined with config dir.
	got := resolveConfigPath("/root/.ctxt/config.toml", ".ctxt\\sub/path")
	want := filepath.Join("/root/.ctxt", "sub/path")
	if got != want {
		t.Errorf("resolveConfigPath(.ctxt\\\\sub/path) = %q, want %q", got, want)
	}
}

func TestResolveProjectPath_EmptyPath(t *testing.T) {
	got := resolveProjectPath("/root", "")
	if got != "" {
		t.Errorf("empty path should return empty, got %q", got)
	}
}

func TestResolveProjectPath_AbsolutePath(t *testing.T) {
	got := resolveProjectPath("/root", "/abs/path")
	if got != "/abs/path" {
		t.Errorf("absolute path should return as-is, got %q", got)
	}
}

func TestResolveProjectPath_EmptyProjectRoot(t *testing.T) {
	got := resolveProjectPath("", "sub/path")
	if got != "sub/path" {
		t.Errorf("empty projectRoot should return path as-is, got %q", got)
	}
}

func TestResolveProjectPath_RelativePath(t *testing.T) {
	got := resolveProjectPath("/root", "sub/path")
	want := filepath.Join("/root", "sub/path")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
