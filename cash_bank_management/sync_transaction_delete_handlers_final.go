package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Handlers HTTP para Transaction Delete Service - Versión Final Compacta
// Endpoints para sincronización bidireccional de eliminaciones

// handleSyncTransactionDeleteBatch procesa sincronización por lotes
func handleSyncTransactionDeleteBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var syncRequest SyncTransactionDeleteBatchRequest
	err := json.NewDecoder(r.Body).Decode(&syncRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := syncRequest.Validate(); err != nil {
		sendErrorResponse(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Processing transaction delete sync for user %s: %d deletions",
		syncRequest.UserID, len(syncRequest.Deletions))

	response, err := processTransactionDeleteSyncBatch(syncRequest)
	if err != nil {
		sendErrorResponse(w, "Error processing sync batch", http.StatusInternalServerError)
		return
	}

	// Invalidate caches
	if cacheManager != nil && response.Success {
		cacheManager.InvalidateCashBankCache(syncRequest.UserID)
		cacheManager.InvalidateDashboardCache(syncRequest.UserID, "monthly")
		log.Printf("✅ Caches invalidated for user: %s", syncRequest.UserID)
	}

	sendSuccessResponse(w, "Transaction delete sync processed successfully", response)
}

// handleSyncTransactionDeleteChanges obtiene cambios del servidor
func handleSyncTransactionDeleteChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}

	lastSync := r.URL.Query().Get("last_sync")
	limitStr := r.URL.Query().Get("limit")

	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 500 {
			limit = parsedLimit
		}
	}

	changesResponse, err := fetchTransactionDeleteChanges(userID, lastSync, limit)
	if err != nil {
		sendErrorResponse(w, "Error fetching changes", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Changes fetched successfully", changesResponse)
}

// handleSyncTransactionDeleteStats proporciona estadísticas
func handleSyncTransactionDeleteStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}

	stats, err := getTransactionDeleteSyncStats(userID)
	if err != nil {
		sendErrorResponse(w, "Error fetching sync stats", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Sync stats fetched successfully", stats)
}

// handleSyncTransactionDeleteConflictResolution resuelve conflictos
func handleSyncTransactionDeleteConflictResolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var conflictRequest SyncTransactionDeleteConflictRequest
	err := json.NewDecoder(r.Body).Decode(&conflictRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if conflictRequest.UserID == "" || conflictRequest.TransactionID == "" || conflictRequest.Resolution == "" {
		sendErrorResponse(w, "user_id, transaction_id, and resolution are required", http.StatusBadRequest)
		return
	}

	result, err := resolveTransactionDeleteConflict(conflictRequest)
	if err != nil {
		sendErrorResponse(w, "Error resolving conflict", http.StatusInternalServerError)
		return
	}

	if cacheManager != nil && result != nil {
		cacheManager.InvalidateCashBankCache(conflictRequest.UserID)
		cacheManager.InvalidateDashboardCache(conflictRequest.UserID, "monthly")
	}

	sendSuccessResponse(w, "Conflict resolved successfully", result)
}

// processTransactionDeleteSyncBatch procesa lote de eliminaciones
func processTransactionDeleteSyncBatch(request SyncTransactionDeleteBatchRequest) (SyncTransactionDeleteBatchResponse, error) {
	response := SyncTransactionDeleteBatchResponse{
		Success:          false,
		ProcessedOps:     0,
		SuccessfulOps:    0,
		FailedOps:        0,
		Results:          make([]SyncTransactionDeleteResult, 0),
		Conflicts:        make([]TransactionDeleteConflictResolution, 0),
		BalanceImpacts:   make([]TransactionDeleteBalanceImpact, 0),
		ValidationErrors: make([]string, 0),
		LastSync:         time.Now().Format(time.RFC3339),
	}

	for _, deletion := range request.Deletions {
		result := SyncTransactionDeleteResult{
			LocalID:          deletion.LocalID,
			TransactionID:    deletion.TransactionID,
			Action:           deletion.Action,
			Status:           "success",
			SyncTimestamp:    time.Now().Format(time.RFC3339),
			ValidationPassed: true,
		}

		// Validate
		if err := validateTransactionDeletionConsistency(deletion); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			result.ValidationPassed = false
			response.FailedOps++
			response.ValidationErrors = append(response.ValidationErrors, err.Error())
		} else {
			// Check if can delete
			exists, canDelete, err := checkTransactionCanBeDeleted(deletion.TransactionID, deletion.UserID)
			if err != nil {
				result.Status = "error"
				result.Error = fmt.Sprintf("Error checking: %v", err)
				response.FailedOps++
			} else if !exists {
				// Conflict: not found
				conflict := TransactionDeleteConflictResolution{
					LocalID:       deletion.LocalID,
					TransactionID: deletion.TransactionID,
					ConflictType:  "not_found",
					Description:   "Transaction not found",
					Priority:      "medium",
					Suggestions:   []string{"May have been deleted", "Sync changes"},
				}
				response.Conflicts = append(response.Conflicts, conflict)
				result.Status = "conflict"
				result.ConflictType = "not_found"
				result.RequiresAction = true
			} else if !canDelete {
				// Conflict: protected
				conflict := TransactionDeleteConflictResolution{
					LocalID:       deletion.LocalID,
					TransactionID: deletion.TransactionID,
					ConflictType:  "protected",
					Description:   "Transaction is protected",
					Priority:      "high",
					Suggestions:   []string{"Check status", "Contact support"},
				}
				response.Conflicts = append(response.Conflicts, conflict)
				result.Status = "conflict"
				result.ConflictType = "protected"
				result.RequiresAction = true
			} else {
				// Execute deletion
				balanceImpact, err := calculateBalanceImpactForDeletion(deletion.TransactionID, deletion.UserID)
				if err != nil {
					result.Status = "error"
					result.Error = fmt.Sprintf("Balance error: %v", err)
					response.FailedOps++
				} else {
					err = executeTransactionDeletion(deletion.TransactionID, deletion.UserID, deletion.DeletionReason)
					if err != nil {
						result.Status = "error"
						result.Error = fmt.Sprintf("Deletion error: %v", err)
						response.FailedOps++
					} else {
						result.BalanceAdjustment = balanceImpact.AdjustmentMade
						response.BalanceImpacts = append(response.BalanceImpacts, balanceImpact)
						response.SuccessfulOps++
					}
				}
			}
		}

		response.Results = append(response.Results, result)
		response.ProcessedOps++
	}

	response.Success = response.FailedOps == 0 && len(response.Conflicts) == 0

	if response.Success {
		response.Message = fmt.Sprintf("Processed %d deletions successfully", response.SuccessfulOps)
	} else {
		response.Message = fmt.Sprintf("Completed with %d errors, %d conflicts", response.FailedOps, len(response.Conflicts))
	}

	return response, nil
}

