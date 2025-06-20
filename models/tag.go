package models

import (
	"database/sql"
	"time"
)

// Tag represents a single tag that can be applied to various items.
type Tag struct {
	ID        int64          `json:"id" readOnly:"true"`
	Name      string         `json:"name" binding:"required"`
	Color     sql.NullString `json:"color,omitempty"` // Optional: e.g., hex code like #FF0000
	CreatedAt time.Time      `json:"created_at" readOnly:"true"`
	UpdatedAt time.Time      `json:"updated_at" readOnly:"true"`
}

// TagAssociation represents the link between a Tag and an item.
type TagAssociation struct {
	ID        int64     `json:"id" readOnly:"true"`
	TagID     int64     `json:"tag_id" binding:"required"`
	ItemID    int64     `json:"item_id" binding:"required"`   // ID of the item being tagged
	ItemType  string    `json:"item_type" binding:"required"` // Type of the item (e.g., "httplog", "domain", "note")
	CreatedAt time.Time `json:"created_at" readOnly:"true"`
	UpdatedAt time.Time `json:"updated_at" readOnly:"true"`
}
