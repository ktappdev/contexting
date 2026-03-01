package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEvaluateSearchMetrics(t *testing.T) {
	root := t.TempDir()
	index := &ContextIndex{
		RootPath:    root,
		GeneratedAt: time.Now().UTC(),
		Tree: &Node{
			FullPath: root,
			Type:     "directory",
			Children: map[string]*Node{
				"config": {
					FullPath: filepath.Join(root, "config"),
					Type:     "directory",
					Children: map[string]*Node{
						"local_store.go": {
							FullPath: filepath.Join(root, "config", "local_store.go"),
							Type:     "file",
							Synonyms: []string{"local storage", "settings"},
							Children: map[string]*Node{},
						},
					},
				},
				"auth": {
					FullPath: filepath.Join(root, "auth"),
					Type:     "directory",
					Children: map[string]*Node{},
				},
			},
		},
	}

	cases := []EvalCase{
		{Query: "check local storage", ExpectAny: []string{"config/local_store.go"}},
		{Query: "authorization", ExpectAny: []string{"auth"}},
	}

	summary, results := EvaluateSearch(index, cases, SearchOptions{Limit: 5, MinScore: 1})
	if len(results) != 2 {
		t.Fatalf("expected 2 eval results, got %d", len(results))
	}
	if summary.Cases != 2 || summary.ScoredCases != 2 {
		t.Fatalf("unexpected summary counters: %+v", summary)
	}
	if summary.Top1Hits < 1 {
		t.Fatalf("expected at least one top1 hit, got %+v", summary)
	}
	if summary.MRR <= 0 {
		t.Fatalf("expected positive MRR, got %+v", summary)
	}
}

func TestFirstMatchRank(t *testing.T) {
	results := []SearchResult{{Path: "src/main.go"}, {Path: "config/local_store.go"}}
	rank := firstMatchRank(results, []string{"config/local_store.go"})
	if rank != 2 {
		t.Fatalf("expected rank 2, got %d", rank)
	}
}
