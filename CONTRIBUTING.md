# Contributing to GoEvals

Thank you for your interest in contributing to GoEvals! This document provides guidelines and best practices for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Pre-commit Hooks](#pre-commit-hooks)
- [Commit Message Guidelines](#commit-message-guidelines)
- [Pull Request Process](#pull-request-process)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Guidelines](#testing-guidelines)
- [Documentation Guidelines](#documentation-guidelines)

---

## Code of Conduct

Be respectful, constructive, and professional in all interactions.

---

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Git
- (Optional) `git-chglog` for changelog generation

### Fork and Clone

```bash
# Fork the repository on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/goevals.git
cd goevals

# Add upstream remote
git remote add upstream https://github.com/rchojn/goevals.git
```

---

## Development Setup

### Build and Run

```bash
# Build the binary
go build -o goevals main.go

# Run with sample data
./goevals evals_sample.jsonl

# Visit http://localhost:3000
```

### Running Tests

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# With race detection
go test -race ./...
```

---

## Pre-commit Hooks

We use pre-commit hooks to catch issues before committing. This ensures code quality and saves time in code review.

### Install pre-commit

```bash
# Install pre-commit (one-time setup)
pip install pre-commit
# OR
pipx install pre-commit

# Install hooks for this repository
pre-commit install
pre-commit install --hook-type commit-msg
```

### What Gets Checked

The hooks automatically check:

- **Formatting**: Trailing whitespace, end-of-file fixes
- **YAML validation**: Checks all YAML files
- **Large files**: Blocks files >1MB
- **Go formatting**: `gofmt`, `go mod tidy`
- **Linting**: Runs `golangci-lint` with project config
- **Commit messages**: Validates conventional commits format

### Running Manually

```bash
# Run on all files
pre-commit run --all-files

# Run on staged files only
pre-commit run

# Skip hooks (not recommended)
git commit --no-verify
```

### If Hooks Fail

1. Review the error output
2. Fix the issues (hooks often auto-fix formatting)
3. Stage the fixed files: `git add .`
4. Commit again

**Example:**

```bash
$ git commit -m "fix: resolve polling bug"

Trim Trailing Whitespace.................................................Passed
Fix End of Files.........................................................Passed
Check Yaml...............................................................Passed
Check for added large files..............................................Passed
go-fmt...................................................................Passed
go-mod-tidy..............................................................Passed
golangci-lint............................................................Failed
- hook id: golangci-lint
- exit code: 1

main.go:150:2: error return value not checked (errcheck)
  _, err := json.Unmarshal(data, &result)

# Fix the error
$ git add .
$ git commit -m "fix: resolve polling bug"
✓ All hooks passed!
```

---

## Commit Message Guidelines

**We use [Conventional Commits](https://www.conventionalcommits.org/)** for all commit messages.

This ensures:
- Automatic changelog generation (via git-chglog)
- Semantic versioning
- Better code review
- Clear project history

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `refactor:` - Code refactoring (no functional changes)
- `test:` - Adding or updating tests
- `perf:` - Performance improvements
- `chore:` - Maintenance tasks (dependency updates, etc.)
- `ci:` - CI/CD changes
- `style:` - Code style changes (formatting, missing semicolons, etc.)

### Scope (optional)

Component affected: `dashboard`, `api`, `parser`, `server`, `docs`, etc.

### Examples

```bash
# Good commit messages
feat(dashboard): add sortable columns for custom metrics
fix(parser): handle malformed JSONL gracefully
docs(readme): update installation instructions
perf(api): reduce polling overhead by 50%
refactor(server): extract file watching logic
test(parser): add edge case tests for timestamps

# Bad commit messages (avoid these)
❌ "updating implementation"
❌ "cleaning"
❌ "improvements"
❌ "fixed stuff"
❌ "added feature"
```

### Breaking Changes

For breaking changes, add `BREAKING CHANGE:` in the footer:

```
feat(api)!: change JSONL format for scores

BREAKING CHANGE: Scores object now requires "combined" field.
Migration guide: Add "combined" score to all JSONL entries.
```

### Commit Message Validation

Pull requests will fail CI if commit messages don't follow conventional commits format. The workflow will check:

- Type is valid (`feat`, `fix`, `docs`, etc.)
- Subject line is present
- Subject line is < 100 characters

---

## Pull Request Process

### 1. Create a Feature Branch

```bash
# Update your main branch
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feat/your-feature-name
```

### 2. Make Your Changes

- Follow code style guidelines (see below)
- Add tests for new features
- Update documentation
- Ensure all tests pass

### 3. Commit Your Changes

```bash
# Stage your changes
git add .

# Commit with conventional commit message
git commit -m "feat(dashboard): add date range filtering"
```

### 4. Push and Create PR

```bash
# Push to your fork
git push origin feat/your-feature-name

# Create PR on GitHub
gh pr create --title "feat(dashboard): add date range filtering" --body "Implements date range filtering for eval results..."
```

### 5. Code Review

- Address review feedback
- Keep commits atomic and well-organized
- Rebase on main if needed:

```bash
git fetch upstream
git rebase upstream/main
git push --force-with-lease origin feat/your-feature-name
```

---

## Code Style Guidelines

### General Principles

1. **No emojis in code**
   - Console output: Use plain text
   - Comments: Use clear English
   - Variable names: Descriptive, no symbols

2. **Clear naming**
   - Functions: Verb + noun (e.g., `ParseEvalResults`, `StartServer`)
   - Variables: Descriptive (e.g., `evalResults`, not `er`)
   - Constants: ALL_CAPS (e.g., `DEFAULT_PORT`)

3. **Error handling**
   - Always check errors
   - Provide context in error messages
   - Use `fmt.Errorf` with `%w` for error wrapping

### Go-Specific

```go
// Good
func LoadEvals(path string) ([]EvalResult, error) {
    if path == "" {
        return nil, fmt.Errorf("path cannot be empty")
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %w", err)
    }

    var results []EvalResult
    // Parse JSONL...
    return results, nil
}

// Bad
func loadEvals(p string) []EvalResult {
    // No error handling
    d, _ := os.ReadFile(p)
    // Unsafe parsing...
    return results
}
```

### Frontend (HTML/Template)

- Use semantic HTML
- Keep templates simple and readable
- Use meaningful class names
- Follow responsive design principles

---

## Testing Guidelines

### Unit Tests

- Test one function per test
- Use table-driven tests for multiple scenarios
- Mock external dependencies (file I/O, etc.)

```go
func TestParseEvalResult(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid JSONL", `{"timestamp":"2025-01-01T00:00:00Z","model":"gpt-4","scores":{"combined":0.9}}`, false},
        {"missing timestamp", `{"model":"gpt-4","scores":{"combined":0.9}}`, true},
        {"invalid JSON", `{invalid}`, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := ParseEvalResult([]byte(tt.input))
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseEvalResult() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Tests

- Test full server flow (load → parse → serve)
- Use temporary test files
- Clean up resources after tests

### Performance Tests

- Use `testing.B` for benchmarks
- Include memory allocation metrics
- Compare before/after for optimizations

---

## Documentation Guidelines

### Code Documentation

- Document exported functions and types
- Explain WHY, not WHAT
- Include examples for complex functions

```go
// ParseEvalResult parses a single line of JSONL into an EvalResult.
// Returns error if JSON is malformed or required fields are missing.
//
// Required fields: timestamp, model, scores.combined
//
// Example:
//   result, err := ParseEvalResult([]byte(`{"timestamp":"...","model":"gpt-4","scores":{"combined":0.9}}`))
func ParseEvalResult(data []byte) (*EvalResult, error) {
    // ...
}
```

### README and Docs

- Keep README.md concise (Quick Start focused)
- Include screenshots showing dashboard
- Add usage examples with real JSONL
- Explain JSONL format clearly

---

## Issue Creation

Use the provided issue templates:

- **Bug Report**: For bugs or unexpected behavior
- **Feature Request**: For new features or enhancements
- **Performance Issue**: For performance problems

Search existing issues before creating a new one.

---

## Generating CHANGELOG

We use `git-chglog` to automatically generate CHANGELOG from conventional commits:

```bash
# Generate CHANGELOG
git-chglog -o CHANGELOG.md

# Generate for specific version
git-chglog -o CHANGELOG.md v2.1.0..HEAD
```

---

## Questions?

- Check README.md for documentation
- Search existing issues and discussions
- Ask in GitHub Discussions

---

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

---

**Happy Contributing!**
