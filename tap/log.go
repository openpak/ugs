// Package tap records the shape of Unity Gaming Services traffic without
// recording its substance. A log line carries the method, the host, the
// path with opaque segments replaced by per-process tags, the query key
// names (never the values), byte counts, the status and the duration.
// Request and response bodies, tokens, identifiers and player data never
// reach a log line; the tests prove it for every constructor here.
package tap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// key is random per process: equal values tag equal within one run so lines
// can be correlated, and the tags cannot be reversed or compared across
// restarts.
var key = func() []byte {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		panic(err)
	}
	return k
}()

// Tag replaces one opaque value with its short per-process tag.
func Tag(v string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(v))
	return "~" + hex.EncodeToString(m.Sum(nil)[:3])
}

// opaque reports whether a path segment looks like an identifier rather
// than an API word: long tokens, mixed letters and digits, UUID shapes, or
// long digit runs. Short words (v1, players, lobby) and small numbers pass
// through untouched.
func opaque(seg string) bool {
	if len(seg) < 8 {
		digits := true
		for _, c := range seg {
			if c < '0' || c > '9' {
				digits = false
			}
		}
		return digits && len(seg) > 4
	}
	letters, digits, underscores := false, false, false
	for _, c := range seg {
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			letters = true
		case c >= '0' && c <= '9':
			digits = true
		case c == '_':
			underscores = true
		case c == '-' || c == '.' || c == '~':
			// identifier punctuation, ignored
		default:
			return false
		}
	}
	// API words are kebab-case or camelCase; generated secrets skew
	// snake_case, mixed-alphanumeric, UUID or long.
	return (letters && digits) || (underscores && len(seg) >= 8) || len(seg) >= 24 || strings.Count(seg, "-") >= 3
}

// Shape returns the path with every opaque segment replaced by its tag,
// so `/v1/players/6f95ce9e/lobbies` logs as `/v1/players/~a1b2c3/lobbies`.
func Shape(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p != "" && opaque(p) {
			parts[i] = Tag(p)
		}
	}
	return strings.Join(parts, "/")
}

// QueryKeys returns the sorted query parameter names of raw, without values.
func QueryKeys(raw string) []string {
	q, err := url.ParseQuery(raw)
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Record is one forwarded call, safe to log as a whole: it holds no body,
// header value, token or identifier.
type Record struct {
	Method     string
	Host       string
	Path       string
	QueryKeys  []string
	ReqBytes   int64
	Status     int
	RespBytes  int64
	Millis     int64
	ClientTag  string
}

// Line renders the record as a single log line.
func (r Record) Line() string {
	return fmt.Sprintf("ugs method=%s host=%s path=%s query=[%s] req=%d status=%d resp=%d ms=%d client=%s",
		r.Method, r.Host, r.Path, strings.Join(r.QueryKeys, ","), r.ReqBytes, r.Status, r.RespBytes, r.Millis, r.ClientTag)
}
