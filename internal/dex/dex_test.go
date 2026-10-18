package dex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vincenzodavinci/poke-league/data"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, err := Load(data.KantoJSON)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestKantoDexIsComplete(t *testing.T) {
	s, err := Load(data.KantoJSON)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(s.Search("", "")); got != 151 {
		t.Fatalf("want 151 Kanto Pokémon, got %d", got)
	}
}

// FireRed predates the Fairy type, so the dex must use Gen 3 typings.
func TestGen3Types(t *testing.T) {
	s, _ := Load(data.KantoJSON)
	cases := map[int]string{35: "normal", 122: "psychic", 81: "electric,steel"}
	for id, want := range cases {
		p, _ := s.Get(id)
		if got := strings.Join(p.Types, ","); got != want {
			t.Errorf("#%d %s: want %s, got %s", id, p.Name, want, got)
		}
	}
	for _, typ := range s.types {
		if typ == "fairy" {
			t.Fatal("fairy type leaked into a Gen 3 dex")
		}
	}
}

func TestAPI(t *testing.T) {
	srv := newServer(t)
	cases := []struct {
		path   string
		status int
	}{
		{"/api/pokemon/137", 200},
		{"/api/pokemon/0", 404},
		{"/api/pokemon/152", 404},
		{"/api/pokemon/../../etc/passwd", 404},
		{"/api/pokemon/abc", 400},
		{"/api/pokemon?type=ghost", 200},
		{"/api/pokemon?q=" + strings.Repeat("a", 33), 400},
	}
	for _, c := range cases {
		res, err := http.Get(srv.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != c.status {
			t.Errorf("%s: want %d, got %d", c.path, c.status, res.StatusCode)
		}
	}
}

func TestSearchByType(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/api/pokemon?type=ghost")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got []Pokemon
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	// The Gastly line: the only Ghosts in Kanto.
	if len(got) != 3 || got[0].Name != "gastly" {
		t.Fatalf("want gastly/haunter/gengar, got %+v", got)
	}
}
