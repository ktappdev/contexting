package main

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
	"time"
)

const (
	defaultModel           = "qwen3.5-4b"
	defaultSynonyms        = 10
	defaultHTTPTimeout     = 45 * time.Second // HTTP client timeout for API requests
)

var defaultEndpoint = "https://llama.kentaylor.dev/v1/chat/completions"

type OpenRouterRequest struct {
	Model       string          `json:"model"`
	Messages   []Message       `json:"messages"`
	Format     json.RawMessage `json:"response_format,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens  *int            `json:"max_tokens,omitempty"`
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
	return GenerateSynonymsBatchWithContext(context.Background(), names, apiKey, model, endpoint, temperature, maxTokens, synonymsPerName)
}

func GenerateSynonymsBatchWithContext(ctx context.Context, names []string, apiKey string, model string, endpoint string, temperature float64, maxTokens int, synonymsPerName int) (SynonymResponse, error) {

	if len(names) == 0 {
		return make(SynonymResponse), nil
	}
	if model == "" {
		model = defaultModel
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if synonymsPerName <= 0 {
		synonymsPerName = defaultSynonyms
	}

	systemPrompt := fmt.Sprintf(
		"You are a helpful assistant. For each folder or file name in the list, generate exactly %d plausible alternative words or short phrases a developer might use when searching for that file in a codebase. Return ONLY a valid JSON object where each key is an exact filename from the input list and each value is an array of %d synonym strings. Example: {\"auth.go\": [\"login\", \"authentication\", \"session\"], \"config\": [\"settings\", \"configuration\", \"options\"]}. No markdown, no prose, no extra text.",
		synonymsPerName,
		synonymsPerName,
	)
	userContent := fmt.Sprintf("File and folder names:\n%s", strings.Join(names, "\n"))

	reqBody := OpenRouterRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Format: json.RawMessage(`{"type":"json_object"}`),
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
	req.Header.Set("HTTP-Referer", "https://github.com/contexting")
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

func GenerateSynonymsForNames(names []string, apiKey string, batchSize int, model string, endpoint string, temperature float64, maxTokens int, synonymsPerName int) (SynonymResponse, error) {
	return GenerateSynonymsForNamesWithContext(context.Background(), names, apiKey, batchSize, model, endpoint, temperature, maxTokens, synonymsPerName, 1)
}

func GenerateSynonymsForNamesWithContext(ctx context.Context, names []string, apiKey string, batchSize int, model string, endpoint string, temperature float64, maxTokens int, synonymsPerName int, parallelLimit int) (SynonymResponse, error) {
	if len(names) == 0 {
		return make(SynonymResponse), nil
	}
	if model == "" {
		model = defaultModel
	}
	if synonymsPerName <= 0 {
		synonymsPerName = defaultSynonyms
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
		fmt.Printf("  Synonyms: processing %d names...\n", len(names))
		return GenerateSynonymsBatchWithContext(ctx, names, apiKey, model, endpoint, temperature, maxTokens, synonymsPerName)
	}

	totalBatches := (len(names) + batchSize - 1) / batchSize
	if totalBatches > 9 {
		logWarnf("Project has %d unique names requiring %d batches (>9). This project may be too large for reliable synonym generation.", len(names), totalBatches)
		if isInteractiveTerminal() {
			continueAnyway, err := askYesNo("Continue anyway? [y/N] ", false)
			if err != nil {
				return nil, fmt.Errorf("prompt failed: %w", err)
			}
			if !continueAnyway {
				return nil, fmt.Errorf("synonym generation aborted: %d batches exceeds threshold", totalBatches)
			}
		}
	}
	fmt.Printf("  Synonyms: %d names in %d batches (parallel=%d)\n", len(names), totalBatches, parallelLimit)

	result := make(SynonymResponse)
	sem := make(chan struct{}, parallelLimit)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < len(names); i += batchSize {
		batchNum := i/batchSize + 1
		end := i + batchSize
		if end > len(names) {
			end = len(names)
		}

		wg.Add(1)
		go func(batchNum int, batch []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			synonyms, err := GenerateSynonymsBatchWithContext(ctx, batch, apiKey, model, endpoint, temperature, maxTokens, synonymsPerName)
			if err != nil {
				fmt.Printf("  ⚠ Synonyms: batch %d/%d failed: %v\n", batchNum, totalBatches, err)
				return
			}
			mu.Lock()
			for name, values := range synonyms {
				result[name] = values
			}
			mu.Unlock()
		}(batchNum, names[i:end])
	}
	wg.Wait()
	fmt.Printf("  Synonyms: %d names processed\n", len(result))

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
