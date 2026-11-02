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
	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
)

func TestBucketListStallAndPageLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"items":[{"name":"b1"}],"nextPageToken":"same"}`))
	}))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("storage", srv.URL)
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Endpoints: ep, MaxPages: 2, ListTimeout: 40 * time.Millisecond})
	if _, err := h.BucketList(false); err == nil {
		t.Fatal("repeated token must fail")
	}
	if got := atomic.LoadInt32(&calls); got > 2 {
		t.Fatalf("bucket list requests=%d, want <=2", got)
	}
}

func TestQueueListStallAndPageLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if strings.Contains(r.URL.Path, "/topics") {
			_, _ = w.Write([]byte(`{"topics":[{"name":"projects/gcpsim-local/topics/q1"}],"nextPageToken":"same"}`))
			return
		}
		_, _ = w.Write([]byte(`{"subscriptions":[{"name":"projects/gcpsim-local/subscriptions/q1"}],"nextPageToken":"same"}`))
	}))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("queue", srv.URL)
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Endpoints: ep, MaxPages: 2, ListTimeout: 40 * time.Millisecond})
	if _, err := h.QueueList(false); err == nil {
		t.Fatal("repeated token must fail")
	}
	if got := atomic.LoadInt32(&calls); got > 4 {
		t.Fatalf("queue list requests=%d, want <=4", got)
	}
}

func TestReceiveHonoursParentDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(100 * time.Millisecond); w.WriteHeader(200) }))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("queue", srv.URL)
	ctx, cancel := context.WithTimeout(hostkit.WithEndpoint(context.Background(), srv.URL), 10*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := a.Receive(ctx, "queue-one", 1, 1, 1)
	if err == nil {
		t.Fatal("deadline should error")
	}
	if elapsed := time.Since(start); elapsed > 80*time.Millisecond {
		t.Fatalf("receive took %v", elapsed)
	}
	_ = ep
}

func TestReceiveDoesNotRetryAmbiguousPull(t *testing.T) {
	var pulls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":pull") {
			atomic.AddInt32(&pulls, 1)
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("hijack unavailable")
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("queue", srv.URL)
	ctx := hostkit.WithEndpoint(context.Background(), srv.URL)
	_, err := a.Receive(ctx, "queue-one", 1, 1, 1)
	if err == nil {
		t.Fatal("ambiguous pull should error")
	}
	if got := atomic.LoadInt32(&pulls); got != 1 {
		t.Fatalf("pulls=%d, want 1", got)
	}
}
