# Monetization & Launch Plan — MakeSense.ai

A step-by-step plan to take the prototype to a paid product: subscription paywall,
auth, payments, hosting, and a landing page that converts. Optimized for a **solo
founder selling to EU + US customers** who wants to stay as close to **$0/month**
as possible.

> **Target decisions (locked in):**
> - Market: **Europe + America** (paying customers, card payments in USD/EUR).
> - Model: **Hard paywall, subscription** — weekly / monthly / yearly tiers at different rates.
> - Everything else: open-source / free tiers wherever possible.

---

## 0. Payments: Stripe (your choice for now)

You've chosen **Stripe** directly for v1. Good news: it has the best developer
experience, the cleanest APIs, and the lowest base fees. The trade-off to go in with
eyes open: **with Stripe you are the Merchant of Record**, which means *you* are
legally responsible for collecting and remitting tax:

- EU **VAT** — you must register (VAT-OSS) and remit VAT, rate depending on the buyer's country.
- US **sales tax** — economic-nexus rules vary state by state.

**Stripe Tax** (0.5% per transaction) automates the *calculation and collection* of the
right tax at checkout and produces filing-ready reports — but it does **not** remove your
legal obligation to register and file in each jurisdiction. It only collects tax where
you've told it you're registered. So Stripe = lower fees + more admin on you. That's a
reasonable place to start at low volume; if the tax paperwork becomes a burden, the
fallback is a Merchant of Record (Lemon Squeezy / Paddle, ~5% + $0.50, they handle all
tax) — see the appendix. **Decision recorded: Stripe now, MoR as the escape hatch later.**

### The Stripe products you'll use

| Stripe product | What it does for you |
|---|---|
| **Stripe Checkout** | Hosted, PCI-compliant payment page. No card UI to build. Supports subscriptions. |
| **Stripe Billing** | Recurring subscriptions (weekly/monthly/yearly), proration, retries, dunning. +0.7% per recurring charge. |
| **Stripe Customer Portal** | Hosted page where users upgrade / cancel / update cards. Zero billing UI for you. |
| **Stripe Tax** | Auto-calculates VAT/sales tax at checkout. +0.5% per transaction. |
| **Webhooks** | Source of truth for subscription state — see Phase 3. |

> ⚠️ **Verify before building:** Stripe accounts registered in **India** have had
> restrictions on charging international (EU/US) customers. Confirm your Stripe account
> can accept cards from your target market and settle to your bank **before** writing
> code — this is an account/entity question, not a code one. (See open questions.)

---

## 1. Current state vs. what a paywall needs

What you have today (from the repo):

- **Backend:** Go (chi) + SSE streaming, SQLite, endpoints for `/analyze`,
  `/analyze/stream`, and notes CRUD. Provider-agnostic LLM layer.
- **Frontend:** Next.js 14 (App Router), single page at `app/page.tsx` (the split-pane editor).
- **No auth, no users.** `storage.go` literally says *"v0 keeps it single-user; userID
  stays empty until auth lands."*
- **SQLite is ephemeral on Cloud Run** (lost on scale-to-zero) — noted in `DEPLOY.md`.

So the four gaps to close, in order:

1. **Identity** — users must sign in (no accounts = no subscriptions).
2. **Persistent, multi-tenant database** — notes scoped per user, survives restarts.
3. **Subscription state + gating** — backend refuses `/analyze` unless the user has an active sub.
4. **Marketing surface** — a landing page that sells the product.

---

## 2. Recommended stack (all free to start)

