package contexting

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWriterGuard(t *testing.T) {
	root := t.TempDir()
	release, err := acquireWriterGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := acquireWriterGuard(root); err == nil {
		other()
		t.Fatal("second writer acquired guard")
	}
	release()
	release, err = acquireWriterGuard(root)
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestIndexSchemaCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.json")
	for _, version := range []int{0, CurrentIndexSchema, CurrentIndexSchema + 1} {
		data, err := json.Marshal(ContextIndex{SchemaVersion: version, Tree: &Node{Type: "directory"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		index, err := LoadContextIndex(path)
		if version > CurrentIndexSchema {
			if err == nil {
				t.Fatal("accepted future schema")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := SaveContextIndex(path, index); err != nil {
			t.Fatal(err)
		}
		index, err = LoadContextIndex(path)
		if err != nil || index.SchemaVersion != CurrentIndexSchema {
			t.Fatalf("migration: %v, %v", index, err)
		}
	}
}

func TestDoctorCustomKeyEnvironment(t *testing.T) {
	t.Setenv("CTXT_TEST_LLM_KEY", "private-value")
	report := DoctorReport{}
	checkAPIKey(&report, CommonFlags{}, LLMConfig{APIKeyEnv: "CTXT_TEST_LLM_KEY"})
	if report.Checks[0].Status != DoctorPass {
		t.Fatal(report)
	}
	if strings.Contains(maskAPIKey("private-value"), "private") {
		t.Fatal("key leaked")
	}
	if got := endpointForLog("https://user:secret@example.com/v1?token=private"); got != "https://example.com" {
		t.Fatalf("endpoint log sanitization: %s", got)
	}
}

func TestWatchParentSynonymsAndPopulatedDirectory(t *testing.T) {
	root := t.TempDir()
	m := NewIndexManager(IndexManagerOptions{RootPath: root, OutputPath: filepath.Join(root, ".ctxt", "index.json"), CachePath: filepath.Join(root, ".ctxt", "cache.json")})
	if _, err := m.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.cache["billing/route.go"] = []string{"invoicing"}
	m.cache["account/route.go"] = []string{"authentication"}
	for _, dir := range []string{"billing", "account"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "route.go"), []byte("package demo\nfunc Handler() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.ApplyChanges(context.Background(), map[string]fsnotify.Op{"billing": fsnotify.Create, "account": fsnotify.Create}); err != nil {
		t.Fatal(err)
	}
	for dir, word := range map[string]string{"billing": "invoicing", "account": "authentication"} {
		node := m.index.Tree.Children[dir].Children["route.go"]
		if node == nil || !strings.Contains(strings.Join(node.Synonyms, " "), word) {
			t.Fatalf("%s synonyms: %v", dir, node)
		}
	}
}

func TestWatchSearchRemainsAvailableDuringLLMRequest(t *testing.T) {
	root := t.TempDir()
	started := make(chan struct{})
	release := make(chan struct{})
	key := filepath.Base(root) + "/new.go"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		content, _ := json.Marshal(SynonymResponse{key: {"concept"}})
		_ = json.NewEncoder(w).Encode(OpenRouterResponse{Choices: []Choice{{Message: Message{Content: string(content)}}}})
	}))
	defer server.Close()
	m := NewIndexManager(IndexManagerOptions{
		RootPath: root, OutputPath: filepath.Join(root, ".ctxt", "index.json"),
		CachePath: filepath.Join(root, ".ctxt", "cache.json"), APIKey: "test",
		UseLLM: true, Endpoint: server.URL,
	})
	if _, err := m.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package demo\nfunc NewThing() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := m.ApplyChanges(context.Background(), map[string]fsnotify.Op{"new.go": fsnotify.Create})
		done <- err
	}()
	<-started
	searchDone := make(chan struct{})
	go func() {
		_ = m.Search("NewThing", SearchOptions{Limit: 5})
		close(searchDone)
	}()
	select {
	case <-searchDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("search blocked on LLM request")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestLLMFailuresAndCancellation(t *testing.T) {
	for _, endpoint := range []string{"http://example.com/v1", "ftp://example.com/v1", "https://user:secret@example.com/v1"} {
		t.Run("reject endpoint "+endpoint, func(t *testing.T) {
			if _, err := GenerateSynonymsBatchWithContext(context.Background(), []string{"one"}, "", "test", endpoint, 0, 0, 1, 5, nil, nil); err == nil {
				t.Fatal("accepted unsafe endpoint")
			}
		})
	}
	t.Run("retry transient status", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(503)
				return
			}
			_ = json.NewEncoder(w).Encode(OpenRouterResponse{Choices: []Choice{{Message: Message{Content: `{"billing":["invoicing"]}`}}}})
		}))
		defer server.Close()
		got, err := GenerateSynonymsBatchWithContext(context.Background(), []string{"billing"}, "", "test", server.URL, 0, 0, 1, 5, nil, nil)
		if err != nil || len(got["billing"]) == 0 || calls.Load() != 2 {
			t.Fatalf("result=%v calls=%d err=%v", got, calls.Load(), err)
		}
	})
	for _, payload := range []string{"not-json", `{"choices":[]}`, `{"choices":[{"message":{"content":"not-json"}}]}`} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(payload)) }))
			defer server.Close()
			if _, err := GenerateSynonymsBatchWithContext(context.Background(), []string{"billing"}, "", "test", server.URL, 0, 0, 1, 5, nil, nil); err == nil {
				t.Fatal("accepted malformed/empty response")
			}
		})
	}
	t.Run("cancel retry wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503); cancel() }))
		defer server.Close()
		start := time.Now()
		_, err := GenerateSynonymsForNamesWithContext(ctx, []string{"one", "two"}, "", 1, "test", server.URL, 0, 0, 1, 5, 1, nil, nil)
		if !errors.Is(err, context.Canceled) || time.Since(start) > time.Second {
			t.Fatalf("cancellation: %v", err)
		}
	})
	t.Run("request deadline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(200 * time.Millisecond):
			}
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		_, err := GenerateSynonymsBatchWithContext(ctx, []string{"one"}, "", "test", server.URL, 0, 0, 1, 5, nil, nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline: %v", err)
		}
	})
	t.Run("partial batches", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(401)
				return
			}
			_ = json.NewEncoder(w).Encode(OpenRouterResponse{Choices: []Choice{{Message: Message{Content: `{"one":["alpha"],"two":["beta"]}`}}}})
		}))
		defer server.Close()
		got, err := GenerateSynonymsForNamesWithContext(context.Background(), []string{"one", "two"}, "", 1, "test", server.URL, 0, 0, 1, 5, 1, nil, nil)
		if err == nil || len(got) != 1 || calls.Load() != 2 {
			t.Fatalf("partial result=%v err=%v calls=%d", got, err, calls.Load())
		}
	})
}
