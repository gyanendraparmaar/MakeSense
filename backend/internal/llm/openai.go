package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatibleClient talks to any OpenAI-style /chat/completions endpoint:
// Groq, OpenAI, OpenRouter, Cerebras, Together, Ollama, LM Studio, DeepSeek, …
//
// We ask the model for a JSON object via response_format=json_object and
// embed the JSON schema into the system prompt. Not every provider honors a
// full `json_schema` response format yet, but every serious one honors
// `json_object`, so this is the lowest-common-denominator that works
// everywhere.
type OpenAICompatibleClient struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

// NewOpenAICompatibleClient builds a client. baseURL must include the API
// prefix (e.g. "https://api.groq.com/openai/v1" or "http://localhost:11434/v1").
func NewOpenAICompatibleClient(apiKey, model, baseURL string) *OpenAICompatibleClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &OpenAICompatibleClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// ModelName satisfies the JSONGenerator interface.
func (c *OpenAICompatibleClient) ModelName() string { return c.Model }

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiResponseFormat struct {
	Type string `json:"type"`
}

type oaiReq struct {
	Model          string             `json:"model"`
	Messages       []oaiMessage       `json:"messages"`
	Temperature    float64            `json:"temperature"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	ResponseFormat *oaiResponseFormat `json:"response_format,omitempty"`
}

type oaiResp struct {
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error,omitempty"`
}

// GenerateJSON sends a chat completion request and returns the parsed JSON
// content from the first choice. The schema is injected into the system
// prompt because not every provider supports a structured `json_schema`
// response format; every major provider does support `json_object`.
//
// Weaker models routed via FreeLLMAPI "auto" sometimes reply with prose before
// the JSON object, or ignore json_object mode entirely. We extract embedded
// JSON when possible and retry once with a stricter prompt before failing.
func (c *OpenAICompatibleClient) GenerateJSON(ctx context.Context, system, user string, schema json.RawMessage, temperature float64) (json.RawMessage, error) {
	if c.APIKey == "" && !strings.Contains(c.BaseURL, "localhost") && !strings.Contains(c.BaseURL, "127.0.0.1") {
		return nil, errors.New("openai-compatible: API key not set")
	}

	sys := augmentSystemForJSON(system, schema)

	text, err := c.chatCompletion(ctx, sys, user, temperature)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseModelJSON(text); ok {
		return parsed, nil
	}

	// One retry — common with small free-tier models behind auto routing.
	retryUser := user + "\n\nYour previous answer was not valid JSON. Reply with ONLY one raw JSON object — no explanation, no markdown fences, no text before or after."
	text, err = c.chatCompletion(ctx, sys, retryUser, 0)
	if err != nil {
		return nil, err
	}
	if parsed, ok := parseModelJSON(text); ok {
		return parsed, nil
	}
	return nil, fmt.Errorf("openai-compatible: model returned non-JSON after retry (text: %s)", truncate(text, 300))
}

func augmentSystemForJSON(system string, schema json.RawMessage) string {
	if len(schema) > 0 {
		return system + "\n\nReturn a single JSON object that conforms exactly to this JSON Schema:\n" +
			string(schema) +
			"\n\nRespond with JSON only — no markdown fences, no commentary, no trailing text."
	}
	if !strings.Contains(strings.ToLower(system), "json") {
		return system + "\n\nReturn JSON only."
	}
	return system
}

func (c *OpenAICompatibleClient) chatCompletion(ctx context.Context, system, user string, temperature float64) (string, error) {
	body := oaiReq{
		Model:       c.Model,
		Temperature: temperature,
		MaxTokens:   2048,
		Messages: []oaiMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ResponseFormat: &oaiResponseFormat{Type: "json_object"},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode >= 400 {
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "application/json") {
			var parsed oaiResp
			if jerr := json.Unmarshal(respRaw, &parsed); jerr == nil && parsed.Error != nil {
				return "", fmt.Errorf("openai-compatible: http %d: %s: %s", resp.StatusCode, parsed.Error.Type, parsed.Error.Message)
			}
		}
		hint := ""
		if resp.StatusCode == http.StatusTooManyRequests {
			hint = " (rate limited — FreeLLMAPI will failover if upstream keys are configured; add keys at http://localhost:3001)"
		}
		return "", fmt.Errorf("openai-compatible: http %d%s: %s", resp.StatusCode, hint, truncate(string(respRaw), 300))
	}

	var parsed oaiResp
	if err := json.Unmarshal(respRaw, &parsed); err != nil {
		return "", fmt.Errorf("openai-compatible: decode response: %w (body: %s)", err, truncate(string(respRaw), 500))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("openai-compatible: %s: %s", parsed.Error.Type, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai-compatible: empty response (status=%d body=%s)", resp.StatusCode, truncate(string(respRaw), 300))
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// parseModelJSON accepts raw JSON, fenced ```json blocks, or prose with an
// embedded {...} object (common when weaker auto-routed models narrate first).
func parseModelJSON(text string) (json.RawMessage, bool) {
	text = stripMarkdownFence(strings.TrimSpace(text))
	if text == "" {
		return nil, false
	}
	if j, ok := tryJSON(text); ok {
		return j, true
	}
	if j, ok := extractJSONObject(text); ok {
		return j, true
	}
	return nil, false
}

func tryJSON(s string) (json.RawMessage, bool) {
	var sanity any
	if err := json.Unmarshal([]byte(s), &sanity); err != nil {
		return nil, false
	}
	return json.RawMessage(s), true
}

func extractJSONObject(s string) (json.RawMessage, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		if end, ok := matchingBraceEnd(s, i); ok {
			if j, ok := tryJSON(s[i : end+1]); ok {
				return j, true
			}
		}
	}
	return nil, false
}

func matchingBraceEnd(s string, start int) (int, bool) {
	if start >= len(s) || s[start] != '{' {
		return -1, false
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return -1, false
}

// stripMarkdownFence removes ```json … ``` wrappers some models add even when
// asked not to.
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