| Concern | Choice | Why / Free tier |
|---|---|---|
| **Auth + Database** | **Supabase** | One service solves both gaps 1 & 2. Free tier: **50,000 monthly active users** for auth, **500 MB Postgres**, open-source. Postgres replaces ephemeral SQLite → your data finally persists. |
| **Payments** | **Stripe** (Checkout + Billing + Tax) | Lowest base fees, best DX. You are MoR — Stripe Tax automates tax math (you still register/file). |
| **Frontend hosting** | **Vercel — Hobby (free)** | Your choice for now. ⚠️ Hobby is officially **personal / non-commercial**, so a paid SaaS technically needs Vercel **Pro ($20/mo)**; there's also no overage — at 100 GB bandwidth / 1M function calls the app *pauses* until reset. Fine to launch on; budget the $20/mo Pro upgrade for when you have paying users or hit limits. (Free commercial alternative if you want $0: Cloudflare Pages.) |
| **Backend hosting** | **Google Cloud Run** | Free tier: 2M requests, 180k vCPU-sec, 360k GiB-sec/month. Already documented in `DEPLOY.md`. Becomes fully stateless once DB moves to Supabase. |
| **LLM** | **FreeLLMAPI / Groq / Gemini** | Keep your existing free-tier setup. |
| **Domain** | Namecheap / Cloudflare Registrar | ~$10–12/year — the one unavoidable cost. |

**Why Supabase over Clerk for auth:** Clerk's free tier is also generous (50k MAU) and
has nicer DX, but it's auth-only — you'd still need a separate database. Supabase gives
you auth **and** Postgres **and** solves your persistence problem in one move. Fewer
moving parts for a solo founder.

> Caveat to know: Supabase pauses a free project after 7 days of *zero* activity. Once
> you have real daily users this never triggers; during quiet early testing, a weekly
> ping keeps it warm.

---

## 3. Architecture with the paywall

```
                 ┌─────────────────────────────────────────┐
                 │  Vercel (Hobby) — Next.js                │
                 │  /         → landing page (public)       │
                 │  /app      → editor (auth required)      │
                 │  /pricing  → tiers + Stripe Checkout     │
                 └───────────────┬─────────────────────────┘
                                 │  Supabase JWT in Authorization header
                                 ▼
                 ┌─────────────────────────────────────────┐
                 │  Go API (Cloud Run)                      │
                 │  middleware: verify Supabase JWT         │
                 │             → load user + sub status     │
                 │  /analyze        (GATED: active sub)     │
                 │  /notes/*        (GATED + scoped to user)│
                 │  /billing/checkout    (creates session)  │
                 │  /webhooks/stripe     (public + signed)  │
                 └──────┬───────────────────────┬───────────┘
                        ▼                        ▼
              ┌──────────────────┐    ┌────────────────────────┐
              │ Supabase Postgres│    │ Stripe                  │
              │ users, subs,     │◄───┤ Checkout + Billing +    │
              │ notes, analyses  │    │ Tax + Customer Portal   │
              └──────────────────┘    └────────────────────────┘
```

**Golden rule:** subscription status lives in *your* database and is set *only* by
verified Stripe webhooks. Never trust the client to say "I'm a paid user."

---

## 4. Implementation plan (phased)

### Phase 1 — Auth + multi-tenant DB (foundation)

This is the biggest code change and unlocks everything else.

1. **Create a Supabase project.** Get the project URL, anon key, service-role key,
   and JWT secret.
2. **Frontend (Next.js):** add `@supabase/supabase-js` + `@supabase/ssr`. Build
   sign-up / sign-in (email magic link or Google OAuth — both built into Supabase).
   Wrap `/app` in an auth guard; redirect signed-out users to `/`.
3. **Send the JWT to the backend.** `lib/api.ts` already centralizes API calls — add
   the Supabase access token as `Authorization: Bearer <jwt>` on every request.
4. **Backend: JWT middleware (Go).** Verify the Supabase JWT (HS256, using the project
   JWT secret) and extract the `sub` claim = `user_id`. Reject requests with no/invalid
   token. Libraries: `github.com/golang-jwt/jwt/v5`.
