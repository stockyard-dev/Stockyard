package engine

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipPool reuses gzip writers to reduce allocations under load.
var gzipPool = sync.Pool{
	New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return gz
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		// If Content-Type not set, let Go detect it from first write
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", http.DetectContentType(b))
		}
		w.wroteHeader = true
	}
	return w.gz.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	w.gz.Flush()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// gzipMiddleware compresses responses for clients that accept gzip.
// Skips binary content (images, tarballs) and streaming responses.
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if client doesn't accept gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip streaming endpoints (SSE)
		if strings.HasSuffix(r.URL.Path, "/stream") || r.URL.Query().Get("stream") == "true" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip binary downloads
		path := r.URL.Path
		if strings.HasSuffix(path, ".png") || strings.HasSuffix(path, ".webp") ||
			strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".zip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipPool.Get().(*gzip.Writer)
		gz.Reset(w)

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		// Remove Content-Length since gzip changes it
		w.Header().Del("Content-Length")

		grw := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		next.ServeHTTP(grw, r)

		gz.Close()
		gzipPool.Put(gz)
	})
}
