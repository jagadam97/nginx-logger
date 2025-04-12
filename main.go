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

var conn clickhouse.Conn

func main() {
	config.LoadEnv()

	var err error
	for {
		conn, err = database.Connect()
		if err == nil {
			fmt.Println("Database connection successfull")
			break
		}

		fmt.Printf("Database connection failed: %v. Retrying...\n", err)
		time.Sleep(2 * time.Second) // Wait before retrying
	}

	ctx := context.Background()

	if err := database.CheckAndCreateTable(ctx, conn); err != nil {
		fmt.Printf("Error creating table: %v\n", err)
		os.Exit(1)
	}

	go startLogListener(ctx)
	api.StartAPI(conn)
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

	var buffer []models.LogEntry
	batchSize := utils.StringToInt(os.Getenv("BATCH_SIZE"))
	batchDelay := utils.StringToInt(os.Getenv("BATCH_DELAY"))
	timeLastLogFired := time.Now()

	for line := range tailer.Lines {
		logEntry, err := log.ParseLogEntry(line.Text)
		if err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			continue
		}

		buffer = append(buffer, logEntry)

		if len(buffer) >= batchSize || time.Since(timeLastLogFired) >= time.Duration(batchDelay)*time.Second {
			if err := database.BatchInsert(ctx, conn, buffer); err != nil {
				fmt.Printf("Error performing batch insert: %v\n", err)
			} else {
				fmt.Printf("Inserted logs: %v in %v\n ", len(buffer), time.Since(timeLastLogFired))
				buffer = buffer[:0]
			}
			timeLastLogFired = time.Now()
		}
	}

	if len(buffer) > 0 {
		if err := database.BatchInsert(ctx, conn, buffer); err != nil {
			fmt.Printf("Error performing final batch insert: %v\n", err)
		}
	}
}
