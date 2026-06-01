#!/usr/bin/env python3
"""
MakeSense.ai evaluation runner.

Calls POST /api/analyze for each test case, scores classification and
structured output, and writes results to tests/results/.

Usage:
  python3 tests/scripts/run_evaluation.py [--api URL] [--delay SEC] [--ids exp-001,todo-001]
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
CASES_FILE = ROOT / "tests" / "test_cases.json"
RESULTS_DIR = ROOT / "tests" / "results"


@dataclass
class CaseScore:
    id: str
    name: str
    category: str
    status: str  # pass | partial | fail | error
    classification_score: float  # 0-1
    extraction_score: float  # 0-1
    insight_score: float  # 0-1
    overall_score: float  # 0-5
    expected_type: str
    actual_type: str | None
    confidence: float | None
    model: str | None
    latency_ms: int
    notes: list[str] = field(default_factory=list)
    response: dict[str, Any] | None = None
    error: str | None = None


def load_cases(filter_ids: set[str] | None) -> list[dict]:
    data = json.loads(CASES_FILE.read_text())
    cases = data["cases"]
    if filter_ids:
        cases = [c for c in cases if c["id"] in filter_ids]
    return cases


def call_analyze(api: str, text: str, timeout: int = 180) -> tuple[dict | None, int, str | None, int]:
    payload = json.dumps({"text": text}).encode()
    req = urllib.request.Request(
        api,
        data=payload,
        headers={"Content-Type": "application/json"},
    )
    start = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read())
            ms = int((time.monotonic() - start) * 1000)
            return body, resp.status, None, ms
    except urllib.error.HTTPError as e:
        ms = int((time.monotonic() - start) * 1000)
        try:
            err_body = e.read().decode()
            err_json = json.loads(err_body)
            msg = err_json.get("error", err_body)
        except Exception:
            msg = str(e)
        return None, e.code, msg, ms


def score_classification(case: dict, actual_type: str | None, confidence: float | None) -> tuple[float, list[str]]:
    notes: list[str] = []
    expected = case["expected_type"]

    if expected == "error":
        return 0.0, ["Expected HTTP error — classification N/A"]

    if actual_type is None:
        return 0.0, ["No type returned"]

    acceptable = case.get("acceptable_types", [expected])
    if expected == "mixed":
        # Mixed notes: partial credit if any reasonable type picked
        if actual_type in acceptable:
            score = 0.5
            notes.append(f"Mixed content classified as '{actual_type}' — v0 single-type limitation")
        else:
            score = 0.25
            notes.append(f"Unexpected type '{actual_type}' for mixed input")
    elif actual_type == expected:
        score = 1.0
    elif actual_type in acceptable:
        score = 0.7
        notes.append(f"Acceptable alternate: expected {expected}, got {actual_type}")
    else:
        score = 0.0
        notes.append(f"Misclassified: expected {expected}, got {actual_type}")

    min_conf = case.get("min_confidence", 0.5)
    if confidence is not None and confidence < min_conf:
        score *= 0.8
        notes.append(f"Low confidence {confidence:.2f} < min {min_conf}")

    return score, notes


def score_extraction(case: dict, actual_type: str | None, structured: Any) -> tuple[float, list[str]]:
    notes: list[str] = []
    checks = case.get("checks", {})

    if checks.get("expect_http_error"):
        return 0.0, ["HTTP error case"]

    if structured is None:
        return 0.0, ["No structured output"]

    if isinstance(structured, str):
        try:
            structured = json.loads(structured)
        except json.JSONDecodeError:
            return 0.0, ["Structured output not valid JSON"]

    points = 0
    total = 0

    if "item_count_min" in checks:
        total += 1
        items = structured.get("items", [])
        if len(items) >= checks["item_count_min"]:
            points += 1
        else:
            notes.append(f"Expected >= {checks['item_count_min']} items, got {len(items)}")

    if "grand_total" in checks:
        total += 1
        gt = structured.get("grand_total")
        if gt == checks["grand_total"]:
            points += 1
        else:
            notes.append(f"grand_total: expected {checks['grand_total']}, got {gt}")

    if "grand_total_min" in checks:
        total += 1
        gt = structured.get("grand_total", 0)
        if gt >= checks["grand_total_min"]:
            points += 1
        else:
            notes.append(f"grand_total {gt} < min {checks['grand_total_min']}")

    if "currency" in checks:
        total += 1
        if structured.get("currency") == checks["currency"]:
            points += 1
        else:
            notes.append(f"currency: expected {checks['currency']}, got {structured.get('currency')}")

    if "has_category" in checks:
        total += 1
        cats = [i.get("category") for i in structured.get("items", [])]
        if checks["has_category"] in cats:
            points += 1
        else:
            notes.append(f"Missing category {checks['has_category']}, got {cats}")

    if checks.get("has_priority"):
        total += 1
        prios = [i.get("priority") for i in structured.get("items", [])]
        if checks["has_priority"] in prios:
            points += 1
        else:
            notes.append(f"Missing priority {checks['has_priority']}")

    if checks.get("has_due_dates"):
        total += 1
        dues = [i.get("due", "") for i in structured.get("items", [])]
        if any(d for d in dues):
            points += 1
        else:
            notes.append("No due dates resolved")

    if checks.get("has_done"):
        total += 1
        dones = [i.get("done") for i in structured.get("items", [])]
        if any(d is True for d in dones):
            points += 1
        else:
            notes.append("No completed tasks detected")

    if checks.get("has_summary"):
        total += 1
        if structured.get("summary"):
            points += 1
        else:
            notes.append("Missing summary")

    if "themes_min" in checks:
        total += 1
        themes = structured.get("themes", [])
        if len(themes) >= checks["themes_min"]:
            points += 1
        else:
            notes.append(f"Expected >= {checks['themes_min']} themes, got {len(themes)}")

    if "note" in checks and total == 0:
        # Qualitative-only check — neutral score
        return 0.5, [checks["note"]]

    if total == 0:
        return 0.5, ["No automated extraction checks defined"]

    return points / total, notes


def score_insights(case: dict, actual_type: str | None, structured: Any) -> tuple[float, list[str]]:
    """Heuristic quality score for usefulness of output."""
    notes: list[str] = []
    if structured is None or isinstance(structured, str):
        return 0.0, ["No output to judge"]

    if isinstance(structured, str):
        try:
            structured = json.loads(structured)
        except json.JSONDecodeError:
            return 0.0, ["Invalid JSON"]

    score = 0.5  # baseline

    if actual_type == "expenses":
        items = structured.get("items", [])
        if items and all(i.get("amount") for i in items):
            score += 0.2
        if structured.get("grand_total") is not None:
            score += 0.1
        flags = structured.get("flags", [])
        if flags:
            score += 0.1
            notes.append(f"Flags raised: {flags}")
        if not items:
            notes.append("No expense items extracted")
            score = 0.2

    elif actual_type == "todo":
        items = structured.get("items", [])
        if items and all(i.get("task") for i in items):
            score += 0.3
        has_prio = any(i.get("priority") != "medium" for i in items)
        has_due = any(i.get("due") for i in items)
        if has_prio:
            score += 0.1
        if has_due:
            score += 0.1
        if not items:
            score = 0.2

    elif actual_type == "generic":
        if structured.get("summary") and len(structured["summary"]) > 20:
            score += 0.2
        if structured.get("themes"):
            score += 0.15
        if structured.get("action_candidates"):
            score += 0.15
        if structured.get("questions"):
            score += 0.1

    return min(score, 1.0), notes


def overall_rating(cls: float, ext: float, ins: float) -> float:
    """Map to 0-5 stars."""
    avg = (cls * 0.35 + ext * 0.35 + ins * 0.30)
    return round(avg * 5, 1)


def status_from_scores(cls: float, ext: float, overall: float, had_error: bool) -> str:
    if had_error:
        return "error"
    if cls >= 0.9 and ext >= 0.8:
        return "pass"
    if overall >= 2.5:
        return "partial"
    return "fail"


def run_case(api: str, case: dict) -> CaseScore:
    checks = case.get("checks", {})
    body, status, err, latency = call_analyze(api, case["input"])

    if checks.get("expect_http_error"):
        if status == checks.get("expect_http_error", 400):
            return CaseScore(
                id=case["id"],
                name=case["name"],
                category=case["category"],
                status="pass",
                classification_score=1.0,
                extraction_score=1.0,
                insight_score=1.0,
                overall_score=5.0,
                expected_type=case["expected_type"],
                actual_type=None,
                confidence=None,
                model=None,
                latency_ms=latency,
                notes=[f"Correctly returned HTTP {status}"],
            )
        return CaseScore(
            id=case["id"],
            name=case["name"],
            category=case["category"],
            status="fail",
            classification_score=0.0,
            extraction_score=0.0,
            insight_score=0.0,
            overall_score=0.0,
            expected_type=case["expected_type"],
            actual_type=None,
            confidence=None,
            model=None,
            latency_ms=latency,
            error=err or f"HTTP {status}",
            notes=[f"Expected HTTP 400, got {status}"],
        )

    if body is None:
        return CaseScore(
            id=case["id"],
            name=case["name"],
            category=case["category"],
            status="error",
            classification_score=0.0,
            extraction_score=0.0,
            insight_score=0.0,
            overall_score=0.0,
            expected_type=case["expected_type"],
            actual_type=None,
            confidence=None,
            model=None,
            latency_ms=latency,
            error=err,
            notes=[f"API error: {err}"],
        )

    actual_type = body.get("type")
    confidence = body.get("confidence")
    structured = body.get("structured")
    model = body.get("model")

    cls_score, cls_notes = score_classification(case, actual_type, confidence)
    ext_score, ext_notes = score_extraction(case, actual_type, structured)
    ins_score, ins_notes = score_insights(case, actual_type, structured)
    overall = overall_rating(cls_score, ext_score, ins_score)

    all_notes = cls_notes + ext_notes + ins_notes
    if checks.get("note"):
        all_notes.append(f"Expected behavior: {checks['note']}")

    return CaseScore(
        id=case["id"],
        name=case["name"],
        category=case["category"],
        status=status_from_scores(cls_score, ext_score, overall, False),
        classification_score=round(cls_score, 2),
        extraction_score=round(ext_score, 2),
        insight_score=round(ins_score, 2),
        overall_score=overall,
        expected_type=case["expected_type"],
        actual_type=actual_type,
        confidence=confidence,
        model=model,
        latency_ms=latency,
        notes=all_notes,
        response=body,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Run MakeSense evaluation suite")
    parser.add_argument("--api", default="http://localhost:8080/api/analyze")
    parser.add_argument("--delay", type=float, default=2.0, help="Seconds between requests")
    parser.add_argument("--ids", default="", help="Comma-separated case IDs to run")
    args = parser.parse_args()

    filter_ids = {x.strip() for x in args.ids.split(",") if x.strip()} or None
    cases = load_cases(filter_ids)

    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    ts = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    out_file = RESULTS_DIR / f"eval_{ts}.json"

    print(f"Running {len(cases)} test cases against {args.api}\n")

    results: list[CaseScore] = []
    for i, case in enumerate(cases):
        print(f"[{i+1}/{len(cases)}] {case['id']} — {case['name']}...", flush=True)
        score = run_case(args.api, case)
        results.append(score)
        icon = {"pass": "✓", "partial": "~", "fail": "✗", "error": "!"}.get(score.status, "?")
        print(f"  {icon} {score.status} | type={score.actual_type} | {score.overall_score}/5 | {score.latency_ms}ms")
        if score.error:
            print(f"  ERROR: {score.error}")
        if i < len(cases) - 1:
            time.sleep(args.delay)

    # Summary
    by_status = {}
    for r in results:
        by_status[r.status] = by_status.get(r.status, 0) + 1

    avg_overall = sum(r.overall_score for r in results) / len(results) if results else 0
    avg_latency = sum(r.latency_ms for r in results) / len(results) if results else 0

    report = {
        "timestamp": ts,
        "api": args.api,
        "total": len(results),
        "summary": by_status,
        "avg_score": round(avg_overall, 2),
        "avg_latency_ms": int(avg_latency),
        "results": [asdict(r) for r in results],
    }

    out_file.write_text(json.dumps(report, indent=2))
    print(f"\n{'='*60}")
    print(f"SUMMARY: {by_status}")
    print(f"Average score: {avg_overall:.2f}/5")
    print(f"Average latency: {avg_latency:.0f}ms")
    print(f"Results written to {out_file}")

    # Also write latest symlink-style copy
    latest = RESULTS_DIR / "latest.json"
    latest.write_text(json.dumps(report, indent=2))

    return 0 if by_status.get("fail", 0) == 0 and by_status.get("error", 0) == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
