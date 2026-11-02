package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
)

type gcpTokenServer struct {
	mu            sync.Mutex
	failFirst     bool
	creates       int
	bucket, topic map[string]string
	subscription  map[string]string
}

func (s *gcpTokenServer) handle(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
		Topic  string            `json:"topic"`
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/storage/v1/b":
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.creates++
		if _, ok := s.bucket[body.Name]; ok {
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"error":{"code":409,"message":"exists","errors":[{"reason":"conflict"}]}}`))
			return true
		}
		s.bucket[body.Name] = body.Labels["lmcloud_create_token"]
		if s.failFirst && s.creates == 1 {
			w.WriteHeader(500)
			return true
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"name":%q}`, body.Name)))
		return true
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/storage/v1/b/") && r.URL.Path != "/storage/v1/b":
		name := strings.TrimPrefix(r.URL.Path, "/storage/v1/b/")
		tok, ok := s.bucket[name]
		if !ok {
			w.WriteHeader(404)
			return true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": name, "labels": map[string]string{"lmcloud_create_token": tok}})
		return true
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/projects/gcpsim-local/topics/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/projects/gcpsim-local/topics/")
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.creates++
		if _, ok := s.topic[name]; ok {
			w.WriteHeader(409)
			return true
		}
		s.topic[name] = body.Labels["lmcloud_create_token"]
		if s.failFirst && s.creates == 1 {
			w.WriteHeader(500)
			return true
		}
		_, _ = w.Write([]byte("{}"))
		return true
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/projects/gcpsim-local/topics/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/projects/gcpsim-local/topics/")
		tok, ok := s.topic[name]
		if !ok {
			w.WriteHeader(404)
			return true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "projects/gcpsim-local/topics/" + name, "labels": map[string]string{"lmcloud_create_token": tok}})
		return true
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/projects/gcpsim-local/subscriptions/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/projects/gcpsim-local/subscriptions/")
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := s.subscription[name]; ok {
			w.WriteHeader(409)
			return true
		}
		s.subscription[name] = body.Labels["lmcloud_create_token"]
		_, _ = w.Write([]byte("{}"))
		return true
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/projects/gcpsim-local/subscriptions/"):
		name := strings.TrimPrefix(r.URL.Path, "/v1/projects/gcpsim-local/subscriptions/")
		tok, ok := s.subscription[name]
		if !ok {
			w.WriteHeader(404)
			return true
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "projects/gcpsim-local/subscriptions/" + name, "topic": "projects/gcpsim-local/topics/" + name, "labels": map[string]string{"lmcloud_create_token": tok}})
		return true
	}
	return false
}

func newTokenHost(t *testing.T, s *gcpTokenServer, url string) *cloudhost.Host {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.handle(w, r) {
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("storage", srv.URL)
	_ = ep.Set("queue", srv.URL)
	return cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Endpoints: ep})
}

func TestGCPLostReplyCreateTokens(t *testing.T) {
	t.Run("bucket lost reply with our token", func(t *testing.T) {
		s := &gcpTokenServer{failFirst: true, bucket: map[string]string{}, topic: map[string]string{}, subscription: map[string]string{}}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.handle(w, r) {
				w.WriteHeader(404)
			}
		}))
		defer srv.Close()
		a := NewAdapter()
		ctx := hostkit.WithCreateToken(hostkit.WithEndpoint(context.Background(), srv.URL), "our-token")
		created, err := a.CreateBucket(ctx, "tok-bkt-one")
		if created || err == nil {
			t.Fatalf("bucket created=%v err=%v", created, err)
		}
		tok, err := a.BucketCreateToken(ctx, "tok-bkt-one")
		if err != nil || tok != "our-token" {
			t.Fatalf("bucket token=%q err=%v", tok, err)
		}
	})
	t.Run("bucket pre-existing different token", func(t *testing.T) {
		s := &gcpTokenServer{bucket: map[string]string{"tok-bkt-one": "other-token"}, topic: map[string]string{}, subscription: map[string]string{}}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.handle(w, r) {
				w.WriteHeader(404)
			}
		}))
		defer srv.Close()
		a := NewAdapter()
		ctx := hostkit.WithCreateToken(hostkit.WithEndpoint(context.Background(), srv.URL), "our-token")
		_, _ = a.CreateBucket(ctx, "tok-bkt-one")
		tok, _ := a.BucketCreateToken(ctx, "tok-bkt-one")
		if tok == "our-token" {
			t.Fatal("foreign bucket token matched")
		}
	})
}

func TestGCPPageLimitUniqueTokens(t *testing.T) {
	var n int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		cur := n
		mu.Unlock()
		if strings.Contains(r.URL.Path, "/storage/v1/b") {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "nextPageToken": fmt.Sprintf("p%d", cur)})
			return
		}
		if strings.Contains(r.URL.Path, "/topics") {
			_ = json.NewEncoder(w).Encode(map[string]any{"topics": []any{}, "nextPageToken": fmt.Sprintf("p%d", cur)})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"subscriptions": []any{}, "nextPageToken": fmt.Sprintf("p%d", cur)})
	}))
	defer srv.Close()
	a := NewAdapter()
	ep := Endpoints()
	_ = ep.Set("storage", srv.URL)
	_ = ep.Set("queue", srv.URL)
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Endpoints: ep, MaxPages: 2})
	if _, err := h.BucketList(false); err == nil {
		t.Fatal("bucket page limit must fail")
	}
	if _, err := h.QueueList(false); err == nil {
		t.Fatal("queue page limit must fail")
	}
}
