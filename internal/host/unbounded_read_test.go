package host

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEmulatorObjectReadComplete(t *testing.T) {
	payload := strings.Repeat("x", 16*1024*1024+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", "")
	t.Setenv("LMGCP_LEDGER", "")
	t.Setenv("LMGCP_ENDPOINT", srv.URL)
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := h.StorageReadObject("bucket", "object"); got != payload {
		t.Fatalf("object incomplete: got %d bytes, want %d; error=%s", len(got), len(payload), h.LastError())
	}
}

func TestEmulatorObjectReadWithoutDeadline(t *testing.T) {
	// A test fixture, deliberately beyond the retired 30-second client deadline.
	// Product code has no timer; parent request cancellation still ends the fixture.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(31 * time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
			_, _ = w.Write([]byte("complete slow response"))
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	t.Setenv("LMGCP_LIVE", "")
	t.Setenv("LMGCP_STATE_DIR", "")
	t.Setenv("LMGCP_LEDGER", "")
	t.Setenv("LMGCP_ENDPOINT", srv.URL)
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := h.StorageReadObject("bucket", "object"); got != "complete slow response" {
		t.Fatalf("slow response incomplete: %q; error=%s", got, h.LastError())
	}
}
