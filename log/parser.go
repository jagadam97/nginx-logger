package log

import (
	"encoding/json"
	"fmt"

	"github.com/jagadam97/nginx-logger/models"
)

func ParseLogEntry(line string) (models.LogEntry, error) {
	var logEntry models.LogEntry
	err := json.Unmarshal([]byte(line), &logEntry)
	if err != nil {
		return logEntry, fmt.Errorf("error parsing JSON: %w", err)
	}
	return logEntry, nil
}
