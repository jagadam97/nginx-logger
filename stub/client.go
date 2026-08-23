// Package stub polls an nginx http_stub_status_module endpoint. It is separate
// from package log because stub_status is scraped over HTTP on a timer, not
// tailed from a file.
package stub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jagadam97/nginx-logger/models"
)

type Client struct {
	url  string
	http *http.Client
}

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{url: url, http: &http.Client{Timeout: timeout}}
}

func (c *Client) Fetch(ctx context.Context) (models.StubStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return models.StubStatus{}, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return models.StubStatus{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.StubStatus{}, fmt.Errorf("stub_status returned %s", resp.Status)
	}

	// The response is four short lines. Cap the read so a URL misconfigured to
	// point at a real site can't stream an unbounded body into memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return models.StubStatus{}, err
	}

	return Parse(string(body))
}

// Parse reads the fixed four-line stub_status body:
//
//	Active connections: 43
//	server accepts handled requests
//	 7368 7368 10993
//	Reading: 0 Writing: 5 Waiting: 38
func Parse(body string) (models.StubStatus, error) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) < 4 {
		return models.StubStatus{}, fmt.Errorf("expected 4 lines, got %d", len(lines))
	}

	// Collect the first parse failure rather than checking seven return values.
	var perr error
	num := func(fields []string, idx int, name string) int64 {
		if perr != nil {
			return 0
		}
		if idx >= len(fields) {
			perr = fmt.Errorf("missing %s", name)
			return 0
		}
		n, err := strconv.ParseInt(fields[idx], 10, 64)
		if err != nil {
			perr = fmt.Errorf("bad %s %q: %w", name, fields[idx], err)
			return 0
		}
		return n
	}

	active := strings.Fields(lines[0]) // Active connections: N
	counts := strings.Fields(lines[2]) // N N N
	states := strings.Fields(lines[3]) // Reading: N Writing: N Waiting: N

	s := models.StubStatus{
		Active:   num(active, 2, "active connections"),
		Accepts:  num(counts, 0, "accepts"),
		Handled:  num(counts, 1, "handled"),
		Requests: num(counts, 2, "requests"),
		Reading:  num(states, 1, "reading"),
		Writing:  num(states, 3, "writing"),
		Waiting:  num(states, 5, "waiting"),
	}
	if perr != nil {
		return models.StubStatus{}, perr
	}
	return s, nil
}
