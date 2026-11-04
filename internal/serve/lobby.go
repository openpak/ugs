package serve

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"openpak/ugs/internal/store"
)

// A lobby that is not heartbeaten for this long is gone (Unity's own rule is 30 s).
const lobbyTTL = 60

func lobbyCode() string {
	const a = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = a[int(b[i])%len(a)]
	}
	return string(b)
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func (s *Server) loadLobby(r *http.Request, id string) (store.Doc, map[string]any, bool) {
	d, err := s.db.GetLobby(r.Context(), id)
	if err != nil {
		return d, nil, false
	}
	m := map[string]any{}
	if json.Unmarshal(d.Body, &m) != nil {
		return d, nil, false
	}
	return d, m, true
}

func (s *Server) saveLobby(r *http.Request, d store.Doc, m map[string]any) error {
	players, _ := m["players"].([]any)
	m["availableSlots"] = int(m["maxPlayers"].(float64)) - len(players)
	m["lastUpdated"] = now()
	m["version"] = int(m["version"].(float64)) + 1
	d.Body, _ = json.Marshal(m)
	d.Private, _ = m["isPrivate"].(bool)
	return s.db.PutLobby(r.Context(), d)
}

func playerObj(id string, in map[string]any) map[string]any {
	p := map[string]any{"id": id, "joined": now(), "lastUpdated": now(), "data": map[string]any{}}
	for _, k := range []string{"data", "allocationId", "connectionInfo"} {
		if v, ok := in[k]; ok {
			p[k] = v
		}
	}
	return p
}

// view hides what only the host may see: the lobby code of a private lobby is the host's to share.
func view(m map[string]any, caller string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	if out["hostId"] != caller && out["isPrivate"] == true {
		delete(out, "lobbyCode")
	}
	return out
}

func (s *Server) lobby() http.Handler {
	mux := http.NewServeMux()
	// Both the bare and the /lobby-prefixed form of the documented base path are accepted.
	for _, pre := range []string{"/v1", "/lobby/v1"} {
		s.lobbyRoutes(mux, pre)
	}
	return notFound(mux)
}

