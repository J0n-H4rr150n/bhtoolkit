package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterModifierRoutes(r chi.Router) {
	r.Post("/modifier/tasks", AddModifierTaskHandler)
	r.Get("/modifier/tasks", GetModifierTasksHandler)
	r.Get("/modifier/tasks/{task_id}", GetModifierTaskDetailsHandler)
	r.Post("/modifier/execute", ExecuteModifiedRequestHandler)
	r.Put("/modifier/tasks/{task_id}", UpdateModifierTaskHandler) // For updating parts of the task, like name
	r.Post("/modifier/tasks/{task_id}/clone", CloneModifierTaskHandler)
	r.Put("/modifier/tasks/order", UpdateModifierTasksOrderHandler) // For updating the order of all tasks
}
