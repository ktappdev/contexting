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

var lowSignalTokens = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"do": {}, "for": {}, "from": {}, "in": {}, "is": {}, "it": {}, "no": {}, "of": {},
	"on": {}, "or": {}, "the": {}, "to": {}, "with": {}, "without": {},
}

func sanitizeSynonyms(values []string, max int) []string {
	if max <= 0 {
		max = 4
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, max)
	for _, value := range values {
		normalized := strings.Join(strings.Fields(strings.ToLower(value)), " ")
		if normalized == "" {
			continue
		}
		if _, bad := genericSynonymStopwords[normalized]; bad {
			continue
		}
		words := strings.Fields(normalized)
		if len(words) == 1 && isLowSignalToken(words[0]) {
			continue
		}
		allLowSignal := true
		for _, word := range words {
			if !isLowSignalToken(word) {
				allLowSignal = false
				break
			}
		}
		if allLowSignal {
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

func isLowSignalToken(token string) bool {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" {
		return true
	}
	if len(token) <= 1 {
		return true
	}
	_, bad := lowSignalTokens[token]
	return bad
}
