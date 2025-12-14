package main

import (
	"encoding/json"
	"fmt"
	"log"
)

func main() {
	jsonLine := `{"timestamp":"2025-12-14T10:00:00Z","model":"gemma2:2b","test_id":"eval_001","question":"What is the capital of France?","response":"The capital of France is Paris.","expected":"Paris","response_time_ms":850,"scores":{"combined":0.95,"accuracy":1.0,"fluency":0.95,"completeness":0.90},"embedding_model":"nomic-embed-text","chunk_size":500,"chunk_overlap":50,"top_k":5,"retrieval_method":"similarity","temperature":0.7}`

	var result EvalResult
	if err := json.Unmarshal([]byte(jsonLine), &result); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Model: %s\n", result.Model)
	fmt.Printf("CustomFields: %+v\n", result.CustomFields)
	fmt.Printf("Number of custom fields: %d\n", len(result.CustomFields))

	for k, v := range result.CustomFields {
		fmt.Printf("  %s = %v (type: %T)\n", k, v, v)
	}
}
