package handlers

import (
	"net/http"
)

func RegisterScopeRuleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/scope-rules", ScopeRulesHandler)     // Handles GET and POST for /scope-rules
	mux.HandleFunc("/scope-rules/", ScopeRuleItemHandler) // Handles GET, PUT, DELETE for /scope-rules/{id}
}
