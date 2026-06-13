package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/jagadam97/nginx-logger/models"
)

type InfluxClient struct {
	client influxdb2.Client
	write  api.WriteAPIBlocking
	query  api.QueryAPI
	bucket string
	org    string
}

// LogRecord is a single log entry returned by the API.
type LogRecord struct {
	Timestamp            time.Time `json:"timestamp"`
	RemoteAddr           string    `json:"remote_addr"`
	RequestURI           string    `json:"request_uri"`
	Status               int64     `json:"status"`
	ServerName           string    `json:"server_name"`
	RequestTime          float64   `json:"request_time"`
	RequestMethod        string    `json:"request_method"`
	BytesSent            int64     `json:"bytes_sent"`
	HTTPHost             string    `json:"http_host"`
	ServerProtocol       string    `json:"server_protocol"`
	UpstreamAddr         string    `json:"upstream_addr"`
	UpstreamResponseTime float64   `json:"upstream_response_time"`
	SSLProtocol          string    `json:"ssl_protocol"`
	SSLCipher            string    `json:"ssl_cipher"`
	HTTPUserAgent        string    `json:"http_user_agent"`
}

// Stats mirrors the Grafana dashboard panels.
type Stats struct {
	TotalRequests            int64            `json:"total_requests"`
	DataSentBytes            int64            `json:"data_sent_bytes"`
	AvgRequestTimeS          float64          `json:"avg_request_time_s"`
	AvgUpstreamResponseTimeS float64          `json:"avg_upstream_response_time_s"`
	ByStatusCode             map[string]int64 `json:"by_status_code"`
	TopHosts                 []TagCount       `json:"top_hosts"`
	TopIPs                   []TagCount       `json:"top_ips"`
}

type TagCount struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// Filters holds the available tag values for frontend dropdowns.
type Filters struct {
	Hosts    []string `json:"hosts"`
	Statuses []string `json:"statuses"`
	Clients  []string `json:"clients"`
}

// TagFilter holds the global, multi-value tag dimensions shared by every query.
// Within a dimension the values are OR'd; across dimensions they are AND'd.
type TagFilter struct {
	Hosts    []string // http_host values (exact)
	Statuses []string // exact codes ("404") or range prefixes ("2xx","3xx","4xx","5xx")
	Clients  []string // remote_addr values (exact)
}

// LogFilters extends the global tag filters with log-only refinements.
type LogFilters struct {
	TagFilter
	URI    string // substring match on request_uri
	Method string // exact request_method
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
		query:  client.QueryAPI(org),
		bucket: bucket,
		org:    org,
	}, nil
}

func (i *InfluxClient) Close() {
	if i != nil && i.client != nil {
		i.client.Close()
	}
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
				"remote_addr":     e.RemoteAddr,
			},
			map[string]interface{}{
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

func escapeFlux(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

// orTagFilter builds a Flux filter line matching a tag against any of the values (OR).
// Returns "" when no usable values are given.
func orTagFilter(tag string, values []string) string {
	var parts []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf(`r["%s"] == "%s"`, tag, escapeFlux(v)))
	}
	if len(parts) == 0 {
		return ""
	}
	return `  |> filter(fn: (r) => ` + strings.Join(parts, " or ") + ")\n"
}

