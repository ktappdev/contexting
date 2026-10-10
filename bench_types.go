package contexting

// SearchEngine is the contract every bench engine implements.
type SearchEngine interface {
	Name() string
	Search(query string, expectAny []string, index *ContextIndex, opts SearchOptions, grepMaxBytes int, corpus BenchCorpus) EngineResult
}

// EngineResult captures timing + outcome for one engine on one query.
type EngineResult struct {
	Ordering            string    `json:"ordering"`
	Execution           string    `json:"execution"`
	SamplesMs           []float64 `json:"samples_ms,omitempty"`
	EngineName          string    `json:"engine_name"`
	Found               bool      `json:"found"`
	Rank                int       `json:"rank"`       // 1-based output position; -1 if not found
	TotalHits           int       `json:"total_hits"` // number of paths returned
	Paths               []string  `json:"paths,omitempty"`
	TimeMs              float64   `json:"time_ms"`
	Error               string    `json:"error,omitempty"`       // empty if no error
	PathBytes           int       `json:"path_bytes"`            // total bytes across all returned paths
	EstimatedPathTokens int       `json:"estimated_path_tokens"` // path bytes/4 heuristic
	HitAt1              bool      `json:"hit_at_1"`
	HitAt3              bool      `json:"hit_at_3"`
	HitAt5              bool      `json:"hit_at_5"`
	NoiseRatio          float64   `json:"noise_ratio"` // 1.0 - (relevantResults / totalResults)
}

// EngineSummary aggregates results for one engine across all cases.
type EngineSummary struct {
	ExecutionErrors        int     `json:"execution_errors"`
	TimingSamples          int     `json:"timing_samples"`
	Ordering               string  `json:"ordering"`
	Execution              string  `json:"execution"`
	EngineName             string  `json:"engine_name"`
	Cases                  int     `json:"cases"`
	SuccessRate            float64 `json:"success_rate"`
	AvgResults             float64 `json:"avg_results"`
	AvgTimeMs              float64 `json:"avg_time_ms"`
	P50TimeMs              float64 `json:"p50_time_ms"`
	P95TimeMs              float64 `json:"p95_time_ms"`
	FailedCases            int     `json:"failed_cases"`
	HitAt1                 float64 `json:"hit_at_1"`
	HitAt3                 float64 `json:"hit_at_3"`
	HitAt5                 float64 `json:"hit_at_5"`
	AvgEstimatedPathTokens float64 `json:"avg_estimated_path_tokens"`
	AvgNoiseRatio          float64 `json:"avg_noise_ratio"`
}
