// Package assets embeds the Pokémon artwork, Kanto badges and item sprites
// fetched by scripts/fetch_assets.py.
package assets

import "embed"

//go:embed pokemon badges items
var FS embed.FS
