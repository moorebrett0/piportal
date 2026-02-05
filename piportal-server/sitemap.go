package main

import (
	"fmt"
	"net/http"
)

func (h *Handler) handleSitemap(w http.ResponseWriter, r *http.Request) {
	domain := h.config.BaseDomain
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://%[1]s/</loc>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://%[1]s/fleet</loc>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://%[1]s/status</loc>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>https://%[1]s/terms</loc>
    <priority>0.3</priority>
  </url>
</urlset>
`, domain)
}
