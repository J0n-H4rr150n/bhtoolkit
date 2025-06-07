package handlers

import (
	"net/http"
)

// GetSitemapEndpointsHandler is a placeholder for getting unique sitemap endpoints.
// @Summary Get sitemap/unique endpoints for a target
// @Description (Not Implemented Yet) Retrieves a list of unique [METHOD] /path combinations discovered for a target.
// @Tags Sitemap
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 501 {object} models.ErrorResponse "Not Implemented Yet"
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Router /sitemap-endpoints [get]
func GetSitemapEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	notImplementedHandler(w, r)
}

// GetEndpointInstancesHandler is a placeholder for getting all instances of a specific endpoint.
// @Summary Get all instances for a specific sitemap endpoint
// @Description (Not Implemented Yet) Retrieves all logged HTTP requests that match a specific [METHOD] /path combination for a target.
// @Tags Sitemap
// @Produce json
// @Param target_id query int true "ID of the target"
// @Param method query string true "HTTP method of the endpoint (e.g., GET, POST)"
// @Param path query string true "Normalized URL path of the endpoint (e.g., /api/users)"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of items per page" default(50)
// @Success 501 {object} models.ErrorResponse "Not Implemented Yet"
// @Failure 400 {object} models.ErrorResponse "Missing or invalid parameters"
// @Router /endpoint-instances [get]
func GetEndpointInstancesHandler(w http.ResponseWriter, r *http.Request) {
	notImplementedHandler(w, r)
}
