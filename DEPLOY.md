# Deployment — free-tier friendly

Recommended pairing for v1:

- **Frontend:** Vercel (free Hobby tier, zero-config for Next.js)
- **Backend:** Google Cloud Run (generous free tier, scale-to-zero, natural fit with Gemini)
- **Database:** SQLite file on Cloud Run — fine for early solo usage. Move to Turso or Cloud SQL when you have real users.

Below are step-by-step instructions, plus two alternatives.

---

## Backend → Google Cloud Run

**Free tier:** 2M requests/mo, 360K GB-seconds memory, 180K vCPU-seconds, 1 GB egress/mo from North America. For a v1 this is effectively free.

### 1. Prereqs

```bash
# Install gcloud and auth
gcloud auth login
gcloud auth configure-docker
export PROJECT_ID=makesense-ai   # pick any unique ID
gcloud projects create $PROJECT_ID
gcloud config set project $PROJECT_ID
gcloud services enable run.googleapis.com artifactregistry.googleapis.com
```

### 2. Build + push image

```bash
cd backend

# Use Cloud Build (easiest — no local Docker needed)
gcloud builds submit --tag gcr.io/$PROJECT_ID/makesense:latest
```

### 3. Deploy

```bash
gcloud run deploy makesense \
  --image gcr.io/$PROJECT_ID/makesense:latest \
  --region asia-south1 \
  --allow-unauthenticated \
  --set-env-vars "GEMINI_API_KEY=YOUR_KEY,GEMINI_MODEL=gemini-2.0-flash,ALLOWED_ORIGIN=https://your-frontend.vercel.app" \
  --memory 256Mi \
  --cpu 1 \
  --min-instances 0 \
  --max-instances 3
```

Cloud Run will print a URL like `https://makesense-abc123-el.a.run.app`. Save it — the frontend needs it.

**Important caveats for SQLite on Cloud Run:**

- Cloud Run instances are ephemeral. The local SQLite file survives only while the instance stays warm. With `--min-instances 0`, data is lost when the container scales to zero.
- For v0 that's acceptable (analyses are just a cache; you can rebuild them). For real user notes, move to:
  - **Turso** (libSQL, drop-in SQLite replacement, generous free tier: 500 DBs, 9 GB storage) — change `storage.Open` to use the libSQL Go driver.
  - **Cloud SQL for PostgreSQL** (smallest tier ~₹600/mo).

### 4. Re-deploy on code changes

```bash
gcloud builds submit --tag gcr.io/$PROJECT_ID/makesense:latest
gcloud run deploy makesense --image gcr.io/$PROJECT_ID/makesense:latest --region asia-south1
```

Consider wiring this to GitHub Actions later (one workflow file → auto-deploy on push to main).

---

## Frontend → Vercel

**Free tier:** Unlimited personal projects, serverless functions, automatic previews, custom domains.

1. Push this repo to GitHub (you already have `gyanendraparmaar/MakeSense` set up).
2. Go to https://vercel.com/new, import the repo.
3. **Root directory**: `frontend`
4. Framework preset: Next.js (auto-detected)
5. Environment variable:
   - `NEXT_PUBLIC_API_URL` = the Cloud Run URL from step 3 above
6. Deploy.

Every `git push` to main auto-deploys. PRs get preview URLs.

---

## Alternative: Fly.io (backend)

Fly is Go-friendly and you can run a persistent volume that survives restarts — better for SQLite than Cloud Run.

```bash
# Install flyctl
curl -L https://fly.io/install.sh | sh

cd backend
fly launch --no-deploy     # answer prompts; say NO to Postgres for now

# Create a persistent volume for SQLite
fly volumes create makesense_data --region bom --size 1

# Edit fly.toml to mount it at /data and set DB_PATH=/data/makesense.db
fly secrets set GEMINI_API_KEY=YOUR_KEY ALLOWED_ORIGIN=https://your-frontend.vercel.app

fly deploy
```

Fly's "hobby" plan is ~$1.94/mo for a 256MB shared-CPU-1 machine. Not quite free, but negligible.

---

## Alternative: Render (backend)

Render has a genuinely free web service tier (cold starts after 15 min idle, which is fine for a note-taking app).

1. Push to GitHub.
2. Go to https://dashboard.render.com → New → Web Service → connect repo.
3. Root: `backend`, Dockerfile: `Dockerfile`.
4. Plan: Free.
5. Add env vars: `GEMINI_API_KEY`, `ALLOWED_ORIGIN`.
6. Deploy.

Render's persistent disk add-on is paid, so with the free tier your SQLite file is lost on redeploy — same caveat as Cloud Run. Turso solves this.

---

## Hosting comparison (what you asked for)

| Option | Free? | Best for | Catch |
|---|---|---|---|
| **Google Cloud Run** | Generous free tier | Go, pairs with Gemini | SQLite data not persistent on scale-to-zero |
| **Vercel** (frontend) | Free Hobby forever | Next.js frontend | Backend functions have 10s timeout on free tier (we run backend separately) |
| **Fly.io** | ~$2/mo after $5 credit | Persistent SQLite, always-on | Not strictly free long-term |
| **Render** | Free web services tier | Simple Heroku-like DX | Cold starts, ephemeral disk on free tier |
| **Railway** | $5 monthly credit | Easy DX, auto-deploys | Billing kicks in at scale |

**My pick for now:** Cloud Run + Vercel + Turso (when you're ready to persist real user data).

---

## Moving from local SQLite to Turso (when you're ready)

1. Sign up at https://turso.tech (free).
2. `turso db create makesense`
3. `turso db show makesense` → note the libSQL URL
4. `turso db tokens create makesense`
5. Replace the driver in `backend/internal/storage/storage.go`:
   - `_ "modernc.org/sqlite"` → `_ "github.com/tursodatabase/libsql-client-go/libsql"`
   - `sql.Open("sqlite", ...)` → `sql.Open("libsql", "libsql://DB.turso.io?authToken=TOKEN")`
6. `go mod tidy`, redeploy.
