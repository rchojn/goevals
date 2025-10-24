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
)

// EvalResult represents a single evaluation result from JSONL
type EvalResult struct {
	Timestamp      string         `json:"timestamp"`
	Model          string         `json:"model"`
	TestID         string         `json:"test_id"`
	Question       string         `json:"question,omitempty"`
	Response       string         `json:"response,omitempty"`
	Expected       string         `json:"expected,omitempty"`
	Scores         ScoreBreakdown `json:"scores"`
	ResponseTimeMS int64          `json:"response_time_ms"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ScoreBreakdown contains all individual scores
type ScoreBreakdown struct {
	Combined           float64 `json:"combined"`
	Polish             float64 `json:"polish,omitempty"`
	Keywords           float64 `json:"keywords,omitempty"`
	Completeness       float64 `json:"completeness,omitempty"`
	MinimumLength      float64 `json:"minimum_length,omitempty"`
	LexicalSimilarity  float64 `json:"lexical_similarity,omitempty"`
}

// DashboardData holds aggregated stats for the dashboard
type DashboardData struct {
	TotalTests    int
	AvgScore      float64
	Models        []string
	Results       []EvalResult
	ModelStats    map[string]ModelStat
}

// ModelStat holds statistics for a single model
type ModelStat struct {
	Model      string
	TestCount  int
	AvgScore   float64
	MinScore   float64
	MaxScore   float64
	AvgTimeMS  float64
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
		TotalTests: len(results),
		Results:    results,
		ModelStats: make(map[string]ModelStat),
	}

	if len(results) == 0 {
		return data
	}

	// Track unique models
	modelSet := make(map[string]bool)
	modelScores := make(map[string][]float64)
	modelTimes := make(map[string][]float64)
	totalScore := 0.0

	for _, result := range results {
		modelSet[result.Model] = true
		totalScore += result.Scores.Combined

		modelScores[result.Model] = append(modelScores[result.Model], result.Scores.Combined)
		modelTimes[result.Model] = append(modelTimes[result.Model], float64(result.ResponseTimeMS))
	}

	// Calculate overall average
	data.AvgScore = totalScore / float64(len(results))

	// Get sorted model list
	for model := range modelSet {
		data.Models = append(data.Models, model)
	}
	sort.Strings(data.Models)

	// Calculate per-model stats
	for _, model := range data.Models {
		scores := modelScores[model]
		times := modelTimes[model]

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

		data.ModelStats[model] = ModelStat{
			Model:     model,
			TestCount: len(scores),
			AvgScore:  sum / float64(len(scores)),
			MinScore:  min,
			MaxScore:  max,
			AvgTimeMS: timeSum / float64(len(times)),
		}
	}

	return data
}

// Global variable to hold loaded eval results
var evalData DashboardData

func main() {
	// Check arguments
	if len(os.Args) < 2 {
		fmt.Println("Usage: goevals <evals.jsonl>")
		fmt.Println("\nExample:")
		fmt.Println("  goevals serve evals.jsonl")
		fmt.Println("  go run main.go evals.jsonl")
		os.Exit(1)
	}

	filename := os.Args[1]

	// Handle "serve" subcommand
	if filename == "serve" && len(os.Args) >= 3 {
		filename = os.Args[2]
	}

	// Parse JSONL file
	log.Printf("Loading evals from %s...", filename)
	results, err := ParseJSONL(filename)
	if err != nil {
		log.Fatalf("Error parsing JSONL: %v", err)
	}

	log.Printf("Loaded %d eval results", len(results))

	// Calculate stats
	evalData = CalculateStats(results)
	log.Printf("Models found: %v", evalData.Models)
	log.Printf("Overall avg score: %.2f", evalData.AvgScore)

	// Setup HTTP handlers
	http.HandleFunc("/", dashboardHandler)
	http.HandleFunc("/health", healthHandler)

	// Start server
	port := ":3000"
	log.Printf("🐹 GoEvals dashboard starting on http://localhost%s", port)
	log.Printf("📊 Showing %d evals from %d models", evalData.TotalTests, len(evalData.Models))

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
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
            max-width: 1200px;
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
        }
        td {
            color: #333;
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
            <table>
                <thead>
                    <tr>
                        <th>Model</th>
                        <th>Tests</th>
                        <th>Avg Score</th>
                        <th>Min</th>
                        <th>Max</th>
                        <th>Avg Time (ms)</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range .Models }}
                    {{ $stat := index $.ModelStats . }}
                    <tr>
                        <td><strong>{{ $stat.Model }}</strong></td>
                        <td>{{ $stat.TestCount }}</td>
                        <td class="score {{ if ge $stat.AvgScore 0.8 }}score-good{{ else if ge $stat.AvgScore 0.6 }}score-fair{{ else }}score-poor{{ end }}">
                            {{ printf "%.2f" $stat.AvgScore }}
                        </td>
                        <td>{{ printf "%.2f" $stat.MinScore }}</td>
                        <td>{{ printf "%.2f" $stat.MaxScore }}</td>
                        <td>{{ printf "%.0f" $stat.AvgTimeMS }}</td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>

        <footer>
            Built with Go stdlib + HTML + common sense 🐹<br>
            <a href="https://github.com/rchojn/goevals" style="color: #3b82f6;">github.com/rchojn/goevals</a>
        </footer>
    </div>
</body>
</html>`

	t := template.Must(template.New("dashboard").Parse(tmpl))
	if err := t.Execute(w, evalData); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
		log.Printf("Template error: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","total_tests":%d,"models":%d}`, evalData.TotalTests, len(evalData.Models))
}
