package database

import (
	"context"
	"fmt"
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/jagadam97/nginx-logger/models"
)

type InfluxClient struct {
	client influxdb2.Client
	write  api.WriteAPIBlocking
}

func InfluxEnabled() bool {
	return os.Getenv("INFLUX_URL") != ""
}

func ConnectInflux() (*InfluxClient, error) {
	url := os.Getenv("INFLUX_URL")
	token := os.Getenv("INFLUX_TOKEN")
	org := os.Getenv("INFLUX_ORG")
	bucket := os.Getenv("INFLUX_BUCKET")

	if url == "" || bucket == "" {
		return nil, fmt.Errorf("INFLUX_URL and INFLUX_BUCKET must be set")
	}

	client := influxdb2.NewClient(url, token)
	ok, err := client.Ping(context.Background())
	if err != nil || !ok {
		client.Close()
		if err == nil {
			err = fmt.Errorf("influxdb ping returned not ok")
		}
		return nil, err
	}

	return &InfluxClient{
		client: client,
		write:  client.WriteAPIBlocking(org, bucket),
	}, nil
}

func (i *InfluxClient) Close() {
	if i != nil && i.client != nil {
		i.client.Close()
	}
}

func (i *InfluxClient) BatchInsert(ctx context.Context, buffer []models.LogEntry) error {
	points := make([]*write.Point, 0, len(buffer))
	for _, e := range buffer {
		p := influxdb2.NewPoint(
			"nginx_logs",
			map[string]string{
				"server_name":     e.ServerName,
				"request_method":  e.RequestMethod,
				"http_host":       e.HTTPHost,
				"server_protocol": e.ServerProtocol,
				"ssl_protocol":    e.SSLProtocol,
				"ssl_cipher":      e.SSLCipher,
				"status":          fmt.Sprintf("%d", e.Status),
			},
			map[string]interface{}{
				"remote_addr":            e.RemoteAddr,
				"request_uri":            e.RequestURI,
				"request_time":           e.RequestTime,
				"bytes_sent":             int64(e.BytesSent),
				"upstream_addr":          e.UpstreamAddr,
				"upstream_response_time": e.UpstreamResponseTime,
				"http_user_agent":        e.HTTPUserAgent,
				"status_code":            int64(e.Status),
			},
			e.TimeLocal,
		)
		points = append(points, p)
	}
	return i.write.WritePoint(ctx, points...)
}

func (i *InfluxClient) Ping(ctx context.Context) error {
	ok, err := i.client.Ping(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("influxdb ping returned not ok")
	}
	return nil
}
