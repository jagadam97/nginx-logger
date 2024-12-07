package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/hpcloud/tail"
	"github.com/joho/godotenv"
)

func connect() (driver.Conn, error) {
	var (
		ctx       = context.Background()
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr: []string{os.Getenv("DB_HOST") + ":9000"},
			Auth: clickhouse.Auth{
				Database: os.Getenv("DB"),
				Username: os.Getenv("DB_USER"),
				Password: os.Getenv("DB_PASSWORD"),
			},
			ClientInfo: clickhouse.ClientInfo{
				Products: []struct {
					Name    string
					Version string
				}{
					{Name: "nginx-logger", Version: "0.1"},
				},
			},

			Debugf: func(format string, v ...interface{}) {
				fmt.Printf(format, v)
			},
			// TLS: &tls.Config{
			// 	InsecureSkipVerify: true,
			// },
		})
	)

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return nil, err
	}
	return conn, nil
}

func checkAndCreateTable(ctx context.Context, conn driver.Conn) error {
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
		log.Fatal("failed to create table")
	} else {
		log.Printf("created table")
	}
	return err
}

func convertTimestamp(input string) string {
	layout := "02/Jan/2006:15:04:05 +0000"
	parsedTime, err := time.Parse(layout, input)
	if err != nil {
		return ""
	}
	output := parsedTime.Format("2006-01-02 15:04:05")

	return output
}

func stringToInt(str string) int {
	i, err := strconv.Atoi(str)
	if err != nil {
		return 0
	}
	return i
}

func stringToFloat(str string) float64 {
	i, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0.0
	}
	return i
}

type LogEntry struct {
	TimeLocal            string `json:"time_local"`
	RemoteAddr           string `json:"remote_addr"`
	RequestURI           string `json:"request_uri"`
	Status               string `json:"status"`
	ServerName           string `json:"server_name"`
	RequestTime          string `json:"request_time"`
	RequestMethod        string `json:"request_method"`
	BytesSent            string `json:"bytes_sent"`
	HTTPHost             string `json:"http_host"`
	ServerProtocol       string `json:"server_protocol"`
	UpstreamAddr         string `json:"upstream_addr"`
	UpstreamResponseTime string `json:"upstream_response_time"`
	SSLProtocol          string `json:"ssl_protocol"`
	SSLCipher            string `json:"ssl_cipher"`
	HTTPUserAgent        string `json:"http_user_agent"`
}

func batchInsert(ctx context.Context, conn driver.Conn, buffer []LogEntry) error {

	batch, err := conn.PrepareBatch(ctx, `INSERT INTO nginxLogger 
	( time_local
	, remote_addr
	, request_uri
	, status
	, server_name
	, request_time
	, request_method
	, bytes_sent
	, http_host
	, server_protocol
	, upstream_addr
	, upstream_response_time
	, ssl_protocol
	, ssl_cipher
	, http_user_agent)`)

	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, entry := range buffer {
		err := batch.Append(
			convertTimestamp(entry.TimeLocal),
			entry.RemoteAddr,
			entry.RequestURI,
			stringToInt(entry.Status),
			entry.ServerName,
			stringToFloat(entry.RequestTime),
			entry.RequestMethod,
			stringToInt(entry.BytesSent),
			entry.HTTPHost,
			entry.ServerProtocol,
			entry.UpstreamAddr,
			stringToFloat(entry.UpstreamResponseTime),
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

func main() {
	err := godotenv.Load()
	if err != nil {
		panic((err))
	}
	conn, err := connect()
	var buffer []LogEntry
	batchSize := stringToInt(os.Getenv("BATCH_SIZE"))
	batchDelay := stringToInt(os.Getenv("BATCH_DELAY"))
	timeLastLogFired := time.Now()

	if err != nil {
		panic((err))
	}

	ctx := context.Background()
	err = checkAndCreateTable(ctx, conn)
	if err != nil {
		fmt.Println(err)
		panic((err))
	}

	filePath := os.Getenv("LOG_FILE_PATH")
	t, err := tail.TailFile(filePath, tail.Config{
		Follow:   true,
		ReOpen:   true,
		Poll:     true,
		Location: &tail.SeekInfo{Offset: 0, Whence: 2},
	})
	if err != nil {
		fmt.Printf("Error tailing file: %v\n", err)
		return
	}
	for line := range t.Lines {
		var logEntry LogEntry
		err := json.Unmarshal([]byte(line.Text), &logEntry)
		if err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			continue
		}
		buffer = append(buffer, logEntry)
		if len(buffer) >= batchSize || time.Since(timeLastLogFired) >= time.Duration(batchDelay)*time.Second {
			if err := batchInsert(ctx, conn, buffer); err != nil {
				fmt.Printf("Error performing batch insert: %v\n", err)
			} else {
				buffer = buffer[:0]
			}
			timeLastLogFired = time.Now()
		}
	}

	if len(buffer) > 0 {
		if err := batchInsert(ctx, conn, buffer); err != nil {
			fmt.Printf("Error performing final batch insert: %v\n", err)
		}
	}
}
