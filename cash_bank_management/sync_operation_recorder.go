package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// addSyncOperation records a sync operation in the sync_operations table
// Uses the new operation_id system with timestamp-based format and automatic generation
// This function implements the pattern from docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md
func addSyncOperation(userID, providedOperationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, provided_operation=%s, action=%s, table=%s, record=%s, device=%s",
		userID, providedOperationID, action, tableName, recordID, deviceID)

	// Generate operation ID if not provided or if provided ID is not valid timestamp format
	var operationID string
	var err error

	operationID, err = validateOperationIdOrGenerate(userID, providedOperationID)
	if err != nil {
		log.Printf("Error validating/generating operation ID: %v", err)
		return fmt.Errorf("failed to validate/generate operation ID: %v", err)
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
		deviceIDsJSON = []byte("[]")
		log.Printf("Device ID empty, storing empty array in device_ids column")
	}

	// Extract timestamp from operation ID for created_at field
	operationTimestamp := extractTimestampFromOperationId(operationID)
	if operationTimestamp == 0 {
		operationTimestamp = time.Now().UnixMilli()
		log.Printf("Warning: Could not extract timestamp from operation ID, using current timestamp: %d", operationTimestamp)
	}

	// Use current server timestamp
	serverTimestamp := time.Now().UnixMilli()

	// Handle client timestamp - use 0 if not provided
	var clientTimestampValue int64
	if clientTimestamp == 0 {
		clientTimestampValue = 0
		log.Printf("Client timestamp is 0, storing 0 in client_timestamp column")
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
		action,                // operation_type (create, update, delete, transfer, update_cash, update_bank)
		tableName,             // entity_type (cash_bank_transfers, cash_bank_updates, etc.)
		recordID,              // entity_id
		string(dataJSON),      // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or empty array
		clientTimestampValue,  // client_timestamp (original from client or 0)
		serverTimestamp,       // server_timestamp (when processed)
		operationTimestamp,    // created_at (extracted from operation_id for ordering)
	)

	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}

	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	log.Printf("✅ Successfully added sync operation with ID: %d, operation_id: %s, timestamp: %d",
		syncOpID, operationID, operationTimestamp)

	return nil
}

// addSyncOperationForCashBankTransfer is a specialized wrapper for cash-bank transfers
// Provides specific entity_type and operation_data structure for transfer operations
func addSyncOperationForCashBankTransfer(userID, operationID, transferType string, amount float64, date string, deviceID string, clientTimestamp int64) error {
	// Create sync operation data for transfer
	syncData := map[string]interface{}{
		"user_id":       userID,
		"amount":        amount,
		"date":          date,
		"transfer_type": transferType, // "cash_to_bank" or "bank_to_cash"
		"processed_at":  time.Now().Format("2006-01-02 15:04:05"),
	}

	// Use year-month as entity_id for transfers to group by month
	yearMonth := date[:7] // "2025-05-01" -> "2025-05"
	entityID := fmt.Sprintf("%s-%s", userID, yearMonth)

	return addSyncOperation(
		userID,
		operationID,
		"transfer",
		"cash_bank_transfers",
		entityID,
		syncData,
		deviceID,
		clientTimestamp,
	)
}

// addSyncOperationForCashBankUpdate is a specialized wrapper for cash/bank updates
// Provides specific entity_type and operation_data structure for amount updates
func addSyncOperationForCashBankUpdate(userID, operationID, updateType string, amount float64, date string, deviceID string, clientTimestamp int64) error {
	// Create sync operation data for update
	syncData := map[string]interface{}{
		"user_id":      userID,
		"amount":       amount,
		"date":         date,
		"update_type":  updateType, // "cash" or "bank"
		"processed_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// Use year-month as entity_id for updates to group by month
	yearMonth := date[:7] // "2025-05-01" -> "2025-05"
	entityID := fmt.Sprintf("%s-%s", userID, yearMonth)

	// Determine operation type based on update type
	operationType := "update_cash"
	if updateType == "bank" {
		operationType = "update_bank"
	}

	return addSyncOperation(
		userID,
		operationID,
		operationType,
		"cash_bank_updates",
		entityID,
		syncData,
		deviceID,
		clientTimestamp,
	)
}
