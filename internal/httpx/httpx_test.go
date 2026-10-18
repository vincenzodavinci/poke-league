package httpx

import (
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