// statusFilter builds a Flux filter line matching any of the given status values.
// Each value is an exact code ("404") or a range prefix ("2xx","3xx","4xx","5xx").
func statusFilter(values []string) string {
	var parts []string
	for _, s := range values {
		s = strings.TrimSpace(s)
		switch s {
		case "":
			continue
		case "2xx", "3xx", "4xx", "5xx":
			parts = append(parts, fmt.Sprintf(`r.status =~ /^%c/`, s[0]))
		default:
			parts = append(parts, fmt.Sprintf(`r["status"] == "%s"`, escapeFlux(s)))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return `  |> filter(fn: (r) => ` + strings.Join(parts, " or ") + ")\n"
}

// buildTagFilter chains one Flux filter stage per dimension (AND across dimensions,
// OR within each). An empty dimension contributes nothing.
func buildTagFilter(tf TagFilter) string {
	return orTagFilter("http_host", tf.Hosts) +
		statusFilter(tf.Statuses) +
		orTagFilter("remote_addr", tf.Clients)
}

// logsCutoff returns the timestamp of the `limit`-th most recent log in the range,
// looking at a single field (status_code) so no pivot is needed. The returned time
// is the lower bound for the pivot stage, keeping it bounded to ~limit rows.
// If fewer than `limit` records exist, it returns `from` (the full range is cheap).
func (i *InfluxClient) logsCutoff(ctx context.Context, from, to time.Time, tagFilter string, limit int) (time.Time, error) {
	q := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> keep(columns: ["_time"])
  |> group()
  |> sort(columns: ["_time"], desc: true)
  |> limit(n: %d)
`, i.bucket, from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano), tagFilter, limit)

	res, err := i.query.Query(ctx, q)
	if err != nil {
		return from, fmt.Errorf("cutoff query failed: %w", err)
	}
	defer res.Close()

	cutoff := to
	n := 0
	for res.Next() {
		t := res.Record().Time()
		if t.Before(cutoff) {
			cutoff = t // results are sorted desc, so the last row is the oldest
		}
		n++
	}
	if res.Err() != nil {
		return from, fmt.Errorf("cutoff result error: %w", res.Err())
	}
	// Fewer rows than the limit means the whole range fits — scan it all.
	if n < limit {
		return from, nil
	}
	return cutoff, nil
}

func (i *InfluxClient) QueryLogs(ctx context.Context, from, to time.Time, f LogFilters, limit int) ([]LogRecord, error) {
	// Tag filters run before pivot (cheap — skip whole series).
	tagFilter := buildTagFilter(f.TagFilter)
	if f.Method != "" {
		tagFilter += fmt.Sprintf(`  |> filter(fn: (r) => r["request_method"] == "%s")`+"\n", escapeFlux(f.Method))
	}

	// URI filter runs after pivot because request_uri is a field, not a tag.
	uriFilter := ""
	if f.URI != "" {
		uriFilter = fmt.Sprintf(`  |> filter(fn: (r) => strings.containsStr(v: r["request_uri"], substr: "%s"))`+"\n", escapeFlux(f.URI))
	}

	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	// Stage 1: find the cutoff time for the newest `limit` records using a single
	// field only. This avoids pivoting the whole range (pivot is memory-heavy and
	// will OOM InfluxDB on large windows). No pivot here — just sort + limit on one
	// field, which InfluxDB streams cheaply.
	cutoff, err := i.logsCutoff(ctx, from, to, tagFilter, limit)
	if err != nil {
		return nil, err
	}

	// Stage 2: pivot only the narrow [cutoff, to] slice — at most ~limit rows.
	query := fmt.Sprintf(`
import "strings"

from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "nginx_logs")
%s  |> pivot(rowKey: ["_time"], columnKey: ["_field"], valueColumn: "_value")
%s  |> group()
  |> sort(columns: ["_time"], desc: true)
  |> limit(n: %d)
`, i.bucket, cutoff.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano), tagFilter, uriFilter, limit)

	result, err := i.query.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer result.Close()

	var logs []LogRecord
	for result.Next() {
		rec := result.Record()
		statusStr, _ := rec.ValueByKey("status").(string)
		statusCode, _ := strconv.ParseInt(statusStr, 10, 64)

		logs = append(logs, LogRecord{
			Timestamp:            rec.Time(),
			RemoteAddr:           strVal(rec.ValueByKey("remote_addr")),
			RequestURI:           strVal(rec.ValueByKey("request_uri")),
			Status:               statusCode,
			ServerName:           strVal(rec.ValueByKey("server_name")),
			RequestTime:          f64Val(rec.ValueByKey("request_time")),
			RequestMethod:        strVal(rec.ValueByKey("request_method")),
			BytesSent:            i64Val(rec.ValueByKey("bytes_sent")),
			HTTPHost:             strVal(rec.ValueByKey("http_host")),
			ServerProtocol:       strVal(rec.ValueByKey("server_protocol")),
			UpstreamAddr:         strVal(rec.ValueByKey("upstream_addr")),
			UpstreamResponseTime: f64Val(rec.ValueByKey("upstream_response_time")),
			SSLProtocol:          strVal(rec.ValueByKey("ssl_protocol")),
			SSLCipher:            strVal(rec.ValueByKey("ssl_cipher")),
			HTTPUserAgent:        strVal(rec.ValueByKey("http_user_agent")),
		})
	}
	if result.Err() != nil {
		return nil, fmt.Errorf("query result error: %w", result.Err())
	}
	return logs, nil
}

// QueryStats runs all Grafana-equivalent panel queries and returns the aggregated result.
// tf scopes every panel to the selected hosts/statuses/clients (empty = all).
func (i *InfluxClient) QueryStats(ctx context.Context, from, to time.Time, tf TagFilter) (*Stats, error) {
	rangeStr := fmt.Sprintf("start: %s, stop: %s", from.Format(time.RFC3339), to.Format(time.RFC3339))
	tagFilter := buildTagFilter(tf)

	// Panel 1: Total Requests — count of status_code field
	totalQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> group()
  |> count()
`, i.bucket, rangeStr, tagFilter)

	// Panel 2: Data Sent — sum of bytes_sent field
	dataSentQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "bytes_sent")
