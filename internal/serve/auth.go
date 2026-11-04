package serve

import (
	"net/http"
	"time"
)

func (s *Server) user(id string) map[string]any {
	return map[string]any{"id": id, "disabled": false, "externalIds": []any{}, "createdAt": time.Now().UTC().Format(time.RFC3339), "username": nil}
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request, player, session string) {
	writeJSON(w, 200, map[string]any{"idToken": s.mint(player), "sessionToken": session, "expiresIn": int(s.cfg.TokenTTL.Seconds()), "user": s.user(player)})
}

// auth: anonymous sign-in, session refresh and the external-token exchange (every console is
// anonymous here: the network's own account is not Unity's).
func (s *Server) auth() http.Handler {
	mux := http.NewServeMux()
	newPlayer := func(w http.ResponseWriter, r *http.Request) {
		p, sess, err := s.db.NewPlayer(r.Context())
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		s.signIn(w, r, p, sess)
	}
	mux.HandleFunc("POST /v1/authentication/anonymous", newPlayer)
	mux.HandleFunc("POST /v1/authentication/external-token", newPlayer)
	mux.HandleFunc("POST /v1/authentication/session-token", func(w http.ResponseWriter, r *http.Request) {
		tok := str(readJSON(r), "sessionToken")
		if tok == "" {
			tok = r.Header.Get("Authorization")
		}
		p, err := s.db.PlayerBySession(r.Context(), tok)
		if err != nil {
			newPlayer(w, r)
			return
		}
		s.signIn(w, r, p, tok)
	})
	link := s.authed(func(w http.ResponseWriter, r *http.Request, p string) { writeJSON(w, 200, s.user(p)) })
	mux.HandleFunc("POST /v1/authentication/link", link)
	mux.HandleFunc("POST /v1/authentication/unlink", link)
	mux.HandleFunc("GET /v1/users/{id}", s.authed(func(w http.ResponseWriter, r *http.Request, p string) {
		if !s.db.PlayerExists(r.Context(), r.PathValue("id")) {
			writeErr(w, 404, "user not found")
			return
		}
		writeJSON(w, 200, s.user(r.PathValue("id")))
	}))
	return notFound(mux)
}
