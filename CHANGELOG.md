# Changelog

All notable changes to GoEvals will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- CI/CD pipeline with GitHub Actions (#1)
  - Cross-platform builds (Ubuntu, macOS, Windows)
  - Multi-version Go testing (1.21, 1.22, 1.23, 1.24)
  - Linting with golangci-lint
  - Code coverage reporting (Linux only)
- Pre-commit hooks for local development (#2)
  - Automatic code formatting (gofmt)
  - Dependency management (go mod tidy)
  - Linting enforcement
  - Commit message validation
- Status badges in README (#4)
  - CI build status
  - Go Report Card
- git-chglog configuration for automated changelog generation (#6)
- CONTRIBUTING.md guide for contributors (#5)
  - Development setup instructions
  - Pre-commit hooks usage
  - Conventional commits format
  - Pull request process
  - Code style and testing guidelines

### Fixed
- CI test failures on Windows platform
  - Added placeholder test file
  - Disabled coverage generation on non-Linux platforms

### Changed
- Updated Go version range to 1.21-1.24

---

## [0.1.0] - 2025-10-24

**First public release!**

This is the initial MVP following AnthonyGG's "boring technology" philosophy.

### Added
- Project mascot (assets/goevals.png) - Gopher with evals theme
- Dashboard screenshot in README (assets/screenshot.png)
- CHANGELOG.md following Keep a Changelog format
- Initial MVP dashboard with Go stdlib
- JSONL parser with streaming support (bufio.Scanner)
- Model comparison table (avg score, min/max, response time)
- Health check endpoint (/health)
- Aggregate statistics (total tests, models tested, average score)
- Auto-calculated color-coded scores (green >0.8, yellow 0.6-0.8, red <0.6)
- Support for standard JSONL eval format (OpenAI, gai/eval compatible)
- Embedded HTML template with inline CSS
- Gopher-themed branding
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

See [GitHub Issues](https://github.com/rchojn/goevals/issues) for planned enhancements.

[Unreleased]: https://github.com/rchojn/goevals/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/rchojn/goevals/releases/tag/v0.1.0
