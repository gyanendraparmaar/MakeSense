# MakeSense.ai

An AI-powered notepad. Type raw text on the left; the right pane renders structured output in real time — expense tables, to-do lists, and more.

**Stack:** Go backend · Next.js frontend · pluggable LLM providers (FreeLLMAPI, Groq, Gemini, OpenAI, Ollama, etc.)

## Quick start

```bash
# Backend
cd backend && cp .env.example .env && go run ./cmd/makesense

# Frontend
cd frontend && cp .env.local.example .env.local && npm install && npm run dev
```

Open [http://localhost:3000](http://localhost:3000). Configure LLM keys in `backend/.env` — see `backend/.env.example` and `FREELLMAPI.md` for setup options.

## Docs

| File | What it covers |
|------|----------------|
| [`FREELLMAPI.md`](FREELLMAPI.md) | FreeLLMAPI proxy setup and provider keys |
| [`smart-notepad-plan.md`](smart-notepad-plan.md) | Product and engineering plan |
| [`DEPLOY.md`](DEPLOY.md) | Deployment (Cloud Run, Vercel) |
| [`tests/README.md`](tests/README.md) | Evaluation test suite |

## License

MIT
