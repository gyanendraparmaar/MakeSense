# Smart Notepad — Product & Engineering Plan

> An AI notepad where the left pane is raw text and the right pane is a live, structured rendering of what you just wrote: tables for expenses, checklists for to-dos, summaries for rambling thoughts, timelines for plans, etc.

---

## 1. Product Vision

**One-liner:** "Write anything. See it organized instantly."

**Core loop:** User types → debounced snapshot → LLM classifies content type → specialized analyzer produces structured output → UI renders it live in the right pane.

**What makes it defensible (vs. "just use ChatGPT"):**
- Zero prompt-writing. It Just Knows what you meant.
- Persistent notes library with live, typed structure (not chat history).
- Multi-type detection: a single note can contain expenses *and* a to-do list *and* a journal entry — each rendered in its own block.
- Offline-capable editor, cloud-synced analysis.

---

## 2. Platform Decision

**Verdict: Web app first (Next.js PWA), mobile wrapper in phase 2.**

Reasons, given your "real product" goal:

| Concern | Why web wins for v1 |
|---|---|
| Iteration speed | Ship in days, not weeks. No App Store review. |
| Cost of LLM experimentation | All prompt/model changes deploy instantly. |
| Distribution | Shareable link, no install friction. |
| Future mobile | Wrap with Capacitor or build a thin SwiftUI/Kotlin client hitting the same API. |
| Dev ergonomics | Rich-text editors (TipTap, Lexical) are mature on the web. |

Mobile-specific features (Apple Pencil, voice, widgets) become a **phase 2** native client that hits the same backend API.

---

## 3. Recommended Tech Stack

