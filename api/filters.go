package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jagadam97/nginx-logger/database"
)

func getFilters(w http.ResponseWriter, r *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")

	ctx := context.Background()
	filters, err := client.QueryFilters(ctx)
	if err != nil {
		http.Error(w, `{"error": "filters query failed"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(filters)
}
