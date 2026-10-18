// Package sprites serves the embedded artwork, badges and item icons.
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
	"regexp"
	"strconv"

	"github.com/vincenzodavinci/poke-league/assets"
)

var itemName = regexp.MustCompile(`^[a-z-]{1,24}$`)

// Routes registers /sprites/... on mux.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /sprites/pokemon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(trimPNG(r.PathValue("id")))
		if err != nil || id < 1 || id > 151 {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "pokemon/"+strconv.Itoa(id)+".png")
	})
	mux.HandleFunc("GET /sprites/badges/{n}", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(trimPNG(r.PathValue("n")))
		if err != nil || n < 1 || n > 8 {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "badges/"+strconv.Itoa(n)+".png")
	})
	mux.HandleFunc("GET /sprites/items/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := trimPNG(r.PathValue("name"))
		if !itemName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		serve(w, r, "items/"+name+".png")
	})
}

func trimPNG(s string) string {
	if len(s) > 4 && s[len(s)-4:] == ".png" {
		return s[:len(s)-4]
	}
	return s
}

func serve(w http.ResponseWriter, r *http.Request, name string) {
	b, err := fs.ReadFile(assets.FS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sum := sha256.Sum256(b)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(b)
}
