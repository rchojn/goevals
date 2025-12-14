package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

// EvalResult represents a single evaluation result from JSONL
type EvalResult struct {
	Timestamp      string         `json:"timestamp"` // ISO8601 timestamp - used for ordering and filtering
	Model          string         `json:"model"`
	TestID         string         `json:"test_id"`
	Question       string         `json:"question,omitempty"`
	Response       string         `json:"response,omitempty"`
	Expected       string         `json:"expected,omitempty"`
	Scores         ScoreBreakdown `json:"scores"`
	ResponseTimeMS int64          `json:"response_time_ms"`
	Metadata       map[string]any `json:"metadata,omitempty"` // Can include run_id, session_id, etc.

	// LLM-as-Judge fields
	JudgeModel             string `json:"judge_model,omitempty"`
	JudgeFactualReasoning  string `json:"judge_factual_reasoning,omitempty"`
	JudgeFaithfulReasoning string `json:"judge_faithful_reasoning,omitempty"`
	JudgeContextReasoning  string `json:"judge_context_reasoning,omitempty"`

	CustomFields map[string]any `json:"-"` // Captures any extra top-level fields dynamically
}

// Known field names for EvalResult (core fields that map to struct)
// All other fields become CustomFields and appear as dynamic table columns
var knownFields = map[string]bool{
	"timestamp":                true,
	"model":                    true,
	"test_id":                  true,
	"question":                 true,
	"response":                 true,
	"expected":                 true,
	"scores":                   true,
	"response_time_ms":         true,
	"metadata":                 true,
	"judge_model":              true,
	"judge_factual_reasoning":  true,
	"judge_faithful_reasoning": true,
	"judge_context_reasoning":  true,
	// Removed from knownFields - now detected as CustomFields:
	// "embedding_model", "chunk_size", "chunk_overlap", "top_k",
	// "retrieval_method", "temperature", "test_run_date", "question_id"
}

// UnmarshalJSON custom unmarshaler to capture custom top-level fields
func (er *EvalResult) UnmarshalJSON(data []byte) error {
	// Create temporary struct with same fields but no custom unmarshaler
	type Alias EvalResult
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(er),
	}

	// First unmarshal into map to get all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Unmarshal known fields into struct
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Capture all unknown fields as custom fields
	er.CustomFields = make(map[string]any)
	for key, value := range raw {
		if !knownFields[key] {
			er.CustomFields[key] = value
		}
	}

	return nil
}

// MarshalJSON custom marshaler to include custom fields in API responses
func (er EvalResult) MarshalJSON() ([]byte, error) {
	// Create map with all known fields
	result := make(map[string]interface{})
	result["timestamp"] = er.Timestamp
	result["model"] = er.Model
	result["test_id"] = er.TestID

	if er.Question != "" {
		result["question"] = er.Question
	}
	if er.Response != "" {
		result["response"] = er.Response
	}
	if er.Expected != "" {
		result["expected"] = er.Expected
	}

	result["scores"] = er.Scores
	result["response_time_ms"] = er.ResponseTimeMS

	if er.Metadata != nil {
		result["metadata"] = er.Metadata
	}

	// Add all custom fields
	for key, value := range er.CustomFields {
		result[key] = value
	}

	return json.Marshal(result)
}

// ScoreBreakdown contains all individual scores
// Uses map for flexibility - any custom scorer can be added
type ScoreBreakdown struct {
	Combined float64            `json:"combined"`
	Custom   map[string]float64 `json:"-"` // Populated dynamically from all other fields
}

// UnmarshalJSON custom unmarshaler to capture all score fields dynamically
func (sb *ScoreBreakdown) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to get all fields
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Extract combined score (required)
	if combined, ok := raw["combined"].(float64); ok {
		sb.Combined = combined
	}

	// Capture all other fields as custom scores
	sb.Custom = make(map[string]float64)
	for key, value := range raw {
		if key == "combined" {
			continue // Skip combined, already handled
		}
		if score, ok := value.(float64); ok {
			sb.Custom[key] = score
		}
	}

	return nil
}

// MarshalJSON custom marshaler to include custom scores in API responses
func (sb ScoreBreakdown) MarshalJSON() ([]byte, error) {
	// Build a map with combined + all custom scores
	result := make(map[string]float64)
	result["combined"] = sb.Combined

	// Add all custom scores
	for key, value := range sb.Custom {
		result[key] = value
	}

	return json.Marshal(result)
}

// DashboardData holds aggregated stats for the dashboard
type DashboardData struct {
	TotalTests       int
	AvgScore         float64
	Models           []string
	Results          []EvalResult
	ModelStats       map[string]ModelStat
	CustomScores     []string          // Names of all custom score types found
	CustomFieldNames []string          // Names of all custom top-level fields found
	CustomFieldTypes map[string]string // field_name -> type (string, number, bool)
}

// ModelStat holds statistics for a single model
type ModelStat struct {
	Model           string // Full config key (for internal use)
	ActualModelName string // Just the model name (for display)
	TestCount       int
	AvgScore        float64
	MinScore        float64
	MaxScore        float64
	CustomScores    map[string]float64 // Average for each custom score type
	AvgTimeMS       float64
	CustomFields    map[string]string // Custom field values (showing first unique value found)
}

// buildConfigKey creates a unique key for aggregation based on model + RAG config params
// This ensures that tests with the same model but different params (chunk_size, etc.) are grouped separately
// EXCLUDES: question_id, test_run_date (test metadata, not config parameters)
func buildConfigKey(result EvalResult) string {
	// Start with model name
	key := result.Model

	// Fields to exclude from aggregation key (test metadata, not RAG config)
	excludedFields := map[string]bool{
		"question_id":   true, // Question identifier - tests should be aggregated across all questions
		"test_run_date": true, // Test execution date - not a configuration parameter
	}

	// Add only RAG configuration fields in sorted order for consistency
	var fields []string
	for fieldName := range result.CustomFields {
		if !excludedFields[fieldName] {
			fields = append(fields, fieldName)
		}
	}
	sort.Strings(fields)

	for _, fieldName := range fields {
		value := result.CustomFields[fieldName]
		key += fmt.Sprintf("|%s=%v", fieldName, value)
	}

	return key
}

