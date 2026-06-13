package api

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Server preference order (best ratio first). The client's Accept-Encoding decides
// what's allowed; among the allowed encodings we pick the first in this list.
var encoderOrder = []string{"zstd", "br", "gzip", "deflate"}

func newEncoder(name string, w io.Writer) io.WriteCloser {
	switch name {
	case "zstd":
		// Concurrency 1 keeps each request from spawning GOMAXPROCS goroutines.
		zw, _ := zstd.NewWriter(w, zstd.WithEncoderConcurrency(1))
		return zw
	case "br":
		return brotli.NewWriterLevel(w, brotli.DefaultCompression)
	case "gzip":
		return gzip.NewWriter(w)
	case "deflate":
		fw, _ := flate.NewWriter(w, flate.DefaultCompression)
		return fw
	}
	return nil
}

// negotiateEncoding parses Accept-Encoding (honoring q-values; q=0 disables an
// encoding) and returns the best supported encoding, or "" for none.
func negotiateEncoding(header string) string {
	if header == "" {
		return ""
	}
	accepted := map[string]float64{}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, q := part, 1.0
		if i := strings.IndexByte(part, ';'); i >= 0 {
			name = strings.TrimSpace(part[:i])
			for _, p := range strings.Split(part[i+1:], ";") {
				if p = strings.TrimSpace(p); strings.HasPrefix(p, "q=") {
					if v, err := strconv.ParseFloat(p[2:], 64); err == nil {
						q = v
					}
				}
			}
		}
		accepted[strings.ToLower(name)] = q
	}
	for _, enc := range encoderOrder {
		if q, ok := accepted[enc]; ok && q > 0 {
			return enc
		}
	}
	return ""
}

type compressWriter struct {
	http.ResponseWriter
	enc io.WriteCloser
}

func (c *compressWriter) Write(b []byte) (int, error) { return c.enc.Write(b) }

func (c *compressWriter) Flush() {
	if f, ok := c.enc.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// compress wraps a handler with content-encoding negotiation across zstd/br/gzip/deflate.
func compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vary is required for correct caching regardless of whether we compress.
		w.Header().Add("Vary", "Accept-Encoding")

		enc := negotiateEncoding(r.Header.Get("Accept-Encoding"))
		if enc == "" {
			next.ServeHTTP(w, r)
			return
		}

		ew := newEncoder(enc, w)
		if ew == nil {
			next.ServeHTTP(w, r)
			return
		}
		defer ew.Close()

		h := w.Header()
		h.Set("Content-Encoding", enc)
		// Length is unknown until compression finishes; let the server chunk it.
		h.Del("Content-Length")

		next.ServeHTTP(&compressWriter{ResponseWriter: w, enc: ew}, r)
	})
}
