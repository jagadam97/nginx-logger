package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jagadam97/nginx-logger/database"
	"github.com/jagadam97/nginx-logger/utils"
)

func getLogs(w http.ResponseWriter, r *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")

	q := r.URL.Query()
	from, to, errMsg := parseRange(q)
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	filters := database.LogFilters{
		TagFilter: parseTagFilter(q),
		URI:       q.Get("uri"),    // substring match, e.g. "/api/payment"
		Method:    q.Get("method"), // exact request_method
	}
	limit := utils.StringToInt(q.Get("limit"))

	ctx := context.Background()
	logs, err := client.QueryLogs(ctx, from, to, filters, limit)
	if err != nil {
		http.Error(w, `{"error": "database query failed: `+strconv.Quote(err.Error())+`"}`, http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []database.LogRecord{}
	}

	json.NewEncoder(w).Encode(logs)
}
