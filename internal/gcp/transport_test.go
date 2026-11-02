package gcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lmctl-ai/lmcloud/hostkit"
)

func testHost(t *testing.T, ctx context.Context, endpoint string) *Host {
	t.Helper()
	h := NewHost()
	h.ctx = ctx
	h.client = hostkit.NewHTTPClient(2 * time.Second)
	if _, err := h.SetEndpoint("storage", endpoint); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestTransportRetries503ThreeTimes(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	h := testHost(t, context.Background(), srv.URL)
	if _, err := h.BucketList(); err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("503 attempts = %d, want 3", got)
	}
}

func TestTransportHonoursCallerDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	h := testHost(t, ctx, srv.URL)
	start := time.Now()
	if _, err := h.BucketList(); err == nil {
		t.Fatal("expected deadline error")
	}
	if elapsed := time.Since(start); elapsed > 80*time.Millisecond {
		t.Fatalf("deadline took %v", elapsed)
	}
}

func TestTransportRejectsOversizeRead(t *testing.T) {
	body := strings.Repeat("x", 16*1024*1024+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer srv.Close()
	h := testHost(t, context.Background(), srv.URL)
	if _, err := h.BucketList(); err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestTransportDoesNotFollowCrossOriginRedirect(t *testing.T) {
	var reached int32
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&reached, 1) }))
	defer dest.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	h := testHost(t, context.Background(), srv.URL)
	_, _ = h.BucketList()
	if atomic.LoadInt32(&reached) != 0 {
		t.Fatal("cross-origin redirect was followed")
	}
}
