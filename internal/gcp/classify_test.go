package gcp

import "testing"

func TestClassifySharedCodes(t *testing.T) {
	h := NewHost()
	cases := []struct {
		status int
		body   string
		want   string
	}{
		{400, `{"error":{"code":400,"message":"bad","errors":[{"reason":"invalid"}]}}`, CodeInvalidArgument},
		{401, `{"error":{"code":401,"message":"auth"}}`, CodeUnauthenticated},
		{403, `{"error":{"code":403,"message":"denied"}}`, CodePermissionDenied},
		{404, `{"error":{"code":404,"message":"missing","errors":[{"reason":"notFound"}]}}`, CodeNotFound},
		{409, `{"error":{"code":409,"message":"exists"}}`, CodeConflict},
		{412, `{"error":{"code":412,"message":"precondition"}}`, CodePreconditionFailed},
		{429, `{"error":{"code":429,"message":"slow"}}`, CodeInternal},
		{503, `{"error":{"code":503,"message":"down"}}`, CodeInternal},
	}
	for _, c := range cases {
		if got := h.classify(c.status, []byte(c.body)).Code; got != c.want {
			t.Errorf("status %d: got %s want %s", c.status, got, c.want)
		}
	}
}
