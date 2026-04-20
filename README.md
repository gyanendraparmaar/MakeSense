# MakeSense.ai

An AI-powered notepad. You type raw text on the left; the right pane renders
structured, analyzed output in real-time — expense tables, to-do lists, and
more, generated automatically as you type.

## Monorepo layout

```
MakeSense/
├── backend/          Go API server (SSE streaming, Gemini, SQLite)
├── frontend/         Next.js 14 (App Router, TipTap editor, Tailwind)
├── smart-notepad-plan.md   Product + engineering design doc
└── README.md
```

## Quick start (local dev)

### 1. Backend (Go)

```bash
cd backend
cp .env.example .env
# edit .env and set GEMINI_API_KEY=...

go mod tidy
go run ./cmd/makesense
# server listens on :8080
```

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
       ├─► Gemini classifier  ("is this expenses? todo? journal?")
       │
       └─► Specialist analyzer (structured JSON)
              │
              ▼
        SQLite (notes + cached analyses)
```

Two analyzers are wired up in v0: **expenses** (→ table) and **todo** (→ checklist).
Unclassified / mixed content falls through to a generic summary.

## Environment variables

### Backend (`backend/.env`)

| Var | Required | Example | Purpose |
|-----|----------|---------|---------|
| `GEMINI_API_KEY` | yes | `AIza…` | Google AI Studio key |
| `GEMINI_MODEL` | no | `gemini-2.0-flash` | Override default model |
| `PORT` | no | `8080` | HTTP port |
| `DB_PATH` | no | `./makesense.db` | SQLite file |
| `ALLOWED_ORIGIN` | no | `http://localhost:3000` | CORS origin |

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
