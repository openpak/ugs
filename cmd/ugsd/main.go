// Command ugsd serves every Unity-online title's backend hostnames: plain
// HTTP on the block base (Traefik ends the guest TLS in front, the same
// arrangement as servers/t17), one handler per hostname, health on base+1.
//
// M1 mode is a transparent tap: calls for known Unity hostnames are
// forwarded to the real upstream untouched so the game keeps working, while
// tap records the shape of each call (never bodies, tokens or identifiers).
// Unknown hostnames are refused. Later milestones answer calls here instead
// of forwarding them; the dispatch point is forward().
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"openpak/ugs/tap"
)

// upstreamOverride replaces the upstream authority (tests only: point it at
// a local httptest server). Empty means the request's own host.
var upstreamOverride = ""

// knownHosts is the Unity surface measured on Gang Beasts v1.28.1
// (docs/provenance.md). Hosts only — paths and fields come from the tap.
var knownHosts = map[string]bool{
	"player-auth.services.api.unity.com":      true,
	"wire.unity3d.com":                        true,
	"lobby.services.api.unity.com":            true,
	"relay-allocations.services.api.unity.com": true,
	"qos-discovery.services.api.unity.com":    true,
	"multiplay.services.api.unity.com":        true,
}

type counter struct {
	n int64
}

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	addr := flag.String("addr", ":22680", "HTTP listen address (Traefik terminates guest TLS in front)")
	healthAddr := flag.String("health-addr", ":22681", "health listen address")
	mode := flag.String("mode", "forward", "forward (tap to real upstream) or refuse (404 everything)")
	upstreamScheme := flag.String("upstream-scheme", "https", "upstream scheme (http only for local tests)")
	timeout := flag.Duration("upstream-timeout", 20*time.Second, "upstream round-trip timeout")
	flag.Parse()

	client := &http.Client{Timeout: *timeout}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if *mode != "forward" || !knownHosts[strings.ToLower(host)] {
			http.Error(w, "unknown host", http.StatusNotFound)
			return
		}
		forward(client, *upstreamScheme, w, r)
	})
	log.Printf("ugsd listening %s health %s mode %s", *addr, *healthAddr, *mode)

	go func() {
		h := http.NewServeMux()
		h.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok"}`)
		})
		if err := http.ListenAndServe(*healthAddr, h); err != nil {
			log.Fatalf("health: %v", err)
		}
	}()
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// forward sends the call to the real upstream and streams the reply back,
// recording only the tap.Record shape of the exchange.
func forward(client *http.Client, scheme string, w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	up, err := http.NewRequest(r.Method, scheme+"://"+upstreamAuthority(host)+r.URL.RequestURI(), r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	for k, vs := range r.Header {
		if isHop(k) {
			continue
		}
		for _, v := range vs {
			up.Header.Add(k, v)
		}
	}
	rec := tap.Record{
		Method:    r.Method,
		Host:      strings.ToLower(host),
		Path:      tap.Shape(r.URL.Path),
		QueryKeys: tap.QueryKeys(r.URL.RawQuery),
		ClientTag: tap.Tag(clientAddr(r)),
	}
	if r.ContentLength >= 0 {
		rec.ReqBytes = r.ContentLength
	}
	resp, err := client.Do(up)
	if err != nil {
		rec.Millis = time.Since(start).Milliseconds()
		rec.Status = 502
		log.Printf("%s upstream_error=%v", rec.Line(), err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		if isHop(k) {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	var body counter
	n, _ := io.Copy(io.MultiWriter(w, &body), resp.Body)
	_ = n
	rec.Status = resp.StatusCode
	rec.RespBytes = body.n
	rec.Millis = time.Since(start).Milliseconds()
	log.Print(rec.Line())
}

func upstreamAuthority(host string) string {
	if upstreamOverride != "" {
		return upstreamOverride
	}
	return host
}

func clientAddr(r *http.Request) string {	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

func isHop(header string) bool {
	switch http.CanonicalHeaderKey(header) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	}
	return false
}
