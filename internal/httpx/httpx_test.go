package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These are the headers OWASP ZAP's baseline scan (the Champion's DAST stage) checks for.
func TestSecureHeaders(t *testing.T) {
	h := Secure(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	want := map[string]string{
		"Content-Security-Policy":      CSP,
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Embedder-Policy": "require-corp",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s: want %q, got %q", k, v, got)
		}
	}
}

func TestReadyRunsItsChecks(t *testing.T) {
	probe := func(checks ...func(context.Context) error) (int, int) {
		mux := http.NewServeMux()
		Health(mux, checks...)
		codes := [2]int{}
		for i, path := range []string{"/healthz", "/readyz"} {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
			codes[i] = rec.Code
		}
		return codes[0], codes[1]
	}
	if live, ready := probe(); live != 200 || ready != 200 {
		t.Fatalf("no checks: want 200/200, got %d/%d", live, ready)
	}
	down := func(context.Context) error { return errors.New("api server unreachable") }
	if live, ready := probe(down); live != 200 || ready != 503 {
		t.Fatalf("failing check: want live 200 and ready 503, got %d/%d", live, ready)
	}
}
