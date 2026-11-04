package httpapi

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeGracefullyDrainsActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "done")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(ctx, listener, handler, testLogger())
	}()

	responseDone := make(chan *http.Response, 1)
	requestErr := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			requestErr <- err
			return
		}
		responseDone <- response
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not reach handler")
	}

	cancel()
	select {
	case err := <-serveDone:
		t.Fatalf("server stopped before the active request drained: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)

	select {
	case err := <-requestErr:
		t.Fatalf("request failed during graceful shutdown: %v", err)
	case response := <-responseDone:
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("response status = %d, want 200", response.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active request did not finish")
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop after the request drained")
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		t.Fatal("listener accepted a connection after shutdown")
	}
}