### Frontend
- **Framework:** Next.js 14+ (App Router) + TypeScript
- **Editor:** [TipTap](https://tiptap.dev) (ProseMirror-based, great extension model) — or Lexical if you want Facebook's flavor
- **UI:** Tailwind CSS + shadcn/ui
- **State:** Zustand (local) + TanStack Query (server)
- **Realtime analysis UI:** Stream panel using SSE / fetch streaming
- **Offline:** IndexedDB via Dexie + service worker (PWA)

### Backend
- **API:** Next.js Route Handlers (same repo) for v1, extract to a dedicated service only when needed
- **Runtime:** Node 20+ on Vercel / Fly.io / Railway
- **Queue (phase 2):** Upstash QStash or Inngest for background re-analysis, embedding generation

### Data
- **Primary DB:** Postgres via Supabase or Neon
- **Vector store:** `pgvector` extension (same Postgres) — for "find notes like this"
- **Blob storage:** S3 / R2 for attachments
- **Cache:** Redis (Upstash) for rate-limit counters and LLM response caching

### Auth & Payments
- **Auth:** Clerk (fastest) or Supabase Auth (if already on Supabase)
- **Payments:** Stripe + Stripe Billing for subscriptions

### LLM Layer
- **Primary:** Anthropic Claude (Sonnet for classification + most analyzers, Haiku for cheap/fast passes, Opus for hard reasoning tasks)
- **Fallback:** OpenAI GPT-4o/mini
- **SDK:** `@anthropic-ai/sdk` with streaming + tool use
- **Schema enforcement:** Zod schemas → tool definitions → typed output

### Observability
- **LLM traces:** Langfuse or Helicone (drop-in proxy, see every prompt)
- **Errors:** Sentry
- **Product analytics:** PostHog (self-hostable, open source)

---

## 4. System Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                         Browser (PWA)                          │
│  ┌──────────────┐      ┌──────────────────────────────────┐    │
│  │  Editor Pane │◄────►│  Local Store (IndexedDB/Dexie)   │    │
│  │  (TipTap)    │      └──────────────────────────────────┘    │
│  └──────┬───────┘                                              │
│         │ debounced(800ms) diff                                │
│  ┌──────▼──────────────────────────────────────────────────┐   │
│  │  Analysis Pane (streams blocks: tables, todos, etc.)    │   │
│  └──────▲──────────────────────────────────────────────────┘   │
│         │ SSE / fetch-stream                                   │
└─────────┼──────────────────────────────────────────────────────┘
          │ HTTPS
┌─────────▼──────────────────────────────────────────────────────┐
│                  Next.js API (Route Handlers)                  │
│                                                                │
│  /api/analyze  ──►  Orchestrator                               │
│                      │                                         │
│                      ├─ 1. Segment text into candidate blocks  │
│                      ├─ 2. Classify each block (Haiku)         │
│                      ├─ 3. Route to specialized analyzer       │
│                      │     (Sonnet, tool-use for JSON)         │
│                      ├─ 4. Stream structured chunks back       │
│                      └─ 5. Persist result + usage to DB        │
│                                                                │
│  /api/notes    ──►  CRUD, realtime sync                        │
│  /api/auth     ──►  Clerk webhooks                             │
└───────┬──────────────────────────┬──────────────────────────┬──┘
        │                          │                          │
  ┌─────▼─────┐            ┌───────▼────────┐         ┌───────▼────────┐
  │ Postgres  │            │  Anthropic API │         │  Langfuse      │
  │ + pgvector│            │  (Claude)      │         │  (LLM traces)  │
  └───────────┘            └────────────────┘         └────────────────┘
```

---

## 5. The Analysis Pipeline (the heart of the app)

### Step 1 — Capture & segment
1. User types. Editor emits changes.
2. Debounce 600–1000ms of inactivity.
3. On fire: grab the full note (or a window around the cursor for long notes).
4. Segment into candidate blocks by paragraph / list / heading boundaries. For short notes (< ~500 tokens), treat the whole note as one block.

### Step 2 — Classify (cheap, fast)
Single Claude **Haiku** call. Tool-use returns:

```json
{
  "blocks": [
    { "span": [0, 124], "type": "expenses", "confidence": 0.92 },
    { "span": [125, 400], "type": "todo", "confidence": 0.88 },
    { "span": [401, 900], "type": "journal", "confidence": 0.71 }
  ]
}
```

Supported `type` enum for v1: `expenses | todo | meeting_notes | journal | research_notes | plan | contacts | recipe | generic_summary`.

Anything with confidence < 0.6 falls through to `generic_summary`.

### Step 3 — Specialized analyzers (parallel, streamed)
Each block gets routed to a dedicated prompt + tool schema. Examples:

**Expenses analyzer → tool schema:**
```ts
{
  currency: "INR",
  items: [
    { date: "2026-04-12", category: "Food", merchant: "Blue Tokai", amount: 420, note: "coffee" }
  ],
  totals_by_category: { Food: 420, Transport: 180 },
  grand_total: 600,
  flags: ["uncategorized: 'misc 50'"]
}
```
Rendered as a sortable table + pie chart.

**Todo analyzer:**
```ts
{
  items: [
    { task: "Call Ravi", due: "2026-04-21", priority: "high", depends_on: null },
    { task: "Send proposal", due: null, priority: "medium", depends_on: "Call Ravi" }
  ]
}
```
Rendered as a checklist with due-date chips.

**Journal / thoughts analyzer:**
```ts
{
  summary: "Reflecting on career direction after the Myntra sprint.",
  themes: ["burnout", "wanting more ownership"],
  emotions: ["frustrated", "hopeful"],
  open_questions: ["Should I pitch the notepad idea internally?"],
  action_candidates: ["Block 2h Thursday to journal again"]
}
```

**Meeting notes analyzer:** decisions / action items (with owner) / open questions / participants.

**Plan analyzer:** milestones / dependencies / timeline (Gantt-ish).

### Step 4 — Stream to UI
- Use Anthropic's streaming API with tool use.
- As JSON fields arrive, push partial updates over SSE.
- Frontend applies them optimistically: a table appears row-by-row, etc.

### Step 5 — Persist
- Save `{noteId, blockId, type, structured_json, model, tokens, cost}` rows.
- Next edit: diff against previous snapshot; only re-analyze changed blocks (saves 70–90% of cost).

---

## 6. Data Model (Postgres)

```sql
users (id, email, plan, created_at)

notes (
  id uuid pk,
  user_id fk,
  title text,
  content_md text,
  content_json jsonb,        -- editor state
  updated_at timestamptz,
  embedding vector(1536)     -- pgvector, for semantic search
)

analyses (
  id uuid pk,
  note_id fk,
  block_hash text,           -- sha256 of block text; dedup key
  block_span int4range,
  type text,                 -- 'expenses' | 'todo' | ...
  structured jsonb,
  confidence float,
  model text,
  input_tokens int,
  output_tokens int,
  cost_usd numeric(10,6),
  created_at timestamptz
)

usage_daily (user_id, day, tokens_in, tokens_out, cost_usd)
-- used for rate limiting + billing
```

Indexing: `analyses(note_id, block_hash)` for cache lookups; HNSW index on `notes.embedding`.

---

## 7. Key UX Decisions

- **Two-pane desktop, tabbed mobile.** Editor left, live analysis right. On mobile, a bottom sheet with the analysis that you can peek/pull up.
- **Analysis is additive, never destructive.** Never rewrite the user's text. Right pane is a mirror.
- **Type badges** on each analysis block, with a "wrong type?" override dropdown — that click is gold training data.
- **Ghost state** while analysis is loading (skeleton rows).
- **Export every block.** CSV for tables, `.ics` for todos with due dates, markdown for summaries.
- **Quiet by default.** No chat bubbles, no "Hi! I noticed…" — just structure appearing.

---

## 8. Prompting Strategy — principles

1. **Classify cheap, analyze focused.** Don't ask one giant prompt "detect and extract everything." One classifier + N specialists is cheaper, faster, and much easier to evaluate.
2. **Tool use > JSON mode > regex parsing.** Always define a tool with a strict schema. Reject malformed outputs and retry once.
3. **Few-shot with real examples.** Store 5–10 hand-labeled examples per type; inject top-3 nearest (by embedding) at runtime.
4. **Temperature 0.2 for extractors, 0.5 for summaries.**
5. **Guardrails:** token budget cap, timeout, per-user rate limits. Fail soft — show "still thinking" rather than an error.
6. **Evaluate continuously.** Build a tiny eval harness: 50 labeled notes per type, run on every prompt/model change, track precision/recall.

---

## 9. Phased Roadmap

### Phase 0 — Prototype (1 week)
- Hardcoded single-user, no auth, localStorage
- Editor + debounced fetch to `/api/analyze`
- Two analyzers only: **expenses** and **todo**
- Goal: prove the magic moment feels good

### Phase 1 — MVP (4–6 weeks)
- Clerk auth, Postgres, note library
- 5 analyzer types: expenses, todo, meeting, journal, generic summary
- Multi-block detection (one note → multiple outputs)
- Block-level caching by content hash
- Export to CSV / ICS / Markdown
- Basic usage dashboard
- Deploy to Vercel, custom domain
- Free tier with daily token cap, Stripe paywall stub

### Phase 2 — Real product (2–3 months)
- iOS wrapper (Capacitor) + share extension ("send to notepad")
- Voice input → transcribe → notepad
- Semantic search ("show me all expense notes from March")
- Weekly digest email ("here's your April spend")
- Collaborative notes (Yjs + CRDT)
- Plugin system: users add custom analyzer types

### Phase 3 — Moat
- On-device fallback for private notes (Llama 3 via WebLLM for offline)
- Cross-note synthesis ("what themes keep showing up in my journals?")
- Integrations: push to-dos to Todoist/Reminders, expenses to Splitwise, meeting notes to Notion

---

## 10. Cost Model (ballpark)

Assume Sonnet-4 pricing ~$3/Mtok in, $15/Mtok out, Haiku ~$0.80/$4.

Average note: 400 tokens in, 250 tokens structured out.
- Classifier (Haiku): ~$0.0005 per save
- Specialist (Sonnet): ~$0.005 per save
- **Per save total: ~$0.0055**

Heavy user: 40 saves/day = **$0.22/day ≈ $6.6/month**.

Priced at **$8–12/month Pro tier**, you have margin after infra (~$1/user/month for Postgres + Vercel + misc).

Free tier: cap at ~15 analyses/day = <$0.10/user/day, sustainable.

**Big cost lever:** block-hash caching. If a user saves 20x while writing one note, only the changed block re-bills. Expect 80%+ cache hit rate in steady-state.

---

## 11. Privacy, Security, Compliance

- **At rest:** Postgres encryption (Supabase/Neon default). Note bodies in a separate schema, easier to encrypt column-level later.
- **In transit:** TLS everywhere.
- **LLM:** Anthropic API with zero-retention mode (enterprise). Disclose in privacy policy.
- **PII detection:** pre-flight regex for Aadhaar / PAN / card numbers; offer to redact before sending to LLM.
- **Data export + delete:** GDPR/DPDP compliant account deletion from day 1.
- **Auth:** Clerk handles MFA, session rotation.
- **Secrets:** Never in client. Edge functions validate user → call Anthropic server-side.

---

## 12. Evaluation & Quality

Build this early — it's the difference between "demo" and "product":

1. **Golden set:** 100 real-ish notes, hand-labeled. Checked into repo.
2. **CI eval:** on every prompt change, run the set, compare structured output against ground truth. Fail PR if F1 drops by >2pp on any type.
3. **Live quality telemetry:** track "wrong type" user overrides. Route to a labeling queue.
4. **A/B harness:** route 10% of traffic to new prompt, compare override rate & latency.

---

## 13. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Latency feels laggy | Stream from token 1; optimistic skeletons; Haiku for classifier |
| Hallucinated data in tables | Strict tool schema + "only what's in the text" rule + show sources (span highlights) |
| LLM cost blows up | Block-hash cache, per-user daily cap, graceful free-tier throttle |
| Editor conflicts w/ LLM rewriting | Never write back into the editor. Analysis is read-only. |
| User privacy concerns | Clear policy + zero-retention mode + future on-device option |
| "Why not just use ChatGPT?" positioning | Lean on *zero prompting*, persistent typed library, multi-block detection |

---

## 14. What I Need From You Next

To move from plan → code, decide:

1. **Brand / name** (placeholder: "MakeSense" — your workspace folder name is already a good candidate 😉)
2. **First two analyzer types** to actually build in Phase 0 — I recommend **expenses + todo** (highest "wow" per line of code)
3. **Hosting preference:** Vercel (easiest) vs. self-hosted (full control)
4. **Auth:** Clerk (recommended) vs. Supabase Auth (if you want one vendor)
5. **Do you want me to scaffold the Phase 0 prototype next** (Next.js repo, editor wired to a stub analyzer, ready to plug your Anthropic key into)?

---

*Document version 1.0 — 2026-04-20*
