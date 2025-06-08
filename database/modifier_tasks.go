package database

import (
	"database/sql"
	"fmt"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// CreateModifierTask inserts a new modifier task into the database.
func CreateModifierTask(task models.ModifierTask) (int64, error) {
	query := `INSERT INTO modifier_tasks (
		target_id, parameterized_url_id, original_log_id, name,
		base_request_method, base_request_url, base_request_headers, base_request_body, notes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := DB.Exec(query,
		task.TargetID, task.ParameterizedURLID, task.OriginalLogID, task.Name,
		task.BaseRequestMethod, task.BaseRequestURL, task.BaseRequestHeaders, task.BaseRequestBody, task.Notes,
	)
	if err != nil {
		logger.Error("CreateModifierTask: Error inserting new task: %v", err)
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("CreateModifierTask: Error getting last insert ID: %v", err)
		return 0, err
	}
	return id, nil
}

// GetModifierTaskByID retrieves a single modifier task by its ID.
func GetModifierTaskByID(id int64) (models.ModifierTask, error) {
	var task models.ModifierTask
	query := `SELECT id, target_id, parameterized_url_id, original_log_id, name,
	                 base_request_method, base_request_url, base_request_headers, base_request_body,
	                 notes, created_at, updated_at
	          FROM modifier_tasks WHERE id = ?`
	err := DB.QueryRow(query, id).Scan(
		&task.ID, &task.TargetID, &task.ParameterizedURLID, &task.OriginalLogID, &task.Name,
		&task.BaseRequestMethod, &task.BaseRequestURL, &task.BaseRequestHeaders, &task.BaseRequestBody,
		&task.Notes, &task.CreatedAt, &task.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return task, fmt.Errorf("modifier task with ID %d not found", id)
		}
		logger.Error("GetModifierTaskByID: Error scanning task ID %d: %v", id, err)
		return task, err
	}
	return task, nil
}

// GetModifierTasks retrieves a list of modifier tasks, ordered by creation date.
// Future enhancements: pagination, filtering by target_id.
func GetModifierTasks(targetID sql.NullInt64) ([]models.ModifierTask, error) {
	var tasks []models.ModifierTask
	// Select a subset of fields for the list view for now.
	// Can be expanded to include more if needed by the list, or a separate GetModifierTaskDetails.
	query := `SELECT id, target_id, parameterized_url_id, original_log_id, name,
	                 base_request_method, base_request_url, display_order, created_at, updated_at
	          FROM modifier_tasks`

	args := []interface{}{}
	if targetID.Valid {
		query += " WHERE target_id = ?"
		args = append(args, targetID.Int64)
	}
	query += " ORDER BY display_order ASC, created_at DESC LIMIT 200" // Order by display_order, then by creation

	rows, err := DB.Query(query, args...)
	if err != nil {
		logger.Error("GetModifierTasks: Error querying tasks: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task models.ModifierTask
		// Scan only the fields selected in the query above
		if err := rows.Scan(&task.ID, &task.TargetID, &task.ParameterizedURLID, &task.OriginalLogID, &task.Name,
			&task.BaseRequestMethod, &task.BaseRequestURL, &task.DisplayOrder, &task.CreatedAt, &task.UpdatedAt); err != nil {
			logger.Error("GetModifierTasks: Error scanning task row: %v", err)
			return nil, err // Or continue and log, depending on desired behavior
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// UpdateModifierTaskName updates the name and updated_at timestamp of a specific modifier task.
func UpdateModifierTaskName(taskID int64, newName string) (models.ModifierTask, error) {
	stmt, err := DB.Prepare("UPDATE modifier_tasks SET name = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		logger.Error("UpdateModifierTaskName: Error preparing statement: %v", err)
		return models.ModifierTask{}, fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	updatedAt := time.Now()
	result, err := stmt.Exec(newName, updatedAt, taskID)
	if err != nil {
		logger.Error("UpdateModifierTaskName: Error executing statement for task ID %d: %v", taskID, err)
		return models.ModifierTask{}, fmt.Errorf("failed to update task name for ID %d: %w", taskID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return models.ModifierTask{}, fmt.Errorf("task with ID %d not found for update", taskID)
	}

	// Fetch the updated task to return it, ensuring all fields are current
	updatedTask, err := GetModifierTaskByID(taskID) // Assumes GetModifierTaskByID correctly fetches the task
	if err != nil {
		return models.ModifierTask{}, fmt.Errorf("failed to retrieve task after update for ID %d: %w", taskID, err)
	}

	logger.Info("Modifier task ID %d name updated to '%s'", taskID, newName)
	return updatedTask, nil
}

// UpdateModifierTasksOrder updates the display_order for a list of tasks.
// It expects a map where keys are task IDs and values are their new display order.
func UpdateModifierTasksOrder(taskOrders map[int64]int) error {
	tx, err := DB.Begin()
	if err != nil {
		logger.Error("UpdateModifierTasksOrder: Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare("UPDATE modifier_tasks SET display_order = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		tx.Rollback()
		logger.Error("UpdateModifierTasksOrder: Failed to prepare statement: %v", err)
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	updatedAt := time.Now()
	for taskID, order := range taskOrders {
		if _, err := stmt.Exec(order, updatedAt, taskID); err != nil {
			tx.Rollback()
			logger.Error("UpdateModifierTasksOrder: Failed to update order for task ID %d: %v", taskID, err)
			return fmt.Errorf("failed to update order for task ID %d: %w", taskID, err)
		}
	}

	logger.Info("Successfully updated display order for %d modifier tasks.", len(taskOrders))
	return tx.Commit()
}
