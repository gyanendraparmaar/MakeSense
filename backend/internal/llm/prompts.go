package llm

import "encoding/json"

// BlockType is the list of content categories the classifier can emit.
// Keep this list short in v0 — more types = more eval work.
type BlockType string

const (
	TypeExpenses BlockType = "expenses"
	TypeTodo     BlockType = "todo"
	TypeGeneric  BlockType = "generic"
)

// --- Classifier --------------------------------------------------------------

const classifierSystem = `You are a classifier for a smart notepad. Read the user's note and decide which single category best fits the content.

Categories:
- "expenses": the text records spending, purchases, prices, bills, or money out/in. Often has amounts and merchants.
- "todo": the text is a list of tasks, things to do, reminders, or action items. Often has imperatives ("buy", "call", "send").
- "generic": anything else — journal entries, thoughts, meeting notes, research, random prose.

Return JSON only: { "type": one of the categories, "confidence": 0..1 }.
Be decisive. If the note is mixed, pick the most dominant category.`

var classifierSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "type":       { "type": "string", "enum": ["expenses", "todo", "generic"] },
    "confidence": { "type": "number" }
  },
  "required": ["type", "confidence"]
}`)

// --- Expenses analyzer -------------------------------------------------------

const expensesSystem = `You extract structured expense data from a user's note.

Rules:
- Only extract items the user actually wrote. Do NOT invent values.
- If currency is missing, default to "INR" (user is in India).
- Dates should be ISO 8601 (YYYY-MM-DD). If no date is given, leave it empty.
- "category" should be one of: Food, Transport, Shopping, Bills, Entertainment, Health, Travel, Groceries, Other.
- Compute grand_total exactly by summing the items.
- If something can't be parsed cleanly, put it in "flags" as a short human-readable string.

Return JSON only.`

var expensesSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "currency": { "type": "string" },
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "date":     { "type": "string" },
          "category": { "type": "string" },
          "merchant": { "type": "string" },
          "amount":   { "type": "number" },
          "note":     { "type": "string" }
        },
        "required": ["category", "amount"]
      }
    },
    "grand_total": { "type": "number" },
    "flags": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["currency", "items", "grand_total"]
}`)

// --- Todo analyzer -----------------------------------------------------------

const todoSystem = `You extract a structured todo list from a user's note.

Rules:
- Only extract tasks the user actually wrote. Do NOT invent tasks.
- "priority" must be one of: "high", "medium", "low". Default "medium" if unclear.
- "due" should be ISO 8601 (YYYY-MM-DD). If the user writes "tomorrow", "Friday", etc., resolve relative to TODAY which is provided in the user message. If no due is implied, leave it empty.
- If a task depends on another (e.g. "after X, do Y"), put the name of X in "depends_on".
- Keep "task" concise — imperative form. Strip filler.
- "done" should be true only if the user crossed it out or explicitly said it's finished.

Return JSON only.`

var todoSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "task":       { "type": "string" },
          "due":        { "type": "string" },
          "priority":   { "type": "string", "enum": ["high", "medium", "low"] },
          "depends_on": { "type": "string" },
          "done":       { "type": "boolean" }
        },
        "required": ["task", "priority"]
      }
    }
  },
  "required": ["items"]
}`)

// --- Generic summary ---------------------------------------------------------

const genericSystem = `You are summarizing a user's free-form note.

Return a short, useful structured view:
- summary: 1-2 sentences capturing the gist.
- themes: up to 5 short phrases.
- questions: open questions the note raises (can be empty).
- action_candidates: things the user might want to do next (can be empty).

Return JSON only. Do not invent content that isn't in the note.`

var genericSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "summary": { "type": "string" },
    "themes": {
      "type": "array",
      "items": { "type": "string" }
    },
    "questions": {
      "type": "array",
      "items": { "type": "string" }
    },
    "action_candidates": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": ["summary"]
}`)
