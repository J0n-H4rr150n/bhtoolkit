package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterScopeRuleRoutes(r chi.Router) {
	// Collection routes for /scope-rules
	r.Get("/scope-rules", getScopeRules) // Existing handler
	r.Post("/scope-rules", addScopeRule) // Existing handler

	// Routes for specific scope rule items, e.g., /scope-rules/{ruleID}
	r.Get("/scope-rules/{ruleID}", GetScopeRuleByIDChiHandler) // New chi-compatible handler to be created
	// r.Put("/scope-rules/{ruleID}", UpdateScopeRuleChiHandler) // Placeholder if you implement update
	r.Delete("/scope-rules/{ruleID}", DeleteScopeRuleChiHandler) // New chi-compatible handler to be created
}
