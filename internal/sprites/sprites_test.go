package sprites

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cases := []struct {
		path   string
		status int
	}{
		{"/sprites/pokemon/137.svg", 200},
		{"/sprites/pokemon/137.png", 200}, // old URLs keep working
		{"/sprites/pokemon/25", 200},
		{"/sprites/pokemon/0.svg", 404},
		{"/sprites/pokemon/152.svg", 404},
		{"/sprites/badges/8.svg", 200},
		{"/sprites/badges/9.svg", 404},
		{"/sprites/favicon.svg", 200},
		{"/sprites/items/poke-ball.png", 404}, // the official item sprites are gone
		{"/sprites/pokemon/..%2f..%2fetc%2fpasswd", 404},
		{"/sprites/pokemon/http:%2f%2f169.254.169.254", 404},
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
		if c.status == 200 && res.Header.Get("Content-Type") != "image/svg+xml" {
			t.Errorf("%s: want image/svg+xml, got %q", c.path, res.Header.Get("Content-Type"))
		}
	}
}

// Every crest is an inert image: no script, no external references.
func TestCrestsAreInert(t *testing.T) {
	mux := http.NewServeMux()
	Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, path := range []string{"/sprites/pokemon/1", "/sprites/pokemon/151", "/sprites/badges/1", "/sprites/favicon.svg"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		// The SVG namespace declaration is the one URL allowed.
		s := strings.ReplaceAll(strings.ToLower(string(body)), `xmlns="http://www.w3.org/2000/svg"`, "")
		for _, bad := range []string{"<script", "javascript:", "onload=", "http://", "https://"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s contains %q", path, bad)
			}
		}
		if !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: SVG served without a sandboxing CSP", path)
		}
	}
}

func TestETag(t *testing.T) {
	mux := http.NewServeMux()
	Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/sprites/pokemon/137.svg")
	res.Body.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/sprites/pokemon/137.svg", nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	res2, _ := http.DefaultClient.Do(req)
	res2.Body.Close()
	if res2.StatusCode != http.StatusNotModified {
		t.Fatalf("want 304 on matching ETag, got %d", res2.StatusCode)
	}
}
