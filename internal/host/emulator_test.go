package host

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmulatorEndpoint(t *testing.T) {
	token := "dummy-private-token"
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("missing bearer")
		}
		if r.URL.Path != "/storage/v1/b" {
			t.Error(r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":401,"status":"UNAUTHENTICATED","message":"` + token + `"}}`))
	}))
	defer srv.Close()
	t.Setenv("LMGCP_ENDPOINT", srv.URL)
	t.Setenv("LMGCP_ACCESS_TOKEN", token)
	t.Setenv("LMGCP_LIVE", "")
	h, err := NewHost()
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if h.fakeSrv != nil {
		t.Fatal("emulator started internal fake")
	}
	h.StorageListBuckets()
	if requests != 1 || !strings.Contains(h.AuthError(), "401") || strings.Contains(h.AuthError(), token) {
		t.Fatalf("requests=%d, error=%s", requests, h.AuthError())
	}
	h.clearErr()
	if h.AuthError() == "" {
		t.Fatal("auth failure lost")
	}
}

func TestRejectUnsafeEndpoint(t *testing.T) {
	for _, endpoint := range []string{"https://googleapis.com", "http://user:secret@localhost", "http://localhost/path", "http://localhost?token=secret"} {
		t.Setenv("LMGCP_ENDPOINT", endpoint)
		if _, err := NewHost(); err == nil {
			t.Fatal("accepted unsafe endpoint")
		} else if strings.Contains(err.Error(), "secret") {
			t.Fatal("leaked endpoint")
		}
	}
}
