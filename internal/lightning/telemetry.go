package lightning

import (
	"encoding/json"
	"fmt"
	"os"
)

// ExportSpansToJSON extrai todos os rastros do DuckDB e salva em JSON compatível com OpenTelemetry.
func ExportSpansToJSON(store *DuckDBStore, outputPath string) error {
	rows, err := store.db.Query("SELECT rollout_id, attempt_id, name, attributes, start_time, end_time, prompt_tokens, completion_tokens FROM spans ORDER BY start_time ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	var allSpans []map[string]interface{}
	for rows.Next() {
		var rid, aid, name string
		var attrJSON string
		var start, end float64
		var pTokens, cTokens int
		
		if err := rows.Scan(&rid, &aid, &name, &attrJSON, &start, &end, &pTokens, &cTokens); err == nil {
			var attr map[string]interface{}
			json.Unmarshal([]byte(attrJSON), &attr)
			
			span := map[string]interface{}{
				"traceId": rid,
				"spanId": aid,
				"name": name,
				"startTime": start,
				"endTime": end,
				"attributes": attr,
				"usage": map[string]int{
					"prompt_tokens": pTokens,
					"completion_tokens": cTokens,
				},
			}
			allSpans = append(allSpans, span)
		}
	}

	data, err := json.MarshalIndent(allSpans, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}
