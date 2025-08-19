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
// Implementation following docs/SYNC_OPERATIONS_IMPLEMENTATION_GUIDE.md

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number}
func isValidOperationId(operationId string) bool {
	if operationId == "" {
		return false
	}
	
	// Expected format: 1755209423000_001
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

// getLastOperationIdGlobal retrieves the globally last operation ID across all users
// This ensures proper chronological ordering across the entire system
func getLastOperationIdGlobal() (string, error) {
	var lastOperationId string
	err := db.QueryRow("SELECT operation_id FROM sync_operations ORDER BY operation_id DESC LIMIT 1").Scan(&lastOperationId)
	
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No previous operations found globally")
			return "", nil
		}
		log.Printf("Error retrieving global last operation ID: %v", err)
		return "", err
	}
	
	log.Printf("Retrieved global last operation ID: %s", lastOperationId)
	return lastOperationId, nil
}

// getLastOperationIdForUser retrieves the last operation ID for a specific user
// DEPRECATED: Use getLastOperationIdGlobal() for proper chronological ordering
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

// generateNextOperationId generates the next operation ID globally
// Gets the global last operation ID and adds +1 millisecond time unit for proper chronological ordering
func generateNextOperationId(userID string) (string, error) {
	log.Printf("Generating next operation ID for user: %s (using global ordering)", userID)
	
	// Get the globally last operation ID across all users and services
	lastOperationId, err := getLastOperationIdGlobal()
	if err != nil {
		return "", fmt.Errorf("failed to get global last operation ID: %v", err)
	}
	
	var nextTimestamp int64
	var sequenceNumber int = 1
	
	if lastOperationId == "" {
		// No previous operations, start with current timestamp
		nextTimestamp = time.Now().UnixMilli()
		log.Printf("No previous operations, starting with timestamp: %d", nextTimestamp)
	} else {
		// Extract timestamp from last operation ID if it follows the standard format
		lastTimestamp := extractTimestampFromOperationId(lastOperationId)
		if lastTimestamp == 0 {
			// Last operation ID doesn't follow standard format, use current timestamp
			// This handles legacy operation IDs or mixed formats
			currentTime := time.Now().UnixMilli()
			log.Printf("Last operation ID (%s) doesn't follow standard format, using current timestamp: %d", lastOperationId, currentTime)
			
			// Ensure we're ahead of any existing timestamp-based operations
			// Check if there are any recent timestamp-based operations
			var lastTimestampBasedId string
			err := db.QueryRow("SELECT operation_id FROM sync_operations WHERE operation_id REGEXP '^[0-9]{13}_[0-9]{3}$' ORDER BY operation_id DESC LIMIT 1").Scan(&lastTimestampBasedId)
			if err == nil && lastTimestampBasedId != "" {
				lastValidTimestamp := extractTimestampFromOperationId(lastTimestampBasedId)
				if lastValidTimestamp > 0 && currentTime <= lastValidTimestamp {
					nextTimestamp = lastValidTimestamp + 1
					log.Printf("Found recent timestamp-based operation %s, incrementing to: %d", lastTimestampBasedId, nextTimestamp)
				} else {
					nextTimestamp = currentTime
				}
			} else {
				nextTimestamp = currentTime
			}
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