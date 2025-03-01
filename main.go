package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jagadam97/nginx-logger/config"
	"github.com/jagadam97/nginx-logger/database"
	"github.com/jagadam97/nginx-logger/log"
	"github.com/jagadam97/nginx-logger/models"
	"github.com/jagadam97/nginx-logger/utils"
)

func main() {
	// Load environment variables
	config.LoadEnv()

	// Connect to ClickHouse
	conn, err := database.Connect()
	if err != nil {
		fmt.Printf("Database connection failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Ensure table exists
	err = database.CheckAndCreateTable(ctx, conn)
	if err != nil {
		fmt.Printf("Error creating table: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Listening for logs...")

	// Fetch log file path from env
	filePath := os.Getenv("LOG_FILE_PATH")
	if filePath == "" {
		fmt.Println("LOG_FILE_PATH is not set in the environment")
		os.Exit(1)
	}

	// Tail the log file
	tailer, err := log.TailLogFile(filePath)
	if err != nil {
		fmt.Printf("Error tailing file: %v\n", err)
		os.Exit(1)
	}

	// Buffer settings
	var buffer []models.LogEntry
	batchSize := utils.StringToInt(os.Getenv("BATCH_SIZE"))
	batchDelay := utils.StringToInt(os.Getenv("BATCH_DELAY"))
	timeLastLogFired := time.Now()

	// Process incoming log lines
	for line := range tailer.Lines {
		logEntry, err := log.ParseLogEntry(line.Text)
		if err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			continue
		}

		buffer = append(buffer, logEntry)

		// Batch insert based on size or time interval
		if len(buffer) >= batchSize || time.Since(timeLastLogFired) >= time.Duration(batchDelay)*time.Second {
			if err := database.BatchInsert(ctx, conn, buffer); err != nil {
				fmt.Printf("Error performing batch insert: %v\n", err)
			} else {
				buffer = buffer[:0] // Reset buffer after successful insert
			}
			timeLastLogFired = time.Now()
		}
	}

	// Final flush if any logs remain
	if len(buffer) > 0 {
		if err := database.BatchInsert(ctx, conn, buffer); err != nil {
			fmt.Printf("Error performing final batch insert: %v\n", err)
		}
	}
}
