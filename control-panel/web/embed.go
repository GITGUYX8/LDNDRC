// Package web embeds the student SPA bundle. UI-1 replaces dist/ with
// the Vite build; the Go server serves it on the control host (see
// httpapi.SPAHandler). Placeholder until then.
package web

import "embed"

// Dist holds the built SPA (index.html + assets).
//
//go:embed dist
var Dist embed.FS
