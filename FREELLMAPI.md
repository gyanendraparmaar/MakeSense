# FreeLLMAPI — Overview & MakeSense Integration Guide

This document explains what [FreeLLMAPI](https://github.com/tashfeenahmed/freellmapi) does, how it works internally, and how to run it alongside MakeSense so your notepad can route LLM calls through a single OpenAI-compatible proxy instead of managing many provider keys directly.

---

## What is FreeLLMAPI?

FreeLLMAPI is a **self-hosted OpenAI-compatible proxy** that aggregates the free tiers of **16 LLM providers** (~1.7 billion tokens/month combined) behind one endpoint:

```
POST /v1/chat/completions
GET  /v1/models
POST /v1/responses          (Codex CLI shim)
```

Instead of wiring your app to Groq, Gemini, OpenRouter, Cerebras, etc. separately, you:

1. Run FreeLLMAPI locally (Docker or Node.js).
2. Add your upstream provider API keys once in its admin dashboard.
3. Point any OpenAI-compatible client at `http://localhost:3001/v1` with a single **unified API key** (`freellmapi-…`).

MakeSense already supports this pattern via its `openai-compatible` LLM provider — no backend code changes are required.

---

## Why it exists

Every major AI lab offers a free tier (millions of tokens/month, thousands of requests/day). Individually, each tier is limited. Stacked together, they add up to substantial inference capacity across 100+ models.

The pain points FreeLLMAPI solves:

| Problem | FreeLLMAPI solution |
|---------|-------------------|
| 16 different SDKs and auth flows | One OpenAI-compatible API |
| Different rate limits per provider | Per-key RPM/RPD/TPM/TPD tracking + cooldowns |
| One provider hits 429 → request fails | Automatic failover across a priority chain |
| Keys scattered in env vars | Encrypted storage (AES-256-GCM in SQLite) |
| Hard to see which model served a call | `X-Routed-Via: platform/model` response header |

**Intended use:** personal experimentation and prototyping — not production. See [Limitations](#limitations) and [Terms of Service](#terms-of-service-notes).

---

## Repository layout

The upstream repo (also cloned locally at `freellmapi/` in this monorepo) is a TypeScript monorepo:

```
freellmapi/
├── server/                 Express API + proxy + router
│   ├── src/
│   │   ├── routes/
│   │   │   ├── proxy.ts        OpenAI /v1/chat/completions
│   │   │   └── responses.ts    OpenAI /v1/responses shim
│   │   ├── services/
│   │   │   ├── router.ts       Model selection + failover
│   │   │   ├── ratelimit.ts    RPM/RPD/TPM/TPD ledger
│   │   │   └── health.ts       Periodic key health probes
│   │   ├── providers/          One adapter per upstream (google.ts, groq.ts, …)
│   │   ├── db/                 SQLite schema + seed data
│   │   └── lib/crypto.ts       AES-256-GCM key encryption
│   └── __tests__/              Vitest suite
├── client/                 React + Vite admin dashboard
│   └── src/pages/          Keys, Fallback Chain, Playground, Analytics
├── shared/                 Shared TypeScript types
├── docker-compose.yml      Recommended deployment path
└── Dockerfile              Multi-arch image (amd64 + arm64)
```

**Tech stack:** Node.js 20+, Express, SQLite (`better-sqlite3`), React + Vite + shadcn/ui.

---

## How it works (request flow)

```
┌──────────────────┐   Bearer freellmapi-…   ┌─────────────────────────┐
│  MakeSense       │ ──────────────────────▶ │  Express proxy (:3001)  │
│  (Go backend)    │ ◀────────────────────── │  /v1/chat/completions   │
└──────────────────┘      JSON response       └────────────┬────────────┘
                                                          │
                                                          ▼
                             ┌────────────────────────────────────────────────┐
                             │  Router (server/src/services/router.ts)        │
                             │   1. Pick highest-priority model that:         │
                             │      (a) has a healthy key                     │
                             │      (b) is under all rate limits              │
                             │   2. Decrypt upstream key, call provider       │
                             │   3. On 429/5xx/timeout → cooldown + retry     │
                             │      next model (up to 20 attempts)            │
                             └────────────────────────────────────────────────┘
                                          │
        ┌──────────────┬────────────┬─────┴──────┬─────────────┬──────────┐
        ▼              ▼            ▼            ▼             ▼          ▼
     Google          Groq       Cerebras    OpenRouter    Mistral    … +10
```

### Core components

| Component | Role |
|-----------|------|
| **Router** | Picks a model per request from your ordered fallback chain. Supports `model: "auto"` for smart routing. Sticky sessions keep multi-turn chats on the same model for 30 minutes. |
| **Rate-limit ledger** | In-memory RPM/RPD/TPM/TPD counters backed by SQLite. Keys that return 429 get a short cooldown. |
| **Provider adapters** | One file per upstream (`server/src/providers/*.ts`). Each implements `chatCompletion()` and `streamChatCompletion()`. |
| **Health service** | Periodic probes mark keys as `healthy`, `rate_limited`, `invalid`, or `error`. |
| **Dashboard** | Web UI at `:3001` for keys, fallback chain ordering, playground, and analytics. |
| **Storage** | SQLite with AES-256-GCM encrypted provider keys. Requires a stable `ENCRYPTION_KEY` across restarts. |

### Supported upstream providers

Google Gemini, Groq, Cerebras, SambaNova, Mistral, OpenRouter, GitHub Models, Cloudflare Workers AI, Cohere, Z.ai (Zhipu), NVIDIA NIM, HuggingFace, Ollama Cloud, Kilo Gateway, Pollinations, LLM7, plus a **custom** slot for any OpenAI-compatible endpoint (Ollama local, LM Studio, vLLM, llama.cpp, etc.).

Some providers work without an API key (Pollinations, LLM7, Kilo free routes).

### Features relevant to MakeSense

- **OpenAI-compatible chat completions** — MakeSense's `OpenAICompatibleClient` (`backend/internal/llm/openai.go`) posts to `/chat/completions` with `response_format: json_object`, which FreeLLMAPI passes through.
- **Streaming and non-streaming** — both supported upstream.
- **Automatic failover** — if Groq is rate-limited, the router tries the next model in your chain (e.g. Gemini, then OpenRouter free tier).
- **Unified auth** — MakeSense only needs one `freellmapi-…` key; upstream keys never leave the proxy.

### Not yet supported

Embeddings, image generation, audio, legacy `/v1/completions`, moderation, `n > 1`, and multi-tenant billing.

---

## How MakeSense fits in

MakeSense's backend selects an LLM provider via `LLM_PROVIDER` in `backend/.env`. Use the dedicated **`freellmapi`** provider (recommended):

```env
LLM_PROVIDER=freellmapi
FREELLMAPI_API_KEY=freellmapi-your-unified-key-from-dashboard
FREELLMAPI_BASE_URL=http://localhost:3001/v1
FREELLMAPI_MODEL=auto
```

Using `FREELLMAPI_MODEL=auto` lets FreeLLMAPI's router pick the best available model from your fallback chain. You can also pin a specific model (e.g. `gemini-2.5-flash`, `llama-3.3-70b-versatile`).

The generic `openai-compatible` provider also works if you prefer:

```env
LLM_PROVIDER=openai-compatible
LLM_BASE_URL=http://localhost:3001/v1
LLM_API_KEY=freellmapi-your-unified-key-from-dashboard
LLM_MODEL=auto
```

---

## Implementation steps

### Prerequisites

- **Docker + Docker Compose** (recommended), or Node.js 20+ for local dev
- **OpenSSL** (to generate `ENCRYPTION_KEY`)
- At least one upstream provider API key (Groq and Google are good starting points — both free, no card)

---

### Step 1 — Start FreeLLMAPI

#### Option A: Docker Compose (recommended)

A copy of the upstream repo already lives at `freellmapi/` in this workspace.

```bash
cd freellmapi

# Generate encryption key for at-rest key storage
ENCRYPTION_KEY="$(openssl rand -hex 32)"
printf "ENCRYPTION_KEY=%s\nPORT=3001\n" "$ENCRYPTION_KEY" > .env

docker compose up -d
```

Verify it is running:

```bash
curl -s http://localhost:3001/api/ping
docker compose logs -f freellmapi
```

Open the dashboard: **http://localhost:3001**

> **LAN access:** By default the container binds to `127.0.0.1` only. To reach it from another machine on your network:
>
> ```bash
> HOST_BIND=0.0.0.0 docker compose up -d
> ```
>
> Only do this on a trusted network — the proxy is single-user and protected only by the unified API key.

#### Option B: Pull pre-built image (no clone needed)

```bash
docker pull ghcr.io/tashfeenahmed/freellmapi:latest
# Use the docker-compose.yml from the repo, or run the image directly with
# ENCRYPTION_KEY, PORT=3001, and a volume for /app/server/data
```

#### Option C: Local Node.js development

```bash
cd freellmapi
npm install
cp .env.example .env
ENCRYPTION_KEY="$(node -e 'console.log(require("crypto").randomBytes(32).toString("hex"))')"
printf "ENCRYPTION_KEY=%s\nPORT=3001\n" "$ENCRYPTION_KEY" >> .env
npm run dev
# API on :3001, Vite dashboard on :5173
```

---

### Step 2 — First-run dashboard setup

1. Open **http://localhost:3001** (or `:5173` in dev mode).
2. **Create an admin account** (email + password) on first visit. This gates the dashboard and `/api/*` routes.
3. Go to the **Keys** page and add upstream provider API keys:
   - **Groq** — https://console.groq.com/keys
   - **Google Gemini** — https://aistudio.google.com/app/apikey
   - **OpenRouter** — https://openrouter.ai/keys (many free `:free` models)
   - Add others as needed from the provider list above.
4. Copy the **unified API key** from the Keys page header (`freellmapi-…`). This is what MakeSense uses — not the upstream keys.
5. Go to **Fallback Chain** and reorder models by priority. Put your preferred fast/smart models at the top (e.g. Gemini 2.5 Flash, Groq Llama 3.3 70B). Disable models you don't have keys for.
6. Click **Check all** on the Keys page — each key should show a green **healthy** dot before MakeSense will get responses.

---

## How to get API keys for every provider

FreeLLMAPI stores **upstream** provider keys in its dashboard. MakeSense only needs the **unified** `freellmapi-…` key. Add upstream keys at **http://localhost:3001 → Keys → Add a provider key**.

**Priority order for MakeSense** (fastest to set up, best free tiers):

1. **Groq + Google Gemini + OpenRouter** — no credit card, generous limits, good JSON output
2. **Pollinations + LLM7 + Kilo** — no signup required (use placeholder key `anon`)
3. **GitHub Models, Cerebras, Mistral, SambaNova** — free signup, moderate limits
4. **Cloudflare, HuggingFace, Ollama Cloud, NVIDIA, Zhipu** — useful extras once basics work

### Quick wins (no signup)

These three accept anonymous traffic. In FreeLLMAPI, select the platform and paste any non-empty string as the API key (e.g. `anon`):

| FreeLLMAPI platform | Dashboard label | API key to paste | Notes |
|---------------------|-----------------|------------------|-------|
| `pollinations` | Pollinations (anon ok) | `anon` | GPT-OSS 20B, ~anon tier |
| `llm7` | LLM7 (anon ok) | `anon` | ~100 req/hr, several models |
| `kilo` | Kilo Gateway (anon ok) | `anon` | ~200 req/hr on `:free` routes |

After adding, click **Check** — status should turn **healthy**.

### Provider-by-provider signup guide

| # | FreeLLMAPI platform | Sign up / get key | What to paste in FreeLLMAPI |
|---|---------------------|-------------------|----------------------------|
| 1 | **Google AI Studio** (`google`) | 1. Go to [aistudio.google.com/app/apikey](https://aistudio.google.com/app/apikey)<br>2. Sign in with Google<br>3. Click **Create API key** | The `AIza…` key string |
| 2 | **Groq** (`groq`) | 1. Go to [console.groq.com](https://console.groq.com)<br>2. Create account (no card)<br>3. **API Keys → Create API Key** at [console.groq.com/keys](https://console.groq.com/keys) | The `gsk_…` key |
| 3 | **OpenRouter** (`openrouter`) | 1. Go to [openrouter.ai](https://openrouter.ai)<br>2. Sign up<br>3. **Keys** at [openrouter.ai/keys](https://openrouter.ai/keys) | The `sk-or-…` key. Use `:free` models in Fallback Chain |
| 4 | **GitHub Models** (`github`) | 1. Go to [github.com/settings/tokens](https://github.com/settings/tokens)<br>2. **Generate new token (classic)**<br>3. Enable the **`models`** scope (read-only is enough)<br>4. Or use a fine-grained PAT with Models access | The `github_pat_…` or `ghp_…` token |
| 5 | **Cerebras** (`cerebras`) | 1. Go to [cloud.cerebras.ai](https://cloud.cerebras.ai)<br>2. Sign up (no card for free tier)<br>3. **API Keys** in the dashboard | The API key from Cerebras Cloud |
| 6 | **SambaNova** (`sambanova`) | 1. Go to [cloud.sambanova.ai](https://cloud.sambanova.ai)<br>2. Create account<br>3. Generate an API key in the developer portal | The SambaNova API key |
| 7 | **Mistral** (`mistral`) | 1. Go to [console.mistral.ai](https://console.mistral.ai)<br>2. Sign up<br>3. **API Keys** → create key | The Mistral API key |
| 8 | **Cloudflare Workers AI** (`cloudflare`) | 1. Go to [dash.cloudflare.com](https://dash.cloudflare.com)<br>2. **Workers & Pages → AI → API Tokens**<br>3. Copy your **Account ID** (right sidebar on dashboard home)<br>4. Create an API token with Workers AI read permission | **Two fields in FreeLLMAPI:** Account ID + API token. FreeLLMAPI stores it as `accountId:token` automatically when you fill both fields |
| 9 | **HuggingFace Router** (`huggingface`) | 1. Go to [huggingface.co/settings/tokens](https://huggingface.co/settings/tokens)<br>2. Create account<br>3. **New token** with **Inference** (read) scope | The `hf_…` token |
| 10 | **Zhipu AI / Z.ai** (`zhipu`) | 1. Go to [open.bigmodel.cn](https://open.bigmodel.cn) or [docs.z.ai](https://docs.z.ai)<br>2. Register<br>3. Create API key in the developer console | The Zhipu API key |
| 11 | **Ollama Cloud** (`ollama`) | 1. Go to [ollama.com](https://ollama.com)<br>2. Sign up for free cloud plan<br>3. **Settings → API keys** | The Ollama API key |
| 12 | **NVIDIA NIM** (`nvidia`) | 1. Go to [build.nvidia.com](https://build.nvidia.com)<br>2. Sign in with NVIDIA account<br>3. Generate API key from any model page | The `nvapi-…` key. Trial/eval use only |
| 13 | **Cohere** (`cohere`) | 1. Go to [dashboard.cohere.com](https://dashboard.cohere.com)<br>2. Sign up for trial<br>3. **API Keys** | Trial API key. ⚠️ ToS restricts personal use — see disclaimer |
| 14 | **Kilo Gateway** (`kilo`) | Optional: [kilo.ai](https://kilo.ai) account for higher limits. Anonymous works with key `anon` | Real Kilo API key, or `anon` |
| 15 | **Pollinations** (`pollinations`) | No signup — paste `anon` | `anon` |
| 16 | **LLM7** (`llm7`) | Optional: [llm7.io](https://llm7.io) for account. Anonymous works with `anon` | Real key or `anon` |
| 17 | **Custom** | Any OpenAI-compatible server (local Ollama, LM Studio, vLLM) | Base URL + model name on the **Custom** form at bottom of Keys page |

### Adding keys in the dashboard (same flow for every provider)

1. Open **http://localhost:3001** and log in.
2. Go to **Keys**.
3. Under **Add a provider key**:
   - **Platform** — pick from dropdown (must match table above).
   - **API key** — paste the key from the provider.
   - **Label** — optional note (e.g. `personal-groq`).
4. Click **Add key**.
5. Click **Check** on the new row — wait for **healthy** (green dot).
6. Repeat for each provider you want in the pool.

### Recommended first batch for MakeSense

If you're hitting rate limits today, add these in order (takes ~15 minutes):

```
1. google     → your GEMINI_API_KEY (already in backend/.env — copy to dashboard)
2. groq       → get free key at console.groq.com/keys
3. openrouter → get free key at openrouter.ai/keys
4. github     → your GitHub PAT with models scope (already in backend/.env)
5. pollinations → key: anon
6. llm7       → key: anon
7. kilo       → key: anon
```

Then open **Fallback Chain** and ensure models for those platforms are **enabled** and near the top:

- Gemini 2.5 Flash (google)
- Llama 3.3 70B (groq)
- Meta Llama 3.3 70B Instruct `:free` (openrouter)
- GPT-4o mini (github)
- GPT-OSS 20B (pollinations / llm7) as last-resort fallbacks

Click **Check all** again. Test:

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Authorization: Bearer YOUR-FREELLMAPI-UNIFIED-KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Reply with one word: ok"}]}' -i
```

Look for `HTTP/1.1 200` and header `X-Routed-Via: groq/…` (or similar).

---

### Step 3 — Configure MakeSense backend

```bash
cd backend
cp .env.example .env   # skip if you already have one
```

Edit `backend/.env`:

```env
# Route all LLM calls through FreeLLMAPI
LLM_PROVIDER=freellmapi
FREELLMAPI_API_KEY=freellmapi-your-unified-key-here
FREELLMAPI_BASE_URL=http://localhost:3001/v1
FREELLMAPI_MODEL=auto

# Server (unchanged)
PORT=8080
DB_PATH=./makesense.db
ALLOWED_ORIGIN=http://localhost:3000
```

Start the backend:

```bash
go mod tidy
go run ./cmd/makesense
```

You should see a log line like:

```
makesense listening on :8080 (provider=freellmapi, model=auto, db=./makesense.db)
```

Start the frontend (separate terminal):

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
# open http://localhost:3000
```

Type in the notepad — analysis requests will flow: **Browser → MakeSense Go API → FreeLLMAPI → upstream provider**.

---

### Step 4 — Verify end-to-end

**Test FreeLLMAPI directly:**

```bash
curl http://localhost:3001/v1/chat/completions \
  -H "Authorization: Bearer freellmapi-your-unified-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Say hello in one word."}]
  }' -i
```

Check response headers:

- `X-Routed-Via: groq/llama-3.3-70b-versatile` (or whichever provider served it)
- `X-Fallback-Attempts: N` (if failover occurred)

**Test MakeSense:**

1. Open http://localhost:3000
2. Type expense-like text (e.g. "Coffee $4.50, lunch $12") and confirm the right pane renders structured output.
3. If it fails, check MakeSense logs and the FreeLLMAPI **Analytics** page for error details.

---

### Step 5 (optional) — Run both services with Docker Compose

You can extend the root `docker-compose.yml` to run FreeLLMAPI alongside the MakeSense backend. Example addition:

```yaml
services:
  freellmapi:
    image: ghcr.io/tashfeenahmed/freellmapi:latest
    ports:
      - "127.0.0.1:3001:3001"
    env_file:
      - ./freellmapi/.env
    volumes:
      - freellmapi_data:/app/server/data
    restart: unless-stopped

  backend:
    build: ./backend
    ports:
      - "8080:8080"
    environment:
      - LLM_PROVIDER=freellmapi
      - FREELLMAPI_API_KEY=${FREELLMAPI_API_KEY}
      - FREELLMAPI_BASE_URL=http://freellmapi:3001/v1
      - FREELLMAPI_MODEL=auto
      - ALLOWED_ORIGIN=${ALLOWED_ORIGIN:-http://localhost:3000}
      - DB_PATH=/data/makesense.db
    volumes:
      - makesense_data:/data
    depends_on:
      - freellmapi
    restart: unless-stopped

volumes:
  freellmapi_data:
  makesense_data:
```

When the backend runs inside Docker, use the service name `freellmapi` as the host — not `localhost`.

---

## Operational notes

### Keep `ENCRYPTION_KEY` stable

Provider keys are encrypted at rest with `ENCRYPTION_KEY`. If you change it, existing keys in SQLite become unreadable. Back up both the `.env` file and the `freellmapi-data` Docker volume.

### Model quality over the day

Top-ranked models (Gemini Pro, GPT-4o via GitHub Models) have the lowest daily caps. As they exhaust limits, the router falls down to smaller models — effective intelligence drops until UTC midnight reset. Order your fallback chain accordingly.

### Rate limits and failover

If MakeSense sees `http 429` errors, FreeLLMAPI may have exhausted all models in the chain. Check the dashboard Analytics page, wait for cooldowns, or add more provider keys.

### Sticky sessions

Multi-turn conversations stay on the same upstream model for 30 minutes. MakeSense's analyze pipeline uses mostly single-shot JSON requests, so this has minimal impact.

### Health checks

Dead or invalid keys are skipped automatically. Re-add or rotate keys on the Keys page if a provider consistently shows `invalid`.

---

## Limitations

- **No frontier models** — free tiers top out around Llama 3.3 70B, GLM-4.5, Gemini 2.5 Pro. Not GPT-5 / Claude Opus class.
- **Variable latency** — Groq/Cerebras are fast; others are slower. You get whichever is available.
- **Free tiers change** — providers tighten or remove limits without notice. Watch for 429s and update keys/catalog.
- **No SLA** — not suitable for production workloads.
- **Single-user** — no multi-tenant auth. Do not expose to the public internet.
- **JSON mode** — MakeSense requests `response_format: json_object`. Most routed models support this; if a weak fallback model ignores it, you may see parse errors in MakeSense logs.

---

## Terms of Service notes

FreeLLMAPI's maintainers reviewed provider ToS for personal, single-user use (May 2026). Summary:

| Verdict | Providers |
|---------|-----------|
| Likely OK | Groq, Cerebras, Mistral, OpenRouter, Zhipu, Ollama Cloud |
| Caution | Google Gemini, NVIDIA NIM, GitHub Models, Z.ai |
| Ambiguous | SambaNova, Cloudflare |
| Avoid | Cohere (forbids personal/household use) |

Rules of thumb: one account per provider, no reselling, don't share your endpoint, don't hammer free tiers as a production backend. Read each provider's ToS yourself — this is informational, not legal advice.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `openai-compatible: API key not set` | Missing `LLM_API_KEY` in `backend/.env` | Copy unified key from FreeLLMAPI Keys page |
| Connection refused to `:3001` | FreeLLMAPI not running | `docker compose up -d` in `freellmapi/` |
| `http 401` from FreeLLMAPI | Wrong unified key | Regenerate or copy key from dashboard |
| `http 422 no_vision_model` | Image request, no vision model enabled | Enable a vision-capable model in Fallback Chain |
| `http 429` after many requests | All keys rate-limited | Wait, add keys, or reorder fallback chain |
| MakeSense returns non-JSON | Weak fallback model ignored JSON instruction | Pin a stronger model: `LLM_MODEL=gemini-2.5-flash` |
| Keys unreadable after restart | Changed `ENCRYPTION_KEY` | Restore original key or re-add provider keys |

---

## Quick reference

| What | Value |
|------|-------|
| FreeLLMAPI dashboard | http://localhost:3001 |
| OpenAI-compatible base URL | http://localhost:3001/v1 |
| MakeSense provider setting | `LLM_PROVIDER=freellmapi` |
| MakeSense unified key env | `FREELLMAPI_API_KEY=freellmapi-…` |
| Auth header | `Authorization: Bearer freellmapi-…` |
| Smart routing model | `LLM_MODEL=auto` |
| Upstream repo | https://github.com/tashfeenahmed/freellmapi |
| Local clone in this repo | `freellmapi/` |

---

## Further reading

- Upstream README: `freellmapi/README.md`
- Docker ops: `freellmapi/docker/README.md`
- MakeSense LLM client: `backend/internal/llm/openai.go`
- MakeSense provider factory: `backend/internal/llm/provider.go`
