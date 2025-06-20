package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"toolkit/logger"
	"toolkit/models"
)

// CreateTag inserts a new tag into the database.
// It ensures the tag name is unique (case-insensitive).
func CreateTag(tag models.Tag) (models.Tag, error) {
	if DB == nil {
		return models.Tag{}, errors.New("database connection is not initialized")
	}
	tag.Name = strings.TrimSpace(tag.Name)
	if tag.Name == "" {
		return models.Tag{}, errors.New("tag name cannot be empty")
	}

	// Check if tag with the same name (case-insensitive) already exists
	var existingTag models.Tag
	err := DB.QueryRow("SELECT id, name, color, created_at, updated_at FROM tags WHERE LOWER(name) = LOWER(?)", tag.Name).Scan(
		&existingTag.ID, &existingTag.Name, &existingTag.Color, &existingTag.CreatedAt, &existingTag.UpdatedAt,
	)
	if err == nil {
		// Tag already exists, return it
		logger.Info("CreateTag: Tag with name '%s' already exists (ID: %d). Returning existing tag.", tag.Name, existingTag.ID)
		return existingTag, nil
	}
	if err != sql.ErrNoRows {
		logger.Error("CreateTag: Error checking for existing tag '%s': %v", tag.Name, err)
		return models.Tag{}, fmt.Errorf("checking for existing tag '%s': %w", tag.Name, err)
	}

	// Tag does not exist, create it
	stmt, err := DB.Prepare("INSERT INTO tags (name, color) VALUES (?, ?)")
	if err != nil {
		logger.Error("CreateTag: Error preparing statement for tag '%s': %v", tag.Name, err)
		return models.Tag{}, fmt.Errorf("preparing insert tag statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(tag.Name, tag.Color)
	if err != nil {
		logger.Error("CreateTag: Error executing insert for tag '%s': %v", tag.Name, err)
		return models.Tag{}, fmt.Errorf("executing insert tag: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("CreateTag: Error getting last insert ID for tag '%s': %v", tag.Name, err)
		return models.Tag{}, fmt.Errorf("getting last insert ID for tag: %w", err)
	}
	tag.ID = id

	// Re-fetch to get created_at and updated_at
	createdTag, fetchErr := GetTagByID(id)
	if fetchErr != nil {
		logger.Error("CreateTag: Error fetching newly created tag %d: %v", id, fetchErr)
		return tag, nil // Return the tag with ID, but timestamps might be zero
	}
	return createdTag, nil
}

// GetTagByID retrieves a single tag by its ID.
func GetTagByID(id int64) (models.Tag, error) {
	var tag models.Tag
	if DB == nil {
		return tag, errors.New("database connection is not initialized")
	}
	err := DB.QueryRow("SELECT id, name, color, created_at, updated_at FROM tags WHERE id = ?", id).Scan(
		&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tag, fmt.Errorf("tag with ID %d not found", id)
		}
		logger.Error("GetTagByID: Error querying tag ID %d: %v", id, err)
		return tag, fmt.Errorf("querying tag ID %d: %w", id, err)
	}
	return tag, nil
}

// GetAllTags retrieves all tags from the database, ordered by name.
func GetAllTags() ([]models.Tag, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	rows, err := DB.Query("SELECT id, name, color, created_at, updated_at FROM tags ORDER BY LOWER(name) ASC")
	if err != nil {
		logger.Error("GetAllTags: Error querying all tags: %v", err)
		return nil, fmt.Errorf("querying all tags: %w", err)
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			logger.Error("GetAllTags: Error scanning tag row: %v", err)
			return nil, fmt.Errorf("scanning tag row: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// DeleteTag removes a tag from the database.
// Associated tag_associations will be deleted by CASCADE.
func DeleteTag(id int64) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare("DELETE FROM tags WHERE id = ?")
	if err != nil {
		logger.Error("DeleteTag: Error preparing statement for tag ID %d: %v", id, err)
		return fmt.Errorf("preparing delete tag statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(id)
	if err != nil {
		logger.Error("DeleteTag: Error executing delete for tag ID %d: %v", id, err)
		return fmt.Errorf("executing delete tag: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("tag with ID %d not found for deletion", id)
	}
	logger.Info("DeleteTag: Tag ID %d deleted.", id)
	return nil
}

// AssociateTag links a tag to a specific item.
// itemID is the ID of the item being tagged (e.g., http_traffic_log.id).
// itemType is a string identifying the type of item (e.g., "httplog", "domain").
func AssociateTag(tagID int64, itemID int64, itemType string) (models.TagAssociation, error) {
	var association models.TagAssociation
	if DB == nil {
		return association, errors.New("database connection is not initialized")
	}
	itemType = strings.ToLower(strings.TrimSpace(itemType))
	if itemType == "" {
		return association, errors.New("item_type cannot be empty")
	}

	// Check if association already exists
	var existingAssociationID int64
	err := DB.QueryRow("SELECT id FROM tag_associations WHERE tag_id = ? AND item_id = ? AND item_type = ?", tagID, itemID, itemType).Scan(&existingAssociationID)
	if err == nil {
		// Association already exists, fetch and return it
		return GetTagAssociationByID(existingAssociationID)
	}
	if err != sql.ErrNoRows {
		logger.Error("AssociateTag: Error checking for existing association for tag_id %d, item_id %d, item_type '%s': %v", tagID, itemID, itemType, err)
		return association, fmt.Errorf("checking for existing tag association: %w", err)
	}

	stmt, err := DB.Prepare("INSERT INTO tag_associations (tag_id, item_id, item_type) VALUES (?, ?, ?)")
	if err != nil {
		logger.Error("AssociateTag: Error preparing statement: %v", err)
		return association, fmt.Errorf("preparing insert tag association statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(tagID, itemID, itemType)
	if err != nil {
		logger.Error("AssociateTag: Error executing insert for tag_id %d, item_id %d, item_type '%s': %v", tagID, itemID, itemType, err)
		return association, fmt.Errorf("executing insert tag association: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("AssociateTag: Error getting last insert ID: %v", err)
		return association, fmt.Errorf("getting last insert ID for tag association: %w", err)
	}
	return GetTagAssociationByID(id)
}

// GetTagAssociationByID retrieves a single tag association by its ID.
func GetTagAssociationByID(id int64) (models.TagAssociation, error) {
	var association models.TagAssociation
	if DB == nil {
		return association, errors.New("database connection is not initialized")
	}
	err := DB.QueryRow("SELECT id, tag_id, item_id, item_type, created_at, updated_at FROM tag_associations WHERE id = ?", id).Scan(
		&association.ID, &association.TagID, &association.ItemID, &association.ItemType, &association.CreatedAt, &association.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return association, fmt.Errorf("tag association with ID %d not found", id)
		}
		logger.Error("GetTagAssociationByID: Error querying association ID %d: %v", id, err)
		return association, fmt.Errorf("querying tag association ID %d: %w", id, err)
	}
	return association, nil
}

// RemoveTagAssociation removes a specific link between a tag and an item.
func RemoveTagAssociation(tagID int64, itemID int64, itemType string) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	itemType = strings.ToLower(strings.TrimSpace(itemType))

	stmt, err := DB.Prepare("DELETE FROM tag_associations WHERE tag_id = ? AND item_id = ? AND item_type = ?")
	if err != nil {
		logger.Error("RemoveTagAssociation: Error preparing statement: %v", err)
		return fmt.Errorf("preparing delete tag association statement: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(tagID, itemID, itemType); err != nil {
		logger.Error("RemoveTagAssociation: Error executing delete for tag_id %d, item_id %d, item_type '%s': %v", tagID, itemID, itemType, err)
		return fmt.Errorf("executing delete tag association: %w", err)
	}
	logger.Info("RemoveTagAssociation: Association removed for tag_id %d, item_id %d, item_type '%s'.", tagID, itemID, itemType)
	return nil
}

// GetTagsForItem retrieves all tags associated with a specific item.
func GetTagsForItem(itemID int64, itemType string) ([]models.Tag, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	itemType = strings.ToLower(strings.TrimSpace(itemType))

	query := `
		SELECT t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN tag_associations ta ON t.id = ta.tag_id
		WHERE ta.item_id = ? AND ta.item_type = ?
		ORDER BY LOWER(t.name) ASC
	`
	rows, err := DB.Query(query, itemID, itemType)
	if err != nil {
		logger.Error("GetTagsForItem: Error querying tags for item_id %d, item_type '%s': %v", itemID, itemType, err)
		return nil, fmt.Errorf("querying tags for item: %w", err)
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			logger.Error("GetTagsForItem: Error scanning tag row: %v", err)
			return nil, fmt.Errorf("scanning tag row for item: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetItemsForTag retrieves all items (IDs and types) associated with a specific tag.
// This is more of a utility function and might need to be adapted for specific UI needs
// if you want to display actual item details rather than just IDs.
func GetItemsForTag(tagID int64) ([]models.TagAssociation, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}

	query := `
		SELECT id, tag_id, item_id, item_type, created_at, updated_at
		FROM tag_associations
		WHERE tag_id = ?
		ORDER BY item_type ASC, item_id ASC
	`
	rows, err := DB.Query(query, tagID)
	if err != nil {
		logger.Error("GetItemsForTag: Error querying items for tag_id %d: %v", tagID, err)
		return nil, fmt.Errorf("querying items for tag: %w", err)
	}
	defer rows.Close()

	var associations []models.TagAssociation
	for rows.Next() {
		var assoc models.TagAssociation
		if err := rows.Scan(&assoc.ID, &assoc.TagID, &assoc.ItemID, &assoc.ItemType, &assoc.CreatedAt, &assoc.UpdatedAt); err != nil {
			logger.Error("GetItemsForTag: Error scanning item association row: %v", err)
			return nil, fmt.Errorf("scanning item association row for tag: %w", err)
		}
		associations = append(associations, assoc)
	}
	return associations, rows.Err()
}

// UpdateTag updates an existing tag's name and/or color.
// It only updates fields that are non-zero/non-empty in the provided models.Tag struct.
func UpdateTag(tag models.Tag) (models.Tag, error) {
	if DB == nil {
		return models.Tag{}, errors.New("database connection is not initialized")
	}
	if tag.ID == 0 {
		return models.Tag{}, errors.New("tag ID is required for update")
	}

	// Build the update query dynamically based on provided fields
	var setClauses []string
	var args []interface{}

	if strings.TrimSpace(tag.Name) != "" {
		setClauses = append(setClauses, "name = ?")
		args = append(args, strings.TrimSpace(tag.Name))
	}
	// For color, we update if it's explicitly provided (Valid=true), even if the string is empty (to clear it)
	if tag.Color.Valid {
		setClauses = append(setClauses, "color = ?")
		args = append(args, tag.Color)
	}

	if len(setClauses) == 0 {
		// No fields to update, just fetch and return the existing tag
		return GetTagByID(tag.ID)
	}

	setClauses = append(setClauses, "updated_at = CURRENT_TIMESTAMP")
	query := fmt.Sprintf("UPDATE tags SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	args = append(args, tag.ID)

	stmt, err := DB.Prepare(query)
	if err != nil {
		logger.Error("UpdateTag: Error preparing update statement for tag %d: %v. Query: %s", tag.ID, err, query)
		return models.Tag{}, fmt.Errorf("preparing tag update: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(args...)
	if err != nil {
		logger.Error("UpdateTag: Error executing update for tag %d: %v", tag.ID, err)
		return models.Tag{}, fmt.Errorf("executing tag update: %w", err)
	}

	logger.Info("Successfully updated tag ID %d", tag.ID)
	return GetTagByID(tag.ID) // Return the updated tag
}

// DeleteTagAndAssociations deletes a tag and all its associations.
func DeleteTagAndAssociations(tagID int64) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	// This will be handled by ON DELETE CASCADE defined in the schema for tag_associations.
	// So, we only need to delete the tag itself.
	return DeleteTag(tagID) // DeleteTag function already exists and handles deleting from 'tags' table.
}

// AssociateTagWithItem creates an association between a tag and an item.
// It checks if the association already exists to prevent duplicates.
func AssociateTagWithItem(tagID int64, itemID int64, itemType string) (models.TagAssociation, error) {
	var association models.TagAssociation
	if DB == nil {
		return association, errors.New("database connection is not initialized")
	}

	itemType = strings.ToLower(strings.TrimSpace(itemType))
	if tagID == 0 || itemID == 0 || itemType == "" {
		return association, errors.New("tag_id, item_id, and item_type are required")
	}

	// Check if association already exists
	var existingID int64
	err := DB.QueryRow("SELECT id FROM tag_associations WHERE tag_id = ? AND item_id = ? AND item_type = ?", tagID, itemID, itemType).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		logger.Error("AssociateTagWithItem: Error checking for existing association: %v", err)
		return association, fmt.Errorf("checking for existing association: %w", err)
	}
	if err == nil { // Association already exists
		logger.Info("AssociateTagWithItem: Association already exists for tag_id %d, item_id %d, item_type '%s' (ID: %d)", tagID, itemID, itemType, existingID)
		// Return the existing association details (or just a success message)
		// For now, let's just return the input as if it were created, or fetch the existing one.
		// For simplicity, we'll assume the frontend is okay with just a success.
		// To be more robust, fetch and return the existing one.
		// For now, we'll just indicate success. The handler expects an association object.
		return models.TagAssociation{ID: existingID, TagID: tagID, ItemID: itemID, ItemType: itemType /* CreatedAt/UpdatedAt would be from DB */}, nil
	}

	stmt, err := DB.Prepare("INSERT INTO tag_associations (tag_id, item_id, item_type) VALUES (?, ?, ?)")
	if err != nil {
		logger.Error("AssociateTagWithItem: Error preparing insert statement: %v", err)
		return association, fmt.Errorf("preparing association insert: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(tagID, itemID, itemType)
	if err != nil {
		logger.Error("AssociateTagWithItem: Error executing insert for tag_id %d, item_id %d, item_type '%s': %v", tagID, itemID, itemType, err)
		return association, fmt.Errorf("executing association insert: %w", err)
	}

	id, _ := result.LastInsertId()
	association.ID = id
	association.TagID = tagID
	association.ItemID = itemID
	association.ItemType = itemType
	// association.CreatedAt and association.UpdatedAt would be set by DB defaults.

	logger.Info("Successfully associated tag %d with item %d (type: %s). New association ID: %d", tagID, itemID, itemType, id)
	return association, nil
}

// GetTagsForMultipleItems retrieves all tags for a list of item IDs of a specific type.
// Returns a map where key is itemID and value is a slice of tags.
func GetTagsForMultipleItems(itemIDs []int64, itemType string) (map[int64][]models.Tag, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	if len(itemIDs) == 0 {
		return make(map[int64][]models.Tag), nil
	}

	itemType = strings.ToLower(strings.TrimSpace(itemType))
	if itemType == "" {
		return nil, errors.New("item_type cannot be empty")
	}

	placeholders := strings.Repeat("?,", len(itemIDs)-1) + "?"
	query := fmt.Sprintf(`
		SELECT ta.item_id, t.id, t.name, t.color, t.created_at, t.updated_at
		FROM tags t
		JOIN tag_associations ta ON t.id = ta.tag_id
		WHERE ta.item_id IN (%s) AND ta.item_type = ?
		ORDER BY ta.item_id ASC, LOWER(t.name) ASC
	`, placeholders)

	args := make([]interface{}, 0, len(itemIDs)+1)
	for _, id := range itemIDs {
		args = append(args, id)
	}
	args = append(args, itemType)

	rows, err := DB.Query(query, args...)
	if err != nil {
		logger.Error("GetTagsForMultipleItems: Error querying tags for items: %v. Query: %s, Args: %v", err, query, args)
		return nil, fmt.Errorf("querying tags for multiple items: %w", err)
	}
	defer rows.Close()

	tagsByItemID := make(map[int64][]models.Tag)
	for rows.Next() {
		var itemID int64
		var tag models.Tag
		if err := rows.Scan(&itemID, &tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			logger.Error("GetTagsForMultipleItems: Error scanning tag row: %v", err)
			continue
		}
		tagsByItemID[itemID] = append(tagsByItemID[itemID], tag)
	}
	return tagsByItemID, rows.Err()
}
