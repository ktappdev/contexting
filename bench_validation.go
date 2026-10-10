package contexting

import (
	"fmt"
	"strings"
)

func validateBenchCases(cases []EvalCase, index *ContextIndex) error {
	if len(cases) == 0 {
		return fmt.Errorf("benchmark case file has no cases")
	}
	files, err := benchIndexedFiles(index, 0)
	if err != nil {
		return err
	}
	indexed := make(map[string]bool, len(files))
	for _, rel := range files {
		indexed[normalizeEvalPath(rel)] = true
	}
	for i, entry := range cases {
		if !validBenchCase(entry) || len(tokenize(entry.Query)) == 0 {
			return fmt.Errorf("benchmark case %d needs a searchable query and expected paths", i+1)
		}
		for _, expected := range entry.ExpectAny {
			normalized := normalizeEvalPath(expected)
			if strings.TrimSpace(expected) == "" || !indexed[normalized] {
				return fmt.Errorf("benchmark case %d (%q): expected path %q is not an indexed file; use its exact project-relative path", i+1, entry.Query, expected)
			}
		}
	}
	return nil
}
