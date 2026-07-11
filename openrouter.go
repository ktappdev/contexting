package contexting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultModel           = "deepseek/deepseek-v4-flash"
	defaultSynonyms        = 5
	defaultSynonymsMax     = 12
	defaultHTTPTimeout     = 45 * time.Second // HTTP client timeout for API requests
)

var defaultEndpoint = "https://openrouter.ai/api/v1/chat/completions"

type OpenRouterRequest struct {
	Model       string          `json:"model"`
	Messages   []Message       `json:"messages"`
	Format     json.RawMessage `json:"response_format,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens  *int            `json:"max_tokens,omitempty"`
	Reasoning  json.RawMessage `json:"reasoning,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

type SynonymResponse map[string][]string

func GetAPIKey() (string, error) {
	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}
	return key, nil
}

func GenerateSynonymsBatch(names []string, apiKey string, model string, endpoint string, temperature float64, maxTokens int, synonymsPerName int) (SynonymResponse, error) {
	return GenerateSynonymsBatchWithContext(context.Background(), names, apiKey, model, endpoint, temperature, maxTokens, synonymsPerName, synonymsPerName, nil, nil)
}

func GenerateSynonymsBatchWithContext(ctx context.Context, names []string, apiKey string, model string, endpoint string, temperature float64, maxTokens int, synonymsMin int, synonymsMax int, symbols map[string][]string, imports map[string][]string) (SynonymResponse, error) {

	if len(names) == 0 {
		return make(SynonymResponse), nil
	}
	if model == "" {
		model = defaultModel
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if synonymsMax <= 0 {
		synonymsMax = defaultSynonymsMax
	}
	if synonymsMin <= 0 {
		synonymsMin = synonymsMax
	}

	systemPrompt := fmt.Sprintf(
		"You are a code search expert. Given source code file and folder names from a software project, generate %d to %d conceptual synonyms for each — words a developer might type when searching for that file WITHOUT knowing its exact name.\n\n"+
			"Think about WHAT THE FILE DOES, not just what it's called. Include:\n"+
			"- Functional purpose (what the code handles)\n"+
			"- Related concepts (terms developers associate with this functionality)\n"+
			"- Alternative vocabulary (different words for the same concept)\n"+
			"- Action verbs (what a developer would DO with this code — e.g. \"authenticate\", \"filter\", \"parse\", \"validate\")\n\n"+
			"The input may include code symbols (in brackets) AND imports (in brackets) for each file.\n"+
			"- Symbols reveal what the code defines (function/class/type names). Use them to understand what the code actually does.\n"+
			"- Imports reveal which external libraries or internal modules the file depends on. Use imports to understand the file's DOMAIN — e.g. \"@clerk/nextjs\" means the file handles Clerk authentication, \"stripe\" means payment processing, \"next/navigation\" means routing, \"@prisma/client\" means database access. This is critical: a file's own symbols may be generic (e.g. POST/GET handlers in a route.ts) while its imports pinpoint the actual domain.\n"+
			"Generate synonyms that reflect the real domain inferred from symbols AND imports — not just the filename word.\n\n"+
			"Do NOT include: word variations of the filename, misspellings, file extensions, or trivial modifications.\n"+
			"Do NOT replace noun synonyms with verb synonyms — include BOTH nouns and verbs.\n\n"+
			"Examples:\n"+
			"{\"auth.go\": [\"login\", \"authentication\", \"session\", \"credentials\", \"signin\", \"access control\", \"identity\"]}\n"+
			"{\"ignore.go\": [\"skip\", \"exclude\", \"filter\", \"block\", \"deny list\", \"whitelist\", \"blacklist\", \"ignore patterns\", \"omit\", \"suppress\"]}\n"+
			"{\"openrouter.go\": [\"llm\", \"ai model\", \"api client\", \"chat completion\", \"language model\", \"inference\", \"synonym generation\"]}\n"+
			"{\"index_manager.go\": [\"index\", \"cache\", \"snapshot\", \"search state\", \"live index\", \"in-memory index\", \"persistence\"]}\n"+
			"{\"watch_helpers.go\": [\"file monitor\", \"fsnotify\", \"file changes\", \"watcher\", \"event handler\", \"live updates\", \"filesystem events\"]}\n"+
			"{\"handler.go\": [\"process request\", \"handle\", \"serve\", \"respond\", \"route\", \"dispatch\", \"receive\", \"accept\"]}\n"+
			"{\"webhook/route.ts\": [\"clerk webhook\", \"user events\", \"clerk event handler\", \"webhook receiver\", \"clerk user sync\", \"auth webhook\"]}\n\n"+
			"Return ONLY a valid JSON object where each key is an exact filename from the input list and each value is an array of synonym strings. Generate between %d and %d synonyms per name. No markdown, no prose, no extra text.",
		synonymsMin, synonymsMax,
		synonymsMin, synonymsMax,
	)

	// Build user content with symbols and imports when available
	var lines []string
	for _, name := range names {
		var parts []string
		parts = append(parts, name)
		if syns, ok := symbols[name]; ok && len(syns) > 0 {
			// Cap symbols at 10 per file to avoid excessive token use
			if len(syns) > 10 {
				syns = syns[:10]
			}
			parts = append(parts, fmt.Sprintf("[symbols: %s]", strings.Join(syns, ", ")))
		}
		if imps, ok := imports[name]; ok && len(imps) > 0 {
			// Cap imports at 5 per file — top dependencies are usually the most
			// domain-revealing; rest is noise.
			if len(imps) > 5 {
				imps = imps[:5]
			}
			parts = append(parts, fmt.Sprintf("[imports: %s]", strings.Join(imps, ", ")))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	userContent := fmt.Sprintf("File and folder names with their code symbols and imports:\n%s", strings.Join(lines, "\n"))

	reqBody := OpenRouterRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Format:    json.RawMessage(`{"type":"json_object"}`),
		Reasoning: json.RawMessage(`{"exclude":true}`),
	}
	if temperature > 0 {
		reqBody.Temperature = &temperature
	}
	if maxTokens > 0 {
		reqBody.MaxTokens = &maxTokens
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/ktappdev/contexting")
	req.Header.Set("X-Title", "Contexting")

	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(apiResp.Choices) == 0 {
		return make(SynonymResponse), nil
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if content == "" {
		return make(SynonymResponse), nil
	}

	var synonyms SynonymResponse
	if err := json.Unmarshal([]byte(content), &synonyms); err != nil {
		// Try to fix common LLM JSON issues (trailing commas, extra text)
		fixed := sanitizeJSON(content)
		if err2 := json.Unmarshal([]byte(fixed), &synonyms); err2 != nil {
			return nil, fmt.Errorf("parse synonyms JSON: %w", err)
		}
	}

	for name, values := range synonyms {
		synonyms[name] = dedupeStrings(values)
	}

	return synonyms, nil
}

func GenerateSynonymsForNames(names []string, apiKey string, batchSize int, model string, endpoint string, temperature float64, maxTokens int, synonymsMin int, synonymsMax int) (SynonymResponse, error) {
	return GenerateSynonymsForNamesWithContext(context.Background(), names, apiKey, batchSize, model, endpoint, temperature, maxTokens, synonymsMin, synonymsMax, 1, nil, nil)
}

func GenerateSynonymsForNamesWithContext(ctx context.Context, names []string, apiKey string, batchSize int, model string, endpoint string, temperature float64, maxTokens int, synonymsMin int, synonymsMax int, parallelLimit int, symbols map[string][]string, imports map[string][]string) (SynonymResponse, error) {
	if len(names) == 0 {
		return make(SynonymResponse), nil
	}
	if model == "" {
		model = defaultModel
	}
	if synonymsMax <= 0 {
		synonymsMax = defaultSynonymsMax
	}
	if synonymsMin <= 0 {
		synonymsMin = synonymsMax
	}
	if parallelLimit <= 0 {
		parallelLimit = 1
	}

	// Smart batching: 0 means "auto" — up to 60 names per request
	if batchSize <= 0 {
		if len(names) <= 60 {
			batchSize = len(names)
		} else {
			numBatches := (len(names) + 59) / 60
			batchSize = (len(names) + numBatches - 1) / numBatches
		}
	}
	if batchSize >= len(names) {
		LogInfof("  Synonyms: processing %d names...", len(names))
		return GenerateSynonymsBatchWithContext(ctx, names, apiKey, model, endpoint, temperature, maxTokens, synonymsMin, synonymsMax, symbols, imports)
	}

	totalBatches := (len(names) + batchSize - 1) / batchSize
	LogInfof("  Synonyms: %d names in %d batches (parallel=%d)", len(names), totalBatches, parallelLimit)

	result := make(SynonymResponse)
	sem := make(chan struct{}, parallelLimit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var completed int32

	for i := 0; i < len(names); i += batchSize {
		batchNum := i/batchSize + 1
		end := i + batchSize
		if end > len(names) {
			end = len(names)
		}
		batchNames := names[i:end]

		// Filter symbols and imports maps for this batch
		batchSymbols := make(map[string][]string)
		for _, name := range batchNames {
			if syns, ok := symbols[name]; ok {
				batchSymbols[name] = syns
			}
		}
		batchImports := make(map[string][]string)
		for _, name := range batchNames {
			if imps, ok := imports[name]; ok {
				batchImports[name] = imps
			}
		}

		wg.Add(1)
		go func(batchNum int, batchNames []string, batchSymbols map[string][]string, batchImports map[string][]string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			synonyms, err := GenerateSynonymsBatchWithContext(ctx, batchNames, apiKey, model, endpoint, temperature, maxTokens, synonymsMin, synonymsMax, batchSymbols, batchImports)
			if err != nil {
				LogWarnf("  ⚠ Synonyms: batch %d/%d failed: %v", batchNum, totalBatches, err)
				return
			}
			mu.Lock()
			for name, values := range synonyms {
				result[name] = values
			}
			mu.Unlock()
			atomic.AddInt32(&completed, 1)
			LogInfof("  Synonyms: batch %d/%d done (%d names)", batchNum, totalBatches, len(batchNames))
		}(batchNum, batchNames, batchSymbols, batchImports)
	}
	wg.Wait()
	LogInfof("  Synonyms: %d names processed", len(result))

	return result, nil
}

// sanitizeJSON fixes common LLM JSON output issues.
// Handles: markdown wrapping, trailing commas, mismatched brackets/braces,
// truncated JSON (missing closing brackets), and extra trailing characters.
func sanitizeJSON(s string) string {
	// Extract just the JSON object if wrapped in markdown or extra text
	if idx := strings.Index(s, "{"); idx > 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "}"); idx >= 0 && idx < len(s)-1 {
		s = s[:idx+1]
	}
	// Remove trailing commas before } or ]
	s = strings.ReplaceAll(s, ",]", "]")
	s = strings.ReplaceAll(s, ",}", "}")
	// Fix mismatched brackets: track expected close chars and fix wrong ones
	fixed := make([]byte, 0, len(s))
	var stack []byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '{':
			stack = append(stack, '}')
			fixed = append(fixed, ch)
		case '[':
			stack = append(stack, ']')
			fixed = append(fixed, ch)
		case '}', ']':
			if len(stack) > 0 && stack[len(stack)-1] == ch {
				stack = stack[:len(stack)-1]
				fixed = append(fixed, ch)
			} else if len(stack) > 0 {
				// Mismatched bracket: replace with expected close
				fixed = append(fixed, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			// If stack empty and extra close char, skip it
		default:
			fixed = append(fixed, ch)
		}
	}
	// Close any unclosed brackets (truncated JSON)
	for i := len(stack) - 1; i >= 0; i-- {
		fixed = append(fixed, stack[i])
	}
	s = string(fixed)
	// Remove stray backslashes before structural JSON characters
	// These sequences are never valid in JSON and indicate LLM hallucination
	s = strings.ReplaceAll(s, "\\,", ",")
	s = strings.ReplaceAll(s, "\\]", "]")
	s = strings.ReplaceAll(s, "\\}", "}")
	s = strings.ReplaceAll(s, "\\[", "[")
	s = strings.ReplaceAll(s, "\\{", "{")
	// Remove trailing commas again after fixes
	s = strings.ReplaceAll(s, ",]", "]")
	s = strings.ReplaceAll(s, ",}", "}")
	return s
}
