package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	openRouterURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultModel  = "openrouter/free"
)

type OpenRouterRequest struct {
	Model    string          `json:"model"`
	Messages []Message       `json:"messages"`
	Format   json.RawMessage `json:"format,omitempty"`
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
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}
	return key, nil
}

func GenerateSynonymsBatch(names []string, apiKey string, model string) (SynonymResponse, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if len(names) == 0 {
		return make(SynonymResponse), nil
	}

	if model == "" {
		model = defaultModel
	}

	systemPrompt := "You are a helpful assistant. For each folder or file name in the list below, generate 4-6 plausible alternative words or phrases a developer might use to refer to it in a codebase. Output ONLY a valid JSON object like {\"name1\": [\"syn1\", \"syn2\", ...], \"name2\": [...]} with no extra text."

	namesList := strings.Join(names, "\n")
	userContent := fmt.Sprintf("Generate synonyms for these names:\n%s", namesList)

	reqBody := OpenRouterRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		Format: json.RawMessage(`{"type": "json_object"}`),
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", openRouterURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/contexting")
	req.Header.Set("X-Title", "Contexting")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return make(SynonymResponse), nil
	}

	content := apiResp.Choices[0].Message.Content

	var synonyms SynonymResponse
	if err := json.Unmarshal([]byte(content), &synonyms); err != nil {
		return nil, fmt.Errorf("failed to parse synonyms: %w", err)
	}

	return synonyms, nil
}

func GenerateSynonymsForNames(names []string, apiKey string, batchSize int, model string) (SynonymResponse, error) {
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
		synonyms, err := GenerateSynonymsBatch(batch, apiKey, model)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}

		for name, syns := range synonyms {
			result[name] = syns
		}
	}

	return result, nil
}
