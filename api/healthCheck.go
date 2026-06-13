package api

import (
	"context"
	"net/http"

	"github.com/jagadam97/nginx-logger/database"
)

func healthcheck(w http.ResponseWriter, _ *http.Request, client *database.InfluxClient) {
	w.Header().Set("Content-Type", "application/json")
	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"` + err.Error() + `"}`))
		return
	}
	w.Write([]byte(`{"status":"healthy"}`))
}
