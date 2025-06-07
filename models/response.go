package models

// ErrorResponse is a generic error response structure for API
type ErrorResponse struct {
	Message string `json:"message" example:"Error message describing the issue"`
	// Code int `json:"code,omitempty" example:"4002"` // Optional internal error code
}