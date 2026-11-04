// Package serve answers the Unity Gaming Services hostnames Gang Beasts dials,
// from OpenPak's own state. Nothing is forwarded to Unity: a console that
// reaches this service never reaches Unity's cloud.
//
// The shapes follow Unity's public REST documentation (docs.unity.com) and have
// not yet been checked against a capture of the game; anything the game asks
// for that is not served answers 404 and is logged by shape (method, host,
// path shape, query key names) so the next run records what is missing.
package serve

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"openpak/ugs/internal/store"
	"openpak/ugs/tap"
)

// Hosts are the Unity hostnames this service answers.
const (
	HostAuth       = "player-auth.services.api.unity.com"
	HostWire       = "wire.unity3d.com"
	HostLobby      = "lobby.services.api.unity.com"
	HostRelay      = "relay-allocations.services.api.unity.com"
	HostQoS        = "qos-discovery.services.api.unity.com"
	HostMultiplay  = "multiplay.services.api.unity.com"
	HostMatchmaker = "matchmaker.services.api.unity.com"
)

// Config is what the service needs besides its store.
type Config struct {
	// RelayHost and RelayPort name the relay transport endpoint handed to clients. Empty RelayHost
	// means no relay runs: allocations and matches are not issued.
	RelayHost string
	RelayPort int
	Region    string
	TokenTTL  time.Duration
}

type Server struct {
	cfg   Config
	db    *store.Store
	key   []byte
	hosts map[string]http.Handler
}

// New builds the service; the signing key is read from (or created in) the store.
func New(ctx context.Context, db *store.Store, cfg Config) (*Server, error) {
	key, err := db.SigningKey(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		cfg.Region = "openpak"
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = time.Hour
	}
	s := &Server{cfg: cfg, db: db, key: key}
	s.hosts = map[string]http.Handler{
		HostAuth: s.auth(), HostLobby: s.lobby(), HostRelay: s.relay(), HostQoS: s.qos(),
		HostMatchmaker: s.matchmaker(), HostWire: s.wire(), HostMultiplay: s.multiplay(),
	}
	return s, nil
}

// ServeHTTP dispatches on the Host header (Traefik keeps the name the game dialled) and logs
// one line per request: host, method, path shape, status and milliseconds. Never a query value,
// header, token or body.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	sw := &statusWriter{ResponseWriter: w}
	h, ok := s.hosts[host]
	if !ok {
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "unknown host") })
	}
	r.Body = http.MaxBytesReader(sw, r.Body, 1<<20)
	h.ServeHTTP(sw, r)
	if sw.status == 0 {
		sw.status = 200
	}
	msg := "ugs"
	if sw.status == 404 {
		msg = "ugs unserved"
	}
	slog.Info(msg, "host", host, "method", r.Method, "path", tap.Shape(r.URL.Path), "query", strings.Join(tap.QueryKeys(r.URL.RawQuery), ","),
		"status", sw.status, "ms", time.Since(start).Milliseconds())
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) {
	if s.status == 0 {
		s.status = c
	}
	s.ResponseWriter.WriteHeader(c)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = 200
	}
	return s.ResponseWriter.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// writeErr uses Unity's problem-details shape.
func writeErr(w http.ResponseWriter, code int, detail string) {
	writeJSON(w, code, map[string]any{"title": http.StatusText(code), "status": code, "detail": detail})
}

func readJSON(r *http.Request) map[string]any {
	b, _ := io.ReadAll(r.Body)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

func str(m map[string]any, k string) string { v, _ := m[k].(string); return v }

func num(m map[string]any, k string, def int) int {
	if v, ok := m[k].(float64); ok {
		return int(v)
	}
	return def
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// ---- tokens: a compact HS256 JWT carrying the player id ----

func (s *Server) mint(player string) string {
	enc := base64.RawURLEncoding
	head := enc.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now()
	body, _ := json.Marshal(map[string]any{"sub": player, "iss": "https://" + HostAuth, "iat": now.Unix(), "exp": now.Add(s.cfg.TokenTTL).Unix()})
	in := head + "." + enc.EncodeToString(body)
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(in))
	return in + "." + enc.EncodeToString(m.Sum(nil))
}

var errToken = errors.New("bad token")

// player returns the player id behind the Bearer token of r.
func (s *Server) player(r *http.Request) (string, error) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", errToken
	}
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(parts[0] + "." + parts[1]))
	want := base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[2])) {
		return "", errToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", errToken
	}
	var c struct {
		Sub string `json:"sub"`
		Exp int64  `json:"exp"`
	}
	if json.Unmarshal(raw, &c) != nil || c.Sub == "" || c.Exp < time.Now().Unix() {
		return "", errToken
	}
	return c.Sub, nil
}

// authed runs h for a request with a valid token, else answers 401.
func (s *Server) authed(h func(w http.ResponseWriter, r *http.Request, player string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.player(r)
		if err != nil {
			writeErr(w, 401, "missing or invalid access token")
			return
		}
		h(w, r, p)
	}
}

func notFound(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "not found") })
	return mux
}

// ---- wire and multiplay ----

// wire.unity3d.com is Unity's configuration and analytics sink: accept and drop, store nothing.
func (s *Server) wire() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]any{})
			return
		}
		w.WriteHeader(204)
	})
}

// multiplay hosts dedicated game servers; OpenPak runs none, so every call is unserved (404, logged)
// until a capture shows what the game needs from it.
func (s *Server) multiplay() http.Handler { return notFound(http.NewServeMux()) }
