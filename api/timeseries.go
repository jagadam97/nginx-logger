package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jagadam97/nginx-logger/database"
)

func getTimeSeries(w http.ResponseWriter, r *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")

	q := r.URL.Query()
	from, to, errMsg := parseRange(q)
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	points, err := client.QueryTimeSeries(ctx, from, to, parseTagFilter(q))
	if err != nil {
		http.Error(w, `{"error": "timeseries query failed"}`, http.StatusInternalServerError)
		return
	}

	if points == nil {
		points = []database.TimeSeriesPoint{}
	}

	json.NewEncoder(w).Encode(points)
}
