package main

import (
	"path/filepath"
	"regexp"
	"strings"
)

var splitNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

var genericSynonymStopwords = map[string]struct{}{
	"file":        {},
	"folder":      {},
	"dir":         {},
	"directory":   {},
	"data":        {},
	"temp":        {},
	"tmp":         {},
	"misc":        {},
	"item":        {},
	"items":       {},
	"thing":       {},
	"stuff":       {},
	"object":      {},
	"objects":     {},
	"resource":    {},
	"resources":   {},
	"information": {},
}

func sanitizeSynonyms(values []string, max int) []string {
	if max <= 0 {
		max = 4
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, max)
	for _, value := range values {
		normalized := strings.TrimSpace(strings.ToLower(value))
		if normalized == "" || len(normalized) <= 1 {
			continue
		}
		if _, bad := genericSynonymStopwords[normalized]; bad {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
		if len(out) >= max {
			break
		}
	}
	return out
}

func lexicalSynonyms(name string) []string {
	base := strings.TrimSpace(name)
	if base == "" {
		return nil
	}

	nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))
	parts := splitIdentifierTokens(base)
	partsNoExt := splitIdentifierTokens(nameNoExt)

	combined := append(parts, partsNoExt...)
	return sanitizeSynonyms(combined, 8)
}

func splitIdentifierTokens(input string) []string {
	if input == "" {
		return nil
	}
	camelSplit := camelBoundary.ReplaceAllString(input, "$1 $2")
	lower := strings.ToLower(camelSplit)
	lower = strings.ReplaceAll(lower, "_", " ")
	lower = strings.ReplaceAll(lower, "-", " ")
	lower = strings.ReplaceAll(lower, ".", " ")
	lower = splitNonAlnum.ReplaceAllString(lower, " ")

	parts := strings.Fields(lower)
	if len(parts) == 0 {
		return nil
	}
	return dedupeStrings(parts)
}
