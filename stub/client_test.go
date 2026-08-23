package stub

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Real nginx emits a trailing space on three of the four lines.
const sample = "Active connections: 43 \n" +
	"server accepts handled requests\n" +
	" 7368 7360 10993 \n" +
	"Reading: 1 Writing: 5 Waiting: 38 \n"

func TestParse(t *testing.T) {
	got, err := Parse(sample)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	for _, c := range []struct {
		name string
		got  int64
		want int64
	}{
		{"Active", got.Active, 43},
		{"Accepts", got.Accepts, 7368},
		{"Handled", got.Handled, 7360},
		{"Requests", got.Requests, 10993},
		{"Reading", got.Reading, 1},
		{"Writing", got.Writing, 5},
		{"Waiting", got.Waiting, 38},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"empty":           "",
		"truncated":       "Active connections: 43\nserver accepts handled requests\n 1 2 3\n",
		"html body":       "<html><head><title>Welcome</title></head>\n<body>\n<h1>hi</h1>\n</body>\n</html>\n",
		"non-numeric":     "Active connections: x \nserver accepts handled requests\n 1 2 3 \nReading: 0 Writing: 0 Waiting: 0 \n",
		"short last line": "Active connections: 1 \nserver accepts handled requests\n 1 2 3 \nReading: 0 Writing: 0 \n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(body); err == nil {
				t.Errorf("Parse(%q) = nil error, want error", name)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, sample)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, time.Second).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.Active != 43 || got.Requests != 10993 || got.Waiting != 38 {
		t.Errorf("Fetch() = %+v", got)
	}
}

func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, time.Second).Fetch(context.Background()); err == nil {
		t.Error("Fetch() on 403 = nil error, want error")
	}
}
