# Changelog

All notable changes to GoEvals will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

<!-- Features and improvements currently in development -->

## [2.0.0] - 2025-10-26

**Major release** - Smart polling, sortable columns, and multiple files support.

### Added
- Smart polling endpoint `/api/evals/since` with 5-second refresh interval (no more flickering!)
- Multiple JSONL files support - `./goevals file1.jsonl file2.jsonl file3.jsonl`
- Sortable columns with visual indicators (⇅, ▲, ▼) - click any header to sort
- Automatic custom score detection from JSONL (framework-agnostic)
- Visual refresh indicator showing last update time and connection status

### Changed
- Improved refresh strategy - 5s smart polling vs 10s full page reload
- Simplified data model - use `timestamp` (ISO8601) for all time tracking

### Removed
- **BREAKING**: `test_run_date` field removed (use `timestamp` instead)

### Fixed
- Template rendering flickering during dashboard updates
- Scroll position lost on page reload
- Performance improvements for result filtering

### Migration from v1.x

**Update your JSONL:**
```jsonl
// Remove test_run_date field
{"timestamp": "2025-10-26T14:30:00Z", "model": "gpt-4", ...}
```

**Multiple files now supported:**
```bash
./goevals baseline.jsonl experimental.jsonl  # Compare runs
```

For detailed technical documentation, see:
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - Technical deep dive
- [docs/PHILOSOPHY.md](docs/PHILOSOPHY.md) - Design decisions
- [docs/v2.0-RELEASE-NOTES.md](docs/v2.0-RELEASE-NOTES.md) - Full release story

## [0.1.0] - 2025-10-24

**First public release!** 🐹

This is the initial MVP following AnthonyGG's "boring technology" philosophy.

### Added
- Project mascot (`assets/goevals.png`) - Gopher with evals theme 🐹
- Dashboard screenshot in README (`assets/screenshot.png`)
- CHANGELOG.md following Keep a Changelog format
- Initial MVP dashboard with Go stdlib
- JSONL parser with streaming support (`bufio.Scanner`)
- Model comparison table (avg score, min/max, response time)
- Health check endpoint (`/health`)
- Aggregate statistics (total tests, models tested, average score)
- Auto-calculated color-coded scores (green >0.8, yellow 0.6-0.8, red <0.6)
- Support for standard JSONL eval format (OpenAI, gai/eval compatible)
- Embedded HTML template with inline CSS
- Gopher-themed branding (🐹 instead of 🚀)
- MIT License

### Technical Details
- Zero dependencies (pure Go stdlib)
- Single binary deployment
- Default port: 3000
- Streaming JSONL parsing (O(1) memory)
- Centered README layout with mascot

### Documentation
- Comprehensive README with quick start
- Market analysis (competitor research, target users)
- Tech stack decision (stdlib MVP → templ/htmx later)
- JSONL format specification
- AnthonyGG tech philosophy guide

### Philosophy
- "Use boring technology" - stdlib over frameworks
- "Ship features, not complexity" - no webpack/babel
- AnthonyGG-inspired approach: simple, fast, maintainable

---

## Planned Features

### v0.2.0 - Essential CLI (Priority: HIGH for SafeReader)
**Goal:** Make it usable for daily SafeReader eval workflow

- [ ] **CLI flags** - `--port`, `--file` for better control
- [ ] **Watch mode** - Auto-reload when evals.jsonl changes (instant feedback!)
- [ ] **Test detail view** - Click model row → see individual test results
- [ ] **Filter controls** - Filter by model, test_id, date range
- [ ] **Error handling** - Better messages when JSONL is malformed

**Why this first?**
- Watch mode = instant feedback during SafeReader testing
- Drill-down = quickly find which tests failed and why
- Filters = focus on specific model/test when debugging

**Use case:**
```bash
# Run in background, auto-updates as tests complete
goevals --watch --port 3000 ../safereader/desktop/tests/evals.jsonl

# In another terminal: run SafeReader tests
go test -run TestEvalRAGModels -v

# Browser auto-refreshes with new results!
```

### v0.3.0 - Visualization (Priority: MEDIUM)
**Goal:** Visual comparison of models and trends

- [ ] **Chart.js integration** - Score distribution histogram
- [ ] **Timeline view** - Score trends over time (detect regression!)
- [ ] **Scatter plot** - Score vs response time (find sweet spot)
- [ ] **Keyword breakdown** - See which keywords are commonly missed

**Why this?**
- Visual comparison easier than reading tables
- Trend detection for SafeReader improvements
- Find performance/quality tradeoffs

### v0.4.0 - Collaboration (Priority: LOW)
**Goal:** Share results with team

- [ ] **Export HTML** - Static report for sharing
- [ ] **Compare multiple files** - Before/after prompt changes
- [ ] **JSON export** - Processed stats for external tools
- [ ] **CLI stats mode** - Quick summary without browser

### v0.5.0 - Polish & UX (Priority: LOW)
**Goal:** Better developer experience

- [ ] Migrate to `a-h/templ` (type-safe templates)
- [ ] Add htmx for dynamic updates (no page reload)
- [ ] Tailwind CSS styling
- [ ] Responsive mobile layout

### v1.0.0 - Production Ready (Priority: FUTURE)
**Goal:** Public release with confidence

- [ ] Comprehensive test suite (tests for the test dashboard!)
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Binary releases (Linux, macOS, Windows)
- [ ] Docker image (optional)
- [ ] Documentation site (GitHub Pages)

---

## Contributors

- [@rchojn](https://github.com/rchojn) - Creator & Maintainer

---

**Note**: This project follows the "build in public" philosophy. All development is transparent and community-driven.
