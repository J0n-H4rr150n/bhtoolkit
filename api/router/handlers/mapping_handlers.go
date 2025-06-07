package handlers

import (
	"net/http"
)

// CreateRelationshipHandler is a placeholder for creating page-to-API relationships.
// @Summary Create a page-to-API relationship
// @Description (Not Implemented Yet) Manually links a discovered web page to a discovered API endpoint.
// @Tags Mapping
// @Accept json
// @Produce json
// @Param relationship_request body object true "Request body containing web_page_id and api_endpoint_id" SchemaExample({\n  "web_page_id": 1,\n  "api_endpoint_id": 2,\n  "trigger_info": "onClick_button_submit"\n})
// @Success 501 {object} models.ErrorResponse "Not Implemented Yet"
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Router /relationships/create [post]
func CreateRelationshipHandler(w http.ResponseWriter, r *http.Request) {
	notImplementedHandler(w, r) // This now calls the helper from within the 'handlers' package
}
