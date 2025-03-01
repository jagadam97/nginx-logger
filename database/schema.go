package database

import (
	"context"
	"log"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

func CheckAndCreateTable(ctx context.Context, conn driver.Conn) error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS nginxLogger (
		time_local DateTime,
		remote_addr String,
		request_uri String,
		status UInt16,
		server_name String,
		request_time Float64,
		request_method String,
		bytes_sent UInt64,
		http_host String,
		server_protocol String,
		upstream_addr String,
		upstream_response_time Float64,
		ssl_protocol String,
		ssl_cipher String,
		http_user_agent String
	) ENGINE = MergeTree()
	  PARTITION BY toDate(time_local)
	  TTL time_local + INTERVAL 90 DAY
	  ORDER BY (time_local)
	  SETTINGS ttl_only_drop_parts = 1;`

	err := conn.Exec(ctx, createTableQuery)
	if err != nil {
		log.Fatal("Failed to create table")
	}
	return err
}
