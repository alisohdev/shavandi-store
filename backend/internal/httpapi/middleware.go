package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync/atomic"
)

type requestIDKey struct{}

var fallbackRequestID atomic.Uint64

type trackingResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *trackingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *trackingResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *trackingResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func requestIDMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := newRequestID()
			if err != nil {
				logger.Error("generate request ID", "error", err)
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func recoverPanics(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracked := &trackingResponseWriter{ResponseWriter: w}
			defer func() {
				value := recover()
				if value == nil {
					return
				}
				logger.Error("HTTP handler panic",
					"request_id", requestIDFrom(r.Context()),
					"panic", value,
					"stack", string(debug.Stack()),
				)
				if tracked.wroteHeader {
					panic(value)
				}
				writeError(tracked, r, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			}()
			next.ServeHTTP(tracked, r)
		})
	}
}

func newRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "fallback-" + strconv.FormatUint(fallbackRequestID.Add(1), 10), err
	}
	return hex.EncodeToString(value[:]), nil
}

func requestIDFrom(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}
