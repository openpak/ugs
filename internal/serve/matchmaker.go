package serve

import (
	"encoding/json"
	"fmt"
	"net/http"

	"openpak/ugs/internal/store"
)

// Players matched together, and how long a lone ticket waits before it is matched anyway.
const (
	matchMin  = 2
	matchMax  = 8
	matchWait = 20
)

// matchmaker: tickets in a queue are grouped into a match; a ticket polled after its match formed
// carries the connection (the relay endpoint). Without a relay no match is issued, the ticket
// stays in progress.
func (s *Server) matchmaker() http.Handler {
	mux := http.NewServeMux()
	for _, v := range []string{"v1", "v2"} {
		pre := "/" + v
		mux.HandleFunc("POST "+pre+"/tickets", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
			in := readJSON(r)
			queue := str(in, "queueName")
			if queue == "" {
				queue = "default"
			}
			body, _ := json.Marshal(in)
			t := store.Ticket{ID: store.ID(16), Player: p, Queue: queue, Body: body}
			if err := s.db.PutTicket(r.Context(), t); err != nil {
				writeErr(w, 500, "internal error")
				return
			}
			writeJSON(w, 200, map[string]any{"id": t.ID})
		}))
		mux.HandleFunc("GET "+pre+"/tickets/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
			t, err := s.db.GetTicket(r.Context(), r.PathValue("id"))
			if err != nil || t.Player != p {
				writeErr(w, 404, "ticket not found")
				return
			}
			match := t.MatchID
			if match == "" && s.cfg.RelayHost != "" {
				match, err = s.db.Match(r.Context(), t.ID, matchMin, matchMax, matchWait)
				if err != nil {
					writeErr(w, 500, "internal error")
					return
				}
			}
			if match == "" {
				writeJSON(w, 200, map[string]any{"id": t.ID, "assignment": map[string]any{"status": "InProgress"}})
				return
			}
			writeJSON(w, 200, map[string]any{"id": t.ID, "assignment": map[string]any{"assignmentType": "CONNECTION", "status": "Found",
				"connection": fmt.Sprintf("%s:%d", s.cfg.RelayHost, s.cfg.RelayPort), "matchId": match}})
		}))
		mux.HandleFunc("DELETE "+pre+"/tickets/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
			if t, err := s.db.GetTicket(r.Context(), r.PathValue("id")); err == nil && t.Player == p {
				_ = s.db.DeleteTicket(r.Context(), t.ID)
			}
			w.WriteHeader(204)
		}))
	}
	return notFound(mux)
}
