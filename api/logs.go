package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type RequestPayload struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
}

func getLogs(w http.ResponseWriter, r *http.Request, conn clickhouse.Conn) {
	w.Header().Set("Content-Type", "application/json")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req RequestPayload
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error": "Invalid JSON format"}`, http.StatusBadRequest)
		return
	}

	if req.From == "" || req.To == "" {
		http.Error(w, `{"error": "Missing 'from' or 'to' in request body"}`, http.StatusBadRequest)
		return
	}

	fromTime, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		http.Error(w, `{"error": "Invalid 'from' time format. Use RFC3339 (e.g., 2024-03-28T12:00:00Z)"}`, http.StatusBadRequest)
		return
	}
	toTime, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		http.Error(w, `{"error": "Invalid 'to' time format. Use RFC3339 (e.g., 2024-03-28T12:00:00Z)"}`, http.StatusBadRequest)
		return
	}
	ctx := context.Background()
	query := `SELECT
				time_local as timestamp,
				concat (server_name,request_uri,
				(
				' replied '
				case
					when status <= 199 then ' with informational messaged with status code: '
					when status <= 299 then ' sucessfully processed with status code: '
					when status <= 399 then ' with redirected with status code: '
					when status <= 499 then ' with client error with status code: '
					else ' with server side error with status code: '
				end
				),
				status,
				' request from IP: ',
				remote_addr ) as "message",
				case
					when status <= 199 then 'debug'
					when status <= 299 then 'info'
					when status <= 399 then 'warning'
					when status <= 499 then  'error'
					else 'critical'
				end as level
				FROM "default"."nginxLogger"
				WHERE ( timestamp BETWEEN ? AND ? )
				ORDER BY timestamp DESC
				LIMIT 1000`
	rows, err := conn.Query(ctx, query, fromTime, toTime)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Database query failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var logEntry LogEntry
		if err := rows.Scan(&logEntry.Timestamp, &logEntry.Message, &logEntry.Level); err != nil {
			fmt.Println(err)
			http.Error(w, `{"error": "Failed to scan row"}`, http.StatusInternalServerError)
			return
		}
		logs = append(logs, logEntry)
	}

	response, err := json.Marshal(logs)
	if err != nil {
		http.Error(w, `{"error": "Failed to encode logs to JSON"}`, http.StatusInternalServerError)
		return
	}

	w.Write(response)
}
