package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEraseEndpoint(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	h := eraseHandler("ugs", eraseNothing)
	call := func(id, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", "/internal/accounts/"+id+"/data", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "")
	if r := call(id, "Bearer k"); r.Code != 503 {
		t.Fatalf("unset key: %d", r.Code)
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "k")
	if r := call(id, ""); r.Code != 401 {
		t.Fatalf("no key: %d", r.Code)
	}
	if r := call(id, "Bearer bad"); r.Code != 401 {
		t.Fatalf("wrong key: %d", r.Code)
	}
	if r := call("nope", "Bearer k"); r.Code != 400 {
		t.Fatalf("bad id: %d", r.Code)
	}
	for i := 0; i < 2; i++ { // idempotent
		if r := call(id, "Bearer k"); r.Code != 200 || !strings.Contains(r.Body.String(), `"service":"ugs","rows":0`) {
			t.Fatalf("ok: %d %s", r.Code, r.Body.String())
		}
	}
}
