package main

import (
	"io/ioutil"
	"log"
	"path/filepath"
)

// initializeSyncOperationsSchema creates the sync_operations table if it doesn't exist
// Uses the schema defined in sync_operations_schema.sql
func initializeSyncOperationsSchema() error {
	log.Println("🔧 Initializing sync operations schema...")

	// Read the schema file
	schemaPath := filepath.Join(".", "sync_operations_schema.sql")
	schemaSQL, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		log.Printf("Warning: Could not read sync operations schema file: %v", err)
		log.Println("Creating schema directly...")

		// Fallback: create the schema directly if file is not found
		return createSyncOperationsSchemaDirectly()
	}

	// Execute the schema SQL
	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		log.Printf("Error executing sync operations schema: %v", err)
		return err
	}

	log.Println("✅ Sync operations schema initialized successfully")
	return nil
}

// createSyncOperationsSchemaDirectly creates the schema without reading from file
// Fallback method if schema file is not accessible
func createSyncOperationsSchemaDirectly() error {
	schemaSQL := `
		-- Create sync_operations table with proper constraints for expense management
		CREATE TABLE IF NOT EXISTS sync_operations (
			operation_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete')),
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			operation_data TEXT NOT NULL,
			device_ids TEXT DEFAULT '[]',
			client_timestamp INTEGER DEFAULT 0,
			server_timestamp INTEGER DEFAULT 0
		);

		-- Create necessary indexes for performance
		CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
			ON sync_operations(operation_id);

		CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
			ON sync_operations(user_id, operation_id);

		CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
			ON sync_operations(user_id, created_at);

		CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
			ON sync_operations(user_id, entity_type, entity_id);
	`

	_, err := db.Exec(schemaSQL)
	if err != nil {
		log.Printf("Error creating sync operations schema directly: %v", err)
		return err
	}

	log.Println("✅ Sync operations schema created directly")
	return nil
}
