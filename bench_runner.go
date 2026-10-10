package contexting

import (
	"sort"
	"strings"
	"time"
)

// BenchCorpus is the shared file inventory prepared by runBench before timing.
type BenchCorpus struct{ files, contentFiles map[string]string }

type BenchInput struct {
	Index        *ContextIndex
	Cases        []EvalCase
	Engines      []SearchEngine
	Limit        int
	MinScore     int
	GrepMaxBytes int
	Runs         int
	Warmup       int
}

type BenchMethodology struct {
	ProjectRoot      string `json:"project_root"`
	IndexedFiles     int    `json:"indexed_files"`
	ContentMaxBytes  int    `json:"content_max_bytes"`
	IndexModel       string `json:"index_model,omitempty"`
	IndexGeneratedAt string `json:"index_generated_at,omitempty"`
	Limit            int    `json:"limit"`
	Runs             int    `json:"runs"`
	Warmup           int    `json:"warmup"`
	Corpus           string `json:"corpus"`
	TimingScope      string `json:"timing_scope"`
	ExpectedPaths    string `json:"expected_paths"`
}

// Quality is evaluated once per case. Timings contain repeated adapter calls,
// excluding warmups, with engine order rotated across cases and repetitions.
func runBench(in BenchInput) BenchOutput {
	if in.Runs <= 0 {
		in.Runs = 1
	}
	if in.Warmup < 0 {
		in.Warmup = 0
	}
	if in.Limit <= 0 {
		in.Limit = 10
	}
	if in.MinScore < 0 {
		in.MinScore = 1
	}
	out := BenchOutput{ReportVersion: 2, Cases: in.Cases, Results: make([][]EngineResult, 0, len(in.Cases)),
		Methodology: BenchMethodology{Limit: in.Limit, Runs: in.Runs, Warmup: in.Warmup, Corpus: "indexed files only; identical result limit for all engines", TimingScope: "adapter calls; ctxt in process, external engines include subprocess startup; index load separate", ExpectedPaths: "exact project-relative paths, case-sensitive"},
	}
	preparationStart := time.Now()
	if in.GrepMaxBytes <= 0 {
		in.GrepMaxBytes = 1048576
	}
	files, corpusErr := benchIndexedFiles(in.Index, 0)
	contentFiles, contentErr := benchIndexedFiles(in.Index, in.GrepMaxBytes)
	if corpusErr == nil {
		corpusErr = contentErr
	}
	corpus := BenchCorpus{files: files, contentFiles: contentFiles}
	out.CorpusPreparationMs = float64(time.Since(preparationStart)) / float64(time.Millisecond)
	out.Methodology.IndexedFiles = len(files)
	out.Methodology.ContentMaxBytes = in.GrepMaxBytes
	if in.Index != nil {
		out.Methodology.ProjectRoot = in.Index.RootPath
		out.Methodology.IndexModel = in.Index.Model
		if !in.Index.GeneratedAt.IsZero() {
			out.Methodology.IndexGeneratedAt = in.Index.GeneratedAt.Format(time.RFC3339Nano)
		}
	}
	opts := SearchOptions{Limit: in.Limit, MinScore: in.MinScore, TypeFilter: "files"}
	for caseIdx, entry := range in.Cases {
		results := make([]EngineResult, len(in.Engines))
		samples := make([][]float64, len(in.Engines))
		errors := make([]string, len(in.Engines))
		for iteration := 0; iteration < in.Warmup+in.Runs; iteration++ {
			for offset := range in.Engines {
				i := (offset + caseIdx + iteration) % len(in.Engines)
				start := time.Now()
				var result EngineResult
				if corpusErr != nil {
					result = benchResult(in.Engines[i].Name(), nil, entry.ExpectAny, opts.Limit, start, corpusErr)
				} else {
					result = in.Engines[i].Search(entry.Query, entry.ExpectAny, in.Index, opts, in.GrepMaxBytes, corpus)
				}
				duration := float64(time.Since(start)) / float64(time.Millisecond)
				if iteration < in.Warmup {
					continue
				}
				if iteration == in.Warmup {
					results[i] = result
				}
				if result.Error != "" {
					errors[i] = result.Error
					continue
				}
				samples[i] = append(samples[i], duration)
			}
		}
		for i := range results {
			results[i].SamplesMs = samples[i]
			if errors[i] != "" {
				results[i].Error = errors[i]
			}
			if len(samples[i]) > 0 {
				sorted := append([]float64(nil), samples[i]...)
				sort.Float64s(sorted)
				results[i].TimeMs = percentile(sorted, 0.5)
			}
		}
		out.Results = append(out.Results, results)
	}
	out.Summary = computeEngineSummaries(in.Cases, out.Results)
	out.Misses = computeBenchMisses(in.Cases, out.Results)
	out.Categories = computeCategorySummaries(in.Cases, out.Results)
	return out
}

func benchHasErrors(out BenchOutput) bool {
	for _, summary := range out.Summary {
		if summary.ExecutionErrors > 0 {
			return true
		}
	}
	return false
}

func validBenchCase(entry EvalCase) bool {
	return strings.TrimSpace(entry.Query) != "" && len(entry.ExpectAny) > 0
}
