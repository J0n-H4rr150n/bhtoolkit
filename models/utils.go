package models

import (
	"database/sql"
	"encoding/base64"
)

// ConvertInt64PtrToSQLNullInt64 converts a pointer to an int64 to sql.NullInt64.
func ConvertInt64PtrToSQLNullInt64(ptr *int64) sql.NullInt64 {
	if ptr == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: *ptr, Valid: true}
}

// Base64Encode encodes a byte slice to a base64 string.
func Base64Encode(data []byte) string {
	if data == nil {
		return "" // Or handle as an error, depending on requirements
	}
	return base64.StdEncoding.EncodeToString(data)
}

// You might also need a Base64Decode function if you store base64 and need to read it back as bytes.
