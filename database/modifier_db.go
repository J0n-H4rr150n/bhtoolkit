package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// DeleteModifierTask removes a task from the modifier_tasks table.
func DeleteModifierTask(taskID int64) error {
	stmt, err := DB.Prepare("DELETE FROM modifier_tasks WHERE id = ?")
	if err != nil {
		logger.Error("DeleteModifierTask: Error preparing statement for task ID %d: %v", taskID, err)
		return fmt.Errorf("preparing delete statement for modifier task ID %d: %w", taskID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(taskID)
	if err != nil {
		logger.Error("DeleteModifierTask: Error executing delete for task ID %d: %v", taskID, err)
		return fmt.Errorf("executing delete for modifier task ID %d: %w", taskID, err)
	}

	rowsAffected, _ := result.RowsAffected() // Error check for RowsAffected is optional here
	logger.Info("DeleteModifierTask: Successfully deleted modifier task ID %d, rows affected: %d", taskID, rowsAffected)
	return nil
}

// CreateModifierTaskFromSource creates a new modifier task based on a source (log or parameterized URL).
func CreateModifierTaskFromSource(req models.AddModifierTaskRequest) (*models.ModifierTask, error) {
	var task models.ModifierTask
	var err error

	if req.HTTPTrafficLogID != 0 {
		task, err = createTaskFromHTTPLog(req.HTTPTrafficLogID)
		if err != nil {
			return nil, fmt.Errorf("creating task from http log ID %d: %w", req.HTTPTrafficLogID, err)
		}
		task.SourceLogID = sql.NullInt64{Int64: req.HTTPTrafficLogID, Valid: true}
	} else if req.ParameterizedURLID != 0 {
		task, err = createTaskFromParameterizedURL(req.ParameterizedURLID)
		if err != nil {
			return nil, fmt.Errorf("creating task from parameterized URL ID %d: %w", req.ParameterizedURLID, err)
		}
		task.SourceParamURLID = sql.NullInt64{Int64: req.ParameterizedURLID, Valid: true}
	} else {
		return nil, fmt.Errorf("either HTTPTrafficLogID or ParameterizedURLID must be provided")
	}

	if req.NameOverride != "" {
		task.Name = req.NameOverride
	} else if task.Name == "" { // Generate a default name if not set by source functions
		task.Name = fmt.Sprintf("Task for %s %s", task.BaseRequestMethod, task.BaseRequestURL)
		if len(task.Name) > 100 { // Truncate if too long
			task.Name = task.Name[:97] + "..."
		}
	}

	// If target_id is provided in request, it overrides the one from source (if any)
	if req.TargetID != 0 {
		task.TargetID = sql.NullInt64{Int64: req.TargetID, Valid: true}
	}

	// Get next display order
	var maxOrder sql.NullInt64
	// Display order is global for modifier tasks, not per target
	err = DB.QueryRow("SELECT MAX(display_order) FROM modifier_tasks").Scan(&maxOrder)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting max display_order for modifier_tasks: %w", err)
	}
	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}
	task.DisplayOrder = nextOrder

	stmt, err := DB.Prepare(`INSERT INTO modifier_tasks
        (target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, source_log_id, source_param_url_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("preparing insert statement for modifier_task: %w", err)
	}
	defer stmt.Close()

	currentTime := time.Now()
	result, err := stmt.Exec(
		task.TargetID, task.Name, task.BaseRequestURL, task.BaseRequestMethod,
		task.BaseRequestHeaders, task.BaseRequestBody, task.DisplayOrder,
		task.SourceLogID, task.SourceParamURLID,
		currentTime, currentTime,
	)
	if err != nil {
		return nil, fmt.Errorf("executing insert for modifier_task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert ID for modifier_task: %w", err)
	}
	task.ID = id
	task.CreatedAt = currentTime
	task.UpdatedAt = currentTime

	logger.Info("Created ModifierTask with ID: %d, Name: %s", task.ID, task.Name)
	return &task, nil
}

// Helper function to create a task from an http_traffic_log entry
func createTaskFromHTTPLog(logID int64) (models.ModifierTask, error) {
	var task models.ModifierTask
	var log models.HTTPTrafficLog // Assuming you have this model
	// Fetch necessary fields from http_traffic_log
	// This query needs to match your models.HTTPTrafficLog structure
	err := DB.QueryRow(`SELECT target_id, request_url, request_method, request_headers, request_body
						FROM http_traffic_log WHERE id = ?`, logID).Scan(
		&log.TargetID, &log.RequestURL, &log.RequestMethod, &log.RequestHeaders, &log.RequestBody,
	)
	if err != nil {
		return task, fmt.Errorf("fetching http_traffic_log entry %d: %w", logID, err)
	}

	task.TargetID = models.ConvertInt64PtrToSQLNullInt64(log.TargetID)
	task.BaseRequestURL = log.RequestURL
	task.BaseRequestMethod = log.RequestMethod
	task.BaseRequestHeaders = log.RequestHeaders // Assuming it's already a JSON string
	task.BaseRequestBody = models.Base64Encode(log.RequestBody)
	task.Name = fmt.Sprintf("From Log #%d: %s", logID, strings.Split(log.RequestURL, "?")[0])
	return task, nil
}

// Helper function to create a task from a parameterized_urls entry
func createTaskFromParameterizedURL(paramURLID int64) (models.ModifierTask, error) {
	var task models.ModifierTask
	var pURL models.ParameterizedURL // Assuming you have this model
	// Fetch necessary fields from parameterized_urls
	err := DB.QueryRow(`SELECT target_id, request_method, request_path, example_full_url
						FROM parameterized_urls WHERE id = ?`, paramURLID).Scan(
		&pURL.TargetID, &pURL.RequestMethod, &pURL.RequestPath, &pURL.ExampleFullURL,
	)
	if err != nil {
		return task, fmt.Errorf("fetching parameterized_urls entry %d: %w", paramURLID, err)
	}

	task.TargetID = sql.NullInt64{Int64: pURL.TargetID, Valid: pURL.TargetID != 0}
	task.BaseRequestURL = pURL.ExampleFullURL // Use example_full_url as a starting point
	task.BaseRequestMethod = pURL.RequestMethod
	// For parameterized URLs, headers and body might be empty initially or fetched from an example log if available
	task.BaseRequestHeaders = "{}" // Default empty JSON object
	task.BaseRequestBody = ""      // Default empty body (empty base64 string)
	task.Name = fmt.Sprintf("From PURL #%d: %s", paramURLID, pURL.RequestPath)
	return task, nil
}

// GetModifierTasks retrieves all tasks for the modifier, optionally filtered by target_id (0 for no filter).
// It now sorts by display_order ASC, then created_at DESC.
func GetModifierTasks(targetIDFilter int64) ([]models.ModifierTask, error) {
	// Implementation for GetModifierTasks (ensure it sorts by display_order)
	// This is a placeholder; you'll need to implement the actual query and scanning.
	query := "SELECT id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, created_at, updated_at, source_log_id, source_param_url_id FROM modifier_tasks ORDER BY display_order ASC, created_at DESC"
	// If targetIDFilter is used, add WHERE clause: " WHERE target_id = ? ORDER BY ..."
	// For now, assuming targetIDFilter = 0 means no filter, which is handled by not adding a WHERE clause.
	// If you need to filter by target_id, you'd adjust the query and parameters.

	rows, err := DB.Query(query) // Add targetIDFilter if used and query is adjusted
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.ModifierTask
	for rows.Next() {
		var t models.ModifierTask
		if err := rows.Scan(&t.ID, &t.TargetID, &t.Name, &t.BaseRequestURL, &t.BaseRequestMethod, &t.BaseRequestHeaders, &t.BaseRequestBody, &t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt, &t.SourceLogID, &t.SourceParamURLID); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetModifierTaskByID retrieves a single modifier task by its ID.
func GetModifierTaskByID(taskID int64) (*models.ModifierTask, error) {
	var t models.ModifierTask
	err := DB.QueryRow("SELECT id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, created_at, updated_at, source_log_id, source_param_url_id FROM modifier_tasks WHERE id = ?", taskID).Scan(
		&t.ID, &t.TargetID, &t.Name, &t.BaseRequestURL, &t.BaseRequestMethod, &t.BaseRequestHeaders, &t.BaseRequestBody, &t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt, &t.SourceLogID, &t.SourceParamURLID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Task not found
		}
		return nil, err
	}
	return &t, nil
}

// UpdateModifierTaskName updates the name of a modifier task.
func UpdateModifierTaskName(taskID int64, newName string) (*models.ModifierTask, error) {
	updatedAt := time.Now()
	_, err := DB.Exec("UPDATE modifier_tasks SET name = ?, updated_at = ? WHERE id = ?", newName, updatedAt, taskID)
	if err != nil {
		return nil, err
	}
	return GetModifierTaskByID(taskID) // Return the updated task
}

// UpdateModifierTasksOrder updates the display_order for multiple tasks.
func UpdateModifierTasksOrder(taskOrders map[int64]int) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare("UPDATE modifier_tasks SET display_order = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	updatedAt := time.Now()
	for id, order := range taskOrders {
		if _, err := stmt.Exec(order, updatedAt, id); err != nil {
			tx.Rollback()
			return fmt.Errorf("updating order for task %d: %w", id, err)
		}
	}
	return tx.Commit()
}
