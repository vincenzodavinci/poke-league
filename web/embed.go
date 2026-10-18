// Package web embeds the League Board front end. It is plain HTML/CSS/JS
// with no build step, so it fits the strict same-origin CSP in internal/httpx.
package web

import "embed"

//go:embed index.html app.js style.css
var FS embed.FS
