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
	"time"
)

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel  = "openrouter/free"
)

type OpenRouterRequest struct {
	Model    string          `json:"model"`
	Messages []Message       `json:"messages"`
	Format   json.RawMessage `json:"response_format,omitempty"`
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

func GenerateSynonymsBatch(names []string, apiKey string, model string, synonymsPerName int) (SynonymResponse, error) {
	return GenerateSynonymsBatchWithContext(context.Background(), names, apiKey, model, synonymsPerName)
}

func GenerateSynonymsBatchWithContext(ctx context.Context, names []string, apiKey string, model string, synonymsPerName int) (SynonymResponse, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if len(names) == 0 {
		return make(SynonymResponse), nil
	}
	if model == "" {
		model = defaultModel
	}
	if synonymsPerName <= 0 {
		synonymsPerName = 4
	}

	systemPrompt := fmt.Sprintf(
		"You are a helpful assistant. For each folder or file name in the list, generate exactly %d plausible alternative words or short phrases a developer might use in a codebase. Return ONLY a valid JSON object shaped like {\\\"name\\\": [\\\"syn1\\\", ...]}. No markdown, no prose.",
		synonymsPerName,
	)
	userContent := fmt.Sprintf("Names:\n%s", strings.Join(names, "\n"))

	reqBody := OpenRouterRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Format: json.RawMessage(`{"type":"json_object"}`),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/contexting")
	req.Header.Set("X-Title", "Contexting")

	client := &http.Client{Timeout: 45 * time.Second}
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
		return nil, fmt.Errorf("parse synonyms JSON: %w", err)
	}

	for name, values := range synonyms {
		synonyms[name] = dedupeStrings(values)
	}

	return synonyms, nil
}

func GenerateSynonymsForNames(names []string, apiKey string, batchSize int, model string, synonymsPerName int) (SynonymResponse, error) {
	return GenerateSynonymsForNamesWithContext(context.Background(), names, apiKey, batchSize, model, synonymsPerName)
}

func GenerateSynonymsForNamesWithContext(ctx context.Context, names []string, apiKey string, batchSize int, model string, synonymsPerName int) (SynonymResponse, error) {
	if batchSize <= 0 {
		batchSize = 8
	}

	result := make(SynonymResponse)
	for i := 0; i < len(names); i += batchSize {
		end := i + batchSize
		if end > len(names) {
			end = len(names)
		}

		batch := names[i:end]
		synonyms, err := GenerateSynonymsBatchWithContext(ctx, batch, apiKey, model, synonymsPerName)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}

		for name, values := range synonyms {
			result[name] = values
		}
	}

	return result, nil
}
