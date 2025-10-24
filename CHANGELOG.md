# Changelog

All notable changes to GoEvals will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

### v0.2.0 - Charts & Visualization
- [ ] Chart.js integration for score distribution
- [ ] Timeline view (score trends over time)
- [ ] Test detail drill-down page
- [ ] Filter by model/test_id

### v0.3.0 - Polish & UX
- [ ] Migrate to `a-h/templ` (type-safe templates)
- [ ] Add htmx for dynamic interactions
- [ ] Tailwind CSS styling
- [ ] Responsive mobile layout

### v0.4.0 - Advanced Features
- [ ] Watch mode (auto-reload on file changes)
- [ ] Export static HTML reports
- [ ] CLI flags (port, file path, watch mode)
- [ ] Multiple JSONL file comparison

### v1.0.0 - Production Ready
- [ ] Comprehensive test suite
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Binary releases (Linux, macOS, Windows)
- [ ] Docker image
- [ ] Full documentation site

---

## Contributors

- [@rchojn](https://github.com/rchojn) - Creator & Maintainer
- Built with assistance from [Claude Code](https://claude.com/claude-code)

---

**Note**: This project follows the "build in public" philosophy. All development is transparent and community-driven.
