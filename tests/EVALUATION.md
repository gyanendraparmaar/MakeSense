# MakeSense Evaluation Report

**Date:** 2026-06-02  
**Provider:** FreeLLMAPI (`auto` routing)  
**Suite:** 30 cases across 7 categories  
**Full results:** `tests/results/latest.json`

---

## Executive Summary

| Metric | Value |
|--------|-------|
| **Average score** | **3.90 / 5** |
| **Pass** | 20 (67%) |
| **Partial** | 5 (17%) |
| **Fail** | 3 (10%) |
| **Error** | 2 (7%) |
| **Avg latency** | 14.7s (25ms when cached) |

The app performs well on **clean, single-intent inputs** — expense lists, todo bullets, journals, and Hinglish spending notes all score 4.7–5.0/5. The main gaps are **mixed-content notes**, **meeting-note misclassification**, **query-vs-note confusion**, and **infrastructure timeouts** through FreeLLMAPI.

---

## Score by Category

| Category | Cases | Avg Score | Verdict |
|----------|-------|-----------|---------|
| Expenses | 8 | 3.9/5 | Strong extraction; income conflation + 1 timeout |
| Language (Hinglish) | 2 | 4.7/5 | Excellent — handles code-mixed input naturally |
| Todo | 6 | 4.8/5 | Best-performing category |
| Generic | 4 | 4.1/5 | Good summaries; meeting notes misrouted |
| Mixed | 3 | 3.0/5 | **Biggest product gap** — always loses half the content |
| Size | 2 | 4.9/5 | Handles tiny and ~150-word inputs well |
| Edge | 5 | 2.1/5 | Negation, queries, sarcasm, timeouts |

---

## Detailed Findings

### What works well (4.7–5.0/5)

**Expense extraction** handles diverse formats:

```
Swiggy 450, petrol 1200, Netflix 649
→ 3 items, grand_total 2299, categories correct, INR default ✓

Petrol ke liye 300 pay kiya, Swiggy order 450 UPI se
→ Hinglish parsed correctly, merchants identified ✓
```

**Todo extraction** resolves relative dates and priorities:

```
tomorrow: submit taxes / urgent: fix prod bug
→ due dates resolved, high priority detected ✓

Done: buy groceries / [x] send invoice
→ completed tasks marked done=true ✓
```

**Generic summaries** produce useful structured insight:

```
"Today felt overwhelming..."
→ summary + themes + questions + action_candidates ✓
```

**Caching** is effective — repeat inputs return in ~25ms vs ~30s cold.

---

### Failures (manual judgment)

#### gen-002 — Meeting notes → classified as `todo` (1.5/5)

**Input:** `Q2 planning - Sarah owns API migration, deadline March 15. Budget approved at 2M.`

**Got:** todo list with tasks "Plan Q2", "Migrate API", "Approve budget"  
**Expected:** generic summary OR dedicated `meeting_notes` with owners/decisions/risks

**Judgment:** The model over-indexes on action verbs and deadlines. Meeting notes with ownership and budget context need a specialist analyzer that extracts `{decisions, owners, deadlines, risks}` — not a flat todo list.

#### edge-004 — Question misclassified as `expenses` (1.2/5)

**Input:** `How much did I spend on food last month?`

**Got:** `expenses` type, empty items, flag `"no expense data provided"`  
**Expected:** `generic` — recognize this as a query, not a spending record

**Judgment:** Classifier lacks an "intent" dimension (record vs query vs reflection). High confidence (0.9) on wrong type is worse than low confidence — UI would render an empty expense table.

#### edge-001 — Negation (2.4/5)

**Input:** `Don't need to buy milk. Already have enough.`

**Got:** `todo` type, empty items array  
**Expected:** `generic` — no active tasks

**Judgment:** Empty extraction saved it from being worse, but wrong type still triggers todo UI. Prompt needs explicit negation rules (Checklist-style INV tests).

---

### Partial passes — architectural limitations

#### mix-001 — Expenses + todos (2.9/5)

**Input:** `Spent 500 on lunch. Also need to call mom and book flight to Delhi.`

**Got:** todo only — "call mom", "book flight"  
**Lost:** ₹500 lunch expense entirely

This is the **#1 product gap**. The product plan describes multi-block segmentation; v0 implements single-type "pick dominant category" which loses information on every mixed note.

#### exp-008 — Income as expense (3.8/5)

**Input:** `Got salary 85000. Paid rent 25000 and electricity 3200.`

**Got:** All three as expense items including salary as ₹85,000 "Other"  
**Expected:** Only outflows; salary should be excluded or tagged as income

#### mix-003 — Trip planning (3.1/5)

**Input:** Goa trip with flight/hotel costs AND todo items  
**Got:** todo only — lost all expense amounts

---

### Infrastructure errors

| Case | Error | Latency |
|------|-------|---------|
| exp-004 (USD currency) | FreeLLMAPI timeout on analyze | 70s |
| edge-002 (sarcastic expense) | FreeLLMAPI timeout on classify | 60s |

Both hit `context deadline exceeded` against `localhost:3001`. The backend HTTP client timeout (~60s) is too tight when FreeLLMAPI rotates through rate-limited providers. For a typing UX targeting <3s, this is unacceptable even when it succeeds.

---

## Model Quality Assessment (subjective)

| Dimension | Rating | Notes |
|-----------|--------|-------|
| Classification accuracy (single-intent) | ★★★★☆ | 90%+ on clean inputs |
| Classification accuracy (mixed/edge) | ★★☆☆☆ | Dominant-type heuristic fails |
| Expense extraction fidelity | ★★★★☆ | Accurate amounts/categories; weak on income/flags |
| Todo extraction fidelity | ★★★★★ | Dates, priority, done-state all work |
| Generic insight quality | ★★★★☆ | Good summaries; meeting notes need specialist |
| Multilingual (Hinglish) | ★★★★★ | Surprisingly strong |
| Latency (cold) | ★★☆☆☆ | 2–70s via FreeLLMAPI auto |
| Latency (cached) | ★★★★★ | ~25ms |
| Reliability | ★★★☆☆ | 7% hard failures in this run |

---

## Test Methodology

Cases designed using patterns from:

1. **Checklist** (Ribeiro et al.) — negation, invariance, vocabulary variation
2. **LMUnit** — focused quality dimensions per output type
3. **Real-world expense NLP** — Hinglish, shorthand, ambiguous dates
4. **Product plan** — mixed-content and segmentation scenarios

Scoring weights: classification 35%, extraction 35%, insight quality 30%.

Run again:

```bash
python3 tests/scripts/run_evaluation.py
```

---

## Key Takeaways

1. **Single-type pipeline is the ceiling** — mixed notes consistently score ~3/5 regardless of model quality
2. **Meeting notes need their own analyzer** — todo extraction loses owner/decision/risk structure
3. **Add query detection** — questions about data should not route to expense analyzer
4. **Prompt hardening for negation and income** — cheap wins without architecture changes
5. **Latency and reliability** — FreeLLMAPI auto routing adds failover but not speed; need fast-path model for classify + retry logic

See `IMPROVEMENT_PLAN.md` for the prioritized roadmap.
