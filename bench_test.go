package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildFixtureBench(t *testing.T) (string, *ContextIndex) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/audio/batch.go":         "package audio\nfunc BatchProcess() {}",
		"src/audio/stream.go":        "package audio\nfunc Stream() {}",
		"src/search/scoring.go":      "package search\nfunc Score() {}",
		"src/cmd/register.go":      "package cmd\nfunc RegisterCommand() {}",
		"src/ignore/rules.go":        "package ignore\nfunc Match() {}",
		"node_modules/lib/index.js":  "module.exports = {}\n", // should be ignored
		".git/config":                "[core]\n",
		"README.md":                  "# bench fixture\n",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	ignored := BuildIgnoreMap(nil)
	tree, err := BuildTree(root, ignored)
	if err != nil {
		t.Fatalf("build tree: %v", err)
	}
	index := &ContextIndex{RootPath: root, Tree: tree}
	return root, index
}

func writeCasesBench(t *testing.T, cases []EvalCase) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cases.json")
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatalf("marshal cases: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write cases: %v", err)
	}
	return path
}

func TestBenchEnginesRespectIgnores(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "lib index", ExpectAny: []string{"node_modules/lib/index.js"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"find", "grep", "combined"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result set, got %d", len(out.Results))
	}
	for _, res := range out.Results[0] {
		for _, p := range res.Paths {
			if strings.Contains(p, "node_modules") {
				t.Errorf("engine %s returned ignored path %s", res.EngineName, p)
			}
		}
	}
}

func TestBenchContextingFindsExpected(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	res := out.Results[0][0]
	if res.EngineName != "ctxt" {
		t.Fatalf("expected ctxt, got %s", res.EngineName)
	}
	if !res.Found || res.Rank < 1 {
		t.Errorf("expected ctxt to find batch.go, got found=%v rank=%d", res.Found, res.Rank)
	}
}

