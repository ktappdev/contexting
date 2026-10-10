package contexting

import "time"

func benchResult(name string, paths, expected []string, limit int, start time.Time, err error) EngineResult {
	ordering, execution := "alphabetical", "subprocess"
	if name == "ctxt" || name == "hybrid" {
		ordering = "relevance"
	}
	if name == "ctxt" {
		execution = "in_process"
	}
	if name == "hybrid" {
		execution = "mixed"
	}
	result := EngineResult{EngineName: name, Ordering: ordering, Execution: execution, Rank: -1}
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if limit == 0 {
		limit = 10
	}
	if limit > 0 && len(paths) > limit {
		paths = paths[:limit]
	}
	result.Paths = paths
	result.TotalHits = len(paths)
	result.HitAt1, result.HitAt3, result.HitAt5, result.Rank = computeHitAtK(paths, expected)
	result.Found = result.Rank > 0
	result.PathBytes, result.EstimatedPathTokens = countTokens(paths)
	result.NoiseRatio = computeNoiseRatio(paths, expected)
	result.TimeMs = float64(time.Since(start)) / float64(time.Millisecond)
	return result
}

func computeHitAtK(paths, expected []string) (bool, bool, bool, int) {
	for i, candidate := range paths {
		for _, want := range expected {
			if pathMatchesExpected(candidate, want) {
				rank := i + 1
				return rank == 1, rank <= 3, rank <= 5, rank
			}
		}
	}
	return false, false, false, -1
}

// Path bytes only: excludes JSON, metadata, snippets, and separators.
func countTokens(paths []string) (int, int) {
	bytes := 0
	for _, path := range paths {
		bytes += len(path)
	}
	return bytes, bytes / 4
}

func computeNoiseRatio(paths, expected []string) float64 {
	if len(paths) == 0 {
		return 0
	}
	return 1 - float64(countRelevantPaths(paths, expected))/float64(len(paths))
}

func countRelevantPaths(paths, expected []string) int {
	count := 0
	for _, candidate := range paths {
		for _, want := range expected {
			if pathMatchesExpected(candidate, want) {
				count++
				break
			}
		}
	}
	return count
}
