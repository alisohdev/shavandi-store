package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthHandlers(t *testing.T) {
	databaseChecks := 0
	handler := NewHandler(func(ctx context.Context) error {
		databaseChecks++
		if _, ok := ctx.Deadline(); !ok {
			t.Error("readiness check has no deadline")
		}
		return nil
	}, nil, testLogger())

	live := httptest.NewRecorder()
	liveRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	liveRequest.Header.Set("X-Request-ID", "untrusted")
	handler.ServeHTTP(live, liveRequest)
	if live.Code != http.StatusOK {
		t.Fatalf("liveness status = %d, want 200", live.Code)
	}
	if got := live.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("liveness content type = %q, want JSON", got)
	}
	if live.Header().Get("X-Request-ID") == "" || live.Header().Get("X-Request-ID") == "untrusted" {
		t.Fatalf("liveness request ID = %q", live.Header().Get("X-Request-ID"))
	}
	if databaseChecks != 0 {
		t.Fatalf("liveness made %d database checks, want 0", databaseChecks)
	}

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200", ready.Code)
	}
	if databaseChecks != 1 {
		t.Fatalf("readiness made %d database checks, want 1", databaseChecks)
	}
}

func TestReadinessReturnsJSONWhenDatabaseIsUnavailable(t *testing.T) {
	handler := NewHandler(func(context.Context) error {
		return errors.New("connection refused")
	}, nil, testLogger())
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	assertErrorResponse(t, response, http.StatusServiceUnavailable, "database_unavailable")
}

func TestNotFoundAndMethodNotAllowedUseJSONErrors(t *testing.T) {
	handler := NewHandler(func(context.Context) error { return nil }, nil, testLogger())

	tests := []struct {
		name   string
		method string
		path   string
		status int
		code   string
	}{
		{name: "missing route", method: http.MethodGet, path: "/missing", status: http.StatusNotFound, code: "not_found"},
		{name: "unsupported method", method: http.MethodPost, path: "/health/live", status: http.StatusMethodNotAllowed, code: "method_not_allowed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			assertErrorResponse(t, response, test.status, test.code)
		})
	}
}

func TestRecoveryReturnsJSONBeforeResponseStarts(t *testing.T) {
	handler := requestIDMiddleware(testLogger())(recoverPanics(testLogger())(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) {
			panic("unexpected failure")
		},
	)))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	assertErrorResponse(t, response, http.StatusInternalServerError, "internal_error")
}

func TestRecoveryDoesNotAppendJSONAfterResponseStarts(t *testing.T) {
	handler := requestIDMiddleware(testLogger())(recoverPanics(testLogger())(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "partial")
			panic("unexpected failure")
		},
	)))
	response := httptest.NewRecorder()

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	if recovered == nil {
		t.Fatal("panic after response commitment was swallowed")
	}
	if response.Code != http.StatusCreated || response.Body.String() != "partial" {
		t.Fatalf("response after panic = %d %q", response.Code, response.Body.String())
	}
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content type = %q, want JSON", got)
	}

	var body errorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q", body.Error.Code, code)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("error request ID %q does not match response header %q",
			body.Error.RequestID, response.Header().Get("X-Request-ID"))
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
