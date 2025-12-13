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

// Known field names for EvalResult
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
	"test_run_date":            true,
	"embedding_model":          true,
	"chunk_size":               true,
	"chunk_overlap":            true,
	"top_k":                    true,
	"retrieval_method":         true,
	"temperature":              true,
	"question_id":              true,
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
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoEvals - LLM Evaluation Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            padding: 2rem;
        }
        .container {
            max-width: 98%;
            margin: 0 auto;
        }
        header {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
        }
        h1 {
            color: #333;
            margin-bottom: 0.5rem;
        }
        .subtitle {
            color: #666;
            font-size: 0.9rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-label {
            color: #666;
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
        }
        .stat-value {
            color: #333;
            font-size: 2rem;
            font-weight: 600;
        }
        .models-section {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h2 {
            color: #333;
            margin-bottom: 1rem;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid #e0e0e0;
        }
        th {
            background: #f9f9f9;
            font-weight: 600;
            color: #666;
            font-size: 0.875rem;
            text-transform: uppercase;
            cursor: pointer;
            user-select: none;
            position: relative;
            padding-right: 20px;
        }
        th:hover {
            background: #f0f0f0;
        }
        th::after {
            content: '⇅';
            position: absolute;
            right: 8px;
            opacity: 0.3;
        }
        th.sorted-asc::after {
            content: '▲';
            opacity: 1;
        }
        th.sorted-desc::after {
            content: '▼';
            opacity: 1;
        }
        td {
            color: #333;
        }
        /* Sticky/Frozen columns for Model + Embedding */
        th:nth-child(1), td:nth-child(1) {
            position: sticky;
            left: 0;
            background: white;
            z-index: 10;
            box-shadow: 2px 0 4px rgba(0,0,0,0.05);
            min-width: 200px;
            max-width: 200px;
        }
        th:nth-child(2), td:nth-child(2) {
            position: sticky;
            left: 200px;
            background: white;
            z-index: 10;
            box-shadow: 2px 0 4px rgba(0,0,0,0.05);
            min-width: 150px;
            max-width: 150px;
        }
        th:nth-child(1) {
            background: #f9f9f9;
            z-index: 11;
        }
        th:nth-child(2) {
            background: #f9f9f9;
            z-index: 11;
        }
        /* Column widths */
        th:nth-child(3), td:nth-child(3) { min-width: 100px; max-width: 100px; } /* Combined Score */
        th:nth-child(4), td:nth-child(4) { min-width: 80px; max-width: 80px; }   /* Top_K */
        th:nth-child(5), td:nth-child(5) { min-width: 100px; max-width: 100px; } /* Chunk_Size */
        th:nth-child(6), td:nth-child(6) { min-width: 100px; max-width: 100px; } /* Chunk_Overlap */
        th:nth-child(7), td:nth-child(7) { min-width: 120px; max-width: 120px; } /* Retrieval */
        th:nth-child(8), td:nth-child(8) { min-width: 80px; max-width: 80px; }   /* Temperature */
        /* Score columns - smaller width */
        .score-cell { min-width: 90px; max-width: 90px; text-align: center; font-weight: 600; }
        tbody tr {
            transition: background-color 0.2s;
        }
        tbody tr:hover {
            background-color: #f9fafb;
        }
        .score {
            font-weight: 600;
        }
        .score-good { color: #10b981; }
        .score-fair { color: #f59e0b; }
        .score-poor { color: #ef4444; }
        footer {
            text-align: center;
            color: #999;
            margin-top: 2rem;
            font-size: 0.875rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>🐹 GoEvals Dashboard</h1>
            <p class="subtitle">Simple, self-hosted LLM evaluation visualization</p>
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
                        <th onclick="sortTable(1)">Embedding</th>
                        <th onclick="sortTable(2)" class="sorted-desc">Combined</th>
                        <th onclick="sortTable(3)">Top K</th>
                        <th onclick="sortTable(4)">Chunk Size</th>
                        <th onclick="sortTable(5)">Chunk Overlap</th>
                        <th onclick="sortTable(6)">Retrieval</th>
                        <th onclick="sortTable(7)">Temp</th>
                        {{ range $idx, $score := $.CustomScores }}
                        <th onclick="sortTable({{ add 8 $idx }})" class="score-cell">{{ $score }}</th>
                        {{ end }}
                        <th onclick="sortTable({{ add 8 (len $.CustomScores) }})">Tests</th>
                        <th onclick="sortTable({{ add 9 (len $.CustomScores) }})">Min</th>
                        <th onclick="sortTable({{ add 10 (len $.CustomScores) }})">Max</th>
                        <th onclick="sortTable({{ add 11 (len $.CustomScores) }})">Time (ms)</th>
                    </tr>
                </thead>
                <tbody id="table-body">
                    {{ range .Models }}
                    {{ $stat := index $.ModelStats . }}
                    <tr style="cursor: pointer;" onclick="window.location='/tests?model={{ $stat.Model }}'">
                        <td><strong>{{ $stat.ActualModelName }}</strong></td>
                        <td>{{ index $stat.CustomFields "embedding_model" }}</td>
                        <td class="score {{ if ge $stat.AvgScore 0.7 }}score-good{{ else if ge $stat.AvgScore 0.5 }}score-fair{{ else }}score-poor{{ end }}">{{ printf "%.2f" $stat.AvgScore }}</td>
                        <td>{{ index $stat.CustomFields "top_k" }}</td>
                        <td>{{ index $stat.CustomFields "chunk_size" }}</td>
                        <td>{{ index $stat.CustomFields "chunk_overlap" }}</td>
                        <td>{{ index $stat.CustomFields "retrieval_method" }}</td>
                        <td>{{ formatTemp (index $stat.CustomFields "temperature") }}</td>
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
            Built with Go stdlib + HTML + common sense 🐹<br>
            <a href="https://github.com/rchojn/goevals" style="color: #3b82f6;">github.com/rchojn/goevals</a><br>
            <div style="margin-top: 0.75rem; display: flex; align-items: center; justify-content: center; gap: 1rem;">
                <label style="display: flex; align-items: center; gap: 0.5rem; cursor: pointer; font-size: 0.875rem; color: #666;">
                    <input type="checkbox" id="autorefresh-toggle" checked style="cursor: pointer;">
                    <span>Auto-refresh (5s)</span>
                </label>
                <span id="refresh-indicator" style="color: #999; font-size: 0.8rem;">Enabled</span>
            </div>
        </footer>
    </div>
    <script>
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
                indicator.style.color = '#999';
            } else {
                indicator.textContent = 'Enabled';
                indicator.style.color = '#999';
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
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Test Results - GoEvals</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            padding: 2rem;
        }
        .container {
            max-width: 95%;
            margin: 0 auto;
        }
        header {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
        }
        h1 {
            color: #333;
            margin-bottom: 0.5rem;
        }
        .subtitle {
            color: #666;
            font-size: 0.9rem;
        }
        .back-link {
            display: inline-block;
            margin-bottom: 1rem;
            color: #3b82f6;
            text-decoration: none;
        }
        .back-link:hover {
            text-decoration: underline;
        }
        .test-card {
            background: white;
            margin-bottom: 1rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .test-header {
            padding: 1rem 1.5rem;
            cursor: pointer;
            display: flex;
            justify-content: space-between;
            align-items: center;
            transition: background-color 0.2s;
            border-left: 4px solid transparent;
        }
        .test-header:hover {
            background-color: #f9fafb;
        }
        .test-header.expanded {
            border-left-color: #3b82f6;
        }
        .test-header-left {
            display: flex;
            gap: 2rem;
            align-items: center;
            flex: 1;
        }
        .test-id {
            font-family: monospace;
            font-weight: 600;
            color: #333;
            min-width: 150px;
        }
        .model-name {
            font-weight: 600;
            color: #3b82f6;
            min-width: 120px;
        }
        .question-preview {
            flex: 1;
            color: #666;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .test-meta {
            display: flex;
            gap: 1.5rem;
            align-items: center;
        }
        .score {
            font-weight: 600;
            font-size: 1.1rem;
            min-width: 50px;
        }
        .score-good { color: #10b981; }
        .score-fair { color: #f59e0b; }
        .score-poor { color: #ef4444; }
        .response-time {
            color: #666;
            font-size: 0.875rem;
        }
        .expand-icon {
            color: #999;
            transition: transform 0.2s;
        }
        .test-header.expanded .expand-icon {
            transform: rotate(90deg);
        }
        .test-details {
            display: none;
            padding: 1.5rem;
            border-top: 1px solid #e5e7eb;
            background: #f9fafb;
        }
        .test-details.expanded {
            display: block;
        }
        .detail-section {
            margin-bottom: 1.5rem;
        }
        .detail-section:last-child {
            margin-bottom: 0;
        }
        .detail-label {
            font-weight: 600;
            color: #374151;
            margin-bottom: 0.5rem;
            font-size: 0.875rem;
            text-transform: uppercase;
        }
        .detail-content {
            padding: 1rem;
            background: white;
            border-radius: 4px;
            border: 1px solid #e5e7eb;
            font-size: 0.9375rem;
            line-height: 1.6;
            white-space: pre-wrap;
            color: #1f2937;
        }
        .scores-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 0.75rem;
        }
        .score-item {
            padding: 0.75rem;
            background: white;
            border-radius: 4px;
            border: 1px solid #e5e7eb;
        }
        .score-item-label {
            font-size: 0.75rem;
            color: #6b7280;
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
            background: white;
            border-radius: 4px;
            border: 1px solid #e5e7eb;
            font-size: 0.8125rem;
        }
        .metadata-key {
            color: #6b7280;
            font-weight: 500;
        }
        .metadata-value {
            color: #1f2937;
            margin-left: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <a href="/" class="back-link">← Back to Dashboard</a>

        <header>
            <h1>🐹 Test Results {{ if . }}({{ len . }} tests){{ end }}</h1>
            <p class="subtitle">Click on any test to see full details</p>
        </header>

        {{ range $index, $result := . }}
        <div class="test-card">
            <div class="test-header" onclick="toggleDetails({{ $index }})">
                <div class="test-header-left">
                    <div class="test-id">{{ $result.TestID }}</div>
                    <div class="model-name">{{ $result.Model }}</div>
                    <div class="question-preview">{{ $result.Question }}</div>
                </div>
                <div class="test-meta">
                    <span class="score {{ if ge $result.Scores.Combined 0.7 }}score-good{{ else if ge $result.Scores.Combined 0.5 }}score-fair{{ else }}score-poor{{ end }}">
                        {{ printf "%.2f" $result.Scores.Combined }}
                    </span>
                    <span class="response-time">{{ $result.ResponseTimeMS }}ms</span>
                    <span class="expand-icon">▶</span>
                </div>
            </div>
            <div class="test-details" id="details-{{ $index }}">
                <div class="detail-section">
                    <div class="detail-label">📝 Question</div>
                    <div class="detail-content">{{ $result.Question }}</div>
                </div>

                <div class="detail-section">
                    <div class="detail-label">🤖 Model Response</div>
                    <div class="detail-content">{{ if $result.Response }}{{ $result.Response }}{{ else }}<em style="color: #9ca3af;">No response recorded</em>{{ end }}</div>
                </div>

                {{ if $result.Expected }}
                <div class="detail-section">
                    <div class="detail-label">✅ Expected Response</div>
                    <div class="detail-content">{{ $result.Expected }}</div>
                </div>
                {{ end }}

                {{ if $result.JudgeModel }}
                <div class="detail-section">
                    <div class="detail-label">⚖️ Judge Evaluation ({{ $result.JudgeModel }})</div>
                    {{ if $result.JudgeFactualReasoning }}
                    <div style="margin-bottom: 0.75rem;">
                        <div style="font-weight: 600; color: #6b7280; font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Factual Correctness</div>
                        <div class="detail-content">{{ $result.JudgeFactualReasoning }}</div>
                    </div>
                    {{ end }}
                    {{ if $result.JudgeFaithfulReasoning }}
                    <div style="margin-bottom: 0.75rem;">
                        <div style="font-weight: 600; color: #6b7280; font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Faithfulness</div>
                        <div class="detail-content">{{ $result.JudgeFaithfulReasoning }}</div>
                    </div>
                    {{ end }}
                    {{ if $result.JudgeContextReasoning }}
                    <div style="margin-bottom: 0;">
                        <div style="font-weight: 600; color: #6b7280; font-size: 0.75rem; margin-bottom: 0.25rem; text-transform: uppercase;">Context Relevance</div>
                        <div class="detail-content">{{ $result.JudgeContextReasoning }}</div>
                    </div>
                    {{ end }}
                </div>
                {{ end }}

                <div class="detail-section">
                    <div class="detail-label">📊 Score Breakdown</div>
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

                {{ if $result.Metadata }}
                <div class="detail-section">
                    <div class="detail-label">🔧 Metadata</div>
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
        {{ end }}
    </div>
    <script>
        function toggleDetails(index) {
            const header = document.querySelectorAll('.test-header')[index];
            const details = document.getElementById('details-' + index);

            header.classList.toggle('expanded');
            details.classList.toggle('expanded');
        }
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