// ParseJSONL reads and parses a JSONL file
func ParseJSONL(filename string) ([]EvalResult, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	var results []EvalResult
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		var result EvalResult

		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			log.Printf("Warning: Skipping invalid JSON at line %d: %v", lineNum, err)
			continue
		}

		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return results, nil
}

// CalculateStats computes aggregate statistics from eval results
func CalculateStats(results []EvalResult) DashboardData {
	data := DashboardData{
		TotalTests:       len(results),
		Results:          results,
		ModelStats:       make(map[string]ModelStat),
		CustomFieldTypes: make(map[string]string),
	}

	if len(results) == 0 {
		return data
	}

	// Track unique configs, custom score types, and custom fields
	// Now aggregating by full config (model + all custom fields) instead of just model
	configSet := make(map[string]bool)
	customScoreSet := make(map[string]bool)
	customFieldSet := make(map[string]bool)
	configScores := make(map[string][]float64)
	configTimes := make(map[string][]float64)
	// configCustomScores[configKey][scoreType] = []scores
	configCustomScores := make(map[string]map[string][]float64)
	// configCustomFields[configKey][fieldName] = value (first seen value for that config)
	configCustomFields := make(map[string]map[string]string)
	totalScore := 0.0

	for _, result := range results {
		configKey := buildConfigKey(result)
		configSet[configKey] = true
		totalScore += result.Scores.Combined

		configScores[configKey] = append(configScores[configKey], result.Scores.Combined)
		configTimes[configKey] = append(configTimes[configKey], float64(result.ResponseTimeMS))

		// Collect all custom scores
		if configCustomScores[configKey] == nil {
			configCustomScores[configKey] = make(map[string][]float64)
		}
		for scoreType, scoreValue := range result.Scores.Custom {
			customScoreSet[scoreType] = true
			configCustomScores[configKey][scoreType] = append(
				configCustomScores[configKey][scoreType],
				scoreValue,
			)
		}

		// Collect all custom fields
		if configCustomFields[configKey] == nil {
			configCustomFields[configKey] = make(map[string]string)
		}
		for fieldName, fieldValue := range result.CustomFields {
			customFieldSet[fieldName] = true

			// Store first value seen for this config+field (or most common pattern)
			if _, exists := configCustomFields[configKey][fieldName]; !exists {
				configCustomFields[configKey][fieldName] = fmt.Sprintf("%v", fieldValue)
			}

			// Detect field type from first occurrence
			if _, exists := data.CustomFieldTypes[fieldName]; !exists {
				switch fieldValue.(type) {
				case float64:
					data.CustomFieldTypes[fieldName] = "number"
				case bool:
					data.CustomFieldTypes[fieldName] = "bool"
				default:
					data.CustomFieldTypes[fieldName] = "string"
				}
			}
		}
	}

	// Calculate overall average
	data.AvgScore = totalScore / float64(len(results))

	// Get sorted config list (configs, not just models)
	for configKey := range configSet {
		data.Models = append(data.Models, configKey)
	}
	sort.Strings(data.Models)

	// Get sorted custom score types
	for scoreType := range customScoreSet {
		data.CustomScores = append(data.CustomScores, scoreType)
	}
	sort.Strings(data.CustomScores)

	// Get sorted custom field names
	for fieldName := range customFieldSet {
		data.CustomFieldNames = append(data.CustomFieldNames, fieldName)
	}
	sort.Strings(data.CustomFieldNames)

	// Calculate per-config stats
	for _, configKey := range data.Models {
		scores := configScores[configKey]
		times := configTimes[configKey]

		if len(scores) == 0 {
			continue
		}

		// Calculate average score
		sum := 0.0
		min := scores[0]
		max := scores[0]
		for _, score := range scores {
			sum += score
			if score < min {
				min = score
			}
			if score > max {
				max = score
			}
		}

		// Calculate average time
		timeSum := 0.0
		for _, t := range times {
			timeSum += t
		}

		// Calculate average for each custom score type
		customAvgs := make(map[string]float64)
		for scoreType, scoreValues := range configCustomScores[configKey] {
			if len(scoreValues) > 0 {
				customSum := 0.0
				for _, v := range scoreValues {
					customSum += v
				}
				customAvgs[scoreType] = customSum / float64(len(scoreValues))
			}
		}

		// Get custom field values for this config
		customFields := make(map[string]string)
		if configFields, exists := configCustomFields[configKey]; exists {
			for fieldName, fieldValue := range configFields {
				customFields[fieldName] = fieldValue
			}
		}

		// Extract actual model name from config key (before first pipe)
		actualModelName := configKey
		if pipeIndex := strings.Index(configKey, "|"); pipeIndex != -1 {
			actualModelName = configKey[:pipeIndex]
		}

		data.ModelStats[configKey] = ModelStat{
			Model:           configKey,
			ActualModelName: actualModelName,
			TestCount:       len(scores),
			AvgScore:        sum / float64(len(scores)),
			MinScore:        min,
			MaxScore:        max,
			CustomScores:    customAvgs,
			AvgTimeMS:       timeSum / float64(len(times)),
			CustomFields:    customFields,
		}
	}

	return data
}

// Global variables
var evalData DashboardData
var evalFilenames []string // Support multiple JSONL files

