// Package llm is a thin Gemini REST client plus our classifier/analyzer pipeline.
//
// We avoid the official SDK so we keep dependencies tiny and the request shape
// transparent. Docs: https://ai.google.dev/api/generate-content
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GeminiClient is a minimal client for Google AI Studio's generateContent endpoint.
type GeminiClient struct {
	APIKey string
	Model  string
	HTTP   *http.Client
}

// NewGeminiClient builds a client. If model is empty it defaults to gemini-2.0-flash.
func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiClient{
		APIKey: apiKey,
		Model:  model,
		HTTP:   &http.Client{Timeout: 60 * time.Second},
	}
}

// ModelName satisfies the JSONGenerator interface.
func (c *GeminiClient) ModelName() string { return c.Model }

// --- Request/response types --------------------------------------------------

type genReq struct {
	Contents          []content          `json:"contents"`
	SystemInstruction *content           `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
	SafetySettings    []map[string]any   `json:"safetySettings,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type generationConfig struct {
	Temperature      float64         `json:"temperature,omitempty"`
	MaxOutputTokens  int             `json:"maxOutputTokens,omitempty"`
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	ResponseSchema   json.RawMessage `json:"responseSchema,omitempty"`
}

type genResp struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// GenerateJSON calls Gemini with the given system + user prompt, asking for a
// JSON response that conforms to responseSchema. Returns the raw JSON bytes
// (already validated by the model against the schema).
func (c *GeminiClient) GenerateJSON(ctx context.Context, system, user string, schema json.RawMessage, temperature float64) (json.RawMessage, error) {
	if c.APIKey == "" {
		return nil, errors.New("gemini: GEMINI_API_KEY not set")
	}

	reqBody := genReq{
		Contents: []content{
			{Role: "user", Parts: []part{{Text: user}}},
		},
		GenerationConfig: &generationConfig{
			Temperature:      temperature,
			MaxOutputTokens:  2048,
			ResponseMimeType: "application/json",
			ResponseSchema:   schema,
		},
	}
	if system != "" {
		reqBody.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.Model, c.APIKey,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed genResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w (body: %s)", err, truncate(string(raw), 500))
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("gemini: %s (%d): %s", parsed.Error.Status, parsed.Error.Code, parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini: empty response (status=%d body=%s)", resp.StatusCode, truncate(string(raw), 300))
	}
	text := parsed.Candidates[0].Content.Parts[0].Text

	// Sanity: model should have returned JSON because we asked for
	// application/json. Validate by re-parsing.
	var sanity any
	if err := json.Unmarshal([]byte(text), &sanity); err != nil {
		return nil, fmt.Errorf("gemini: model returned non-JSON: %w (text: %s)", err, truncate(text, 300))
	}
	return json.RawMessage(text), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
