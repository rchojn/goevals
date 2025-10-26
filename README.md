<div align="center">
  <img src="assets/goevals.png" alt="GoEvals Logo" width="200"/>

  # GoEvals

  **Fast, local-first LLM evaluation dashboard with smart refresh and sortable metrics**

  [![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
  [![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

  ![GoEvals Dashboard](assets/screenshot.png)
</div>

---

## Why GoEvals?

Most LLM evaluation dashboards are either **cloud-only** (vendor lock-in), **Python-heavy** (complex setup), or **overkill** (full observability platforms with databases).

**GoEvals** is different:

- ✅ **Single binary** - No Python, no Docker, no dependencies
- ✅ **Local-first** - Your data stays on your machine
- ✅ **Smart refresh** - Polls for new results without flickering (5s intervals)
- ✅ **Fast** - Starts in <100ms, handles thousands of evals
- ✅ **Simple** - Works with standard JSONL files

Built for **Go developers creating AI applications** who want a lightweight, hackable eval dashboard.

---

## Features

### 🎯 Core Features
- **Smart polling** - Efficient updates without full page reload
- **Sortable columns** - Click any header to sort by that metric
- **Color-coded scores** - Instant visual feedback (green >0.7, yellow 0.4-0.7, red <0.4)
- **Expandable details** - Click any test card to see full question, response, and metadata
- **Multiple files** - Load and compare results from multiple JSONL files
- **Custom metrics** - Automatically detects and displays any custom score fields

### 📊 Dashboard Views
- **Overview** - Total tests, models tested, average scores
- **Model comparison** - Side-by-side metrics with min/max/avg
- **Test results** - Detailed view with full questions, responses, and scoring breakdowns

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/rchojn/goevals
cd goevals

# Run with sample data
go run main.go evals_sample.jsonl

# Visit http://localhost:3000
```

### Build as Binary

```bash
# Build
go build -o goevals main.go

# Run
./goevals evals.jsonl

# Run on custom port
PORT=8080 ./goevals evals.jsonl
```

### Multiple Files

```bash
# Compare multiple test runs
./goevals run1.jsonl run2.jsonl run3.jsonl

# Compare yesterday vs today
./goevals yesterday.jsonl today.jsonl
```

---

## JSONL Format

GoEvals automatically detects all score fields in your JSONL and displays them in the dashboard.

### Minimal Example

The bare minimum (one JSON object per line):

```jsonl
{"timestamp":"2025-10-26T14:30:00Z","model":"gpt-4","scores":{"combined":0.85}}
{"timestamp":"2025-10-26T14:31:00Z","model":"claude-3","scores":{"combined":0.92}}
```

**Required fields:**
- `timestamp` - ISO8601 timestamp for ordering and smart polling
- `model` - Model name (string)
- `scores.combined` - Overall score 0.0-1.0 (float)

### Full Example

With all optional fields:

```jsonl
{
  "timestamp": "2025-10-26T14:30:00Z",
  "model": "gemma2:2b",
  "test_id": "eval_001",
  "question": "What is the capital of France?",
  "response": "The capital of France is Paris.",
  "expected": "Paris",
  "response_time_ms": 1234,
  "scores": {
    "combined": 0.85,
    "accuracy": 0.90,
    "fluency": 0.88,
    "completeness": 0.82
  },
  "metadata": {
    "run_id": "morning_test_run",
    "temperature": 0.7,
    "max_tokens": 2048
  }
}
```

**Optional fields:**
- `test_id` - Unique test identifier
- `question` - Input question/prompt
- `response` - Model's generated response
- `expected` - Expected/ground truth answer
- `response_time_ms` - Generation time in milliseconds
- `scores.*` - **Any custom score metrics** (auto-detected!)
- `metadata` - Any additional context

### Custom Scores

Just add them to the `scores` object - they'll automatically appear as sortable columns:

```jsonl
{"timestamp":"2025-10-26T14:30:00Z","model":"gpt-4","scores":{"combined":0.85,"accuracy":0.90,"creativity":0.88,"safety":0.95}}
```

---

## How It Works

### Smart Polling (No WebSockets Needed!)

GoEvals uses efficient HTTP polling instead of WebSockets:

1. Dashboard loads and remembers the latest `timestamp`
2. Every 5 seconds, fetches `/api/evals/since?ts=<timestamp>`
3. Server returns **only new results** added since that timestamp
4. If new results found, dashboard refreshes to recalculate stats
5. No flickering, no full reload, no WebSocket complexity

This is perfect for local development where you have:
- One developer, one browser tab
- Infrequent updates (tests complete in batches)
- Zero infrastructure complexity

### Architecture

```
┌─────────────┐         ┌─────────────┐         ┌──────────────┐
│  Tests      │         │  GoEvals    │         │  Browser     │
│  (append    │────────►│  Server     │◄────────│  Dashboard   │
│   to JSONL) │  write  │  (reload)   │  poll   │  (refresh)   │
└─────────────┘         └─────────────┘         └──────────────┘
```

**No database, no queue, no complexity** - just JSONL files and HTTP.

---

## Configuration

GoEvals uses sensible defaults but can be customized via environment variables:

```bash
# Custom port
PORT=9090 ./goevals evals.jsonl

# Auto-refresh interval is hardcoded to 5s (can be changed in code)
```

---

## Compatible With

GoEvals works with eval outputs from:

- [gai/eval](https://github.com/maragudk/gai) (Go) ← **Recommended**
- [OpenAI Evals](https://github.com/openai/evals)
- Any custom evaluation framework that outputs JSONL

### Example: Logging from Go

```go
f, _ := os.OpenFile("evals.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
json.NewEncoder(f).Encode(map[string]any{
    "timestamp": time.Now().Format(time.RFC3339),
    "model": "gpt-4",
    "test_id": "test_001",
    "scores": map[string]float64{
        "combined": 0.85,
        "accuracy": 0.90,
    },
    "response_time_ms": 1234,
})
```

---

## Roadmap

See [CHANGELOG.md](CHANGELOG.md) for recent updates.

**Future improvements:**
- [ ] Date range filtering in UI
- [ ] Charts and graphs (Chart.js integration)
- [ ] Export to CSV/JSON
- [ ] Type-safe templates ([a-h/templ](https://templ.guide))
- [ ] Test run comparison view
- [ ] WebSocket option for real-time updates

---

## Tech Stack

**Current (v2.0):**
- Pure Go stdlib (`net/http`, `html/template`, `encoding/json`)
- Zero external dependencies
- ~1000 lines of code
- Single file deployment

**Philosophy:**
- Local-first, no cloud required
- Simple > Complex
- Files > Databases
- HTTP polling > WebSockets (for this use case)

---

## Contributing

⭐ Star the repo if you find it useful!

🐛 Report bugs or request features in [Issues](https://github.com/rchojn/goevals/issues)

🔧 PRs welcome! Please open an issue first to discuss major changes.

---

## License

MIT License - Free forever, use anywhere.

See [LICENSE](LICENSE) for details.

---

## Author

Built by [@rchojn](https://github.com/rchojn) - Go developer building AI/ML tools.

Inspired by [evals.fun](https://evals.fun), [Langfuse](https://langfuse.com), and the philosophy that **simple tools > complex platforms** for local development.

---

<div align="center">
  <strong>Built with Go stdlib and common sense 🐹</strong>
  <br><br>
  <a href="https://github.com/rchojn/goevals">github.com/rchojn/goevals</a>
</div>
