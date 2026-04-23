package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JSONGenerator is the minimal contract the pipeline needs from an LLM backend.
// Any provider (Gemini, Groq, OpenAI, Ollama, …) that can take a system +
// user prompt and return a JSON object conforming to a schema implements this.
type JSONGenerator interface {
	GenerateJSON(ctx context.Context, system, user string, schema json.RawMessage, temperature float64) (json.RawMessage, error)
	ModelName() string
}

// Pipeline orchestrates classification + specialist analysis.
//
//	raw text ──► classifier ──► {expenses | todo | generic}
//	                 │
//	                 ▼
//	          specialist analyzer ──► structured JSON
type Pipeline struct {
	gen JSONGenerator
}

func NewPipeline(gen JSONGenerator) *Pipeline {
	return &Pipeline{gen: gen}
}

// AnalysisResult is the full output the API returns for one analyze call.
type AnalysisResult struct {
	Type       BlockType       `json:"type"`
	Confidence float64         `json:"confidence"`
	Structured json.RawMessage `json:"structured"`
	Model      string          `json:"model"`
}

// ClassifyResult is just the classifier output.
type ClassifyResult struct {
	Type       BlockType `json:"type"`
	Confidence float64   `json:"confidence"`
}

// Classify picks a block type for the given text.
func (p *Pipeline) Classify(ctx context.Context, text string) (*ClassifyResult, error) {
	raw, err := p.gen.GenerateJSON(ctx, classifierSystem, text, classifierSchema, 0.1)
	if err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	var out ClassifyResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("classify: decode: %w", err)
	}
	// Defensive: if model emits a type we don't support, collapse to generic.
	switch out.Type {
	case TypeExpenses, TypeTodo, TypeGeneric:
	default:
		out.Type = TypeGeneric
		if out.Confidence > 0.5 {
			out.Confidence = 0.5
		}
	}
	return &out, nil
}

// Analyze runs classifier + specialist and returns the full structured output.
func (p *Pipeline) Analyze(ctx context.Context, text string) (*AnalysisResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty input")
	}
	cls, err := p.Classify(ctx, text)
	if err != nil {
		return nil, err
	}
	return p.AnalyzeWith(ctx, text, cls)
}

// AnalyzeWith runs ONLY the specialist analyzer, reusing an already-computed
// classification. Use this from the SSE handler so we don't classify twice.
func (p *Pipeline) AnalyzeWith(ctx context.Context, text string, cls *ClassifyResult) (*AnalysisResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("empty input")
	}

	var (
		system string
		schema json.RawMessage
		temp   = 0.2
	)
	switch cls.Type {
	case TypeExpenses:
		system = expensesSystem
		schema = expensesSchema
	case TypeTodo:
		system = todoSystem
		schema = todoSchema
	default:
		cls.Type = TypeGeneric
		system = genericSystem
		schema = genericSchema
		temp = 0.4
	}

	// For todo, include today's date so relative expressions can resolve.
	userPrompt := text
	if cls.Type == TypeTodo {
		userPrompt = fmt.Sprintf("TODAY: %s\n\nNOTE:\n%s", time.Now().UTC().Format("2006-01-02"), text)
	}

	structured, err := p.gen.GenerateJSON(ctx, system, userPrompt, schema, temp)
	if err != nil {
		return nil, fmt.Errorf("analyze (%s): %w", cls.Type, err)
	}

	return &AnalysisResult{
		Type:       cls.Type,
		Confidence: cls.Confidence,
		Structured: structured,
		Model:      p.gen.ModelName(),
	}, nil
}
