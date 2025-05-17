package database

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jagadam97/nginx-logger/models"
)

func BatchInsert(ctx context.Context, conn driver.Conn, buffer []models.LogEntry) error {
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO nginxLogger 
	( time_local, remote_addr, request_uri, status, server_name, request_time,
	  request_method, bytes_sent, http_host, server_protocol, upstream_addr, 
	  upstream_response_time, ssl_protocol, ssl_cipher, http_user_agent )`)

	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, entry := range buffer {
		err := batch.Append(
			entry.TimeLocal,
			entry.RemoteAddr,
			entry.RequestURI,
			entry.Status,
			entry.ServerName,
			entry.RequestTime,
			entry.RequestMethod,
			entry.BytesSent,
			entry.HTTPHost,
			entry.ServerProtocol,
			entry.UpstreamAddr,
			entry.UpstreamResponseTime,
			entry.SSLProtocol,
			entry.SSLCipher,
			entry.HTTPUserAgent,
		)
		if err != nil {
			return fmt.Errorf("failed to append to batch: %w", err)
		}
	}
	return batch.Send()
}
