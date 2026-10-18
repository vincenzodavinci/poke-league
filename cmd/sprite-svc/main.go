// sprite-svc serves Pokémon artwork, badges and items compiled into the binary.
package main

import (
	"net/http"
	"os"

	"github.com/vincenzodavinci/poke-league/internal/httpx"
	"github.com/vincenzodavinci/poke-league/internal/sprites"
)

func main() {
	log := httpx.Logger("sprite-svc")
	mux := http.NewServeMux()
	httpx.Health(mux)
	sprites.Routes(mux)
	if err := httpx.Serve(log, httpx.Env("ADDR", ":8080"), mux); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
