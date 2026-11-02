package gcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lmctl-ai/lmcloud/hostkit"
)

func TestCreateTokenReaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/storage/v1/b/b1" {
			_, _ = w.Write([]byte(`{"name":"b1","labels":{"lmcloud_create_token":"tok-b"}}`))
			return
		}
		if r.URL.Path == "/v1/projects/gcpsim-local/topics/q1" {
			_, _ = w.Write([]byte(`{"name":"projects/gcpsim-local/topics/q1","labels":{"lmcloud_create_token":"tok-q"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()
	h := NewHost()
	h.ctx = context.Background()
	h.client = hostkit.NewHTTPClient(2e9)
	if _, err := h.SetEndpoint("storage", srv.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetEndpoint("queue", srv.URL); err != nil {
		t.Fatal(err)
	}
	if got, err := h.BucketCreateToken("b1"); err != nil || got != "tok-b" {
		t.Fatalf("bucket token %q %v", got, err)
	}
	if got, err := h.QueueCreateToken("q1"); err != nil || got != "tok-q" {
		t.Fatalf("queue token %q %v", got, err)
	}
}
