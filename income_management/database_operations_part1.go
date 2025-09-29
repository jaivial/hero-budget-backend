package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// addIncome inserta un nuevo ingreso en la base de datos
func addIncome(income Income) (int, error) {
	// Insert income into the database with category_id support
	query := `
		INSERT INTO incomes (
			user_id, amount, date, category, category_id, payment_method, description
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		income.UserID,
		income.Amount,
		income.Date,
		income.Category,
		income.CategoryID, // New category_id field
		income.PaymentMethod,
		income.Description,
	)

	if err != nil {
		return 0, err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// updateIncome actualiza un ingreso existente en la base de datos
func updateIncome(income Income) error {
	// Update income in the database with category_id support
	query := `
		UPDATE incomes
		SET amount = ?, date = ?, category = ?, category_id = ?, payment_method = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(
		query,
		income.Amount,
		income.Date,
		income.Category,
		income.CategoryID, // New category_id field
		income.PaymentMethod,
		income.Description,
		income.ID,
		income.UserID,
	)

	return err
}

// deleteIncome elimina un ingreso de la base de datos
func deleteIncome(incomeID int, userID string) error {
	// Delete income from the database
	query := `
		DELETE FROM incomes
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(query, incomeID, userID)
	return err
}

// getIncomeByID obtiene un ingreso específico por ID y userID
func getIncomeByID(userID string, incomeID int) (Income, error) {
	// Query to get a specific income including category_id
	query := `
		SELECT id, user_id, amount, date, category, category_id, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE id = ? AND user_id = ?
	`

	var income Income
	err := db.QueryRow(query, incomeID, userID).Scan(
		&income.ID,
		&income.UserID,
		&income.Amount,
		&income.Date,
		&income.Category,
		&income.CategoryID,
		&income.PaymentMethod,
		&income.Description,
		&income.CreatedAt,
		&income.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return income, err // Return empty income and error
	} else if err != nil {
		return income, err
	}

	return income, nil
}

// getIncomes obtiene ingresos con filtros opcionales
func getIncomes(userID, category, startDate, endDate, paymentMethod string) ([]Income, error) {
	// Base query including category_id
	query := `
		SELECT id, user_id, amount, date, category, category_id, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE user_id = ?
	`

	// Build parameters slice
	params := []interface{}{userID}

	// Add filters dynamically
	if category != "" {
		query += " AND category = ?"
		params = append(params, category)
	}

	if startDate != "" {
		query += " AND date >= ?"
		params = append(params, startDate)
	}

	if endDate != "" {
		query += " AND date <= ?"
		params = append(params, endDate)
	}

	if paymentMethod != "" {
		query += " AND payment_method = ?"
		params = append(params, paymentMethod)
	}

	query += " ORDER BY date DESC"

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incomes []Income

	for rows.Next() {
		var income Income
		if err := rows.Scan(
			&income.ID,
			&income.UserID,
			&income.Amount,
			&income.Date,
			&income.Category,
			&income.CategoryID,
			&income.PaymentMethod,
			&income.Description,
			&income.CreatedAt,
			&income.UpdatedAt,
		); err != nil {
			return nil, err
		}

		incomes = append(incomes, income)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return incomes, nil
}

// addSyncOperation records a sync operation in the sync_operations table
// Uses the new operation_id system with timestamp-based format and automatic generation
// Implementation following docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md
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
		action,                // operation_type (create, update, delete)
		tableName,             // entity_type (incomes, expenses, etc.)
		recordID,              // entity_id
		string(dataJSON),      // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or null
		clientTimestampValue,  // client_timestamp (original from client or null)
		serverTimestamp,       // server_timestamp (when processed)
		operationTimestamp,    // created_at (extracted from operation_id for ordering)
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
