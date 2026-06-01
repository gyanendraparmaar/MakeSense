# MakeSense.ai

An AI-powered notepad. Type raw text on the left; the right pane renders structured output in real time — expense tables, to-do lists, and more.

**Stack:** Go backend · Next.js frontend · pluggable LLM providers (FreeLLMAPI, Groq, Gemini, OpenAI, Ollama, etc.)

## Quick start

**Docker (backend + FreeLLMAPI):**

```bash
cp freellmapi/.env.example freellmapi/.env   # set ENCRYPTION_KEY
cp backend/.env.example backend/.env         # set FREELLMAPI_API_KEY after dashboard setup
docker compose up -d
```

**Local dev (no Docker):**

```bash
cd backend && cp .env.example .env && go run ./cmd/makesense
cd frontend && cp .env.local.example .env.local && npm install && npm run dev
```

Open [http://localhost:3000](http://localhost:3000). See `backend/.env.example` and [`FREELLMAPI.md`](FREELLMAPI.md) for LLM setup.

## Docs

| File | What it covers |
|------|----------------|
| [`FREELLMAPI.md`](FREELLMAPI.md) | FreeLLMAPI proxy setup and provider keys |
| [`smart-notepad-plan.md`](smart-notepad-plan.md) | Product and engineering plan |
| [`DEPLOY.md`](DEPLOY.md) | Deployment (Cloud Run, Vercel) |
| [`tests/README.md`](tests/README.md) | Evaluation test suite |

## License

MIT
