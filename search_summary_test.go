package main

import "testing"

func TestSummarizeDirectories(t *testing.T) {
	results := []SearchResult{
		{Path: "src/routes/api.go", Type: "file", Score: 12, Matches: []string{"path:routes", "basename:api"}},
		{Path: "src/routes/auth.go", Type: "file", Score: 10, Matches: []string{"path:routes", "basename:auth"}},
		{Path: "docs/routing.md", Type: "file", Score: 7, Matches: []string{"path:routing"}},
	}

	summaries := SummarizeDirectories(results, 5, 2)
	if len(summaries) != 2 {
		t.Fatalf("expected 2 directory summaries, got %d", len(summaries))
	}
	if summaries[0].Path != "src/routes" {
		t.Fatalf("expected src/routes first, got %s", summaries[0].Path)
	}
	if summaries[0].MatchCount != 2 {
		t.Fatalf("expected 2 matches in src/routes, got %d", summaries[0].MatchCount)
	}
	if len(summaries[0].TopMatches) != 2 {
		t.Fatalf("expected drill-down top matches, got %d", len(summaries[0].TopMatches))
	}
	if summaries[0].Rationale == "" {
		t.Fatalf("expected non-empty rationale")
	}
}

func TestSummarizeDirectoriesDrillLimit(t *testing.T) {
	results := []SearchResult{
		{Path: "src/a.go", Type: "file", Score: 9, Matches: []string{"path:src"}},
		{Path: "src/b.go", Type: "file", Score: 8, Matches: []string{"path:src"}},
		{Path: "src/c.go", Type: "file", Score: 7, Matches: []string{"path:src"}},
	}
	summaries := SummarizeDirectories(results, 5, 1)
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if len(summaries[0].TopMatches) != 1 {
		t.Fatalf("expected drill limit 1, got %d", len(summaries[0].TopMatches))
	}
}
