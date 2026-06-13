package api

import (
	"net/url"
	"strings"
	"time"

	"github.com/jagadam97/nginx-logger/database"
)

// multiValues collects a multi-value query parameter. It accepts both repeated
// params (?host=a&host=b) and comma-separated values (?host=a,b), trimming blanks.
func multiValues(q url.Values, key string) []string {
	var out []string
	for _, raw := range q[key] {
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

// parseTagFilter reads the global host/status/client filter dimensions.
func parseTagFilter(q url.Values) database.TagFilter {
	return database.TagFilter{
		Hosts:    multiValues(q, "host"),
		Statuses: multiValues(q, "status"),
		Clients:  multiValues(q, "client_ip"),
	}
}

// parseRange reads and validates the required from/to RFC3339 params.
func parseRange(q url.Values) (from, to time.Time, errMsg string) {
	fromStr, toStr := q.Get("from"), q.Get("to")
	if fromStr == "" || toStr == "" {
		return from, to, `{"error": "missing 'from' or 'to' query parameters (RFC3339)"}`
	}
	var err error
	if from, err = time.Parse(time.RFC3339, fromStr); err != nil {
		return from, to, `{"error": "invalid 'from' format, use RFC3339"}`
	}
	if to, err = time.Parse(time.RFC3339, toStr); err != nil {
		return from, to, `{"error": "invalid 'to' format, use RFC3339"}`
	}
	return from, to, ""
}
