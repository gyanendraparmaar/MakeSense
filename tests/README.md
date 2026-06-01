# MakeSense Test Inputs

Curated inputs for manual and automated testing. Each file maps to one or more cases in `test_cases.json`.

## Categories

| Folder | Purpose |
|--------|---------|
| `expenses/` | Spending records, receipts, bills |
| `todo/` | Tasks, reminders, errands |
| `generic/` | Journal, meeting notes, research |
| `mixed/` | Multi-type notes (segmentation stress tests) |
| `edge/` | Ambiguity, negation, code, queries |

## Sources

Test design draws from:

- **Checklist** (Ribeiro et al., ACL 2020) — invariance, negation, vocabulary perturbation patterns
- **LMUnit** (Contextual AI) — natural-language unit tests for response quality dimensions
- **Real-world expense NLP** — Hinglish, shorthand, mixed currency (Vitmora, SmartExpenseBot patterns)
- **Product plan** (`smart-notepad-plan.md`) — segmentation + multi-block detection requirements

## Running tests

```bash
# Full suite (~20 min with FreeLLMAPI auto routing)
python3 tests/scripts/run_evaluation.py

# Subset
python3 tests/scripts/run_evaluation.py --ids exp-001,todo-001,mix-001

# Custom API
python3 tests/scripts/run_evaluation.py --api http://localhost:8080/api/analyze --delay 3
```

Results land in `tests/results/latest.json`.
