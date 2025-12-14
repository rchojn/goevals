# GoEvals Task Runner for Windows
# PowerShell equivalent of Makefile for cross-platform development

param(
    [Parameter(Position=0)]
    [string]$Task = "help",

    [Parameter(Position=1, ValueFromRemainingArguments=$true)]
    [string[]]$Args
)

function Show-Help {
    Write-Host "GoEvals - Available commands:" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  help           " -ForegroundColor Green -NoNewline; Write-Host "Show this help message"
    Write-Host "  build          " -ForegroundColor Green -NoNewline; Write-Host "Build the binary (output: bin\goevals.exe)"
    Write-Host "  test           " -ForegroundColor Green -NoNewline; Write-Host "Run all tests with race detector"
    Write-Host "  test-short     " -ForegroundColor Green -NoNewline; Write-Host "Run tests without race detector (faster)"
    Write-Host "  run            " -ForegroundColor Green -NoNewline; Write-Host "Run with sample data (requires evals.jsonl)"
    Write-Host "  run-empty      " -ForegroundColor Green -NoNewline; Write-Host "Run with empty dashboard (no data file)"
    Write-Host "  clean          " -ForegroundColor Green -NoNewline; Write-Host "Clean build artifacts"
    Write-Host "  install        " -ForegroundColor Green -NoNewline; Write-Host "Install binary to GOPATH\bin"
    Write-Host "  fmt            " -ForegroundColor Green -NoNewline; Write-Host "Format code and tidy dependencies"
    Write-Host "  check          " -ForegroundColor Green -NoNewline; Write-Host "Run fmt and test (full check before commit)"
    Write-Host ""
    Write-Host "Usage: .\task.ps1 <command>" -ForegroundColor Yellow
    Write-Host "Example: .\task.ps1 build" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Linux/Mac users: Use 'make' instead" -ForegroundColor Gray
}

function Invoke-Build {
    Write-Host "Building GoEvals..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Force -Path "bin" | Out-Null
    go build -o bin\goevals.exe main.go
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build complete: bin\goevals.exe" -ForegroundColor Green
    } else {
        Write-Host "Build failed" -ForegroundColor Red
        exit 1
    }
}

function Invoke-Test {
    Write-Host "Running tests..." -ForegroundColor Cyan
    go test -v -race -cover .\...
}

function Invoke-TestShort {
    Write-Host "Running tests (short mode)..." -ForegroundColor Cyan
    go test -v .\...
}

function Invoke-Run {
    Write-Host "Starting GoEvals dashboard..." -ForegroundColor Cyan
    if (Test-Path "evals.jsonl") {
        go run main.go evals.jsonl
    } else {
        Write-Host "Error: evals.jsonl not found" -ForegroundColor Red
        Write-Host "Create sample file or specify path: .\task.ps1 run <path\to\evals.jsonl>" -ForegroundColor Yellow
        exit 1
    }
}

function Invoke-RunEmpty {
    Write-Host "Starting GoEvals with empty dashboard..." -ForegroundColor Cyan
    $tempFile = "$env:TEMP\goevals-empty.jsonl"
    New-Item -ItemType File -Force -Path $tempFile | Out-Null
    go run main.go $tempFile
}

function Invoke-Clean {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Cyan
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue bin\
    Remove-Item -Force -ErrorAction SilentlyContinue "$env:TEMP\goevals-*.jsonl"
    Write-Host "Clean complete" -ForegroundColor Green
}

function Invoke-Install {
    Write-Host "Installing to GOPATH\bin..." -ForegroundColor Cyan
    go install
    if ($LASTEXITCODE -eq 0) {
        $gopath = go env GOPATH
        Write-Host "Installed: $gopath\bin\goevals.exe" -ForegroundColor Green
    }
}

function Invoke-Format {
    Write-Host "Formatting code..." -ForegroundColor Cyan
    gofmt -s -w .
    go mod tidy
    Write-Host "Format complete" -ForegroundColor Green
}

function Invoke-Check {
    Write-Host "Running full check..." -ForegroundColor Cyan
    Invoke-Format
    Invoke-Test
    if ($LASTEXITCODE -eq 0) {
        Write-Host "All checks passed!" -ForegroundColor Green
    } else {
        Write-Host "Checks failed" -ForegroundColor Red
        exit 1
    }
}

# Main task dispatcher
switch ($Task.ToLower()) {
    "help"       { Show-Help }
    "build"      { Invoke-Build }
    "test"       { Invoke-Test }
    "test-short" { Invoke-TestShort }
    "run"        { Invoke-Run }
    "run-empty"  { Invoke-RunEmpty }
    "clean"      { Invoke-Clean }
    "install"    { Invoke-Install }
    "fmt"        { Invoke-Format }
    "check"      { Invoke-Check }
    default {
        Write-Host "Unknown task: $Task" -ForegroundColor Red
        Write-Host ""
        Show-Help
        exit 1
    }
}
