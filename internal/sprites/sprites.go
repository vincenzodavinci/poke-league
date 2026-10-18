// Package sprites serves the league's embedded, original artwork: a Dex Crest
// per Pokémon, the eight gym sigils and the favicon (see scripts/make_crests.py).
//
// Everything is compiled into the binary: there is no upstream fetch to turn
// into an SSRF and no disk path to traverse. Team Rocket's "PokéMart" lab
// (docs/team-rocket.md) shows what goes wrong when a sprite service proxies
// arbitrary URLs instead.
package sprites

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/vincenzodavinci/poke-league/assets"
)

// Routes registers /sprites/... on mux.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /sprites/pokemon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(trimExt(r.PathValue("id")))
		if err != nil || id < 1 || id > 151 {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "crests/"+strconv.Itoa(id)+".svg")
	})
	mux.HandleFunc("GET /sprites/badges/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(trimExt(r.PathValue("n")))
		if err != nil || n < 1 || n > 8 {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "badges/"+strconv.Itoa(n)+".svg")
	})
	mux.HandleFunc("GET /sprites/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		serve(w, r, "favicon.svg")
	})
}

func trimExt(s string) string {
	return strings.TrimSuffix(strings.TrimSuffix(s, ".svg"), ".png")
}

func serve(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(assets.FS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sum := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "public, max-age=86400")
	// SVG is a document format: if one is ever opened directly, it must not
	// be able to run script or load anything.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Type", "image/svg+xml")
	_, _ = w.Write(b)
}
