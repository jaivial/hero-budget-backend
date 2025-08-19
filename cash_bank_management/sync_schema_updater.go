package main

import (
	"database/sql"
	"log"
)

// updateSyncOperationsSchema ensures sync_operations table has the correct schema
// for cash_bank operations (transfer, update_cash, update_bank)
// This function updates the CHECK constraint to include cash_bank operation types
func updateSyncOperationsSchema() error {
	log.Println("🔄 Checking and updating sync_operations schema for cash_bank operations...")
	
	// Check if sync_operations table exists
	var tableName string
	checkTableSQL := "SELECT name FROM sqlite_master WHERE type='table' AND name='sync_operations'"
	err := db.QueryRow(checkTableSQL).Scan(&tableName)
	
	if err == sql.ErrNoRows {
		log.Println("⚠️ sync_operations table does not exist - will be created by centralized schema")
		return nil
	} else if err != nil {
		log.Printf("❌ Error checking sync_operations table: %v", err)
		return err
	}
	
	// Check if the constraint allows cash_bank operation types
	// We do this by trying to insert a test row with operation_type='transfer'
	testSQL := `INSERT INTO sync_operations (operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data, device_ids) 
				VALUES ('test_schema_check', 'test_user', 0, 'transfer', 'test_entity', 'test_id', '{}', '[]')`
	
	_, err = db.Exec(testSQL)
	
	if err != nil {
		// Error means the constraint doesn't allow 'transfer', so we need to update
		log.Println("🔧 sync_operations schema needs update - adding cash_bank operation types")
		
		// Begin transaction for schema update
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		
		// Create new table with updated constraint
		createNewTableSQL := `
		CREATE TABLE sync_operations_new (
			operation_id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete', 'pay', 'transfer', 'update_cash', 'update_bank')),
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			operation_data TEXT NOT NULL,
			device_ids TEXT DEFAULT '[]',
			client_timestamp INTEGER DEFAULT 0,
			server_timestamp INTEGER DEFAULT 0
		)`
		
		_, err = tx.Exec(createNewTableSQL)
		if err != nil {
			log.Printf("❌ Error creating new sync_operations table: %v", err)
			return err
		}
		
		// Copy existing data to new table
		copyDataSQL := `INSERT INTO sync_operations_new 
						SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, 
							   operation_data, device_ids, client_timestamp, server_timestamp 
						FROM sync_operations`
		
		_, err = tx.Exec(copyDataSQL)
		if err != nil {
			log.Printf("❌ Error copying data to new table: %v", err)
			return err
		}
		
		// Drop old table and rename new one
		_, err = tx.Exec("DROP TABLE sync_operations")
		if err != nil {
			log.Printf("❌ Error dropping old table: %v", err)
			return err
		}
		
		_, err = tx.Exec("ALTER TABLE sync_operations_new RENAME TO sync_operations")
		if err != nil {
			log.Printf("❌ Error renaming new table: %v", err)
			return err
		}
		
		// Recreate indexes
		indexes := []string{
			"CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id ON sync_operations(operation_id)",
			"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation ON sync_operations(user_id, operation_id)",
			"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created ON sync_operations(user_id, created_at)",
			"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity ON sync_operations(user_id, entity_type, entity_id)",
			"CREATE INDEX IF NOT EXISTS idx_sync_operations_cash_bank ON sync_operations(user_id, entity_type) WHERE entity_type IN ('cash_bank_transfers', 'cash_bank_updates')",
		}
		
		for _, indexSQL := range indexes {
			_, err = tx.Exec(indexSQL)
			if err != nil {
				log.Printf("⚠️ Warning: Error creating index: %v", err)
				// Continue with other indexes
			}
		}
		
		// Commit transaction
		err = tx.Commit()
		if err != nil {
			log.Printf("❌ Error committing schema update: %v", err)
			return err
		}
		
		log.Println("✅ sync_operations schema updated successfully - now supports cash_bank operation types")
		
	} else {
		// Test insert succeeded, so we need to clean it up
		_, err = db.Exec("DELETE FROM sync_operations WHERE operation_id='test_schema_check'")
		if err != nil {
			log.Printf("⚠️ Warning: Could not clean up test record: %v", err)
		}
		log.Println("✅ sync_operations schema is already correct - supports cash_bank operation types")
	}
	
	return nil
}