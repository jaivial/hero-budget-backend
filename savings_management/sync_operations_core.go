package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Operation ID utility functions for timestamp-based format
// Implements the sync operations system following the implementation guide

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number} (e.g., 1755209423000_001)
func isValidOperationId(operationId string) bool {
	if operationId == "" {
		return false
	}
	
	// Expected format: 1755209423000_001 (13 digits timestamp + underscore + 3 digits sequence)
	operationIdPattern := `^\d{13}_\d{3}$`
	matched, err := regexp.MatchString(operationIdPattern, operationId)
	if err != nil {
		log.Printf("Error validating operation ID pattern: %v", err)
		return false
	}
	
	return matched
}

// extractTimestampFromOperationId extracts timestamp from operation ID
// Returns 0 if the operation ID format is invalid
func extractTimestampFromOperationId(operationId string) int64 {
	if !isValidOperationId(operationId) {
		return 0
	}
	
	parts := strings.Split(operationId, "_")
	if len(parts) != 2 {
		return 0
	}
	
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		log.Printf("Error parsing timestamp from operation ID: %v", err)
		return 0
	}
	
	return timestamp
}

// getLastGlobalOperationId retrieves the globally last valid timestamp-format operation ID across all users
// Used for generating chronologically ordered operation IDs that maintain global sync order
func getLastGlobalOperationId() (string, error) {
	// Get all operation IDs and filter in Go code since SQLite REGEXP is not always available
	rows, err := db.Query("SELECT operation_id FROM sync_operations ORDER BY operation_id DESC")
	if err != nil {
		log.Printf("Error querying sync_operations: %v", err)
		return "", err
	}
	defer rows.Close()
	
	// Look for the first operation ID that matches timestamp format: 13-digit timestamp + underscore + 3-digit sequence
	timestampPattern := regexp.MustCompile(`^\d{13}_\d{3}$`)
	
	for rows.Next() {
		var operationId string
		if err := rows.Scan(&operationId); err != nil {
			log.Printf("Error scanning operation_id: %v", err)
			continue
		}
		
		// Check if this operation ID matches our timestamp format
		if timestampPattern.MatchString(operationId) {
			log.Printf("Retrieved last global timestamp-format operation ID: %s", operationId)
			return operationId, nil
		}
	}
	
	// No timestamp-format operation IDs found
	log.Printf("No previous timestamp-format operations found in sync_operations table")
	return "", nil
}

// generateNextOperationId generates the next operation ID maintaining global chronological order
// Gets the globally last operation ID and adds +1 millisecond for proper sync ordering
func generateNextOperationId(userID string) (string, error) {
	log.Printf("Generating next operation ID for user: %s (using global ordering)", userID)
	
	// Get the globally last operation ID (across all users) for proper sync ordering
	lastOperationId, err := getLastGlobalOperationId()
	if err != nil {
		return "", fmt.Errorf("failed to get last global operation ID: %v", err)
	}
	
	var nextTimestamp int64
	var sequenceNumber int = 1
	
	if lastOperationId == "" {
		// No previous operations in the entire table, start with current timestamp
		nextTimestamp = time.Now().UnixMilli()
		log.Printf("No previous operations found globally, starting with timestamp: %d", nextTimestamp)
	} else {
		// Extract timestamp from the globally last operation ID
		lastTimestamp := extractTimestampFromOperationId(lastOperationId)
		if lastTimestamp == 0 {
			// Invalid last operation ID format, use current timestamp
			nextTimestamp = time.Now().UnixMilli()
			log.Printf("Invalid last operation ID format, using current timestamp: %d", nextTimestamp)
		} else {
			// Add 1 millisecond to ensure chronological ordering globally
			nextTimestamp = lastTimestamp + 1
			log.Printf("Incremented timestamp from global last %d to %d", lastTimestamp, nextTimestamp)
		}
	}
	
	// Format as {timestamp_ms}_{sequence_number} with zero-padded sequence
	operationId := fmt.Sprintf("%d_%03d", nextTimestamp, sequenceNumber)
	
	log.Printf("Generated operation ID: %s (maintains global sync order)", operationId)
	return operationId, nil
}

