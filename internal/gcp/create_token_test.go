package gcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"context"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
)

func TestBucketCreateTokenConflict(t *testing.T) {
	var mu sync.Mutex
	token := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/b":
			var body struct {
				Labels map[string]string `json:"labels"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			token = body.Labels["lmcloud_create_token"]
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodGet && r.URL.Path == "/storage/v1/b/bkt-one":
			mu.Lock()
			tok := token
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "bkt-one", "labels": map[string]string{"lmcloud_create_token": tok}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("storage", srv.URL)
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Endpoints: ep})
	if created, err := h.BucketCreate(false, "bkt-one"); err != nil {
		t.Fatal(err)
	} else if created {
		t.Fatal("token-bearing conflict should be created=true")
	}
	mu.Lock()
	token = ""
	mu.Unlock()
	if created, err := h.BucketCreate(false, "bkt-one"); err != nil {
		t.Fatal(err)
	} else if created {
		t.Fatal("pre-existing conflict without token must be false")
	}
}

func TestQueueCreateTokenConflict(t *testing.T) {
	var mu sync.Mutex
	token := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/projects/gcpsim-local/topics/queue-one":
			var body struct {
				Labels map[string]string `json:"labels"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			token = body.Labels["lmcloud_create_token"]
			mu.Unlock()
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/projects/gcpsim-local/subscriptions/queue-one":
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects/gcpsim-local/topics/queue-one":
			mu.Lock()
			tok := token
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "projects/gcpsim-local/topics/queue-one", "labels": map[string]string{"lmcloud_create_token": tok}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	a := NewAdapter()
	ctx := hostkit.WithCreateToken(hostkit.WithEndpoint(context.Background(), srv.URL), "our-token")
	if created, err := a.CreateQueue(ctx, "queue-one"); err == nil || created {
		t.Fatalf("conflict should be error, created=%v err=%v", created, err)
	}
	got, err := a.QueueCreateToken(ctx, "queue-one")
	if err != nil {
		t.Fatal(err)
	}
	if got != "our-token" {
		t.Fatalf("token = %q, want our-token", got)
	}
	mu.Lock()
	token = ""
	mu.Unlock()
	got, err = a.QueueCreateToken(ctx, "queue-one")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("pre-existing token = %q, want empty", got)
	}
}
