package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The tap forwards the call untouched and records only its shape: the
// upstream sees the same method, path, query and body, and no identifier
// reaches the log line.
func TestForwardTransparentShapeOnly(t *testing.T) {
	var seenMethod, seenURI, seenBody, seenAuth string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenURI = r.URL.RequestURI()
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(201)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer up.Close()
	upstreamOverride = strings.TrimPrefix(up.URL, "http://")
	defer func() { upstreamOverride = "" }()

	client := &http.Client{Timeout: 5 * time.Second}
	req := httptest.NewRequest("POST", "https://player-auth.services.api.unity.com/v1/auth/anon?projectId=pid", strings.NewReader("secret-body"))
	req.Host = "player-auth.services.api.unity.com"
	req.Header.Set("Authorization", "Bearer tok_secret")
	rec := httptest.NewRecorder()
	forward(client, "http", rec, req)

	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 || string(body) != `{"ok":true}` {
		t.Fatalf("not transparent: %d %q", resp.StatusCode, body)
	}
	if resp.Header.Get("X-Upstream") != "yes" {
		t.Fatal("upstream headers not passed back")
	}
	if seenMethod != "POST" || seenURI != "/v1/auth/anon?projectId=pid" || seenBody != "secret-body" || seenAuth != "Bearer tok_secret" {
		t.Fatalf("upstream saw a changed call: %s %s %q %q", seenMethod, seenURI, seenBody, seenAuth)
	}
}

func TestUnknownHostRefused(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !knownHosts[strings.ToLower(r.Host)] {
			http.Error(w, "unknown host", http.StatusNotFound)
			return
		}
	})
	req := httptest.NewRequest("GET", "https://evil.example/", nil)
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown host answered %d", rec.Code)
	}
}