// updateSyncOperationsSchema ensures the sync_operations table has proper constraints
// Auto-detects and fixes missing operation types (like 'update_savings', 'delete_savings')
func updateSyncOperationsSchema() error {
	log.Printf("🔧 Checking and updating sync_operations schema for savings management...")
	
	// Test if we can insert an operation with savings-specific operation types
	testOperationId := fmt.Sprintf("%d_001", time.Now().UnixMilli())
	
	// Try to insert a test operation to detect constraint violations
	testQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, operation_type, entity_type, entity_id, operation_data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	testData, _ := json.Marshal(map[string]interface{}{"test": true})
	deviceIds, _ := json.Marshal([]string{})
	currentTime := time.Now().UnixMilli()
	
	_, err := db.Exec(testQuery,
		"test_user",
		testOperationId,
		"update",  // Test standard operation type
		"savings",
		"test_id",
		string(testData),
		string(deviceIds),
		currentTime,
		currentTime,
		currentTime,
	)
	
	if err != nil {
		log.Printf("⚠️ Standard operation type failed, schema might need updating: %v", err)
		
		// Try to update the schema to include all required operation types
		log.Printf("🔄 Attempting to update sync_operations schema...")
		
		// SQLite doesn't support ALTER CHECK constraint, so we need to recreate the table
		updateSchemaQuery := `
			-- Create temporary table with updated schema
			CREATE TABLE IF NOT EXISTS sync_operations_new (
				operation_id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete', 'pay', 'transfer', 'update_cash', 'update_bank', 'update_savings', 'delete_savings')),
				entity_type TEXT NOT NULL,
				entity_id TEXT NOT NULL,
				operation_data TEXT NOT NULL,
				device_ids TEXT DEFAULT '[]',
				client_timestamp INTEGER DEFAULT 0,
				server_timestamp INTEGER DEFAULT 0
			);
			
			-- Copy existing data to new table
			INSERT OR IGNORE INTO sync_operations_new 
			SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, 
				   operation_data, device_ids, client_timestamp, server_timestamp 
			FROM sync_operations;
			
			-- Drop old table and rename new one
			DROP TABLE IF EXISTS sync_operations_old;
			ALTER TABLE sync_operations RENAME TO sync_operations_old;
			ALTER TABLE sync_operations_new RENAME TO sync_operations;
			DROP TABLE sync_operations_old;
			
			-- Recreate indexes for performance
			CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
				ON sync_operations(operation_id);
			
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
				ON sync_operations(user_id, operation_id);
			
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
				ON sync_operations(user_id, created_at);
			
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
				ON sync_operations(user_id, entity_type, entity_id);
		`
		
		_, err = db.Exec(updateSchemaQuery)
		if err != nil {
			log.Printf("❌ Failed to update sync_operations schema: %v", err)
			return fmt.Errorf("failed to update sync_operations schema: %v", err)
		}
		
		log.Printf("✅ Successfully updated sync_operations schema")
	} else {
		// Clean up test data
		db.Exec("DELETE FROM sync_operations WHERE user_id = ?", "test_user")
		log.Printf("✅ sync_operations schema is already compatible")
	}
	
	return nil
}

// Enhanced addSyncOperation function following the implementation guide
// Records a sync operation with proper operation ID generation and validation
func addSyncOperation(userID, providedOperationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, provided_operation=%s, action=%s, table=%s, record=%s, device=%s", 
		userID, providedOperationID, action, tableName, recordID, deviceID)
	
	// Generate operation ID if not provided or if provided ID is not valid timestamp format
	var operationID string
	var err error
	
	if providedOperationID != "" && isValidOperationId(providedOperationID) {
		// Use provided operation ID if it's valid
		operationID = providedOperationID
		log.Printf("Using provided operation ID: %s", operationID)
	} else {
		// Generate new timestamp-based operation ID
		operationID, err = generateNextOperationId(userID)
		if err != nil {
			log.Printf("Error generating operation ID: %v", err)
			return fmt.Errorf("failed to generate operation ID: %v", err)
		}
		log.Printf("Generated new operation ID: %s (provided was: %s)", operationID, providedOperationID)
	}
	
	// Validate that we have a valid operation ID
	if !isValidOperationId(operationID) {
		return fmt.Errorf("invalid operation ID format: %s", operationID)
	}
	
	// Serialize operation data to JSON for storage
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling sync operation data: %v", err)
		return err
	}
	
	// Prepare device_ids JSON array - store null if deviceID is empty
	var deviceIDsJSON []byte
	if deviceID != "" {
		deviceIDs := []string{deviceID}
		deviceIDsJSON, err = json.Marshal(deviceIDs)
		if err != nil {
			log.Printf("Error marshaling device_ids: %v", err)
			return err
		}
	} else {
		deviceIDsJSON = []byte("null")
		log.Printf("Device ID empty, storing null in device_ids column")
	}
	
	// Extract timestamp from operation ID for created_at field
	operationTimestamp := extractTimestampFromOperationId(operationID)
	if operationTimestamp == 0 {
		operationTimestamp = time.Now().UnixMilli()
		log.Printf("Warning: Could not extract timestamp from operation ID, using current timestamp: %d", operationTimestamp)
	}
	
	// Use current server timestamp
	serverTimestamp := time.Now().UnixMilli()
	
	// Handle client timestamp - use null if 0
	var clientTimestampValue interface{}
	if clientTimestamp == 0 {
		clientTimestampValue = nil
		log.Printf("Client timestamp is 0, storing null in client_timestamp column")
	} else {
		clientTimestampValue = clientTimestamp
	}
	
	// Insert sync operation record with operation_id-based ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, operation_type, entity_type, entity_id, operation_data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	result, err := db.Exec(
		insertQuery,
		userID,
		operationID,
		action,            // operation_type (create, update, delete, update_savings, delete_savings)
		tableName,         // entity_type (savings)
		recordID,          // entity_id
		string(dataJSON),  // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or null
		clientTimestampValue,  // client_timestamp (original from client or null)
		serverTimestamp,   // server_timestamp (when processed)
		operationTimestamp, // created_at (extracted from operation_id for ordering)
	)
	
	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}
	
	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	log.Printf("Successfully added sync operation with ID: %d, operation_id: %s, timestamp: %d", 
		syncOpID, operationID, operationTimestamp)
	
	return nil
}