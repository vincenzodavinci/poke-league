// league-board serves the League Board UI and its /api/league feed, and
// proxies the dex and sprites so the browser only ever talks to one origin.
package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/vincenzodavinci/poke-league/internal/board"
	"github.com/vincenzodavinci/poke-league/internal/httpx"
	"github.com/vincenzodavinci/poke-league/web"
)

func main() {
	log := httpx.Logger("league-board")
	cfg, err := kubeConfig()
	if err != nil {
		log.Error("kube config", "err", err)
		os.Exit(1)
	}
	b := &board.Board{
		Kube:       kubernetes.NewForConfigOrDie(cfg),
		Dyn:        dynamic.NewForConfigOrDie(cfg),
		Namespaces: strings.Split(httpx.Env("LEAGUE_NAMESPACES", "pallet-town,indigo-plateau"), ","),
	}

	mux := http.NewServeMux()
	// Ready only while the API server answers: without it the board can't read the league.
	httpx.Health(mux, func(ctx context.Context) error {
		return b.Kube.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Error()
	})
	mux.Handle("GET /api/league", cached(b, 5*time.Second))
	mux.Handle("GET /api/pokemon", proxy(httpx.Env("DEX_API_URL", "http://dex-api")))
	mux.Handle("GET /api/pokemon/", proxy(httpx.Env("DEX_API_URL", "http://dex-api")))
	mux.Handle("GET /api/types", proxy(httpx.Env("DEX_API_URL", "http://dex-api")))
	mux.Handle("GET /sprites/", proxy(httpx.Env("SPRITE_URL", "http://sprite-svc")))
	static, _ := fs.Sub(web.FS, ".")
	mux.Handle("GET /", revalidate(http.FileServerFS(static)))

	if err := httpx.Serve(log, httpx.Env("ADDR", ":8080"), mux); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// kubeConfig prefers the in-cluster ServiceAccount, falling back to
// ~/.kube/config so `go run ./cmd/league-board` works against kind.
func kubeConfig() (*rest.Config, error) {
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{CurrentContext: httpx.Env("KUBE_CONTEXT", "kind-kanto")}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
}

// proxy forwards to a fixed upstream. The target comes from config, never
// from the request, so there is nothing for a caller to redirect.
func proxy(target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	return &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(u)
		r.Out.Host = u.Host
	}}
}

// revalidate makes browsers check for a new UI on every load (cheap: the
// file server answers 304 when nothing changed), so a deploy is never masked
// by a stale cached app.js.
func revalidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// cached keeps the API server from being hit on every browser refresh.
func cached(b *board.Board, ttl time.Duration) http.Handler {
	var (
		mu   sync.Mutex
		body []byte
		at   time.Time
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if body == nil || time.Since(at) > ttl {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			league, err := b.Collect(ctx)
			if err != nil {
				http.Error(w, "the league is closed: "+err.Error(), http.StatusBadGateway)
				return
			}
			body, _ = json.Marshal(league)
			at = time.Now()
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	})
}