%s  |> group()
  |> sum()
`, i.bucket, rangeStr, tagFilter)

	// Panel 7: Avg Request Time — mean of request_time field
	avgRTQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "request_time")
%s  |> group()
  |> mean()
`, i.bucket, rangeStr, tagFilter)

	// Panel 8: Avg Upstream Response Time — mean of upstream_response_time field
	avgUpstreamQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "upstream_response_time")
%s  |> group()
  |> mean()
`, i.bucket, rangeStr, tagFilter)

	// Panel 3: HTTP Status Codes — count per exact status code tag
	byStatusQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> group(columns: ["status"])
  |> count()
  |> group()
`, i.bucket, rangeStr, tagFilter)

	// Panel 6: Top Websites — count per http_host, top 5
	topHostsQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> group(columns: ["http_host"])
  |> count()
  |> group()
  |> sort(columns: ["_value"], desc: true)
  |> limit(n: 5)
`, i.bucket, rangeStr, tagFilter)

	// Panel 4: Top Client IPs — count per remote_addr, top 5
	topIPsQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> group(columns: ["remote_addr"])
  |> count()
  |> group()
  |> sort(columns: ["_value"], desc: true)
  |> limit(n: 5)
`, i.bucket, rangeStr, tagFilter)

	stats := &Stats{ByStatusCode: make(map[string]int64)}

	if r, err := i.query.Query(ctx, totalQuery); err == nil {
		if r.Next() {
			stats.TotalRequests = i64Val(r.Record().Value())
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, dataSentQuery); err == nil {
		if r.Next() {
			stats.DataSentBytes = i64Val(r.Record().Value())
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, avgRTQuery); err == nil {
		if r.Next() {
			stats.AvgRequestTimeS = f64Val(r.Record().Value())
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, avgUpstreamQuery); err == nil {
		if r.Next() {
			stats.AvgUpstreamResponseTimeS = f64Val(r.Record().Value())
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, byStatusQuery); err == nil {
		for r.Next() {
			code := strVal(r.Record().ValueByKey("status"))
			stats.ByStatusCode[code] = i64Val(r.Record().Value())
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, topHostsQuery); err == nil {
		for r.Next() {
			stats.TopHosts = append(stats.TopHosts, TagCount{
				Label: strVal(r.Record().ValueByKey("http_host")),
				Count: i64Val(r.Record().Value()),
			})
		}
		r.Close()
	}

	if r, err := i.query.Query(ctx, topIPsQuery); err == nil {
		for r.Next() {
			stats.TopIPs = append(stats.TopIPs, TagCount{
				Label: strVal(r.Record().ValueByKey("remote_addr")),
				Count: i64Val(r.Record().Value()),
			})
		}
		r.Close()
	}

	return stats, nil
}

// TimeSeriesPoint is one window bucket in a time series query.
type TimeSeriesPoint struct {
	Time                 time.Time `json:"time"`
	Requests             int64     `json:"requests"`
	BytesSent            int64     `json:"bytes_sent"`
	AvgRequestTimeS      float64   `json:"avg_request_time_s"`
	AvgUpstreamTimeS     float64   `json:"avg_upstream_response_time_s"`
}

// windowPeriod picks a bucket size that avoids spike mountains for the given range.
func windowPeriod(from, to time.Time) string {
	d := to.Sub(from)
	switch {
	case d <= time.Hour:
		return "1m"
	case d <= 6*time.Hour:
		return "5m"
	case d <= 24*time.Hour:
		return "30m"
	case d <= 7*24*time.Hour:
		return "2h"
	default:
		return "12h"
	}
}

// QueryTimeSeries returns window-aggregated metrics for charting.
func (i *InfluxClient) QueryTimeSeries(ctx context.Context, from, to time.Time, tf TagFilter) ([]TimeSeriesPoint, error) {
	rangeStr := fmt.Sprintf("start: %s, stop: %s", from.Format(time.RFC3339), to.Format(time.RFC3339))
	every := windowPeriod(from, to)
	tagFilter := buildTagFilter(tf)

	countQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "status_code")
%s  |> group()
  |> aggregateWindow(every: %s, fn: count, createEmpty: true)
  |> fill(value: 0)
`, i.bucket, rangeStr, tagFilter, every)

	bytesQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "bytes_sent")
%s  |> group()
  |> aggregateWindow(every: %s, fn: sum, createEmpty: true)
  |> fill(value: 0)
`, i.bucket, rangeStr, tagFilter, every)

	rtQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "request_time")
%s  |> group()
  |> aggregateWindow(every: %s, fn: mean, createEmpty: true)
  |> fill(value: 0.0)
`, i.bucket, rangeStr, tagFilter, every)

	upstreamQuery := fmt.Sprintf(`
from(bucket: "%s")
  |> range(%s)
  |> filter(fn: (r) => r._measurement == "nginx_logs" and r._field == "upstream_response_time")
%s  |> group()
  |> aggregateWindow(every: %s, fn: mean, createEmpty: true)
  |> fill(value: 0.0)
`, i.bucket, rangeStr, tagFilter, every)

	// Collect all four series into a map keyed by timestamp.
	type bucket struct {
		requests         int64
		bytesSent        int64
		avgRequestTime   float64
		avgUpstreamTime  float64
	}
	buckets := make(map[time.Time]*bucket)

	ensure := func(t time.Time) *bucket {
		if buckets[t] == nil {
			buckets[t] = &bucket{}
		}
		return buckets[t]
	}

	if r, err := i.query.Query(ctx, countQuery); err == nil {
		for r.Next() {
			ensure(r.Record().Time()).requests = i64Val(r.Record().Value())
		}
		r.Close()
	}
	if r, err := i.query.Query(ctx, bytesQuery); err == nil {
		for r.Next() {
			ensure(r.Record().Time()).bytesSent = i64Val(r.Record().Value())
		}
		r.Close()
	}
	if r, err := i.query.Query(ctx, rtQuery); err == nil {
		for r.Next() {
			ensure(r.Record().Time()).avgRequestTime = f64Val(r.Record().Value())
		}
		r.Close()
	}
	if r, err := i.query.Query(ctx, upstreamQuery); err == nil {
		for r.Next() {
			ensure(r.Record().Time()).avgUpstreamTime = f64Val(r.Record().Value())
		}
		r.Close()
	}

	points := make([]TimeSeriesPoint, 0, len(buckets))
	for t, b := range buckets {
		points = append(points, TimeSeriesPoint{
			Time:             t,
			Requests:         b.requests,
			BytesSent:        b.bytesSent,
			AvgRequestTimeS:  b.avgRequestTime,
			AvgUpstreamTimeS: b.avgUpstreamTime,
		})
	}
	// Sort ascending by time.
	for i := 0; i < len(points); i++ {
		for j := i + 1; j < len(points); j++ {
			if points[j].Time.Before(points[i].Time) {
				points[i], points[j] = points[j], points[i]
			}
		}
	}
	return points, nil
}

// QueryFilters returns available tag values for frontend dropdowns (Grafana variable equivalent).
func (i *InfluxClient) QueryFilters(ctx context.Context) (*Filters, error) {
	hostsQuery := fmt.Sprintf(`
import "influxdata/influxdb/schema"
schema.tagValues(bucket: "%s", tag: "http_host")
`, i.bucket)

	statusesQuery := fmt.Sprintf(`
import "influxdata/influxdb/schema"
schema.tagValues(bucket: "%s", tag: "status")
`, i.bucket)

	clientsQuery := fmt.Sprintf(`
import "influxdata/influxdb/schema"
schema.tagValues(bucket: "%s", tag: "remote_addr")
`, i.bucket)

	filters := &Filters{}

	collect := func(query string, dst *[]string) {
		if r, err := i.query.Query(ctx, query); err == nil {
			for r.Next() {
				if v := strVal(r.Record().Value()); v != "" {
					*dst = append(*dst, v)
				}
			}
			r.Close()
		}
	}

	collect(hostsQuery, &filters.Hosts)
	collect(statusesQuery, &filters.Statuses)
	collect(clientsQuery, &filters.Clients)

	return filters, nil
}

func strVal(v interface{}) string {
	s, _ := v.(string)
	return s
}

func f64Val(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}

func i64Val(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	}
	return 0
}
