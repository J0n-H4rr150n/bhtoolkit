package handlers

import (
	"net/http"
)

func RegisterSitemapRoutes(mux *http.ServeMux) {
	// Placeholders
	mux.HandleFunc("GET /sitemap-endpoints", GetSitemapEndpointsHandler)
	mux.HandleFunc("GET /endpoint-instances", GetEndpointInstancesHandler)
}
