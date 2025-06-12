package database

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
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
		// SourceLogID is set here because this is the direct source.
		task.SourceLogID = sql.NullInt64{Int64: req.HTTPTrafficLogID, Valid: true}
	} else if req.ParameterizedURLID != 0 {
		task, err = createTaskFromParameterizedURL(req.ParameterizedURLID)
		if err != nil {
			return nil, fmt.Errorf("creating task from parameterized URL ID %d: %w", req.ParameterizedURLID, err)
		}
		// SourceLogID (for the example log) is set inside createTaskFromParameterizedURL.
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
	task.CreatedAt = sql.NullTime{Time: currentTime, Valid: true}
	task.UpdatedAt = sql.NullTime{Time: currentTime, Valid: true}

	logger.Info("Created ModifierTask with ID: %d, Name: %s", task.ID, task.Name)
	return &task, nil
}

// Helper function to create a task from an http_traffic_log entry
func createTaskFromHTTPLog(logID int64) (models.ModifierTask, error) {
	var task models.ModifierTask
	var log models.HTTPTrafficLog // HTTPTrafficLog already uses sql.NullString for RequestURL and RequestMethod
	err := DB.QueryRow(`SELECT target_id, request_url, request_method, request_headers, request_body 
						FROM http_traffic_log WHERE id = ?`, logID).Scan(
		&log.TargetID, &log.RequestURL, &log.RequestMethod, &log.RequestHeaders, &log.RequestBody,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Info("createTaskFromHTTPLog: http_traffic_log entry with ID %d not found", logID)
			return task, fmt.Errorf("source http log entry with ID %d not found: %w", logID, err)
		}
		logger.Error("createTaskFromHTTPLog: Error scanning http_traffic_log entry %d: %v", logID, err)
		return task, fmt.Errorf("fetching http_traffic_log entry %d: %w", logID, err)
	}

	if !log.RequestURL.Valid || log.RequestURL.String == "" {
		return task, fmt.Errorf("source log ID %d has no request URL, cannot create modifier task", logID)
	}
	if !log.RequestMethod.Valid || log.RequestMethod.String == "" {
		return task, fmt.Errorf("source log ID %d has no request method, cannot create modifier task", logID)
	}

	task.TargetID = models.ConvertInt64PtrToSQLNullInt64(log.TargetID)
	task.BaseRequestURL = log.RequestURL.String
	task.BaseRequestMethod = log.RequestMethod.String
	task.BaseRequestHeaders = log.RequestHeaders
	task.BaseRequestBody = sql.NullString{String: models.Base64Encode(log.RequestBody), Valid: true}

	// Set name to the path, similar to PURL logic.
	// If path is empty, CreateModifierTaskFromSource will generate a default name.
	parsedURL, parseErr := url.Parse(log.RequestURL.String)
	if parseErr == nil && parsedURL.Path != "" {
		task.Name = parsedURL.Path
	} // else task.Name remains empty for default generation
	return task, nil
}

// Helper function to create a task from a parameterized_urls entry
func createTaskFromParameterizedURL(paramURLID int64) (models.ModifierTask, error) {
	var task models.ModifierTask
	var pURL models.ParameterizedURL
	err := DB.QueryRow(`SELECT target_id, request_method, request_path, example_full_url, http_traffic_log_id 
						FROM parameterized_urls WHERE id = ?`, paramURLID).Scan(
		&pURL.TargetID, &pURL.RequestMethod, &pURL.RequestPath, &pURL.ExampleFullURL, &pURL.HTTPTrafficLogID,
	)
	if err != nil {
		return task, fmt.Errorf("fetching parameterized_urls entry %d: %w", paramURLID, err)
	}

	if !pURL.ExampleFullURL.Valid || pURL.ExampleFullURL.String == "" {
		return task, fmt.Errorf("source PURL ID %d has no example full URL, cannot create modifier task", paramURLID)
	}
	if !pURL.RequestMethod.Valid || pURL.RequestMethod.String == "" {
		return task, fmt.Errorf("source PURL ID %d has no request method, cannot create modifier task", paramURLID)
	}

	task.TargetID = pURL.TargetID
	task.BaseRequestURL = pURL.ExampleFullURL.String
	task.BaseRequestMethod = pURL.RequestMethod.String
	task.BaseRequestHeaders = sql.NullString{String: "{}", Valid: true}
	if pURL.HTTPTrafficLogID != 0 { // pURL.HTTPTrafficLogID is int64, not sql.NullInt64
		task.SourceLogID = sql.NullInt64{Int64: pURL.HTTPTrafficLogID, Valid: true}
	}
	task.BaseRequestBody = sql.NullString{String: "", Valid: true}
	if pURL.RequestPath.Valid && pURL.RequestPath.String != "" {
		task.Name = pURL.RequestPath.String // Use the path as the name
	} else {
		// Fallback if PURL's path is empty, try to derive from ExampleFullURL
		parsedExampleURL, errParse := url.Parse(pURL.ExampleFullURL.String)
		if errParse == nil && parsedExampleURL.Path != "" {
			task.Name = parsedExampleURL.Path
		} // else task.Name remains empty for default generation by CreateModifierTaskFromSource
	}
	return task, nil
}

