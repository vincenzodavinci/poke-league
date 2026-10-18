package sprites

import (
	"net/http"
	"net/http/httptest"
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
		{"/sprites/pokemon/137.png", 200},
		{"/sprites/pokemon/25", 200},
		{"/sprites/pokemon/0.png", 404},
		{"/sprites/pokemon/152.png", 404},
		{"/sprites/badges/8.png", 200},
		{"/sprites/badges/9.png", 404},
		{"/sprites/items/poke-ball.png", 200},
		{"/sprites/items/..%2f..%2fetc%2fpasswd", 404},
		{"/sprites/items/http:%2f%2f169.254.169.254", 404},
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
		if c.status == 200 && res.Header.Get("Content-Type") != "image/png" {
			t.Errorf("%s: want image/png, got %q", c.path, res.Header.Get("Content-Type"))
		}
	}
}

func TestETag(t *testing.T) {
	mux := http.NewServeMux()
	Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/sprites/pokemon/137.png")
	res.Body.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/sprites/pokemon/137.png", nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	res2, _ := http.DefaultClient.Do(req)
	res2.Body.Close()
	if res2.StatusCode != http.StatusNotModified {
		t.Fatalf("want 304 on matching ETag, got %d", res2.StatusCode)
	}
}