// reloadData reloads eval results from all JSONL files
func reloadData() error {
	var allResults []EvalResult

	for _, filename := range evalFilenames {
		results, err := ParseJSONL(filename)
		if err != nil {
			log.Printf("Warning: Failed to parse %s: %v", filename, err)
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 {
		log.Println("Warning: No results yet - dashboard will show empty until first eval")
		// Initialize with empty data instead of crashing
		evalData = CalculateStats([]EvalResult{})
	} else {
		log.Printf("Loaded %d eval results total", len(allResults))
		evalData = CalculateStats(allResults)
	}
	return nil
}

func main() {
	// Check arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: goevals <file1.jsonl> [file2.jsonl] [...]")
		fmt.Println("\nExamples:")
		fmt.Println("  goevals evals.jsonl")
		fmt.Println("  goevals run1.jsonl run2.jsonl run3.jsonl")
		fmt.Println("  go run main.go evals.jsonl")
		os.Exit(1)
	}

	// Collect all file arguments
	evalFilenames = os.Args[1:]

	// Handle legacy "serve" subcommand
	if evalFilenames[0] == "serve" {
		if len(evalFilenames) < 2 {
			log.Fatal("Error: 'serve' requires at least one file argument")
		}
		evalFilenames = evalFilenames[1:] // Skip "serve"
	}

	// Load all files
	log.Printf("Loading evals from %d file(s)...", len(evalFilenames))
	var allResults []EvalResult
	for _, filename := range evalFilenames {
		results, err := ParseJSONL(filename)
		if err != nil {
			log.Printf("Warning: Failed to parse %s: %v", filename, err)
			continue
		}
		log.Printf("  ✓ %s: %d results", filename, len(results))
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 {
		log.Println("Warning: No results yet - starting with empty dashboard")
		evalData = CalculateStats([]EvalResult{})
	} else {
		log.Printf("Loaded %d eval results total", len(allResults))
		evalData = CalculateStats(allResults)
		log.Printf("Models found: %v", evalData.Models)
		log.Printf("Custom scores found: %v", evalData.CustomScores)
		log.Printf("Custom fields found: %v", evalData.CustomFieldNames)
		log.Printf("Overall avg score: %.2f", evalData.AvgScore)
	}

	// Setup HTTP handlers
	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/tests", testsHandler)
	http.HandleFunc("/api/evals", evalsAPIHandler)         // Full data API endpoint
	http.HandleFunc("/api/evals/since", evalsSinceHandler) // Smart polling endpoint
	http.HandleFunc("/health", healthHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	portStr := ":" + port

	log.Printf("🐹 GoEvals dashboard starting on http://localhost:%s", port)
	log.Printf("📊 Showing %d evals from %d models", evalData.TotalTests, len(evalData.Models))

	if err := http.ListenAndServe(portStr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Reload latest data from file
	if err := reloadData(); err != nil {
		log.Printf("Error reloading data: %v", err)
	}

	tmpl := `<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoEvals - LLM Evaluation Dashboard</title>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-secondary: #f8fafc;
            --bg-tertiary: #f1f5f9;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-tertiary: #94a3b8;
            --border-color: #e2e8f0;
            --accent: #3b82f6;
            --accent-hover: #2563eb;
            --success: #10b981;
            --warning: #f59e0b;
            --error: #ef4444;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
        }
        [data-theme="dark"] {
            --bg-primary: #1e293b;
            --bg-secondary: #0f172a;
            --bg-tertiary: #334155;
            --text-primary: #f1f5f9;
            --text-secondary: #cbd5e1;
            --text-tertiary: #64748b;
            --border-color: #334155;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.3);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2);
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            padding: 2rem;
            transition: background-color 0.3s ease, color 0.3s ease;
        }
        .container {
            max-width: 98%;
            margin: 0 auto;
        }
        header {
            background: var(--bg-primary);
            padding: 2rem;
            border-radius: 12px;
            box-shadow: var(--shadow-md);
            margin-bottom: 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            transition: background-color 0.3s ease, box-shadow 0.3s ease;
        }
        .header-left h1 {
            color: var(--text-primary);
            margin-bottom: 0.5rem;
            font-size: 1.875rem;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .subtitle {
            color: var(--text-secondary);
            font-size: 0.875rem;
        }
        .header-right {
            display: flex;
            gap: 0.75rem;
            align-items: center;
        }
        .theme-toggle, .help-btn {
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            padding: 0.5rem 0.75rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.875rem;
            transition: all 0.2s ease;
            font-weight: 500;
        }
        .theme-toggle:hover, .help-btn:hover {
            background: var(--bg-primary);
            border-color: var(--accent);
            color: var(--accent);
            transform: translateY(-1px);
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-primary);
            padding: 1.5rem;
            border-radius: 12px;
            box-shadow: var(--shadow-sm);
            border: 1px solid var(--border-color);
            transition: all 0.2s ease;
        }
        .stat-card:hover {
            box-shadow: var(--shadow-md);
            transform: translateY(-2px);
        }
        .stat-label {
            color: var(--text-tertiary);
            font-size: 0.75rem;
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            font-weight: 600;
        }
        .stat-value {
            color: var(--text-primary);
            font-size: 2rem;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .models-section {
            background: var(--bg-primary);
            padding: 2rem;
            border-radius: 12px;
            box-shadow: var(--shadow-md);
            border: 1px solid var(--border-color);
            transition: background-color 0.3s ease, box-shadow 0.3s ease;
        }
        .section-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
        }
        h2 {
            color: var(--text-primary);
            font-size: 1.5rem;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .auto-refresh-toggle {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            font-size: 0.875rem;
            color: var(--text-secondary);
            cursor: pointer;
        }
        .status-indicator {
            color: var(--success);
            animation: pulse 2s ease-in-out infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid var(--border-color);
            transition: background-color 0.2s ease;
        }
        th {
            background: var(--bg-tertiary);
            font-weight: 600;
            color: var(--text-secondary);
            font-size: 0.875rem;
            text-transform: uppercase;
            cursor: pointer;
            user-select: none;
            position: relative;
            padding-right: 20px;
        }
        th:hover {
            background: var(--bg-secondary);
        }
        th::after {
            content: '↕';
            position: absolute;
            right: 8px;
            opacity: 0.3;
            color: var(--text-tertiary);
        }
        th.sorted-asc::after {
            content: '↑';
            opacity: 1;
            color: var(--accent);
        }
        th.sorted-desc::after {
            content: '↓';
            opacity: 1;
            color: var(--accent);
        }
        td {
            color: var(--text-primary);
        }
        /* Sticky/Frozen column for Model name */
        th:nth-child(1), td:nth-child(1) {
            position: sticky;
            left: 0;
            background: var(--bg-primary);
            z-index: 10;
            box-shadow: 2px 0 4px rgba(0,0,0,0.05);
            min-width: 200px;
            max-width: 200px;
        }
        th:nth-child(1) {
            background: var(--bg-tertiary);
            z-index: 11;
        }
        /* Default column widths (dynamic columns) */
        th, td {
            min-width: 100px;
        }
        /* Score columns - narrower */
        .score-cell {
            min-width: 90px;
            max-width: 90px;
            text-align: center;
            font-weight: 600;
        }
        tbody tr {
            transition: background-color 0.2s ease;
        }
        tbody tr:hover {
            background-color: var(--bg-secondary);
        }
        .score {
            font-weight: 600;
        }
        .score-good { color: #10b981; }
        .score-fair { color: #f59e0b; }
        .score-poor { color: #ef4444; }
        footer {
            text-align: center;
            color: var(--text-tertiary);
            margin-top: 2rem;
            font-size: 0.875rem;
        }
        footer a {
            color: var(--accent);
            transition: color 0.2s ease;
        }
        footer a:hover {
            color: var(--accent-hover);
        }
        .help-modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }
        .help-modal.show {
            display: flex;
        }
        .help-content {
            background: var(--bg-primary);
            padding: 2rem;
            border-radius: 12px;
            max-width: 500px;
            box-shadow: var(--shadow-md);
        }
        .help-content h3 {
            color: var(--text-primary);
            margin-bottom: 1rem;
        }
        .help-content table {
            width: 100%;
            border-collapse: collapse;
        }
        .help-content td {
            padding: 0.5rem;
            border-bottom: 1px solid var(--border-color);
            color: var(--text-secondary);
        }
        .help-content td:first-child {
            font-family: monospace;
            font-weight: 600;
            color: var(--accent);
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <div class="header-left">
                <h1>GoEvals Dashboard</h1>
                <p class="subtitle">Simple, self-hosted LLM evaluation visualization</p>
            </div>
            <div class="header-right">
                <button id="theme-toggle" class="theme-toggle">
                    <span id="theme-icon">Dark</span>
                </button>
                <button id="help-btn" class="help-btn">?</button>
            </div>
        </header>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Total Tests</div>
                <div class="stat-value">{{ .TotalTests }}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Models Tested</div>
                <div class="stat-value">{{ len .Models }}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Average Score</div>
                <div class="stat-value">{{ printf "%.2f" .AvgScore }}</div>
            </div>
        </div>

        <div class="models-section">
            <h2>Model Comparison</h2>
            <div style="overflow-x: auto;">
            <table id="comparison-table">
                <thead>
                    <tr>
                        <th onclick="sortTable(0)">Model</th>
                        <th onclick="sortTable(1)" class="sorted-desc">Combined</th>
                        {{ range $idx, $fieldName := $.CustomFieldNames }}
                        <th onclick="sortTable({{ add 2 $idx }})">{{ $fieldName }}</th>
                        {{ end }}
                        {{ range $idx, $score := $.CustomScores }}
                        <th onclick="sortTable({{ add (add 2 (len $.CustomFieldNames)) $idx }})" class="score-cell">{{ $score }}</th>
                        {{ end }}
                        <th onclick="sortTable({{ add (add 2 (len $.CustomFieldNames)) (len $.CustomScores) }})">Tests</th>
                        <th onclick="sortTable({{ add (add 3 (len $.CustomFieldNames)) (len $.CustomScores) }})">Min</th>
                        <th onclick="sortTable({{ add (add 4 (len $.CustomFieldNames)) (len $.CustomScores) }})">Max</th>
                        <th onclick="sortTable({{ add (add 5 (len $.CustomFieldNames)) (len $.CustomScores) }})">Time (ms)</th>
                    </tr>
                </thead>
                <tbody id="table-body">
                    {{ range .Models }}
                    {{ $stat := index $.ModelStats . }}
                    <tr style="cursor: pointer;" onclick="window.location='/tests?model={{ $stat.Model }}'">
                        <td><strong>{{ $stat.ActualModelName }}</strong></td>
                        <td class="score {{ if ge $stat.AvgScore 0.7 }}score-good{{ else if ge $stat.AvgScore 0.5 }}score-fair{{ else }}score-poor{{ end }}">{{ printf "%.2f" $stat.AvgScore }}</td>
                        {{ range $fieldName := $.CustomFieldNames }}
                        <td>{{ formatValue (index $stat.CustomFields $fieldName) }}</td>
                        {{ end }}
                        {{ range $scoreType := $.CustomScores }}
                        {{ $customScore := index $stat.CustomScores $scoreType }}
                        <td class="score-cell score {{ if ge $customScore 0.7 }}score-good{{ else if ge $customScore 0.4 }}score-fair{{ else }}score-poor{{ end }}">{{ printf "%.2f" $customScore }}</td>
                        {{ end }}
                        <td>{{ $stat.TestCount }}</td>
                        <td>{{ printf "%.2f" $stat.MinScore }}</td>
                        <td>{{ printf "%.2f" $stat.MaxScore }}</td>
                        <td>{{ printf "%.0f" $stat.AvgTimeMS }}</td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
            </div>
        </div>

        <footer>
            Built with Go stdlib + HTML + common sense<br>
            <a href="https://github.com/rchojn/goevals">github.com/rchojn/goevals</a><br>
            <div style="margin-top: 0.75rem; display: flex; align-items: center; justify-content: center; gap: 1rem;">
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; font-size: 0.875rem;">
                    <input type="checkbox" id="autorefresh-toggle" checked style="cursor: pointer;">
                    <span>Auto-refresh (5s)</span>
                </label>
                <span id="refresh-indicator" style="font-size: 0.8rem;">Enabled</span>
            </div>
        </footer>
    </div>

    <div id="help-modal" class="help-modal">
        <div class="help-content">
            <h3>Keyboard Shortcuts</h3>
            <table>
                <tr><td>D</td><td>Toggle dark mode</td></tr>
                <tr><td>R</td><td>Refresh dashboard</td></tr>
                <tr><td>?</td><td>Show this help</td></tr>
                <tr><td>Esc</td><td>Close help</td></tr>
            </table>
        </div>
    </div>

    <script>
        // Dark mode toggle
        const themeToggle = document.getElementById('theme-toggle');
        const themeIcon = document.getElementById('theme-icon');
        const html = document.documentElement;
        const savedTheme = localStorage.getItem('theme') || 'light';
        html.setAttribute('data-theme', savedTheme);
        themeIcon.textContent = savedTheme === 'light' ? 'Dark' : 'Light';

        themeToggle.addEventListener('click', () => {
            const currentTheme = html.getAttribute('data-theme');
            const newTheme = currentTheme === 'light' ? 'dark' : 'light';
            html.setAttribute('data-theme', newTheme);
            localStorage.setItem('theme', newTheme);
            themeIcon.textContent = newTheme === 'light' ? 'Dark' : 'Light';
        });

        // Help modal
        const helpBtn = document.getElementById('help-btn');
        const helpModal = document.getElementById('help-modal');

        helpBtn.addEventListener('click', () => {
            helpModal.classList.add('show');
        });

        helpModal.addEventListener('click', (e) => {
            if (e.target === helpModal) {
                helpModal.classList.remove('show');
            }
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            if (e.key === 'd' || e.key === 'D') {
                e.preventDefault();
                themeToggle.click();
            }
            if (e.key === '?') {
                e.preventDefault();
                helpModal.classList.add('show');
            }
            if (e.key === 'r' || e.key === 'R') {
                e.preventDefault();
                location.reload();
            }
            if (e.key === 'Escape') {
                helpModal.classList.remove('show');
            }
        });

        // Smart polling - fetch only new results every 5 seconds
        let lastTimestamp = new Date().toISOString();
        let pollInterval = 5000; // 5 seconds
        const indicator = document.getElementById('refresh-indicator');
        const toggleCheckbox = document.getElementById('autorefresh-toggle');

        // Load autorefresh preference from localStorage (default: enabled)
        let autoRefreshEnabled = localStorage.getItem('autorefresh') !== 'false';
        toggleCheckbox.checked = autoRefreshEnabled;

        // Update indicator based on state
        function updateIndicator() {
            if (!autoRefreshEnabled) {
                indicator.textContent = 'Disabled';
            } else {
                indicator.textContent = 'Enabled';
            }
        }
        updateIndicator();

        // Toggle handler
        toggleCheckbox.addEventListener('change', function() {
            autoRefreshEnabled = this.checked;
            localStorage.setItem('autorefresh', autoRefreshEnabled);
            updateIndicator();
            if (autoRefreshEnabled) {
                pollForUpdates(); // Poll immediately when re-enabled
            }
        });

        async function pollForUpdates() {
            if (!autoRefreshEnabled) {
                return; // Skip if disabled
            }

            try {
                const response = await fetch('/api/evals/since?ts=' + encodeURIComponent(lastTimestamp));
                if (!response.ok) {
                    indicator.textContent = '⚠️ Update failed';
                    return;
                }

                const newEvals = await response.json();
                if (newEvals && newEvals.length > 0) {
                    // New data found - reload to recalculate stats
                    console.log('Found ' + newEvals.length + ' new eval(s), refreshing...');
                    location.reload();
                } else {
                    // No new data - update indicator
                    indicator.textContent = '✓ Up to date (checked ' + new Date().toLocaleTimeString() + ')';
                }
            } catch (error) {
                console.error('Poll error:', error);
                indicator.textContent = '⚠️ Connection error';
            }
        }

        // Poll every 5 seconds (but function checks autoRefreshEnabled)
        setInterval(pollForUpdates, pollInterval);

        // Initial poll after 5 seconds
        setTimeout(pollForUpdates, pollInterval);

        // Table sorting
        let sortDirection = {};
        function sortTable(colIndex) {
            const table = document.getElementById('comparison-table');
            const tbody = document.getElementById('table-body');
            const rows = Array.from(tbody.querySelectorAll('tr'));

            // Toggle sort direction
            sortDirection[colIndex] = sortDirection[colIndex] === 'asc' ? 'desc' : 'asc';
            const direction = sortDirection[colIndex];

            // Update header indicators
            table.querySelectorAll('th').forEach(th => {
                th.classList.remove('sorted-asc', 'sorted-desc');
            });
            const th = table.querySelectorAll('th')[colIndex];
            th.classList.add(direction === 'asc' ? 'sorted-asc' : 'sorted-desc');

            // Sort rows
            rows.sort((a, b) => {
                const aVal = a.cells[colIndex].textContent.trim();
                const bVal = b.cells[colIndex].textContent.trim();

                // Try to parse as numbers
                const aNum = parseFloat(aVal);
                const bNum = parseFloat(bVal);

                if (!isNaN(aNum) && !isNaN(bNum)) {
                    return direction === 'asc' ? aNum - bNum : bNum - aNum;
                }

                // String comparison
                return direction === 'asc'
                    ? aVal.localeCompare(bVal)
                    : bVal.localeCompare(aVal);
            });

            // Re-append sorted rows
            rows.forEach(row => tbody.appendChild(row));
        }

        // Default sort by Avg Score descending
        const headers = document.querySelectorAll('#comparison-table th');
        headers.forEach((th, idx) => {
            if (th.classList.contains('sorted-desc')) {
                sortDirection[idx] = 'desc';
            }
        });

    </script>
</body>
</html>`

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"formatTemp": func(val interface{}) string {
			if val == nil {
				return "-"
			}
			switch v := val.(type) {
			case float64:
				return fmt.Sprintf("%.1f", v)
			case string:
				// Try to parse string as float
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					return fmt.Sprintf("%.1f", parsed)
				}
				return v
			default:
				return fmt.Sprintf("%v", v)
			}
		},
		"formatValue": func(val string) string {
			// Try to parse as float
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				// Round to 1 decimal place for temperatures
				// Round to 0 decimals for integers
				if parsed == float64(int64(parsed)) {
					return fmt.Sprintf("%.0f", parsed)
				}
				return fmt.Sprintf("%.1f", parsed)
			}
			return val
		},
	}
	t := template.Must(template.New("dashboard").Funcs(funcMap).Parse(tmpl))
	if err := t.Execute(w, evalData); err != nil {
		// Don't call http.Error here - headers already sent by Execute
		log.Printf("Template error: %v", err)
	}
}

func testsHandler(w http.ResponseWriter, r *http.Request) {
	// Reload latest data from file
	if err := reloadData(); err != nil {
		log.Printf("Error reloading data: %v", err)
	}

	// Filter by model or run_id if provided
	modelFilter := r.URL.Query().Get("model")
	runIDFilter := r.URL.Query().Get("run_id")

	var filteredResults []EvalResult
	if modelFilter != "" || runIDFilter != "" {
		for _, result := range evalData.Results {
			// Use buildConfigKey to match the full config key (model + params)
			configKey := buildConfigKey(result)
			matchModel := modelFilter == "" || configKey == modelFilter

			// Extract run_id from metadata
			runID := ""
			if result.Metadata != nil {
				if rid, ok := result.Metadata["run_id"].(string); ok {
					runID = rid
				}
			}
			matchRunID := runIDFilter == "" || runID == runIDFilter

			if matchModel && matchRunID {
				filteredResults = append(filteredResults, result)
			}
		}
	} else {
		filteredResults = evalData.Results
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(filteredResults, func(i, j int) bool {
		return filteredResults[i].Timestamp > filteredResults[j].Timestamp
	})

	tmpl := `<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Test Results - GoEvals</title>
    <style>
        :root {
            --bg-primary: #ffffff;
            --bg-secondary: #f8fafc;
            --bg-tertiary: #f1f5f9;
            --text-primary: #0f172a;
            --text-secondary: #475569;
            --text-tertiary: #94a3b8;
            --border-color: #e2e8f0;
            --accent: #3b82f6;
            --accent-hover: #2563eb;
            --success: #10b981;
            --warning: #f59e0b;
            --error: #ef4444;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
        }
        [data-theme="dark"] {
            --bg-primary: #1e293b;
            --bg-secondary: #0f172a;
            --bg-tertiary: #334155;
            --text-primary: #f1f5f9;
            --text-secondary: #cbd5e1;
            --text-tertiary: #64748b;
            --border-color: #334155;
            --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.3);
            --shadow-md: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -1px rgba(0, 0, 0, 0.2);
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: var(--bg-secondary);
            color: var(--text-primary);
            padding: 2rem;
            transition: background-color 0.3s ease, color 0.3s ease;
        }
        .container {
            max-width: 95%;
            margin: 0 auto;
        }
        header {
            background: var(--bg-primary);
            padding: 2rem;
            border-radius: 12px;
            box-shadow: var(--shadow-md);
            margin-bottom: 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            transition: background-color 0.3s ease, box-shadow 0.3s ease;
        }
        h1 {
            color: var(--text-primary);
            margin-bottom: 0.5rem;
        }
        .subtitle {
            color: var(--text-secondary);
            font-size: 0.875rem;
        }
        .back-link {
            display: inline-block;
            margin-bottom: 1rem;
            color: var(--accent);
            text-decoration: none;
        }
        .back-link:hover {
            text-decoration: underline;
        }
        .header-left h1 {
            color: var(--text-primary);
            margin-bottom: 0.5rem;
            font-size: 1.875rem;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .header-right {
            display: flex;
            gap: 0.75rem;
            align-items: center;
        }
        .theme-toggle, .help-btn {
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            padding: 0.5rem 0.75rem;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.875rem;
            transition: all 0.2s ease;
            font-weight: 500;
        }
        .theme-toggle:hover, .help-btn:hover {
            background: var(--bg-primary);
            border-color: var(--accent);
            color: var(--accent);
            transform: translateY(-1px);
        }
        .help-modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
        }
        .help-modal.show {
            display: flex;
        }
        .help-content {
            background: var(--bg-primary);
            padding: 2rem;
            border-radius: 12px;
            max-width: 500px;
            box-shadow: var(--shadow-md);
        }
        .help-content h3 {
            color: var(--text-primary);
            margin-bottom: 1rem;
        }
        .help-content table {
            width: 100%;
            border-collapse: collapse;
        }
        .help-content td {
            padding: 0.5rem;
            border-bottom: 1px solid var(--border-color);
            color: var(--text-secondary);
        }
        .help-content td:first-child {
            font-family: monospace;
            font-weight: 600;
            color: var(--accent);
        }
        .tests-table {
            background: var(--bg-primary);
            border-radius: 12px;
            border: 1px solid var(--border-color);
            overflow: hidden;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th {
            background: var(--bg-tertiary);
            padding: 0.75rem 1rem;
            text-align: left;
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-tertiary);
            border-bottom: 1px solid var(--border-color);
        }
        tbody tr {
            border-bottom: 1px solid var(--border-color);
            cursor: pointer;
            transition: background-color 0.15s ease;
        }
        tbody tr:hover {
            background: var(--bg-secondary);
        }
        tbody tr:last-child {
            border-bottom: none;
        }
        td {
            padding: 1rem;
            color: var(--text-primary);
            font-size: 0.875rem;
        }
        .test-id {
            font-family: monospace;
            font-size: 0.8125rem;
            color: var(--text-tertiary);
        }
        .model-name {
            font-weight: 500;
            color: var(--text-primary);
        }
        .score-badge {
            display: inline-flex;
            align-items: center;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8125rem;
            font-weight: 500;
            font-family: monospace;
        }
        .score-good {
            background: rgba(16, 185, 129, 0.1);
            color: var(--success);
        }
        .score-fair {
            background: rgba(245, 158, 11, 0.1);
            color: var(--warning);
        }
        .score-poor {
            background: rgba(239, 68, 68, 0.1);
            color: var(--error);
        }
        .time-badge {
            font-family: monospace;
            font-size: 0.8125rem;
            color: var(--text-tertiary);
        }
        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.5);
            z-index: 1000;
            align-items: center;
            justify-content: center;
            backdrop-filter: blur(4px);
        }
        .modal.show {
            display: flex;
        }
        .modal-content {
            background: var(--bg-primary);
            border-radius: 12px;
            max-width: 900px;
            max-height: 90vh;
            overflow-y: auto;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
            border: 1px solid var(--border-color);
        }
        .modal-header {
            padding: 1.5rem;
            border-bottom: 1px solid var(--border-color);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .modal-title {
            font-size: 1.125rem;
            font-weight: 600;
            color: var(--text-primary);
        }
        .modal-close {
            background: transparent;
            border: none;
            color: var(--text-tertiary);
            cursor: pointer;
            font-size: 1.5rem;
            padding: 0;
            width: 2rem;
            height: 2rem;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 6px;
            transition: all 0.15s ease;
        }
        .modal-close:hover {
            background: var(--bg-secondary);
            color: var(--text-primary);
        }
        .modal-body {
            padding: 1.5rem;
        }
        .detail-section {
            margin-bottom: 1.5rem;
        }
        .detail-section:last-child {
            margin-bottom: 0;
        }
        .detail-label {
            font-weight: 600;
            color: var(--text-secondary);
            margin-bottom: 0.5rem;
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .detail-content {
            padding: 1rem;
            background: var(--bg-secondary);
            border-radius: 8px;
            font-size: 0.875rem;
            line-height: 1.6;
            white-space: pre-wrap;
            color: var(--text-primary);
        }
        .scores-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 0.75rem;
        }
        .score-item {
            padding: 0.75rem;
            background: var(--bg-primary);
            border-radius: 4px;
            border: 1px solid var(--border-color);
        }
        .score-item-label {
            font-size: 0.75rem;
            color: var(--text-tertiary);
            margin-bottom: 0.25rem;
        }
        .score-item-value {
            font-size: 1.25rem;
            font-weight: 600;
        }
        .metadata-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 0.5rem;
        }
        .metadata-item {
            padding: 0.5rem 0.75rem;
            background: var(--bg-primary);
            border-radius: 4px;
            border: 1px solid var(--border-color);
            font-size: 0.8125rem;
        }
        .metadata-key {
            color: var(--text-tertiary);
            font-weight: 500;
        }
        .metadata-value {
            color: var(--text-primary);
            margin-left: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link">← Back to Dashboard</a>

        <header>
            <div class="header-left">
                <h1>Test Results {{ if . }}({{ len . }} tests){{ end }}</h1>
                <p class="subtitle">Click on any test to see full details</p>
            </div>
            <div class="header-right">
                <button id="theme-toggle" class="theme-toggle">
                    <span id="theme-icon">Dark</span>
                </button>
                <button id="help-btn" class="help-btn">?</button>
            </div>
        </header>

        <div id="help-modal" class="help-modal">
            <div class="help-content">
                <h3>Keyboard Shortcuts</h3>
                <table>
                    <tr><td>D</td><td>Toggle dark mode</td></tr>
                    <tr><td>R</td><td>Refresh page</td></tr>
                    <tr><td>?</td><td>Show this help</td></tr>
                    <tr><td>Esc</td><td>Close help</td></tr>
                </table>
            </div>
        </div>

        <div class="tests-table">
            <table>
                <thead>
                    <tr>
                        <th>Test ID</th>
                        <th>Model</th>
                        <th>Question</th>
                        <th>Score</th>
                        <th>Time</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range $index, $result := . }}
                    <tr onclick="showTestModal({{ $index }})">
                        <td class="test-id">{{ $result.TestID }}</td>
                        <td class="model-name">{{ $result.Model }}</td>
                        <td style="max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ $result.Question }}</td>
                        <td>
                            <span class="score-badge {{ if ge $result.Scores.Combined 0.7 }}score-good{{ else if ge $result.Scores.Combined 0.5 }}score-fair{{ else }}score-poor{{ end }}">
                                {{ printf "%.2f" $result.Scores.Combined }}
                            </span>
                        </td>
                        <td class="time-badge">{{ $result.ResponseTimeMS }}ms</td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>

        {{ range $index, $result := . }}
        <div id="modal-{{ $index }}" class="modal">
            <div class="modal-content">
                <div class="modal-header">
                    <div class="modal-title">{{ $result.TestID }}</div>
                    <button class="modal-close" onclick="closeTestModal({{ $index }})">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="detail-section">
                        <div class="detail-label">Question</div>
                        <div class="detail-content">{{ $result.Question }}</div>
                    </div>

                    <div class="detail-section">
                        <div class="detail-label">Model Response</div>
                        <div class="detail-content">{{ if $result.Response }}{{ $result.Response }}{{ else }}<em style="color: #9ca3af;">No response recorded</em>{{ end }}</div>
                    </div>

                    {{ if $result.Expected }}
                    <div class="detail-section">
                        <div class="detail-label">Expected Response</div>
                        <div class="detail-content">{{ $result.Expected }}</div>
                    </div>
                    {{ end }}

                    {{ if $result.JudgeModel }}
                    <div class="detail-section">
                        <div class="detail-label">Judge Evaluation ({{ $result.JudgeModel }})</div>
                        {{ if $result.JudgeFactualReasoning }}
                        <div style="margin-bottom: 0.75rem;">
                            <div style="font-weight: 600; color: var(--text-tertiary); font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Factual Correctness</div>
                            <div class="detail-content">{{ $result.JudgeFactualReasoning }}</div>
                        </div>
                        {{ end }}
                        {{ if $result.JudgeFaithfulReasoning }}
                        <div style="margin-bottom: 0.75rem;">
                            <div style="font-weight: 600; color: var(--text-tertiary); font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Faithfulness</div>
                            <div class="detail-content">{{ $result.JudgeFaithfulReasoning }}</div>
                        </div>
                        {{ end }}
                        {{ if $result.JudgeContextReasoning }}
                        <div style="margin-bottom: 0;">
                            <div style="font-weight: 600; color: var(--text-tertiary); font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Context Relevance</div>
                            <div class="detail-content">{{ $result.JudgeContextReasoning }}</div>
                        </div>
                        {{ end }}
                    </div>
                    {{ end }}

                    <div class="detail-section">
                        <div class="detail-label">Score Breakdown</div>
                        <div class="scores-grid">
                            <div class="score-item">
                                <div class="score-item-label">Combined</div>
                                <div class="score-item-value score {{ if ge $result.Scores.Combined 0.7 }}score-good{{ else if ge $result.Scores.Combined 0.5 }}score-fair{{ else }}score-poor{{ end }}">
                                    {{ printf "%.3f" $result.Scores.Combined }}
                                </div>
                            </div>
                            {{ range $key, $value := $result.Scores.Custom }}
                            <div class="score-item">
                                <div class="score-item-label">{{ $key }}</div>
                                <div class="score-item-value score {{ if ge $value 0.7 }}score-good{{ else if ge $value 0.4 }}score-fair{{ else }}score-poor{{ end }}">
                                    {{ printf "%.3f" $value }}
                                </div>
                            </div>
                            {{ end }}
                        </div>
                    </div>

                    {{ if $result.CustomFields }}
                    <div class="detail-section">
                        <div class="detail-label">Configuration</div>
                        <div class="metadata-grid">
                            {{ range $key, $value := $result.CustomFields }}
                            <div class="metadata-item">
                                <span class="metadata-key">{{ $key }}:</span>
                                <span class="metadata-value">{{ $value }}</span>
                            </div>
                            {{ end }}
                        </div>
                    </div>
                    {{ end }}

                    {{ if $result.Metadata }}
                    <div class="detail-section">
                        <div class="detail-label">Metadata</div>
                        <div class="metadata-grid">
                            {{ range $key, $value := $result.Metadata }}
                            <div class="metadata-item">
                                <span class="metadata-key">{{ $key }}:</span>
                                <span class="metadata-value">{{ $value }}</span>
                            </div>
                            {{ end }}
                        </div>
                    </div>
                    {{ end }}
                </div>
            </div>
        </div>
        {{ end }}
    </div>
    <script>
        // Dark mode toggle
        const themeToggle = document.getElementById('theme-toggle');
        const themeIcon = document.getElementById('theme-icon');
        const html = document.documentElement;
        const savedTheme = localStorage.getItem('theme') || 'light';
        html.setAttribute('data-theme', savedTheme);
        themeIcon.textContent = savedTheme === 'light' ? 'Dark' : 'Light';

        themeToggle.addEventListener('click', () => {
            const currentTheme = html.getAttribute('data-theme');
            const newTheme = currentTheme === 'light' ? 'dark' : 'light';
            html.setAttribute('data-theme', newTheme);
            localStorage.setItem('theme', newTheme);
            themeIcon.textContent = newTheme === 'light' ? 'Dark' : 'Light';
        });

        // Help modal
        const helpBtn = document.getElementById('help-btn');
        const helpModal = document.getElementById('help-modal');

        helpBtn.addEventListener('click', () => {
            helpModal.classList.add('show');
        });

        helpModal.addEventListener('click', (e) => {
            if (e.target === helpModal) {
                helpModal.classList.remove('show');
            }
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            if (e.key === 'd' || e.key === 'D') {
                e.preventDefault();
                themeToggle.click();
            }
            if (e.key === '?') {
                e.preventDefault();
                helpModal.classList.add('show');
            }
            if (e.key === 'r' || e.key === 'R') {
                e.preventDefault();
                location.reload();
            }
            if (e.key === 'Escape') {
                helpModal.classList.remove('show');
                // Close all test modals on Escape
                document.querySelectorAll('.modal').forEach(modal => {
                    modal.classList.remove('show');
                });
            }
        });

        // Modal functions
        function showTestModal(index) {
            const modal = document.getElementById('modal-' + index);
            modal.classList.add('show');
        }

        function closeTestModal(index) {
            const modal = document.getElementById('modal-' + index);
            modal.classList.remove('show');
        }

        // Close modal when clicking outside
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('modal')) {
                e.target.classList.remove('show');
            }
        });
    </script>
</body>
</html>`

	t := template.Must(template.New("tests").Parse(tmpl))
	if err := t.Execute(w, filteredResults); err != nil {
		// Don't call http.Error here - headers already sent by Execute
		log.Printf("Template error: %v", err)
	}
}

// evalsAPIHandler returns all eval results and dashboard data as JSON
func evalsAPIHandler(w http.ResponseWriter, r *http.Request) {
	// Reload latest data
	if err := reloadData(); err != nil {
		http.Error(w, fmt.Sprintf("Error reloading data: %v", err), http.StatusInternalServerError)
		return
	}

	// Optional filters
	modelFilter := r.URL.Query().Get("model")

	// Prepare response with full dashboard data
	response := struct {
		DashboardData
		// Add custom scores serialization for API
		ResultsWithScores []EvalResult `json:"results"`
	}{
		DashboardData:     evalData,
		ResultsWithScores: evalData.Results,
	}

	// Apply model filter if specified
	if modelFilter != "" {
		var filtered []EvalResult
		for _, result := range evalData.Results {
			if result.Model == modelFilter {
				filtered = append(filtered, result)
			}
		}
		response.ResultsWithScores = filtered
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// evalsSinceHandler returns only eval results after given timestamp (smart polling)
func evalsSinceHandler(w http.ResponseWriter, r *http.Request) {
	// Reload latest data
	if err := reloadData(); err != nil {
		http.Error(w, fmt.Sprintf("Error reloading data: %v", err), http.StatusInternalServerError)
		return
	}

	// Get timestamp filter from query param
	sinceTimestamp := r.URL.Query().Get("ts")
	if sinceTimestamp == "" {
		http.Error(w, "Missing 'ts' query parameter", http.StatusBadRequest)
		return
	}

	// Filter results - only return evals after the given timestamp
	var newResults []EvalResult
	for _, result := range evalData.Results {
		if result.Timestamp > sinceTimestamp {
			newResults = append(newResults, result)
		}
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(newResults); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","total_tests":%d,"models":%d}`, evalData.TotalTests, len(evalData.Models))
}
