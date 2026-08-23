package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jagadam97/nginx-logger/database"
)

// getStub returns the stub_status series for the requested range. Unlike the
// other endpoints it takes no tag filter — stub_status is proxy-wide and has no
// host/status/client dimension to slice by.
func getStub(w http.ResponseWriter, r *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")

	from, to, errMsg := parseRange(r.URL.Query())
	if errMsg != "" {
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	points, err := client.QueryStubSeries(context.Background(), from, to)
	if err != nil {
		http.Error(w, `{"error": "stub_status query failed"}`, http.StatusInternalServerError)
		return
	}

	if points == nil {
		points = []database.StubPoint{}
	}

	json.NewEncoder(w).Encode(points)
}
