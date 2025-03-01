package database

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jagadam97/nginx-logger/models"
	"github.com/jagadam97/nginx-logger/utils"
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
			utils.ConvertTimestamp(entry.TimeLocal),
			entry.RemoteAddr,
			entry.RequestURI,
			utils.StringToInt(entry.Status),
			entry.ServerName,
			utils.StringToFloat(entry.RequestTime),
			entry.RequestMethod,
			utils.StringToInt(entry.BytesSent),
			entry.HTTPHost,
			entry.ServerProtocol,
			entry.UpstreamAddr,
			utils.StringToFloat(entry.UpstreamResponseTime),
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
