package database

import (
	"database/sql"
	"fmt"
	"toolkit/logger"
	"toolkit/models"
)

// DeleteModifierTasksByTargetID deletes all modifier tasks associated with a given target_id.
// It returns the number of tasks deleted and an error if any occurred.
// This function assumes that any child records in tables like 'modifier_task_requests'
// or 'modifier_task_responses' that reference 'modifier_tasks.id'
// are handled by 'ON DELETE CASCADE' in the database schema, or are not present.
func DeleteModifierTasksByTargetID(targetID int64) (int64, error) {
	stmt, err := DB.Prepare("DELETE FROM modifier_tasks WHERE target_id = ?")
	if err != nil {
		logger.Error("DeleteModifierTasksByTargetID: Error preparing statement for target %d: %v", targetID, err)
		return 0, fmt.Errorf("preparing delete modifier tasks statement for target %d: %w", targetID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(targetID)
	if err != nil {
		logger.Error("DeleteModifierTasksByTargetID: Error executing statement for target %d: %v", targetID, err)
		return 0, fmt.Errorf("executing delete modifier tasks for target %d: %w", targetID, err)
	}

	rowsAffected, _ := result.RowsAffected() // Error on RowsAffected is less critical here if Exec was successful
	logger.Info("Deleted %d modifier tasks for target ID %d", rowsAffected, targetID)
	return rowsAffected, nil
}

// CreateModifierTaskFromSource creates a new modifier task based on a source log or parameterized URL.
func CreateModifierTaskFromSource(req models.AddModifierTaskRequest) (*models.ModifierTask, error) {
	var task models.ModifierTask
	var sourceName string
	var originalReqHeaders, originalReqBody, originalResHeaders, originalResBody sql.NullString

	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if req.HTTPTrafficLogID != 0 {
		var log models.HTTPTrafficLog
		// Fetch necessary details from http_traffic_log
		// Fetch request and response data
		err := tx.QueryRow(`SELECT target_id, request_method, request_url, request_headers, request_body, response_headers, response_body
							FROM http_traffic_log WHERE id = ?`, req.HTTPTrafficLogID).Scan(
			&log.TargetID, &log.RequestMethod, &log.RequestURL, &log.RequestHeaders, &log.RequestBody, &log.ResponseHeaders, &log.ResponseBody,
		)
		if err != nil {
			return nil, fmt.Errorf("fetching http_traffic_log %d: %w", req.HTTPTrafficLogID, err)
		}
		// log.TargetID is *int64, task.TargetID is sql.NullInt64
		if log.TargetID != nil {
			task.TargetID = sql.NullInt64{Int64: *log.TargetID, Valid: true}
		} else {
			task.TargetID = sql.NullInt64{Valid: false}
		}
		// Set original request/response data from the source log
		originalReqHeaders = log.RequestHeaders
		originalReqBody = models.NullString(string(log.RequestBody))
		originalResHeaders = log.ResponseHeaders
		originalResBody = models.NullString(string(log.ResponseBody))
		task.BaseRequestMethod = log.RequestMethod.String
		task.BaseRequestURL = log.RequestURL.String
		task.BaseRequestHeaders = log.RequestHeaders
		task.BaseRequestBody = models.NullString(string(log.RequestBody))
		task.SourceLogID = sql.NullInt64{Int64: req.HTTPTrafficLogID, Valid: true}
		sourceName = fmt.Sprintf("From Log %d", req.HTTPTrafficLogID)
	} else if req.ParameterizedURLID != 0 {
		var pURL models.ParameterizedURL
		// Fetch necessary details from parameterized_urls
		err := tx.QueryRow(`SELECT target_id, request_method, request_path, example_full_url, http_traffic_log_id 
							FROM parameterized_urls WHERE id = ?`, req.ParameterizedURLID).Scan(
			&pURL.TargetID, &pURL.RequestMethod, &pURL.RequestPath, &pURL.ExampleFullURL, &pURL.HTTPTrafficLogID,
		)
		if err != nil {
			return nil, fmt.Errorf("fetching parameterized_url %d: %w", req.ParameterizedURLID, err)
		}
		// pURL.TargetID is sql.NullInt64, task.TargetID is sql.NullInt64
		task.TargetID = pURL.TargetID
		task.BaseRequestMethod = pURL.RequestMethod.String
		task.BaseRequestURL = pURL.ExampleFullURL.String // Use example_full_url as the base
		// For parameterized URLs, headers and body might need to be fetched from the example log if available
		// If no example log, original fields will remain null.
		if pURL.HTTPTrafficLogID.Valid {
			var logHeaders, logBody, logResHeaders, logResBody sql.NullString
			errLog := tx.QueryRow(`SELECT request_headers, request_body, response_headers, response_body FROM http_traffic_log WHERE id = ?`, pURL.HTTPTrafficLogID.Int64).Scan(&logHeaders, &logBody, &logResHeaders, &logResBody)
			if errLog == nil {
				originalReqHeaders = logHeaders
				originalReqBody = logBody
				task.BaseRequestHeaders = logHeaders
				task.BaseRequestBody = logBody
				originalResHeaders = logResHeaders
				originalResBody = logResBody
			} else if errLog != sql.ErrNoRows {
				logger.Warn("CreateModifierTaskFromSource: Could not fetch headers/body from example log %d for PURL %d: %v", pURL.HTTPTrafficLogID.Int64, req.ParameterizedURLID, errLog)
			}
		}
		task.SourceParameterizedURLID = sql.NullInt64{Int64: req.ParameterizedURLID, Valid: true}
		sourceName = fmt.Sprintf("From PURL %d", req.ParameterizedURLID)
	} else {
		return nil, fmt.Errorf("no valid source (log_id or purl_id) provided")
	}

	task.Name = fmt.Sprintf("Task - %s", sourceName) // Default name

	// Get max display_order for the target (or globally if no target) and increment
	var maxOrder sql.NullInt64
	queryOrder := "SELECT MAX(display_order) FROM modifier_tasks"
	argsOrder := []interface{}{}
	if task.TargetID.Valid {
		queryOrder += " WHERE target_id = ?"
		argsOrder = append(argsOrder, task.TargetID.Int64)
	}
	err = tx.QueryRow(queryOrder, argsOrder...).Scan(&maxOrder)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting max display_order: %w", err)
	}
	task.DisplayOrder = int(maxOrder.Int64) + 1

	stmt, err := tx.Prepare(`INSERT INTO modifier_tasks 
		(target_id, name, base_request_method, base_request_url, base_request_headers, base_request_body, original_request_headers, original_request_body, original_response_headers, original_response_body, source_log_id, source_param_url_id, display_order, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	if err != nil {
		return nil, fmt.Errorf("preparing insert statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(task.TargetID, task.Name, task.BaseRequestMethod, task.BaseRequestURL, task.BaseRequestHeaders, task.BaseRequestBody, originalReqHeaders, originalReqBody, originalResHeaders, originalResBody, task.SourceLogID, task.SourceParameterizedURLID, task.DisplayOrder)
	if err != nil {
		return nil, fmt.Errorf("executing insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert ID: %w", err)
	}
	task.ID = id

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	logger.Info("Created modifier task ID %d: %s", task.ID, task.Name)
	return &task, nil
}

// GetModifierTasks retrieves all modifier tasks, optionally filtered by target_id.
func GetModifierTasks(targetID int64) ([]models.ModifierTask, error) {
	var tasks []models.ModifierTask
	query := `SELECT id, target_id, name, base_request_method, base_request_url, base_request_headers, base_request_body, original_request_headers, original_request_body, original_response_headers, original_response_body, last_executed_log_id, source_log_id, source_param_url_id, display_order, created_at, updated_at 
			  FROM modifier_tasks`
	args := []interface{}{}
	if targetID != 0 {
		query += " WHERE target_id = ?"
		args = append(args, targetID)
	}
	query += " ORDER BY display_order ASC, created_at DESC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying modifier tasks: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t models.ModifierTask // Ensure all new fields are scanned
		if err := rows.Scan(&t.ID, &t.TargetID, &t.Name, &t.BaseRequestMethod, &t.BaseRequestURL, &t.BaseRequestHeaders, &t.BaseRequestBody, &t.OriginalRequestHeaders, &t.OriginalRequestBody, &t.OriginalResponseHeaders, &t.OriginalResponseBody, &t.LastExecutedLogID, &t.SourceLogID, &t.SourceParameterizedURLID, &t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning modifier task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetModifierTaskByID retrieves a single modifier task by its ID.
func GetModifierTaskByID(taskID int64) (*models.ModifierTask, error) {
	var t models.ModifierTask
	err := DB.QueryRow(`SELECT id, target_id, name, base_request_method, base_request_url, base_request_headers, base_request_body, original_request_headers, original_request_body, original_response_headers, original_response_body, last_executed_log_id, source_log_id, source_param_url_id, display_order, created_at, updated_at 
					   FROM modifier_tasks WHERE id = ?`, taskID).Scan( // Ensure all new fields are selected
		&t.ID, &t.TargetID, &t.Name, &t.BaseRequestMethod, &t.BaseRequestURL, &t.BaseRequestHeaders, &t.BaseRequestBody, &t.OriginalRequestHeaders, &t.OriginalRequestBody, &t.OriginalResponseHeaders, &t.OriginalResponseBody, &t.LastExecutedLogID, &t.SourceLogID, &t.SourceParameterizedURLID, &t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt, // Ensure all new fields are scanned
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Task not found
		}
		return nil, fmt.Errorf("querying modifier task %d: %w", taskID, err)
	}
	return &t, nil
}

// DeleteModifierTask deletes a modifier task by its ID.
func DeleteModifierTask(taskID int64) error {
	_, err := DB.Exec("DELETE FROM modifier_tasks WHERE id = ?", taskID)
	if err != nil {
		return fmt.Errorf("deleting modifier task %d: %w", taskID, err)
	}
	logger.Info("Deleted modifier task ID %d", taskID)
	return nil
}

// UpdateModifierTaskBaseRequestDetails updates the base request fields of a task.
func UpdateModifierTaskBaseRequestDetails(taskID int64, method, url, headersJSON, bodyBase64 string) error {
	_, err := DB.Exec(`UPDATE modifier_tasks SET base_request_method = ?, base_request_url = ?, base_request_headers = ?, base_request_body = ?, updated_at = CURRENT_TIMESTAMP 
					   WHERE id = ?`, method, url, models.NullString(headersJSON), models.NullString(bodyBase64), taskID)
	if err != nil {
		return fmt.Errorf("updating base request for task %d: %w", taskID, err)
	}
	return nil
}

// UpdateModifierTaskLastExecutedLogID updates the last_executed_log_id for a task.
func UpdateModifierTaskLastExecutedLogID(taskID int64, logID int64) error {
	_, err := DB.Exec("UPDATE modifier_tasks SET last_executed_log_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", logID, taskID)
	if err != nil {
		return fmt.Errorf("updating last_executed_log_id for task %d: %w", taskID, err)
	}
	return nil
}

// UpdateModifierTaskName updates the name of a modifier task.
func UpdateModifierTaskName(taskID int64, name string) (*models.ModifierTask, error) {
	_, err := DB.Exec("UPDATE modifier_tasks SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", name, taskID)
	if err != nil {
		return nil, fmt.Errorf("updating name for task %d: %w", taskID, err)
	}
	return GetModifierTaskByID(taskID) // Return the updated task
}

// CloneModifierTaskDB creates a new task by copying an existing one.
func CloneModifierTaskDB(originalTaskID int64) (*models.ModifierTask, error) {
	originalTask, err := GetModifierTaskByID(originalTaskID)
	if err != nil {
		return nil, fmt.Errorf("fetching original task %d for clone: %w", originalTaskID, err)
	}
	if originalTask == nil {
		return nil, fmt.Errorf("original task %d not found for clone", originalTaskID)
	}

	clonedTask := *originalTask // Shallow copy, then adjust
	clonedTask.ID = 0           // Will be set on insert
	clonedTask.Name = originalTask.Name + " (Clone)"
	clonedTask.LastExecutedLogID = sql.NullInt64{Valid: false} // Cloned task hasn't been executed

	// Get new display_order
	var maxOrder sql.NullInt64
	queryOrder := "SELECT MAX(display_order) FROM modifier_tasks"
	argsOrder := []interface{}{}
	if clonedTask.TargetID.Valid {
		queryOrder += " WHERE target_id = ?"
		argsOrder = append(argsOrder, clonedTask.TargetID.Int64)
	}
	err = DB.QueryRow(queryOrder, argsOrder...).Scan(&maxOrder)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting max display_order for clone: %w", err)
	}
	clonedTask.DisplayOrder = int(maxOrder.Int64) + 1

	stmt, err := DB.Prepare(`INSERT INTO modifier_tasks 
		(target_id, name, base_request_method, base_request_url, base_request_headers, base_request_body, original_request_headers, original_request_body, original_response_headers, original_response_body, source_log_id, source_param_url_id, display_order, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`) // Add new columns to INSERT
	if err != nil {
		return nil, fmt.Errorf("preparing insert for clone: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(clonedTask.TargetID, clonedTask.Name, clonedTask.BaseRequestMethod, clonedTask.BaseRequestURL, clonedTask.BaseRequestHeaders, clonedTask.BaseRequestBody, clonedTask.OriginalRequestHeaders, clonedTask.OriginalRequestBody, clonedTask.OriginalResponseHeaders, clonedTask.OriginalResponseBody, clonedTask.SourceLogID, clonedTask.SourceParameterizedURLID, clonedTask.DisplayOrder) // Pass original fields
	if err != nil {
		return nil, fmt.Errorf("executing insert for clone: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert ID for clone: %w", err)
	}
	clonedTask.ID = id
	logger.Info("Cloned modifier task ID %d to new task ID %d: %s", originalTaskID, clonedTask.ID, clonedTask.Name)
	return &clonedTask, nil
}

// UpdateModifierTasksOrder updates the display_order for multiple tasks.
func UpdateModifierTasksOrder(orders map[int64]int) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for task order update: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("UPDATE modifier_tasks SET display_order = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing task order update statement: %w", err)
	}
	defer stmt.Close()

	for taskID, order := range orders {
		if _, err := stmt.Exec(order, taskID); err != nil {
			return fmt.Errorf("updating order for task %d: %w", taskID, err)
		}
	}
	return tx.Commit()
}
