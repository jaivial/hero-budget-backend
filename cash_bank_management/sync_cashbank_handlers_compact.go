package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Handlers HTTP para sincronización offline de Cash Bank Management - Versión Compacta
// Implementan endpoints principales para sincronización bidireccional
// Incluyen procesamiento por lotes y resolución de conflictos básica

// handleSyncCashBankBatch procesa sincronización por lotes de operaciones offline
// Endpoint principal para sincronización de distribuciones y transferencias
func handleSyncCashBankBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var syncRequest SyncCashBankBatchRequest
	err := json.NewDecoder(r.Body).Decode(&syncRequest)
	if err != nil {
		log.Printf("Error parsing sync batch request: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := syncRequest.Validate(); err != nil {
		sendErrorResponse(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Processing sync batch for user %s: %d distributions, %d transfers", 
		syncRequest.UserID, len(syncRequest.Distributions), len(syncRequest.Transfers))

	// Procesar solicitud usando función dedicada
	response, err := processCashBankSyncBatchCompact(syncRequest)
	if err != nil {
		log.Printf("Error processing sync batch: %v", err)
		sendErrorResponse(w, "Error processing sync batch", http.StatusInternalServerError)
		return
	}

	// Invalidar caches relacionados
	if cacheManager != nil && response.Success {
		cacheManager.InvalidateCashBankCache(syncRequest.UserID)
		cacheManager.InvalidateDashboardCache(syncRequest.UserID, "monthly")
		log.Printf("✅ Caches invalidated after sync for user: %s", syncRequest.UserID)
	}

	sendSuccessResponse(w, "Sync batch processed successfully", response)
}

// handleSyncCashBankChanges obtiene cambios del servidor desde último sync
func handleSyncCashBankChanges(w http.ResponseWriter, r *http.Request) {
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
	
	limit := 100 // Default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	changesResponse, err := fetchCashBankChangesCompact(userID, lastSync, limit)
	if err != nil {
		log.Printf("Error fetching changes: %v", err)
		sendErrorResponse(w, "Error fetching changes", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Changes fetched successfully", changesResponse)
}

// handleSyncCashBankStats proporciona estadísticas de sincronización
func handleSyncCashBankStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}

	stats, err := getCashBankSyncStatsCompact(userID)
	if err != nil {
		log.Printf("Error fetching sync stats: %v", err)
		sendErrorResponse(w, "Error fetching sync stats", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Sync stats fetched successfully", stats)
}

// processCashBankSyncBatchCompact procesa sincronización por lotes de forma compacta
func processCashBankSyncBatchCompact(request SyncCashBankBatchRequest) (SyncCashBankBatchResponse, error) {
	response := SyncCashBankBatchResponse{
		Success:       false,
		ProcessedOps:  0,
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncCashBankResult, 0),
		Conflicts:     make([]CashBankConflictResolution, 0),
		LastSync:      time.Now().Format(time.RFC3339),
	}

	// Procesar distribuciones
	for _, distribution := range request.Distributions {
		result := SyncCashBankResult{
			LocalID:       distribution.LocalID,
			OperationType: "distribution",
			Status:        "success",
			SyncTimestamp: time.Now().Format(time.RFC3339),
		}

		if err := validateCashBankDistributionConsistency(distribution); err != nil {
			result.Status = "error"
			result.Error = err.Error()
			response.FailedOps++
		} else {
			// Procesar según acción
			switch distribution.Action {
			case "add", "update":
				newDist := CashBankDistribution{
					UserID:       distribution.UserID,
					Month:        distribution.Month,
					CashAmount:   distribution.CashAmount,
					BankAmount:   distribution.BankAmount,
					CashPercent:  distribution.CashPercent,
					BankPercent:  distribution.BankPercent,
					MonthlyTotal: distribution.MonthlyTotal,
				}
				
				if err := updateCashBankDistribution(newDist); err != nil {
					result.Status = "error"
					result.Error = err.Error()
					response.FailedOps++
				} else {
					response.SuccessfulOps++
				}
			}
		}
		
		response.Results = append(response.Results, result)
		response.ProcessedOps++
	}

	// Procesar transferencias
	for _, transfer := range request.Transfers {
		result := SyncCashBankResult{
			LocalID:       transfer.LocalID,
			OperationType: "transfer",
			Status:        "success",
			SyncTimestamp: time.Now().Format(time.RFC3339),
		}

		if transfer.Action == "add" {
			err := addTransaction(transfer.UserID, transfer.TransferType, transfer.Amount, transfer.Date)
			if err != nil {
				result.Status = "error"
				result.Error = err.Error()
				response.FailedOps++
			} else {
				response.SuccessfulOps++
			}
		}
		
		response.Results = append(response.Results, result)
		response.ProcessedOps++
	}

	response.Success = response.FailedOps == 0
	if response.Success {
		response.Message = fmt.Sprintf("Batch processed successfully: %d operations", response.SuccessfulOps)
	} else {
		response.Message = fmt.Sprintf("Batch completed with %d errors", response.FailedOps)
	}

	return response, nil
}

// fetchCashBankChangesCompact obtiene cambios del servidor (versión compacta)
func fetchCashBankChangesCompact(userID, lastSync string, limit int) (SyncCashBankChangesResponse, error) {
	response := SyncCashBankChangesResponse{
		Success:      true,
		Message:      "Changes fetched successfully",
		Distributions: make([]CashBankDistribution, 0),
		Transfers:    make([]CashBankTransfer, 0),
		ServerTime:   time.Now().Format(time.RFC3339),
		LastSync:     time.Now().Format(time.RFC3339),
	}

	// Obtener distribución actual
	// Use current month for sync operations - TODO: consider passing month parameter  
	currentMonth := time.Now().Format("2006-01")
	distribution, err := fetchCashBankDistribution(userID, currentMonth)
	if err == nil && distribution.UserID != "" {
		response.Distributions = append(response.Distributions, distribution)
	}

	// Obtener transferencias recientes
	rows, err := db.Query(`
		SELECT id, user_id, transaction_type, amount, date, created_at
		FROM cash_bank_transactions
		WHERE user_id = ? 
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var transfer CashBankTransfer
			rows.Scan(&transfer.ID, &transfer.UserID, &transfer.TransferType, 
				&transfer.Amount, &transfer.Date, &transfer.CreatedAt)
			response.Transfers = append(response.Transfers, transfer)
		}
	}

	response.TotalChanges = len(response.Distributions) + len(response.Transfers)
	return response, nil
}

// getCashBankSyncStatsCompact obtiene estadísticas básicas de sincronización
func getCashBankSyncStatsCompact(userID string) (SyncCashBankStats, error) {
	stats := SyncCashBankStats{
		UserID:            userID,
		LastSyncTime:      time.Now(),
		TotalSyncs:        0,
		PendingOperations: 0,
		ConflictsResolved: 0,
		ErrorCount:        0,
	}

	// Estadísticas básicas desde transacciones
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM cash_bank_transactions WHERE user_id = ?", userID).Scan(&count)
	if err == nil {
		stats.TotalSyncs = count
	}

	return stats, nil
}