package handlers

import (
	"net/http"
)

func RegisterRelationshipRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /relationships/create", CreateRelationshipHandler)
}