5. **Migrate storage to Postgres + add `user_id`.**
   - Swap the driver in `backend/internal/storage/storage.go`: `modernc.org/sqlite` →
     `github.com/jackc/pgx/v5/stdlib` (or keep `database/sql` and point at Supabase's
     Postgres connection string). `DEPLOY.md` already anticipates a driver swap (for Turso);
     Postgres is the same shape of change.
   - Add `user_id TEXT NOT NULL` to `notes` and `analyses`. Add it to every query
     `WHERE user_id = $1`. This closes the multi-tenant gap the code comment flags.
   - Add a `subscriptions` table (see Phase 3).

> Alternative if you want to keep SQLite: Turso (libSQL) also works and `DEPLOY.md`
> documents it. But since you need auth anyway, Supabase Postgres consolidates the stack.

### Phase 2 — Pricing page + Stripe Checkout

1. In the **Stripe dashboard**, create one **Product** ("MakeSense Pro") with three
   recurring **Prices**: Weekly, Monthly, Yearly. Note each `price_...` id. Enable
   **Stripe Tax** and **Stripe Billing**. Turn on the **Customer Portal** in settings.
2. **Backend: `POST /billing/checkout`** (gated by login). Given a `price_id`, it creates
   a Stripe **Checkout Session** (`mode: "subscription"`) with the user's email,
   `client_reference_id = user_id` (so the webhook maps the purchase back), `automatic_tax`
   enabled, and `success_url` / `cancel_url` pointing at your frontend. Use the Go library
   `github.com/stripe/stripe-go/v79`. Return the session URL.
3. Add a `/pricing` route in Next.js with three cards. Each "Subscribe" button calls
   `/billing/checkout` and redirects the browser to the returned Stripe-hosted Checkout URL.
4. After payment, Stripe redirects to your `success_url`. **Real entitlement comes from the
   webhook (next phase), not this redirect** — the redirect can be faked.

**Suggested launch pricing (EU/US market — adjust after testing):**

| Plan | Price | Effective monthly | Purpose |
|---|---|---|---|
| Weekly | $2.99/wk | ~$13 | Low-commitment trial-by-paying; impulse buy |
| Monthly | $7.99/mo | $7.99 | The default plan |
| Yearly | $59/yr | ~$4.92 | Best value; locks in revenue & cuts churn |

> Anchor on the yearly plan ("save 38%"). Consider a **7-day free trial** on the
> monthly/yearly plans (Stripe Checkout supports `trial_period_days` natively) — a pure hard paywall
> maximizes friction and usually tanks conversion. A short trial keeps it "pay to use"
> while letting people feel the value first. Your call.

### Phase 3 — Webhooks + gating (the actual paywall)

1. **Add `subscriptions` table** in Postgres:
   `user_id`, `stripe_customer_id`, `stripe_subscription_id`, `status` (`active` /
   `trialing` / `past_due` / `canceled` / `incomplete`), `plan`, `current_period_end`,
   `updated_at`.
2. **Add `POST /webhooks/stripe`** to the Go server (public route, **but** verify the
   `Stripe-Signature` header with `webhook.ConstructEvent` and your webhook signing
   secret — reject if it doesn't match). Handle: `checkout.session.completed` (first map
   `client_reference_id` → `stripe_customer_id`), `customer.subscription.created`,
   `customer.subscription.updated`, `customer.subscription.deleted`, and
   `invoice.payment_failed`. Upsert the row from the event's subscription object
   (status + `current_period_end`).
