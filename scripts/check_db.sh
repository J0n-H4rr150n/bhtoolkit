#!/bin/bash

# Define the path to your SQLite database
DB_FILE_PATH="${HOME}/.config/toolkit/bountytool.db"

# Check if the database file exists
if [ ! -f "$DB_FILE_PATH" ]; then
    echo "Error: Database file not found at $DB_FILE_PATH"
    exit 1
fi

# Command to list tables in SQLite
SQLITE_COMMAND=".tables"

echo "Tables in database: $DB_FILE_PATH"
echo "------------------------------------"

# Execute the command using the sqlite3 CLI
sqlite3 "$DB_FILE_PATH" "$SQLITE_COMMAND"

echo "------------------------------------"
