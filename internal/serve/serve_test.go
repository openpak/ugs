package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"openpak/ugs/internal/store"
)

// Needs a Postgres: UGS_TEST_DATABASE_URL (the tests create and use schema ugs).
func testServer(t *testing.T, relay string) *Server {
	dsn := os.Getenv("UGS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set UGS_TEST_DATABASE_URL to run against Postgres")
	}
	db, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	s, err := New(context.Background(), db, Config{RelayHost: relay, RelayPort: 9000})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func call(t *testing.T, s *Server, host, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rdr bytes.Buffer
	if body != nil {
		json.NewEncoder(&rdr).Encode(body)
	}
	r := httptest.NewRequest(method, path, &rdr)
	r.Host = host
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func signIn(t *testing.T, s *Server) (string, string) {
	code, out := call(t, s, HostAuth, "POST", "/v1/authentication/anonymous", "", nil)
	if code != 200 {
		t.Fatalf("sign in: %d", code)
	}
	return out["idToken"].(string), out["user"].(map[string]any)["id"].(string)
}

func TestAuthLobbyRelayMatch(t *testing.T) {
	s := testServer(t, "203.0.113.9")
	a, aid := signIn(t, s)
	b, bid := signIn(t, s)
	if code, _ := call(t, s, HostLobby, "POST", "/v1/lobbies/query", "", map[string]any{}); code != 401 {
		t.Fatalf("no token: %d", code)
	}
	code, lob := call(t, s, HostLobby, "POST", "/v1/lobbies", a, map[string]any{"name": "gb-" + aid[:6], "maxPlayers": 4})
	if code != 201 || lob["hostId"] != aid {
		t.Fatalf("create: %d %v", code, lob)
	}
	id := lob["id"].(string)
	if code, out := call(t, s, HostLobby, "POST", "/lobby/v1/lobbies/"+id+"/join", b, map[string]any{}); code != 200 || out["availableSlots"].(float64) != 2 {
		t.Fatalf("join: %d %v", code, out)
	}
	code, q := call(t, s, HostLobby, "POST", "/v1/lobbies/query", b, map[string]any{"count": 100})
	found := false
	for _, x := range q["results"].([]any) {
		found = found || x.(map[string]any)["id"] == id
	}
	if code != 200 || !found {
		t.Fatalf("query: %d", code)
	}
	if code, _ := call(t, s, HostLobby, "DELETE", "/v1/lobbies/"+id+"/players/"+bid, b, nil); code != 204 {
		t.Fatalf("leave: %d", code)
	}
	if code, _ := call(t, s, HostLobby, "DELETE", "/v1/lobbies/"+id, b, nil); code != 403 {
		t.Fatalf("non-host delete: %d", code)
	}
	call(t, s, HostLobby, "DELETE", "/v1/lobbies/"+id, a, nil)

	code, al := call(t, s, HostRelay, "POST", "/v1/allocations", a, map[string]any{"maxConnections": 4})
	if code != 201 {
		t.Fatalf("allocate: %d %v", code, al)
	}
	allocID := al["id"].(string)
	_, jc := call(t, s, HostRelay, "POST", "/v1/join-codes", a, map[string]any{"allocationId": allocID})
	code, j := call(t, s, HostRelay, "POST", "/v1/join", b, map[string]any{"joinCode": strings.ToLower(jc["joinCode"].(string))})
	if code != 200 || j["allocation"].(map[string]any)["allocationId"] != allocID {
		t.Fatalf("join relay: %d %v", code, j)
	}

	_, ta := call(t, s, HostMatchmaker, "POST", "/v2/tickets", a, map[string]any{"queueName": "q-" + aid})
	_, tb := call(t, s, HostMatchmaker, "POST", "/v2/tickets", b, map[string]any{"queueName": "q-" + aid})
	_, pa := call(t, s, HostMatchmaker, "GET", "/v2/tickets/"+ta["id"].(string), a, nil)
	_, pb := call(t, s, HostMatchmaker, "GET", "/v2/tickets/"+tb["id"].(string), b, nil)
	asg := func(m map[string]any) map[string]any { return m["assignment"].(map[string]any) }
	if asg(pa)["status"] != "Found" || asg(pb)["matchId"] != asg(pa)["matchId"] || asg(pa)["connection"] != "203.0.113.9:9000" {
		t.Fatalf("match: %v %v", pa, pb)
	}
}

func TestNoRelayNoMatch(t *testing.T) {
	s := testServer(t, "")
	a, aid := signIn(t, s)
	if code, _ := call(t, s, HostRelay, "POST", "/v1/allocations", a, map[string]any{}); code != 503 {
		t.Fatalf("allocate without relay: %d", code)
	}
	_, tk := call(t, s, HostMatchmaker, "POST", "/v2/tickets", a, map[string]any{"queueName": "x-" + aid})
	_, p := call(t, s, HostMatchmaker, "GET", "/v2/tickets/"+tk["id"].(string), a, nil)
	if p["assignment"].(map[string]any)["status"] != "InProgress" {
		t.Fatalf("%v", p)
	}
}

func TestUnserved(t *testing.T) {
	s := testServer(t, "")
	for _, h := range []string{HostMultiplay, "example.com"} {
		if code, _ := call(t, s, h, "GET", "/v1/anything", "", nil); code != 404 {
			t.Fatalf("%s: %d", h, code)
		}
	}
	if code, _ := call(t, s, HostWire, "POST", "/x", "", nil); code != 204 {
		t.Fatalf("wire: %d", code)
	}
	if _, err := s.player(httptest.NewRequest("GET", "/", nil)); err == nil {
		t.Fatal("empty token accepted")
	}
}
