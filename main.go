package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jagadam97/nginx-logger/api"
	"github.com/jagadam97/nginx-logger/config"
	"github.com/jagadam97/nginx-logger/database"
	"github.com/jagadam97/nginx-logger/log"
	"github.com/jagadam97/nginx-logger/models"
	"github.com/jagadam97/nginx-logger/stub"
	"github.com/jagadam97/nginx-logger/utils"
)

var influxClient *database.InfluxClient

func main() {
	config.LoadEnv()

	if !database.InfluxEnabled() {
		fmt.Println("No database backend configured. Set INFLUX_URL for InfluxDB.")
		os.Exit(1)
	}

	ctx := context.Background()

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

	go startLogListener(ctx)
	go startStubScraper(ctx)

	api.StartAPI(influxClient)
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
		if err := influxClient.BatchInsert(ctx, buffer); err != nil {
			fmt.Printf("Error performing InfluxDB batch insert: %v\n", err)
		} else {
			fmt.Printf("Inserted %d logs to InfluxDB in %v\n", len(buffer), time.Since(timeLastLogFired))
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

// startStubScraper polls nginx's stub_status endpoint on a timer. It is opt-in:
// with STUB_STATUS_URL unset the collector behaves exactly as before. Scrape and
// write failures are logged and retried on the next tick — a proxy that is down
// must not take the log pipeline with it.
func startStubScraper(ctx context.Context) {
	url := os.Getenv("STUB_STATUS_URL")
	if url == "" {
		return
	}

	interval := 10 * time.Second
	if v := os.Getenv("STUB_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		} else {
			fmt.Printf("Invalid STUB_INTERVAL %q, using %v\n", v, interval)
		}
	}

	// Keep the HTTP timeout below the interval so a hung endpoint can't stack up scrapes.
	timeout := interval / 2
	if timeout > 5*time.Second {
		timeout = 5 * time.Second
	}

	client := stub.NewClient(url, timeout)
	fmt.Printf("Scraping stub_status at %s every %v\n", url, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if s, err := client.Fetch(ctx); err != nil {
			fmt.Printf("stub_status scrape failed: %v\n", err)
		} else if err := influxClient.WriteStubStatus(ctx, s, time.Now()); err != nil {
			fmt.Printf("stub_status write failed: %v\n", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