func (s *Server) lobbyRoutes(mux *http.ServeMux, pre string) {
	mux.HandleFunc("POST "+pre+"/lobbies", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		in := readJSON(r)
		max := num(in, "maxPlayers", 0)
		if max < 1 || max > 100 {
			writeErr(w, 422, "maxPlayers must be 1 to 100")
			return
		}
		priv, _ := in["isPrivate"].(bool)
		locked, _ := in["isLocked"].(bool)
		pl, _ := in["player"].(map[string]any)
		id := store.ID(8)
		code := lobbyCode()
		m := map[string]any{"id": id, "lobbyCode": code, "name": str(in, "name"), "maxPlayers": float64(max), "isPrivate": priv, "isLocked": locked,
			"hasPassword": false, "hostId": p, "players": []any{playerObj(p, pl)}, "data": in["data"], "created": now(), "version": float64(0)}
		if m["data"] == nil {
			m["data"] = map[string]any{}
		}
		if err := s.saveLobby(r, store.Doc{ID: id, Code: code, Owner: p}, m); err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 201, m)
	}))
	mux.HandleFunc("GET "+pre+"/lobbies/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		_, m, ok := s.loadLobby(r, r.PathValue("id"))
		if !ok {
			writeErr(w, 404, "lobby not found")
			return
		}
		writeJSON(w, 200, view(m, p))
	}))
	mux.HandleFunc("PUT "+pre+"/lobbies/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		d, m, ok := s.loadLobby(r, r.PathValue("id"))
		if !ok {
			writeErr(w, 404, "lobby not found")
			return
		}
		if m["hostId"] != p {
			writeErr(w, 403, "only the host may update the lobby")
			return
		}
		in := readJSON(r)
		for _, k := range []string{"name", "isPrivate", "isLocked", "data"} {
			if v, ok := in[k]; ok {
				m[k] = v
			}
		}
		if v := num(in, "maxPlayers", 0); v > 0 {
			m["maxPlayers"] = float64(v)
		}
		if v := str(in, "hostId"); v != "" {
			m["hostId"], d.Owner = v, v
		}
		if s.saveLobby(r, d, m) != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, m)
	}))
	mux.HandleFunc("DELETE "+pre+"/lobbies/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		_, m, ok := s.loadLobby(r, r.PathValue("id"))
		if ok && m["hostId"] != p {
			writeErr(w, 403, "only the host may delete the lobby")
			return
		}
		_ = s.db.DeleteLobby(r.Context(), r.PathValue("id"))
		w.WriteHeader(204)
	}))
	mux.HandleFunc("POST "+pre+"/lobbies/{id}/heartbeat", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		if s.db.Touch(r.Context(), r.PathValue("id")) != nil {
			writeErr(w, 404, "lobby not found")
			return
		}
		w.WriteHeader(204)
	}))
	join := func(w http.ResponseWriter, r *http.Request, p string, d store.Doc, m map[string]any) {
		in := readJSON(r)
		players, _ := m["players"].([]any)
		for _, x := range players {
			if x.(map[string]any)["id"] == p {
				writeJSON(w, 200, view(m, p))
				return
			}
		}
		if locked, _ := m["isLocked"].(bool); locked || len(players) >= int(m["maxPlayers"].(float64)) {
			writeErr(w, 409, "lobby is full or locked")
			return
		}
		pl, _ := in["player"].(map[string]any)
		m["players"] = append(players, playerObj(p, pl))
		if s.saveLobby(r, d, m) != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, view(m, p))
	}
	mux.HandleFunc("POST "+pre+"/lobbies/{id}/join", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		d, m, ok := s.loadLobby(r, r.PathValue("id"))
		if !ok {
			writeErr(w, 404, "lobby not found")
			return
		}
		join(w, r, p, d, m)
	}))
	mux.HandleFunc("POST "+pre+"/lobbies/join", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		in := readJSON(r)
		var d store.Doc
		var err error
		if id := str(in, "lobbyId"); id != "" {
			d, err = s.db.GetLobby(r.Context(), id)
		} else {
			d, err = s.db.LobbyByCode(r.Context(), strings.ToUpper(str(in, "lobbyCode")))
		}
		m := map[string]any{}
		if err != nil || json.Unmarshal(d.Body, &m) != nil {
			writeErr(w, 404, "lobby not found")
			return
		}
		join(w, r, p, d, m)
	}))
	mux.HandleFunc("POST "+pre+"/lobbies/quickjoin", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		docs, _ := s.db.PublicLobbies(r.Context(), time.Now().Unix()-lobbyTTL, 50)
		for _, d := range docs {
			m := map[string]any{}
			if json.Unmarshal(d.Body, &m) != nil || m["isLocked"] == true || m["availableSlots"].(float64) < 1 {
				continue
			}
			join(w, r, p, d, m)
			return
		}
		writeErr(w, 404, "no lobby to join")
	}))
	mux.HandleFunc("POST "+pre+"/lobbies/query", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		in := readJSON(r)
		count := num(in, "count", 20)
		if count < 1 || count > 100 {
			count = 20
		}
		docs, err := s.db.PublicLobbies(r.Context(), time.Now().Unix()-lobbyTTL, 200)
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		filters, _ := in["filter"].([]any)
		if filters == nil {
			filters, _ = in["filters"].([]any)
		}
		out := []any{}
		for _, d := range docs {
			m := map[string]any{}
			if json.Unmarshal(d.Body, &m) != nil || !matches(m, filters) {
				continue
			}
			delete(m, "lobbyCode")
			out = append(out, m)
			if len(out) >= count {
				break
			}
		}
		writeJSON(w, 200, map[string]any{"results": out})
	}))
	mux.HandleFunc("DELETE "+pre+"/lobbies/{id}/players/{pid}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		d, m, ok := s.loadLobby(r, r.PathValue("id"))
		if !ok {
			w.WriteHeader(204)
			return
		}
		pid := r.PathValue("pid")
		if pid != p && m["hostId"] != p {
			writeErr(w, 403, "not allowed")
			return
		}
		var keep []any
		for _, x := range m["players"].([]any) {
			if x.(map[string]any)["id"] != pid {
				keep = append(keep, x)
			}
		}
		if len(keep) == 0 {
			_ = s.db.DeleteLobby(r.Context(), d.ID)
		} else {
			m["players"] = keep
			if m["hostId"] == pid { // the host left: the longest-standing player hosts
				h := keep[0].(map[string]any)["id"]
				m["hostId"], d.Owner = h, h.(string)
			}
			_ = s.saveLobby(r, d, m)
		}
		w.WriteHeader(204)
	}))
	mux.HandleFunc("PUT "+pre+"/lobbies/{id}/players/{pid}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		d, m, ok := s.loadLobby(r, r.PathValue("id"))
		if !ok {
			writeErr(w, 404, "lobby not found")
			return
		}
		pid := r.PathValue("pid")
		if pid != p {
			writeErr(w, 403, "not allowed")
			return
		}
		in := readJSON(r)
		for _, x := range m["players"].([]any) {
			if pl := x.(map[string]any); pl["id"] == pid {
				for _, k := range []string{"data", "allocationId", "connectionInfo"} {
					if v, ok := in[k]; ok {
						pl[k] = v
					}
				}
				pl["lastUpdated"] = now()
			}
		}
		if s.saveLobby(r, d, m) != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, view(m, p))
	}))
}

// matches applies Unity's query filters to the lobby fields that can be filtered on: the
// available slots, the slot count and the indexed data entries (S1-S5, N1-N5).
func matches(m map[string]any, filters []any) bool {
	for _, f := range filters {
		fm, _ := f.(map[string]any)
		field, op, val := str(fm, "field"), str(fm, "op"), str(fm, "value")
		var have string
		switch field {
		case "AvailableSlots", "0":
			have = fmt.Sprint(m["availableSlots"])
		case "MaxPlayers":
			have = fmt.Sprint(m["maxPlayers"])
		default:
			data, _ := m["data"].(map[string]any)
			for _, e := range data {
				if em, ok := e.(map[string]any); ok && em["index"] == field {
					have = fmt.Sprint(em["value"])
				}
			}
		}
		var a, b float64
		_, ea := fmt.Sscan(have, &a)
		_, eb := fmt.Sscan(val, &b)
		numeric := ea == nil && eb == nil
		ok := true
		switch strings.ToUpper(op) {
		case "EQ", "0":
			ok = have == val
		case "NE", "1":
			ok = have != val
		case "GT", "2":
			ok = numeric && a > b
		case "GE", "3":
			ok = numeric && a >= b
		case "LT", "4":
			ok = numeric && a < b
		case "LE", "5":
			ok = numeric && a <= b
		case "CONTAINS", "6":
			ok = strings.Contains(strings.ToLower(have), strings.ToLower(val))
		}
		if !ok {
			return false
		}
	}
	return true
}
