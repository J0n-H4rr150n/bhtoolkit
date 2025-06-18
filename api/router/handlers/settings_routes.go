package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterSettingsRoutes(r chi.Router) {
	r.Get("/settings/current-target", GetCurrentTargetSettingHandler)
	r.Post("/settings/current-target", SetCurrentTargetSettingHandler)

	r.Route("/settings/custom-headers", func(r chi.Router) {
		r.Get("/", GetCustomHeadersSettingHandler)
		r.Post("/", SetCustomHeadersSettingHandler)
		r.Put("/", SetCustomHeadersSettingHandler)
	})

	r.Post("/settings/table-column-widths/reset", ResetTableColumnWidthsHandler)

	r.Route("/settings/table-column-widths", func(r chi.Router) {
		r.Get("/", GetTableColumnWidthsHandler)
		r.Post("/", SetTableColumnWidthsHandler)
		r.Put("/", SetTableColumnWidthsHandler)
	})

	r.Route("/ui-settings", func(r chi.Router) {
		r.Get("/", GetUISettingsHandler)
		r.Put("/", SetUISettingsHandler)
		r.Post("/", SetUISettingsHandler) // If you want to allow POST as well
	})

	r.Route("/settings/proxy-exclusions", func(r chi.Router) {
		r.Get("/", GetProxyExclusionRulesHandler)
		r.Post("/", SetProxyExclusionRulesHandler)
		r.Put("/", SetProxyExclusionRulesHandler)
	})

	// New route for general application settings (UI, Missions, etc.)
	r.Route("/settings/app", func(r chi.Router) {
		r.Get("/", GetApplicationSettingsHandler)  // New handler
		r.Put("/", SaveApplicationSettingsHandler) // New handler
	})
}
