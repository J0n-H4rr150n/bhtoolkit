package handlers

import (
	"net/http"
)

func RegisterPlatformRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/platforms", PlatformsCollectionHandler) // Handles GET and POST for /platforms
	mux.HandleFunc("/platforms/", PlatformItemHandler)       // Handles GET, PUT, DELETE for /platforms/{id}
}
