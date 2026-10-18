// Package dex serves the FireRed-accurate Kanto Pokédex.
package dex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Pokemon is one Pokédex entry: names, Gen 3 types and base stats (facts only;
// no copyrighted game text).
type Pokemon struct {
	ID     int            `json:"id"`
	Name   string         `json:"name"`
	Types  []string       `json:"types"`
	Stats  map[string]int `json:"stats"`
	Height int            `json:"height"`
	Weight int            `json:"weight"`
}

// Store is an immutable, in-memory dex.
type Store struct {
	byID  map[int]Pokemon
	all   []Pokemon
	types []string
}

// Load parses the embedded kanto.json.
func Load(raw []byte) (*Store, error) {
	var all []Pokemon
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, fmt.Errorf("parse dex: %w", err)
	}
	s := &Store{byID: make(map[int]Pokemon, len(all)), all: all}
	seen := map[string]bool{}
	for _, p := range all {
		s.byID[p.ID] = p
		for _, t := range p.Types {
			if !seen[t] {
				seen[t] = true
				s.types = append(s.types, t)
			}
		}
	}
	slices.Sort(s.types)
	return s, nil
}

// Search returns entries whose name contains q and that have type typ.
// Empty arguments match everything.
func (s *Store) Search(q, typ string) []Pokemon {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []Pokemon{}
	for _, p := range s.all {
		if q != "" && !strings.Contains(p.Name, q) {
			continue
		}
		if typ != "" && !slices.Contains(p.Types, typ) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// Get returns the entry with the given national dex number.
func (s *Store) Get(id int) (Pokemon, bool) {
	p, ok := s.byID[id]
	return p, ok
}

// Routes registers the dex API on mux.
func (s *Store) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/pokemon", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Bound user input before it touches anything.
		if len(q.Get("q")) > 32 || len(q.Get("type")) > 16 {
			http.Error(w, "query too long", http.StatusBadRequest)
			return
		}
		writeJSON(w, s.Search(q.Get("q"), q.Get("type")))
	})
	mux.HandleFunc("GET /api/pokemon/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "id must be a number", http.StatusBadRequest)
			return
		}
		p, ok := s.Get(id)
		if !ok {
			http.Error(w, "no such Pokémon in the Kanto dex", http.StatusNotFound)
			return
		}
		writeJSON(w, p)
	})
	mux.HandleFunc("GET /api/types", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.types)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(v)
}
