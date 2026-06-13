package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jagadam97/nginx-logger/database"
)

func getStats(w http.ResponseWriter, r *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")

	q := r.URL.Query()
	from, to, errMsg := parseRange(q)
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	stats, err := client.QueryStats(ctx, from, to, parseTagFilter(q))
	if err != nil {
		http.Error(w, `{"error": "stats query failed"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}
