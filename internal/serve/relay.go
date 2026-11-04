package serve

import (
	"crypto/rand"
	"net/http"
	"strings"

	"openpak/ugs/internal/store"
)

func rnd(n int) []byte { b := make([]byte, n); rand.Read(b); return b }

func joinCode() string {
	const a = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := rnd(6)
	for i := range b {
		b[i] = a[int(b[i])%len(a)]
	}
	return string(b)
}

func (s *Server) endpoints() []any {
	var out []any
	for _, t := range []string{"udp", "dtls"} {
		out = append(out, map[string]any{"connectionType": t, "host": s.cfg.RelayHost, "port": s.cfg.RelayPort, "network": "udp", "secure": t == "dtls"})
	}
	return out
}

// allocation is the allocation body for a client; key and connection data are opaque blobs the
// relay transport defines, issued fresh per call.
func (s *Server) allocation(a store.Allocation) map[string]any {
	idb, _ := hexBytes(a.ID)
	return map[string]any{"allocationId": a.ID, "allocationIdBytes": b64(idb), "region": a.Region,
		"serverEndpoints": s.endpoints(), "relayServer": map[string]any{"ipV4": s.cfg.RelayHost, "port": s.cfg.RelayPort},
		"key": b64(rnd(32)), "connectionData": b64(rnd(16)), "hostConnectionData": b64(rnd(16)), "maxConnections": a.MaxConnections}
}

func hexBytes(h string) ([]byte, error) {
	out := make([]byte, 0, len(h)/2)
	for i := 0; i+1 < len(h); i += 2 {
		var v byte
		for _, c := range []byte(h[i : i+2]) {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= c - '0'
			case c >= 'a' && c <= 'f':
				v |= c - 'a' + 10
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// reply answers with the fields at the top level and again under "data", so a client reading
// either envelope finds them.
func reply(w http.ResponseWriter, code int, m map[string]any) {
	out := map[string]any{"data": map[string]any{}}
	for k, v := range m {
		out[k] = v
	}
	out["data"] = m
	writeJSON(w, code, out)
}

func (s *Server) noRelay(w http.ResponseWriter) bool {
	if s.cfg.RelayHost == "" {
		writeErr(w, 503, "relay not available")
		return true
	}
	return false
}

// relay serves allocations and join codes. The relay transport itself (UDP) is not part of this
// service yet; without UGS_RELAY_HOST nothing is allocated.
func (s *Server) relay() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/allocations", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		if s.noRelay(w) {
			return
		}
		in := readJSON(r)
		id := str(in, "allocationId")
		if id == "" {
			id = store.ID(16)
		}
		a := store.Allocation{ID: id, Player: p, JoinCode: joinCode(), Region: s.cfg.Region, MaxConnections: num(in, "maxConnections", 8)}
		if err := s.db.PutAllocation(r.Context(), a); err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		reply(w, 201, map[string]any{"id": a.ID, "allocation": s.allocation(a)})
	}))
	mux.HandleFunc("POST /v1/join-codes", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		a, err := s.db.AllocationByID(r.Context(), str(readJSON(r), "allocationId"))
		if err != nil || a.Player != p {
			writeErr(w, 404, "allocation not found")
			return
		}
		reply(w, 200, map[string]any{"joinCode": a.JoinCode})
	}))
	mux.HandleFunc("POST /v1/join", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		a, err := s.db.AllocationByCode(r.Context(), strings.ToUpper(str(readJSON(r), "joinCode")))
		if err != nil {
			writeErr(w, 404, "join code not found")
			return
		}
		reply(w, 200, map[string]any{"allocation": s.allocation(a)})
	}))
	mux.HandleFunc("GET /v1/regions", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		reply(w, 200, map[string]any{"regions": []any{map[string]any{"id": s.cfg.Region, "description": "OpenPak"}}})
	}))
	return notFound(mux)
}

// qos lists the relay region for latency probing; the probe is answered by the relay transport.
func (s *Server) qos() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/qos-servers", func(w http.ResponseWriter, r *http.Request) {
		if s.noRelay(w) {
			return
		}
		srv := []any{map[string]any{"regionId": s.cfg.Region, "ipv4": s.cfg.RelayHost, "ipv6": nil, "port": s.cfg.RelayPort}}
		reply(w, 200, map[string]any{"servers": srv})
	})
	return notFound(mux)
}
