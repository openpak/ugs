package tap

import (
	"strings"
	"testing"
)

func TestTagStableUniqueAndHiding(t *testing.T) {
	values := []string{"player-id-abc123", "tok_secret_value", "Some Nickname"}
	for _, v := range values {
		if Tag(v) != Tag(v) {
			t.Fatalf("tag for %q is not stable", v)
		}
		line := "ugs client=" + Tag(v)
		if strings.Contains(line, v) {
			t.Fatalf("%q leaked into %q", v, line)
		}
	}
	if Tag(values[0]) == Tag(values[1]) {
		t.Fatal("two values share a tag")
	}
}

func TestShapeKeepsWordsTagsIDs(t *testing.T) {
	shape := Shape("/v1/players/6f95ce9e-3a91-40ce-b11e-49c0864676a9/lobbies")
	if !strings.HasPrefix(shape, "/v1/players/~") || !strings.HasSuffix(shape, "/lobbies") {
		t.Fatalf("identifier not tagged: %q", shape)
	}
	if strings.Contains(shape, "6f95ce9e") {
		t.Fatalf("identifier leaked: %q", shape)
	}
	plain := Shape("/v1/players/me/lobbies")
	if plain != "/v1/players/me/lobbies" {
		t.Fatalf("API words rewritten: %q", plain)
	}
}

func TestQueryKeysDropValues(t *testing.T) {
	keys := QueryKeys("projectId=secret-proj&limit=20")
	line := strings.Join(keys, ",")
	if line != "limit,projectId" {
		t.Fatalf("unexpected keys %q", line)
	}
	if strings.Contains(line, "secret-proj") {
		t.Fatalf("query value leaked: %q", line)
	}
}

func TestRecordLineCarriesNoSubstance(t *testing.T) {
	secret := "tok_secret_value"
	r := Record{Method: "POST", Host: "player-auth.services.api.unity.com", Path: Shape("/v1/auth/" + secret), QueryKeys: QueryKeys("id=" + secret), ReqBytes: 412, Status: 200, RespBytes: 1830, Millis: 91, ClientTag: Tag("10.0.0.9")}
	line := r.Line()
	for _, leaked := range []string{secret, "10.0.0.9"} {
		if strings.Contains(line, leaked) {
			t.Fatalf("%q leaked into %q", leaked, line)
		}
	}
}
