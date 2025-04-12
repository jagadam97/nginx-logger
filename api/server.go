package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func StartAPI(conn clickhouse.Conn) {
	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		healthcheck(w, r, conn)
	})

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		getLogs(w, r, conn)
	})
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Starting API server on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
