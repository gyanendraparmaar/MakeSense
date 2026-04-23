# MakeSense.ai

An AI-powered notepad. You type raw text on the left; the right pane renders
structured, analyzed output in real-time — expense tables, to-do lists, and
more, generated automatically as you type.

Ships with a **pluggable LLM backend** so you can run it on whichever model
provider is cheapest/fastest for you: Groq (default, free), Gemini, OpenAI,
OpenRouter, Ollama (fully local), or any OpenAI-compatible endpoint.

## Monorepo layout

```
MakeSense/
├── backend/          Go API server (SSE streaming, multi-LLM, SQLite)
│   └── internal/llm/ Provider clients: gemini.go, openai.go, provider.go, pipeline.go
├── frontend/         Next.js 14 (App Router, TipTap editor, Tailwind)
├── smart-notepad-plan.md   Product + engineering design doc
└── README.md
```

## Quick start (local dev)

### 1. Backend (Go)

```bash
cd backend
cp .env.example .env
# edit .env — by default LLM_PROVIDER=groq, so just set GROQ_API_KEY=...
# (or flip LLM_PROVIDER and fill in the matching key)

go mod tidy
go run ./cmd/makesense
# server listens on :8080
```

Grab a free Groq key at https://console.groq.com/keys (no card required) or a
Gemini key at https://aistudio.google.com/app/apikey.

### 2. Frontend (Next.js)

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
# open http://localhost:3000
```

## Architecture at a glance

```
Browser (Next.js + TipTap)
       │   debounced POST /api/analyze
       ▼
   Go API (chi, SSE)
       │
       ├─► LLM classifier  ("is this expenses? todo? journal?")
       │
       └─► Specialist analyzer (structured JSON)
              │
              ▼
        SQLite (notes + cached analyses)
```

The classifier and analyzer both hit the same `JSONGenerator` interface, so
the model provider is a runtime choice (see below). Two analyzers are wired up
in v0: **expenses** (→ table) and **todo** (→ checklist). Unclassified / mixed
content falls through to a generic summary.

## LLM providers

Set `LLM_PROVIDER` in `backend/.env` to any of the following. Each provider
reads its own API-key env var so you can keep several keys side-by-side and
flip between them by changing a single line.

| Provider | `LLM_PROVIDER` | API key env | Default model | Notes |
|---|---|---|---|---|
| Groq | `groq` | `GROQ_API_KEY` | `llama-3.3-70b-versatile` | Default. Free + fast, OpenAI-compatible. |
| Google Gemini | `gemini` | `GEMINI_API_KEY` | `gemini-2.0-flash` | Native structured-output support. |
| OpenAI | `openai` | `OPENAI_API_KEY` | `gpt-4o-mini` | Paid. |
| OpenRouter | `openrouter` | `OPENROUTER_API_KEY` | `meta-llama/llama-3.3-70b-instruct:free` | Aggregates many models. |
| Ollama | `ollama` | _(none)_ | `llama3.1:8b` | 100% local; run `ollama pull llama3.1:8b` first. |
| Any OpenAI-compatible | `openai-compatible` | `LLM_API_KEY` | `LLM_MODEL` (required) | Also requires `LLM_BASE_URL`. Works with Together, Cerebras, DeepSeek, LM Studio, vLLM, … |

Health check the running server: `curl localhost:8080/api/health` returns
`{"ok":true,"provider":"groq","model":"llama-3.3-70b-versatile"}`.

## Environment variables

### Backend (`backend/.env`)

| Var | Required | Default | Purpose |
|-----|----------|---------|---------|
| `LLM_PROVIDER` | no | `gemini` | Which provider to use (see table above). |
| `GROQ_API_KEY` / `GEMINI_API_KEY` / … | yes* | — | Key for whichever provider is active. *Not required for `ollama`. |
| `GROQ_MODEL` / `GEMINI_MODEL` / … | no | provider default | Override the default model. |
| `LLM_BASE_URL` | only for `openai-compatible` | — | e.g. `https://api.together.xyz/v1` |
| `PORT` | no | `8080` | HTTP port |
| `DB_PATH` | no | `./makesense.db` | SQLite file |
| `ALLOWED_ORIGIN` | no | `http://localhost:3000` | CORS origin |

See `backend/.env.example` for the full, annotated list.

### Frontend (`frontend/.env.local`)

| Var | Required | Example |
|-----|----------|---------|
| `NEXT_PUBLIC_API_URL` | yes | `http://localhost:8080` |

## Deployment

See `DEPLOY.md` for step-by-step deployment on:

- **Backend:** Google Cloud Run (generous free tier; pairs with Gemini)
- **Frontend:** Vercel (free hobby tier)
- **Database:** Turso (free libSQL/SQLite cloud) or Cloud SQL

## License

MIT