// fetchTransactionDeleteChanges obtiene cambios del servidor
func fetchTransactionDeleteChanges(userID, lastSync string, limit int) (SyncTransactionDeleteChangesResponse, error) {
	response := SyncTransactionDeleteChangesResponse{
		Success:    true,
		Message:    "Changes fetched",
		Deletions:  make([]TransactionDeletionRecord, 0),
		ServerTime: time.Now().Format(time.RFC3339),
		LastSync:   time.Now().Format(time.RFC3339),
	}

	if lastSync == "" {
		lastSync = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	}

	rows, err := db.Query(`
		SELECT id, user_id, transaction_id, transaction_type, original_amount, 
			   original_date, deletion_reason, deleted_at, deleted_by, balance_adjusted, can_be_restored
		FROM transaction_deletion_log WHERE user_id = ? AND deleted_at > ? ORDER BY deleted_at DESC LIMIT ?`,
		userID, lastSync, limit)
	if err != nil {
		return response, fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var record TransactionDeletionRecord
		var deletedAt time.Time

		err := rows.Scan(&record.ID, &record.UserID, &record.TransactionID,
			&record.TransactionType, &record.OriginalAmount, &record.OriginalDate,
			&record.DeletionReason, &deletedAt, &record.DeletedBy,
			&record.BalanceAdjusted, &record.CanBeRestored)
		if err != nil {
			continue
		}

		record.DeletedAt = deletedAt
		response.Deletions = append(response.Deletions, record)
	}

	response.TotalChanges = len(response.Deletions)
	response.HasMore = len(response.Deletions) == limit

	return response, nil
}

// getTransactionDeleteSyncStats obtiene estadísticas
func getTransactionDeleteSyncStats(userID string) (SyncTransactionDeleteStats, error) {
	stats := SyncTransactionDeleteStats{
		UserID:       userID,
		LastSyncTime: time.Now(),
	}

	// Get total deletions
	db.QueryRow("SELECT COUNT(*) FROM transaction_deletion_log WHERE user_id = ?", userID).Scan(&stats.TotalDeletions)

	// Get total amount deleted
	db.QueryRow("SELECT COALESCE(SUM(original_amount), 0) FROM transaction_deletion_log WHERE user_id = ?", userID).Scan(&stats.TotalAmountDeleted)

	// Get balance adjustments
	db.QueryRow("SELECT COUNT(*) FROM transaction_deletion_log WHERE user_id = ? AND balance_adjusted = 1", userID).Scan(&stats.BalanceAdjustments)

	stats.DataSizeBytes = int64(stats.TotalDeletions * 256)
	stats.AverageLatency = 150.0

	return stats, nil
}
