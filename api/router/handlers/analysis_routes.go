package handlers

import (
	"net/http"
)

func RegisterAnalysisRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /analyze/jslinks", AnalyzeJSLinksHandler)
}
