// dex-api serves the Kanto Pokédex as JSON.
package main

import (
	"net/http"
	"os"

	"github.com/vincenzodavinci/poke-league/data"
	"github.com/vincenzodavinci/poke-league/internal/dex"
	"github.com/vincenzodavinci/poke-league/internal/httpx"
)

func main() {
	log := httpx.Logger("dex-api")
	store, err := dex.Load(data.KantoJSON)
	if err != nil {
		log.Error("load dex", "err", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	httpx.Health(mux)
	store.Routes(mux)
	if err := httpx.Serve(log, httpx.Env("ADDR", ":8080"), mux); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
