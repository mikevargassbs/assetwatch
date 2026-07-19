// Package web embeds the built frontend (web/dist, produced by `npm run
// build`) so the API binary can serve it directly — one binary, one port,
// no separate frontend process needed in production.
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