func TestBenchFindMatchesBasename(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "scoring", ExpectAny: []string{"src/search/scoring.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"find"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	res := out.Results[0][0]
	if !res.Found {
		t.Errorf("expected find to match scoring.go")
	}
	foundPath := false
	for _, p := range res.Paths {
		if strings.HasSuffix(p, "scoring.go") {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Errorf("expected find paths to include scoring.go, got %v", res.Paths)
	}
}

func TestBenchGrepMatchesContent(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "BatchProcess", ExpectAny: []string{"src/audio/batch.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"grep"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	res := out.Results[0][0]
	if !res.Found {
		t.Errorf("expected grep to match BatchProcess")
	}
	foundPath := false
	for _, p := range res.Paths {
		if strings.HasSuffix(p, "batch.go") {
			foundPath = true
			break
		}
	}
	if !foundPath {
		t.Errorf("expected grep paths to include batch.go, got %v", res.Paths)
	}
}

func TestBenchCombinedIsSuperset(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "scoring", ExpectAny: []string{"src/search/scoring.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"find", "grep", "combined"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	if len(out.Results[0]) != 3 {
		t.Fatalf("expected 3 engines, got %d", len(out.Results[0]))
	}
	findPaths := map[string]bool{}
	grepPaths := map[string]bool{}
	combinedPaths := map[string]bool{}
	for _, res := range out.Results[0] {
		switch res.EngineName {
		case "find":
			for _, p := range res.Paths {
				findPaths[p] = true
			}
		case "grep":
			for _, p := range res.Paths {
				grepPaths[p] = true
			}
		case "combined":
			for _, p := range res.Paths {
				combinedPaths[p] = true
			}
		}
	}
	for p := range findPaths {
		if !combinedPaths[p] {
			t.Errorf("combined missing find path %s", p)
		}
	}
	for p := range grepPaths {
		if !combinedPaths[p] {
			t.Errorf("combined missing grep path %s", p)
		}
	}
}

func TestBenchEmptyQueryIsFailed(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "", ExpectAny: []string{"src/audio/batch.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt", "find"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result set, got %d", len(out.Results))
	}
	for _, res := range out.Results[0] {
		if res.Found {
			t.Errorf("empty query should not find anything in %s", res.EngineName)
		}
	}
	if len(out.Summary) == 0 {
		t.Fatalf("expected summary")
	}
	for _, s := range out.Summary {
		if s.FailedCases != 1 {
			t.Errorf("expected 1 missed case for %s, got %d", s.EngineName, s.FailedCases)
		}
	}
}

func TestBenchJSONOutput(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "register command", ExpectAny: []string{"src/cmd/register.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt", "find", "grep", "combined"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	jsonStr, err := resultsToJSONBench(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	for _, key := range []string{"cases", "results", "summary"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing top-level key %s", key)
		}
	}
}

func TestBenchLoadEvalCasesRoundTrip(t *testing.T) {
	cases := []EvalCase{
		{Query: "one", ExpectAny: []string{"a.go"}},
		{Query: "two", ExpectAny: []string{"b.go", "c.go"}},
	}
	path := writeCasesBench(t, cases)
	loaded, err := LoadEvalCases(path)
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(loaded))
	}
	if loaded[0].Query != "one" {
		t.Errorf("expected query one, got %q", loaded[0].Query)
	}
}

func TestComputeHitAtK(t *testing.T) {
	paths := []string{"src/a.go", "src/b.go", "src/c.go", "src/d.go"}
	expectAny := []string{"src/b.go"}

	hit1, hit3, hit5, rank := computeHitAtK(paths, expectAny)
	if hit1 {
		t.Errorf("expected hit1=false for rank 2, got true")
	}
	if !hit3 {
		t.Errorf("expected hit3=true for rank 2, got false")
	}
	if !hit5 {
		t.Errorf("expected hit5=true for rank 2, got false")
	}
	if rank != 2 {
		t.Errorf("expected rank 2, got %d", rank)
	}

	// Test rank 1
	expectAny = []string{"src/a.go"}
	hit1, hit3, hit5, rank = computeHitAtK(paths, expectAny)
	if !hit1 {
		t.Errorf("expected hit1=true for rank 1, got false")
	}
	if !hit3 {
		t.Errorf("expected hit3=true for rank 1, got false")
	}
	if !hit5 {
		t.Errorf("expected hit5=true for rank 1, got false")
	}
	if rank != 1 {
		t.Errorf("expected rank 1, got %d", rank)
	}

	// Test not found
	expectAny = []string{"src/z.go"}
	hit1, hit3, hit5, rank = computeHitAtK(paths, expectAny)
	if hit1 {
		t.Errorf("expected hit1=false for not found, got true")
	}
	if hit3 {
		t.Errorf("expected hit3=false for not found, got true")
	}
	if hit5 {
		t.Errorf("expected hit5=false for not found, got true")
	}
	if rank != -1 {
		t.Errorf("expected rank -1 for not found, got %d", rank)
	}
}

func TestCountTokens(t *testing.T) {
	paths := []string{"src/a.go", "src/b.go", "src/c.go"}
	chars, tokens := countTokens(paths)
	expectedChars := len("src/a.go") + len("src/b.go") + len("src/c.go")
	if chars != expectedChars {
		t.Errorf("expected chars %d, got %d", expectedChars, chars)
	}
	expectedTokens := expectedChars / 4
	if tokens != expectedTokens {
		t.Errorf("expected tokens %d, got %d", expectedTokens, tokens)
	}
}

func TestEngineResultTokenCounting(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	res := out.Results[0][0]
	if res.Chars == 0 {
		t.Errorf("expected non-zero chars, got %d", res.Chars)
	}
	if res.Tokens == 0 {
		t.Errorf("expected non-zero tokens, got %d", res.Tokens)
	}
	if res.Tokens != res.Chars/4 {
		t.Errorf("expected tokens %d (chars/4), got %d", res.Chars/4, res.Tokens)
	}
}

func TestSummaryHitAtK(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}},
		{Query: "stream audio", ExpectAny: []string{"src/audio/stream.go"}},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	if len(out.Summary) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(out.Summary))
	}
	s := out.Summary[0]
	if s.HitAt1 < 0 || s.HitAt1 > 1 {
		t.Errorf("expected HitAt1 between 0 and 1, got %f", s.HitAt1)
	}
	if s.HitAt3 < 0 || s.HitAt3 > 1 {
		t.Errorf("expected HitAt3 between 0 and 1, got %f", s.HitAt3)
	}
	if s.HitAt5 < 0 || s.HitAt5 > 1 {
		t.Errorf("expected HitAt5 between 0 and 1, got %f", s.HitAt5)
	}
	if s.AvgTokens < 0 {
		t.Errorf("expected non-negative AvgTokens, got %f", s.AvgTokens)
	}
}

