package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func healthcheck(w http.ResponseWriter, _ *http.Request, conn clickhouse.Conn) {
	ctx := context.Background()
	err := conn.Ping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(fmt.Sprintf("Unhealthy: Database unreachable (%s)", err.Error())))
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Healthy!!"))
}