3. **Gate the product endpoints.** In the JWT middleware path, after resolving `user_id`,
   look up the subscription. If status isn't `active`/`trialing` → return `402 Payment
   Required` (or `403`). Apply this to `/analyze`, `/analyze/stream`, and notes writes.
4. **Frontend reacts to 402:** show a "Subscribe to continue" modal linking to `/pricing`.
5. **Customer portal:** add `POST /billing/portal` that creates a Stripe Billing Portal
   session and redirects there, so users upgrade / cancel / update cards themselves —
   zero billing UI for you to build.

**Test with Stripe test mode + the Stripe CLI** (`stripe listen --forward-to`) end to
end before going live: subscribe with a test card → webhook fires → row goes `active` →
`/analyze` works → cancel in the portal → row flips → `/analyze` returns 402.

### Phase 4 — Landing page that converts (build it with Claude)

Right now `/` *is* the app. Move the editor to `/app` and make `/` a marketing page.
Same Next.js project, no extra hosting.

**You'll build this page with Claude.** When you're ready, just say *"build the landing
page"* and I'll generate the actual `app/page.tsx` (plus any components) — Tailwind is
already installed, so it'll drop straight into your repo. Helpful context to give me:
your one-line value prop, the brand color/vibe, and whether you want the live editor
demo embedded. The structure below is what I'd build toward.

A high-converting structure for a tool like this:

1. **Hero** — one-line value prop ("Type messy notes. Get structured answers as you
   type."), a primary CTA ("Start free" / "Try it"), and — most important — a **live or
   looping demo** of the split-pane: raw text on the left turning into an expense table /
   todo list on the right. *The product demoing itself is your best salesperson.* You can
   embed a read-only, capped version of the real editor as an interactive teaser.
2. **Problem → solution** — 3 short benefit blocks (auto-classification, real-time
   structuring, no manual formatting).
3. **Use cases** — expenses, to-dos, journaling, meeting notes (your analyzers).
4. **Social proof** — testimonials / "as seen on Product Hunt" once you have them; a
   simple usage counter works early on.
5. **Pricing table** — the three tiers, yearly highlighted.
6. **FAQ** — "Is my data private?", "Can I cancel anytime?", "What happens after I stop paying?"
7. **Footer** — Privacy Policy + Terms. With Stripe you're the seller, so you need your
   own legal pages (esp. a privacy policy, since you store user notes). A generator like
   Termly/iubenda is fine to start.
8. **SEO basics** — metadata, Open Graph image, `sitemap.xml`. Use Next's static
   rendering for `/` so it's fast and ranks.

Keep it one page, fast, mobile-first. Tailwind is already set up.

### Phase 5 — Deploy

1. **Backend → Cloud Run.** Follow `DEPLOY.md`. New env vars: `DATABASE_URL`
   (Supabase Postgres), `SUPABASE_JWT_SECRET`, `STRIPE_SECRET_KEY`,
   `STRIPE_WEBHOOK_SECRET`, and your three `STRIPE_PRICE_*` ids. Since data now lives in
   Supabase, `--min-instances 0` (scale-to-zero) is safe — no data loss.
2. **Frontend → Vercel (Hobby).** Already documented in `DEPLOY.md`: import the repo at
   vercel.com/new, root = `frontend`, Next.js auto-detected. Env: `NEXT_PUBLIC_API_URL`
   (Cloud Run URL), `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`,
   `NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY`. Every push to main auto-deploys.
   *(Reminder: Hobby is non-commercial and pauses at its caps — plan to move to Pro
   $20/mo once you have paying users.)*
3. **CORS:** update `ALLOWED_ORIGIN` in the backend to your real domain.
4. **Webhook URL:** in the Stripe dashboard, add an endpoint pointing at
   `https://<your-cloud-run-url>/webhooks/stripe` and subscribe to the events from Phase 3.
   Copy the signing secret into `STRIPE_WEBHOOK_SECRET`.
5. **Domain:** buy one, point DNS at Vercel, add it to your Stripe Checkout allowed domains.

---

## 5. Cost reality check

| Item | Cost at launch | When it starts costing |
|---|---|---|
| Supabase (auth + Postgres) | **$0** | Pro ($25/mo) at >500 MB DB or >50k MAU |
| Vercel Hobby (frontend) | **$0** | Move to Pro ($20/mo) when you have paying users or hit caps |
| Cloud Run (backend) | **$0** | After ~2M requests/mo |
| LLM (Groq/Gemini/FreeLLMAPI) | **$0** | When you outgrow free provider tiers |
| Stripe | **$0 fixed** | Per sale: 2.9% + $0.30 (US card) **+0.7%** Billing **+0.5%** Tax, plus **+1.5%** non-US card and **+1%** currency conversion. A EU card in EUR can total ~7–8% + $0.30. |
| Domain | **~$12/yr** | Always |

