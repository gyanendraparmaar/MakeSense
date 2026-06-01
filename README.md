# MakeSense.ai

An AI-powered notepad. You type raw text on the left; the right pane renders
structured, analyzed output in real-time — expense tables, to-do lists, and
more, generated automatically as you type.

Ships with a **pluggable LLM backend**. The recommended setup routes through **FreeLLMAPI** (`freellmapi/`) — a local proxy that stacks free tiers from 16 providers with automatic failover when one is rate-limited. You can also point directly at Groq, Gemini, OpenAI, OpenRouter, Ollama, or any OpenAI-compatible endpoint.

```
MakeSense/
├── backend/          Go API server (SSE streaming, multi-LLM, SQLite)
│   └── internal/llm/ Provider clients: gemini.go, openai.go, provider.go, pipeline.go
├── frontend/         Next.js 14 (App Router, TipTap editor, Tailwind)
├── freellmapi/       FreeLLMAPI proxy (optional, recommended for rate-limit resilience)
├── FREELLMAPI.md     Setup guide + provider key signup steps
├── smart-notepad-plan.md   Product + engineering design doc
└── README.md
```

## Quick start (local dev)

### 1. Backend (Go)

**Recommended — via FreeLLMAPI** (automatic failover across free tiers):

```bash
# Start FreeLLMAPI (if not already running)
cd freellmapi
ENCRYPTION_KEY="$(openssl rand -hex 32)"
printf "ENCRYPTION_KEY=%s\nPORT=3001\n" "$ENCRYPTION_KEY" > .env
docker compose up -d
# Open http://localhost:3001 → add provider keys (see FREELLMAPI.md)

cd ../backend
cp .env.example .env
# Set FREELLMAPI_API_KEY to the unified key from the FreeLLMAPI Keys page

go mod tidy
go run ./cmd/makesense
# server listens on :8080
```

**Direct single provider** (simpler, but hits rate limits on one provider):

```bash
cd backend
cp .env.example .env
# Set LLM_PROVIDER=groq and GROQ_API_KEY=... (or gemini, etc.)

go mod tidy
go run ./cmd/makesense
```

See `FREELLMAPI.md` for the full provider key signup guide.

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
| **FreeLLMAPI** | `freellmapi` | `FREELLMAPI_API_KEY` | `auto` | **Recommended.** Local proxy; add upstream keys in dashboard. See `FREELLMAPI.md`. |
| Groq | `groq` | `GROQ_API_KEY` | `llama-3.3-70b-versatile` | Free + fast, single provider. |
| Google Gemini | `gemini` | `GEMINI_API_KEY` | `gemini-2.0-flash` | Native structured-output support. |
| OpenAI | `openai` | `OPENAI_API_KEY` | `gpt-4o-mini` | Paid. |
| OpenRouter | `openrouter` | `OPENROUTER_API_KEY` | `meta-llama/llama-3.3-70b-instruct:free` | Aggregates many models. |
| Ollama | `ollama` | _(none)_ | `llama3.1:8b` | 100% local; run `ollama pull llama3.1:8b` first. |
| Any OpenAI-compatible | `openai-compatible` | `LLM_API_KEY` | `LLM_MODEL` (required) | Also requires `LLM_BASE_URL`. Works with Together, Cerebras, DeepSeek, LM Studio, vLLM, … |

Health check the running server: `curl localhost:8080/health` returns
`{"ok":true,"provider":"freellmapi","model":"auto"}`.

## Environment variables

### Backend (`backend/.env`)

| Var | Required | Default | Purpose |
|-----|----------|---------|---------|
| `LLM_PROVIDER` | no | `freellmapi` | Which provider to use (see table above). |
| `FREELLMAPI_API_KEY` | when using freellmapi | — | Unified key from FreeLLMAPI dashboard |
| `FREELLMAPI_BASE_URL` | no | `http://localhost:3001/v1` | FreeLLMAPI endpoint |
| `FREELLMAPI_MODEL` | no | `auto` | Model or `auto` for smart routing |
| `GROQ_API_KEY` / `GEMINI_API_KEY` / … | yes* | — | Key for whichever direct provider is active. *Not required for `ollama` or `freellmapi`. |
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
