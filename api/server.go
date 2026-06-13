package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/jagadam97/nginx-logger/database"
)

func StartAPI(client *database.InfluxClient) {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		healthcheck(w, r, client)
	})
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		getLogs(w, r, client)
	})
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		getStats(w, r, client)
	})
	mux.HandleFunc("/api/filters", func(w http.ResponseWriter, r *http.Request) {
		getFilters(w, r, client)
	})
	mux.HandleFunc("/api/timeseries", func(w http.ResponseWriter, r *http.Request) {
		getTimeSeries(w, r, client)
	})

	// Serve the frontend (single-page dashboard) for all non-/api routes.
	frontendDir := os.Getenv("FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "frontend"
	}
	mux.Handle("/", http.FileServer(http.Dir(frontendDir)))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Starting API server on port %s\n", port)
	// Negotiate response compression (zstd > br > gzip > deflate) for every route.
	http.ListenAndServe(":"+port, compress(mux))
}
