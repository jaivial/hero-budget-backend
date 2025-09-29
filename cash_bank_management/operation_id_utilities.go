package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Operation ID utility functions for timestamp-based format
// These functions implement the sync operations pattern from the implementation guide
// Expected format: {timestamp_ms}_{sequence_number} (e.g., 1755209423000_001)

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number}
func isValidOperationId(operationId string) bool {
	if operationId == "" {
		return false
	}

	// Expected format: 1755209423000_001 (13 digits + underscore + 3 digits)
	operationIdPattern := `^\d{13}_\d{3}$`
	matched, err := regexp.MatchString(operationIdPattern, operationId)
	if err != nil {
		log.Printf("Error validating operation ID pattern: %v", err)
		return false
	}

	return matched
}

// extractTimestampFromOperationId extracts timestamp from operation ID
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

// getLastOperationIdForUser retrieves the last operation ID for a specific user
func getLastOperationIdForUser(userID string) (string, error) {
	var lastOperationId string
	err := db.QueryRow("SELECT operation_id FROM sync_operations WHERE user_id = ? ORDER BY operation_id DESC LIMIT 1", userID).Scan(&lastOperationId)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No previous operations found for user: %s", userID)
			return "", nil
		}
		log.Printf("Error retrieving last operation ID for user %s: %v", userID, err)
		return "", err
	}

	log.Printf("Retrieved last operation ID for user %s: %s", userID, lastOperationId)
	return lastOperationId, nil
}

// generateNextOperationId generates the next operation ID for a user
// Gets the last operation ID and adds +1 millisecond time unit
func generateNextOperationId(userID string) (string, error) {
	log.Printf("Generating next operation ID for user: %s", userID)

	// Get the last operation ID for this user
	lastOperationId, err := getLastOperationIdForUser(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get last operation ID: %v", err)
	}

	var nextTimestamp int64
	var sequenceNumber int = 1

	if lastOperationId == "" {
		// No previous operations, start with current timestamp
		nextTimestamp = time.Now().UnixMilli()
		log.Printf("No previous operations, starting with timestamp: %d", nextTimestamp)
	} else {
		// Extract timestamp from last operation ID
		lastTimestamp := extractTimestampFromOperationId(lastOperationId)
		if lastTimestamp == 0 {
			// Invalid last operation ID format, use current timestamp
			nextTimestamp = time.Now().UnixMilli()
			log.Printf("Invalid last operation ID format, using current timestamp: %d", nextTimestamp)
		} else {
			// Add 1 millisecond to ensure chronological ordering
			nextTimestamp = lastTimestamp + 1
			log.Printf("Incremented timestamp from %d to %d", lastTimestamp, nextTimestamp)
		}
	}

	// Format as {timestamp_ms}_{sequence_number}
	operationId := fmt.Sprintf("%d_%03d", nextTimestamp, sequenceNumber)

	log.Printf("Generated operation ID: %s", operationId)
	return operationId, nil
}

// validateOperationIdOrGenerate validates provided operation ID or generates a new one
// This function implements the logic for handling both provided and auto-generated operation IDs
func validateOperationIdOrGenerate(userID, providedOperationID string) (string, error) {
	// If a valid operation ID is provided, use it
	if providedOperationID != "" && isValidOperationId(providedOperationID) {
		log.Printf("Using provided operation ID: %s", providedOperationID)
		return providedOperationID, nil
	}

	// Generate new timestamp-based operation ID
	operationID, err := generateNextOperationId(userID)
	if err != nil {
		log.Printf("Error generating operation ID: %v", err)
		return "", fmt.Errorf("failed to generate operation ID: %v", err)
	}

	if providedOperationID != "" {
		log.Printf("Generated new operation ID: %s (provided was invalid: %s)", operationID, providedOperationID)
	} else {
		log.Printf("Generated new operation ID: %s (none provided)", operationID)
	}

	return operationID, nil
}
