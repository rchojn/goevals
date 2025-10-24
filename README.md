# GoEvals - LLM Evaluation Dashboard

**Simple, self-hosted, Go-native dashboard for visualizing LLM evaluation results.**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Why GoEvals?

Existing LLM evaluation tools are either **Python-heavy** (complex setup), **cloud-only** (vendor lock-in), or **overkill** (full observability platforms).

**GoEvals** is different:
- ✅ **Single binary** - No Python, no Docker, no dependencies
- ✅ **Self-hosted** - Your data stays on your machine
- ✅ **Fast** - Starts in <100ms, handles millions of evals
- ✅ **Simple** - Works with standard JSONL format

Perfect for **Go developers building AI/ML applications** who want a lightweight eval dashboard.

---

## Quick Start

```bash
# Download binary (Linux/macOS/Windows)
# TODO: Add releases

# Or run from source
git clone https://github.com/rchojn/goevals
cd goevals
go run main.go evals_sample.jsonl

# Visit http://localhost:8080
```

---

## Features

### Current (MVP)
- ✅ Parse JSONL eval files (industry standard format)
- ✅ Model comparison table (avg score, min/max, response time)
- ✅ Aggregate statistics dashboard
- ✅ Zero-config setup

### Planned
- 📊 Interactive charts (score distribution, trends over time)
- 🔍 Test detail view (drill into individual evals)
- 📈 Export static HTML reports
- ⚡ Real-time updates (watch JSONL file for changes)

---

## JSONL Format

GoEvals expects **JSON Lines** format (one JSON object per line):

```jsonl
{"timestamp":"2025-10-24T18:43:35Z","model":"gpt-4","test_id":"test1","scores":{"combined":0.85},"response_time_ms":1234}
{"timestamp":"2025-10-24T18:44:12Z","model":"claude-3","test_id":"test1","scores":{"combined":0.92},"response_time_ms":987}
```

**Required fields:**
- `model` (string) - Model identifier
- `scores.combined` (float) - Overall score (0.0-1.0)

**Optional fields:**
- `timestamp` (ISO8601) - When eval was run
- `test_id` (string) - Test case identifier
- `response_time_ms` (int) - Generation time in milliseconds
- `scores.*` (float) - Additional score breakdowns
- `metadata` (object) - Custom metadata

**Why JSONL?**
- Streaming-friendly (parse line-by-line, low memory)
- Append-only (no file rewrite needed)
- Unix-friendly (`grep`, `jq`, `tail -f` work!)
- Industry standard (OpenAI, Google, Azure use it)

---

## Usage Examples

```bash
# Basic dashboard
goevals evals.jsonl

# Specify port
goevals --port 3000 evals.jsonl

# Watch mode (auto-reload on file changes)
goevals --watch evals.jsonl

# Export static HTML
goevals --export report.html evals.jsonl
```

---

## Generating JSONL Evals

### From Go (gai/eval)

```go
import "maragu.dev/gai/eval"

// Run eval
result := eval.LexicalSimilarityScorer(eval.ExactMatch)(sample)

// Log to JSONL
f, _ := os.OpenFile("evals.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
json.NewEncoder(f).Encode(map[string]any{
    "timestamp":        time.Now().Format(time.RFC3339),
    "model":           "gpt-4",
    "test_id":         "test1",
    "scores":          map[string]float64{"combined": float64(result.Score)},
    "response_time_ms": 1234,
})
```

### From Python

```python
import json
from datetime import datetime

eval_result = {
    "timestamp": datetime.now().isoformat(),
    "model": "gpt-4",
    "test_id": "test1",
    "scores": {"combined": 0.85},
    "response_time_ms": 1234
}

with open('evals.jsonl', 'a') as f:
    f.write(json.dumps(eval_result) + '\n')
```

### Compatible Tools

GoEvals works with eval outputs from:
- [gai/eval](https://github.com/maragudk/gai) (Go)
- [OpenAI Evals](https://github.com/openai/evals)
- [LangChain evaluators](https://python.langchain.com/docs/guides/evaluation/)
- Custom evaluation pipelines (just output JSONL!)

---

## Tech Stack

**Current (MVP):**
- Go stdlib (`net/http`, `html/template`, `encoding/json`)
- Zero dependencies

**Planned:**
- [a-h/templ](https://templ.guide) - Type-safe templates
- [htmx](https://htmx.org) - Dynamic interactions
- [Chart.js](https://www.chartjs.org) - Interactive charts

---

## Why Go?

Go is perfect for CLI/dashboard tools:
- **Single binary** - Distribute as one file (vs Python venv hell)
- **Fast startup** - <100ms (vs 5s for Python imports)
- **Low memory** - 20MB (vs 500MB+ for Python ML tools)
- **Cross-platform** - Compile once, run anywhere

---

## Roadmap

- [x] **Week 1**: MVP with basic dashboard
- [ ] **Month 1**: Chart.js integration, test detail view
- [ ] **Month 2**: Community feedback, feature requests
- [ ] **Month 3**: Enterprise features (if needed)

---

## Contributing

**Status**: Early MVP - Not accepting PRs yet!

**Want to help?**
1. ⭐ Star the repo
2. 🐛 Report bugs in [Issues](https://github.com/rchojn/goevals/issues)
3. 💬 Share feedback
4. 📢 Spread the word in Go/AI communities

---

## License

MIT License - Free forever, use anywhere.

See [LICENSE](LICENSE) for details.

---

## Author

Built by [@rchojn](https://github.com/rchojn) - Go developer exploring AI/ML tooling.

Inspired by:
- [evals.fun](https://evals.fun) - Simple eval visualization
- [Langfuse](https://langfuse.com) - LLM observability
- The Go + AI/ML community

---

**Built with Go stdlib and common sense.** 🚀