func TestCountRelevantPaths(t *testing.T) {
	paths := []string{"src/a.go", "src/b.go", "src/c.go"}
	expectAny := []string{"src/b.go"}
	if got := countRelevantPaths(paths, expectAny); got != 1 {
		t.Errorf("expected 1 relevant path, got %d", got)
	}
	expectAny = []string{"src/b.go", "src/c.go"}
	if got := countRelevantPaths(paths, expectAny); got != 2 {
		t.Errorf("expected 2 relevant paths, got %d", got)
	}
	expectAny = []string{"missing.go"}
	if got := countRelevantPaths(paths, expectAny); got != 0 {
		t.Errorf("expected 0 relevant paths, got %d", got)
	}
}

func TestNoiseRatioEmptyResults(t *testing.T) {
	got := computeNoiseRatio([]string{}, []string{"src/a.go"})
	if got != 0 {
		t.Errorf("expected noise ratio 0 for empty results, got %f", got)
	}
}

func TestNoiseRatioAllRelevant(t *testing.T) {
	paths := []string{"src/a.go", "src/b.go"}
	expectAny := []string{"src/a.go", "src/b.go"}
	got := computeNoiseRatio(paths, expectAny)
	if got != 0 {
		t.Errorf("expected noise ratio 0 for all relevant paths, got %f", got)
	}
}

func TestNoiseRatioMixed(t *testing.T) {
	paths := []string{
		"src/a.go",
		"src/b.go",
		"src/c.go",
		"src/d.go",
		"src/e.go",
		"src/f.go",
		"src/g.go",
		"src/h.go",
		"src/i.go",
		"src/j.go",
	}
	expectAny := []string{"src/a.go", "src/b.go", "src/c.go"}
	got := computeNoiseRatio(paths, expectAny)
	want := 0.7
	if got != want {
		t.Errorf("expected noise ratio %f, got %f", want, got)
	}
}

func TestSummaryNoiseRatio(t *testing.T) {
	paths := []string{"src/a.go", "src/b.go", "src/c.go", "src/d.go"}
	expectAny := []string{"src/a.go"}
	results := []EngineResult{{
		EngineName: "ctxt",
		Paths:      paths,
		TotalHits:  len(paths),
		NoiseRatio: computeNoiseRatio(paths, expectAny),
		HitAt1:     true,
		HitAt3:     true,
		HitAt5:     true,
		Found:      true,
		Rank:       1,
		Tokens:     4,
	}}
	cases := []EvalCase{
		{Query: "a", ExpectAny: expectAny, Category: "symbol-lookup"},
	}
	summary := computeEngineSummaries(cases, [][]EngineResult{results})
	if len(summary) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summary))
	}
	want := 0.75
	if summary[0].AvgNoiseRatio != want {
		t.Errorf("expected avg noise ratio %f, got %f", want, summary[0].AvgNoiseRatio)
	}
}

func TestBenchOutputIndexLoadMs(t *testing.T) {
	out := BenchOutput{IndexLoadMs: 42}
	jsonStr, err := resultsToJSONBench(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(jsonStr, "index_load_ms") {
		t.Errorf("expected JSON output to contain index_load_ms")
	}
}

func TestPrintCategoryReport(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}, Category: "audio"},
		{Query: "stream audio", ExpectAny: []string{"src/audio/stream.go"}, Category: "audio"},
		{Query: "scoring", ExpectAny: []string{"src/search/scoring.go"}, Category: "search"},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	// Just verify it doesn't panic
	printCategoryReport(cases, out.Results)
}

func TestCategoryReportJSON(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}, Category: "audio"},
		{Query: "scoring", ExpectAny: []string{"src/search/scoring.go"}, Category: "search"},
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	jsonStr, err := categoryReportToJSON(cases, out.Results, 5)
	if err != nil {
		t.Fatalf("categoryReportToJSON error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, ok := parsed["categories"]; !ok {
		t.Errorf("missing 'categories' key in JSON")
	}
	if _, ok := parsed["overall"]; !ok {
		t.Errorf("missing 'overall' key in JSON")
	}
}

func TestCategoryReportFallback(t *testing.T) {
	_, index := buildFixtureBench(t)
	cases := []EvalCase{
		{Query: "batch audio processing", ExpectAny: []string{"src/audio/batch.go"}}, // No category
		{Query: "scoring", ExpectAny: []string{"src/search/scoring.go"}}, // No category
	}
	out := runBench(BenchInput{
		Index:        index,
		Cases:        cases,
		Engines:      instantiateEngines([]string{"ctxt"}),
		Limit:        10,
		MinScore:     1,
		GrepMaxBytes: 1048576,
	})
	// With empty categories, should use flat report (verify it doesn't panic)
	printBenchSummary(out)
	// Also verify category report still works (groups as uncategorized)
	printCategoryReport(cases, out.Results)
}
