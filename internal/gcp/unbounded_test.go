package gcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
	"github.com/lmctl-ai/lmcloud/hostkit/fakeadapter"
)

type noLimitStore struct {
	*fakeadapter.Fake
	t     *testing.T
	calls int
}

func (s *noLimitStore) GetObject(ctx context.Context, b, k string, limit int64) ([]byte, error) {
	if _, has := ctx.Deadline(); has {
		s.t.Error("cloud read acquired an implicit operation deadline")
	}
	if limit != 0 {
		s.t.Errorf("cloud read acquired an implicit %d-byte bound", limit)
	}
	return []byte(strings.Repeat("x", 16*1024*1024+1)), nil
}
func (s *noLimitStore) ListBuckets(ctx context.Context, token string) ([]string, string, error) {
	if _, has := ctx.Deadline(); has && s.calls == 0 {
		s.t.Error("cloud list acquired an implicit deadline")
	}
	s.calls++
	next := ""
	if s.calls <= 1000 {
		next = strconv.Itoa(s.calls)
	}
	return []string{strconv.Itoa(s.calls)}, next, nil
}
func TestCloudReadDefaultsAreUnbounded(t *testing.T) {
	s := &noLimitStore{Fake: fakeadapter.New(fakeadapter.Options{}), t: t}
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: s, Endpoints: &hostkit.EndpointResolver{Getenv: func(string) string { return "" }}})
	got, err := h.ObjectGet(false, "valid-bucket", "key")
	if err != nil || len(got) != 16*1024*1024+1 {
		t.Fatalf("cloud read incomplete: bytes=%d error=%v", len(got), err)
	}
}

func TestCloudPaginationDefaultsAreUnbounded(t *testing.T) {
	s := &noLimitStore{Fake: fakeadapter.New(fakeadapter.Options{}), t: t}
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: s, Endpoints: &hostkit.EndpointResolver{Getenv: func(string) string { return "" }}})
	names, err := h.BucketList(false)
	if err != nil || len(names) != 1001 {
		t.Fatalf("cloud list incomplete: count=%d error=%v", len(names), err)
	}
}
func TestCloudHTTPReadDefaultsAreUnbounded(t *testing.T) {
	payload := strings.Repeat("x", 16*1024*1024+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(payload)) }))
	defer srv.Close()
	client := hostkit.EnforceClient(&http.Client{})
	if client.Timeout != 0 {
		t.Errorf("default client deadline %v", client.Timeout)
	}
	if h := NewHost(); h.client.Timeout != 0 {
		t.Errorf("direct GCP cloud-host deadline %v", h.client.Timeout)
	}
	a := NewAdapter()
	if a.client.Timeout != 0 {
		t.Errorf("adapter default deadline %v", a.client.Timeout)
	}
	ctx := hostkit.WithEndpoint(context.Background(), srv.URL)
	got, err := a.GetObject(ctx, "valid-bucket", "key", 0)
	if err != nil || string(got) != payload {
		t.Fatalf("adapter read incomplete: bytes=%d error=%v", len(got), err)
	}
}

type noDeadlineQueue struct {
	*fakeadapter.Fake
	t *testing.T
}

func (q *noDeadlineQueue) Receive(ctx context.Context, _ string, _, _, _ int) ([]hostkit.RawMessage, error) {
	if _, has := ctx.Deadline(); has {
		q.t.Error("long-poll wait became a transport deadline")
	}
	return nil, nil
}
func TestCloudReceiveWaitIsNotTransportDeadline(t *testing.T) {
	q := &noDeadlineQueue{Fake: fakeadapter.New(fakeadapter.Options{LongPoll: true}), t: t}
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Queue: q, Endpoints: &hostkit.EndpointResolver{Getenv: func(string) string { return "" }}})
	if _, err := h.QueueReceive(false, "wait-q", 1, 1, 30); err != nil {
		t.Fatal(err)
	}
}
