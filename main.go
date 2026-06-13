package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/jagadam97/nginx-logger/api"
	"github.com/jagadam97/nginx-logger/config"
	"github.com/jagadam97/nginx-logger/database"
	"github.com/jagadam97/nginx-logger/log"
	"github.com/jagadam97/nginx-logger/models"
	"github.com/jagadam97/nginx-logger/utils"
)

var (
	conn         clickhouse.Conn
	influxClient *database.InfluxClient
)

func main() {
	config.LoadEnv()

	chEnabled := database.ClickHouseEnabled()
	influxEnabled := database.InfluxEnabled()

	if !chEnabled && !influxEnabled {
		fmt.Println("No database backend configured. Set DB_HOST for ClickHouse and/or INFLUX_URL for InfluxDB.")
		os.Exit(1)
	}

	ctx := context.Background()

	if chEnabled {
		var err error
		for {
			conn, err = database.Connect()
			if err == nil {
				fmt.Println("ClickHouse connection successful")
				break
			}
			fmt.Printf("ClickHouse connection failed: %v. Retrying...\n", err)
			time.Sleep(2 * time.Second)
		}

		if err := database.CheckAndCreateTable(ctx, conn); err != nil {
			fmt.Printf("Error creating table: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("ClickHouse not configured, skipping")
	}

	if influxEnabled {
		var err error
		for {
			influxClient, err = database.ConnectInflux()
			if err == nil {
				fmt.Println("InfluxDB connection successful")
				break
			}
			fmt.Printf("InfluxDB connection failed: %v. Retrying...\n", err)
			time.Sleep(2 * time.Second)
		}
		defer influxClient.Close()
	} else {
		fmt.Println("InfluxDB not configured, skipping")
	}

	go startLogListener(ctx)

	if influxEnabled {
		api.StartAPI(influxClient)
	} else {
		select {}
	}
}

func startLogListener(ctx context.Context) {
	fmt.Println("Listening for logs...")

	filePath := os.Getenv("LOG_FILE_PATH")
	if filePath == "" {
		fmt.Println("LOG_FILE_PATH is not set in the environment")
		os.Exit(1)
	}

	tailer, err := log.TailLogFile(filePath)
	if err != nil {
		fmt.Printf("Error tailing file: %v\n", err)
		os.Exit(1)
	}

	batchSize := utils.StringToInt(os.Getenv("BATCH_SIZE"))
	buffer := make([]models.LogEntry, 0, batchSize)
	batchDelay := utils.StringToInt(os.Getenv("BATCH_DELAY"))
	timeLastLogFired := time.Now()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if conn != nil {
			if err := database.BatchInsert(ctx, conn, buffer); err != nil {
				fmt.Printf("Error performing ClickHouse batch insert: %v\n", err)
			} else {
				fmt.Printf("Inserted %d logs to ClickHouse in %v\n", len(buffer), time.Since(timeLastLogFired))
			}
		}
		if influxClient != nil {
			if err := influxClient.BatchInsert(ctx, buffer); err != nil {
				fmt.Printf("Error performing InfluxDB batch insert: %v\n", err)
			} else {
				fmt.Printf("Inserted %d logs to InfluxDB in %v\n", len(buffer), time.Since(timeLastLogFired))
			}
		}
		buffer = buffer[:0]
		timeLastLogFired = time.Now()
	}

	for line := range tailer.Lines {
		logEntry, err := log.ParseLogEntry(line.Text)
		if err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			continue
		}

		buffer = append(buffer, logEntry)

		if len(buffer) >= batchSize || time.Since(timeLastLogFired) >= time.Duration(batchDelay)*time.Second {
			flush()
		}
	}

	flush()
}