// GetModifierTasks retrieves all tasks for the modifier, optionally filtered by target_id (0 for no filter).
// It now sorts by display_order ASC, then created_at DESC.
func GetModifierTasks(targetIDFilter int64) ([]models.ModifierTask, error) {
	query := "SELECT id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, created_at, updated_at, source_log_id, source_param_url_id, last_executed_log_id FROM modifier_tasks ORDER BY display_order ASC, created_at DESC"
	// Add targetIDFilter logic if needed

	rows, err := DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying modifier tasks: %w", err)
	}
	defer rows.Close()
	var tasks []models.ModifierTask
	for rows.Next() {
		var t models.ModifierTask
		if err := rows.Scan(&t.ID, &t.TargetID, &t.Name, &t.BaseRequestURL, &t.BaseRequestMethod,
			&t.BaseRequestHeaders, &t.BaseRequestBody,
			&t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt,
			&t.SourceLogID, &t.SourceParamURLID, &t.LastExecutedLogID); err != nil {
			return nil, fmt.Errorf("scanning modifier task row: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetModifierTaskByID retrieves a single modifier task by its ID.
func GetModifierTaskByID(taskID int64) (*models.ModifierTask, error) {
	var t models.ModifierTask
	err := DB.QueryRow("SELECT id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, created_at, updated_at, source_log_id, source_param_url_id, last_executed_log_id FROM modifier_tasks WHERE id = ?", taskID).Scan(
		&t.ID, &t.TargetID, &t.Name, &t.BaseRequestURL, &t.BaseRequestMethod, &t.BaseRequestHeaders, &t.BaseRequestBody, &t.DisplayOrder, &t.CreatedAt, &t.UpdatedAt, &t.SourceLogID, &t.SourceParamURLID, &t.LastExecutedLogID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Task not found
		}
		return nil, fmt.Errorf("querying/scanning modifier task ID %d: %w", taskID, err)
	}
	return &t, nil
}

// UpdateModifierTaskLastExecutedLogID updates the last_executed_log_id for a modifier task.
func UpdateModifierTaskLastExecutedLogID(taskID int64, logID int64) error {
	stmt, err := DB.Prepare("UPDATE modifier_tasks SET last_executed_log_id = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		logger.Error("UpdateModifierTaskLastExecutedLogID: Error preparing statement for task ID %d: %v", taskID, err)
		return fmt.Errorf("preparing update last_executed_log_id statement for task ID %d: %w", taskID, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(logID, time.Now(), taskID)
	if err != nil {
		logger.Error("UpdateModifierTaskLastExecutedLogID: Error executing update for task ID %d, log ID %d: %v", taskID, logID, err)
		return fmt.Errorf("executing update last_executed_log_id for task ID %d: %w", taskID, err)
	}
	return nil
}

// UpdateModifierTaskBaseRequestDetails updates the core request fields of a modifier task.
func UpdateModifierTaskBaseRequestDetails(taskID int64, method, url, headersJSON, bodyBase64 string) error {
	stmt, err := DB.Prepare(`UPDATE modifier_tasks SET
		base_request_method = ?,
		base_request_url = ?,
		base_request_headers = ?,
		base_request_body = ?,
		updated_at = ?
		WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("preparing update base request statement for task ID %d: %w", taskID, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(method, url, models.NullString(headersJSON), models.NullString(bodyBase64), time.Now(), taskID)
	if err != nil {
		return fmt.Errorf("executing update base request for task ID %d: %w", taskID, err)
	}
	logger.Info("Updated base request details for ModifierTask ID: %d", taskID)
	return nil
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

// CloneModifierTaskDB creates a new task by cloning an existing one.
func CloneModifierTaskDB(originalTaskID int64) (*models.ModifierTask, error) {
	originalTask, err := GetModifierTaskByID(originalTaskID)
	if err != nil {
		return nil, fmt.Errorf("fetching original task %d for clone: %w", originalTaskID, err)
	}
	if originalTask == nil {
		return nil, fmt.Errorf("original modifier task with ID %d not found for cloning", originalTaskID)
	}

	clonedTask := *originalTask

	clonedTask.ID = 0
	clonedTask.Name = "Copy of " + originalTask.Name
	if len(clonedTask.Name) > 255 {
		clonedTask.Name = clonedTask.Name[:252] + "..."
	}

	var maxOrder sql.NullInt64
	err = DB.QueryRow("SELECT MAX(display_order) FROM modifier_tasks").Scan(&maxOrder)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("getting max display_order for cloned task: %w", err)
	}
	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}
	clonedTask.DisplayOrder = nextOrder

	// Ensure LastExecutedLogID is included in the INSERT statement
	stmt, err := DB.Prepare(`INSERT INTO modifier_tasks
        (target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, source_log_id, source_param_url_id, last_executed_log_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, fmt.Errorf("preparing insert statement for cloned modifier_task: %w", err)
	}
	defer stmt.Close()

	currentTime := time.Now()
	result, err := stmt.Exec(
		clonedTask.TargetID, clonedTask.Name, clonedTask.BaseRequestURL, clonedTask.BaseRequestMethod,
		clonedTask.BaseRequestHeaders, clonedTask.BaseRequestBody, clonedTask.DisplayOrder,
		clonedTask.SourceLogID, clonedTask.SourceParamURLID, clonedTask.LastExecutedLogID, // Added LastExecutedLogID here
		currentTime, currentTime,
	)
	if err != nil {
		return nil, fmt.Errorf("executing insert for cloned modifier_task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting last insert ID for cloned modifier_task: %w", err)
	}
	clonedTask.ID = id
	clonedTask.CreatedAt = sql.NullTime{Time: currentTime, Valid: true}
	clonedTask.UpdatedAt = sql.NullTime{Time: currentTime, Valid: true}

	logger.Info("Cloned ModifierTask ID %d to new ID: %d, Name: %s", originalTaskID, clonedTask.ID, clonedTask.Name)
	return &clonedTask, nil
}