**Bottom line:** roughly **$1/month (just the domain)** until you have real revenue or
real scale. Stripe's fees only apply to money you're already receiving — but note they
stack higher than a single headline rate (Billing + Tax + international-card + FX), so a
EUR sale to a European card lands around **7–8% all-in**, comparable to a Merchant of
Record once you add the admin you take on. The biggest *future* cost lever is LLM usage —
watch it as volume grows. Two known step-ups: Vercel Pro (~$20/mo) once you're commercial,
and your own tax registration/filing effort (or a switch to MoR) as you sell across more
EU/US jurisdictions.

---

## 6. Suggested build order (milestones)

1. **M1 — Auth + Postgres migration.** Users can sign in; notes are per-user and persist.
   *(Largest effort; everything depends on it.)*
2. **M2 — Stripe products + `/pricing` + Checkout.** Money can theoretically flow.
3. **M3 — Webhooks + gating.** The paywall actually works end-to-end (Stripe test mode).
4. **M4 — Landing page** at `/`, app moved to `/app` (build with Claude).
5. **M5 — Deploy** (Cloud Run + Vercel + domain) and switch Stripe to live mode.
6. **M6 — Launch** (Product Hunt, relevant subreddits, X) and watch conversion + LLM cost.

---

## 7. Things not to skip

- **Webhook signature verification** — verify the `Stripe-Signature` header with
  `webhook.ConstructEvent`. Without it, anyone can POST themselves a free sub.
- **Tax registration** — Stripe Tax calculates tax but you must actually register to
  collect it in the relevant jurisdictions and file returns. Sort this out as revenue grows.
- **Server-side gating** — never gate only in the frontend; the Go API must enforce it.
- **Privacy policy** — you store users' notes; you need one. State your data/retention practices.
- **A "what happens when I cancel" path** — keep notes readable (or export) but block
  new analyses, so cancelling doesn't feel like losing data (reduces refund disputes).
- **Rate limiting** on `/analyze` per user — protects your LLM free-tier budget from abuse.

---

## Open questions for you

These would sharpen the plan further — answer when you're ready:

1. **Free trial or pure hard paywall?** I'd lean to a 7-day trial for conversion, but you said pay-to-access — your call.
2. **Auth method** — email magic-link, Google sign-in, or both?
3. **What exactly is gated?** Just `/analyze` (the AI), or also saving notes? (Affects whether free users can use a plain notepad.)
4. **Stripe account + banking** — can your Stripe account (esp. if India-registered)
   actually charge EU/US customers and settle to your bank? Indian Stripe accounts have
   had cross-border restrictions. Verify this **before** building — it's the one thing
   that can block the whole plan, and it's an account/entity question, not a code one.

---

## Appendix — Merchant-of-Record fallback (if Stripe's tax admin gets heavy)

You chose Stripe for v1. If, as you scale across EU/US jurisdictions, registering for and
filing taxes everywhere becomes more hassle than it's worth, switch to a **Merchant of
Record** — they become the legal seller and handle *all* VAT/sales tax for you:

| | Lemon Squeezy | Paddle |
|---|---|---|
| Fee | 5% + $0.50/txn | 5% + $0.50/txn |
| Handles EU VAT + US sales tax | Yes (full MoR) | Yes (full MoR) |
| Hosted checkout + customer portal | Yes | Yes |
| Notes | Stripe-backed now; simplest DX | Watch 2–3% FX margin if payout currency ≠ sale currency |

The migration is mostly swapping the checkout call and webhook handler — the auth, DB,
and gating you build for Stripe all carry over unchanged. So starting on Stripe doesn't
lock you in; it just means you own tax compliance until you decide to hand it off.
