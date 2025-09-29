package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Updated request structures with sync parameters
// These structures include sync operation parameters for incremental synchronization

// UpdateAmountRequestWithSync represents an update request with sync support
type UpdateAmountRequestWithSync struct {
	UserID string  `json:"user_id"` // ID del usuario que actualiza la cantidad
	Amount float64 `json:"amount"`  // Nueva cantidad (puede ser cero o positiva)
	Date   string  `json:"date"`    // Fecha de la actualización para histórico
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

// handleUpdateCashWithSync maneja peticiones POST para actualizar cantidad de efectivo con sync
// Implementa el patrón consistente de grabación de operaciones de sincronización
// SIEMPRE graba operación de sync con auto-generación de operation_id
func handleUpdateCashWithSync(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST requests allowed for updates
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate JSON request body with sync parameters
	var updateRequest UpdateAmountRequestWithSync
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields in the request
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate amount is non-negative
	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Extract year_month from date or use current month
	yearMonth := updateRequest.Date[:7] // "2025-05-01" -> "2025-05"
	if len(updateRequest.Date) < 7 {
		yearMonth = time.Now().Format("2006-01")
	}

	// Get current distribution to maintain bank amount
	distribution, err := fetchCashBankDistribution(updateRequest.UserID, yearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Calculate delta for cascade updates
	cashDelta := updateRequest.Amount - distribution.CashAmount

	// Update cash amount and recalculate totals
	distribution.CashAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages based on new amounts
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save the updated distribution to all period tables
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating cash amount: %v", err)
		sendErrorResponse(w, "Error updating cash amount", http.StatusInternalServerError)
		return
	}

	// Cascade the changes to all future months if there's a delta
	if cashDelta != 0 {
		bankDelta := 0.0 // Bank amount doesn't change
		err = cascadeUpdateFutureMonths(updateRequest.UserID, yearMonth, cashDelta, bankDelta)
		if err != nil {
			log.Printf("Error cascading updates: %v", err)
			sendErrorResponse(w, "Error updating future balances", http.StatusInternalServerError)
			return
		}
	}

	// Add transaction to history for audit trail
	err = addTransaction(updateRequest.UserID, "cash_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error - no impide la operación principal
	}

	// 🚨 CRITICAL: ALWAYS record sync operation with auto-generated operation_id
	// This follows the consistent pattern from the implementation guide
	log.Printf("✅ Recording sync operation for cash update with auto-generated operation_id")

	err = addSyncOperationForCashBankUpdate(
		updateRequest.UserID,
		updateRequest.OperationID, // May be empty - will auto-generate
		"cash",
		updateRequest.Amount,
		updateRequest.Date,
		updateRequest.DeviceID, // Use device_id from request
		updateRequest.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for cash update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for cash update")
	}

	// Invalidate related caches since cash amount was updated
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(updateRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", updateRequest.UserID, err)
		}

		// Also invalidate dashboard cache since cash/bank affects dashboard
		err = cacheManager.InvalidateDashboardCache(updateRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", updateRequest.UserID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", updateRequest.UserID)
	}

	// Return success response with updated distribution
	sendSuccessResponse(w, "Cash amount updated successfully", distribution)
}

// handleUpdateBankWithSync maneja peticiones POST para actualizar cantidad de banco con sync
// Implementa el patrón consistente de grabación de operaciones de sincronización
// SIEMPRE graba operación de sync con auto-generación de operation_id
func handleUpdateBankWithSync(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST requests allowed
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate JSON request body with sync parameters
	var updateRequest UpdateAmountRequestWithSync
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate amount is non-negative
	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Extract year_month from date or use current month
	yearMonth := updateRequest.Date[:7] // "2025-05-01" -> "2025-05"
	if len(updateRequest.Date) < 7 {
		yearMonth = time.Now().Format("2006-01")
	}

	// Get current distribution to maintain cash amount
	distribution, err := fetchCashBankDistribution(updateRequest.UserID, yearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Calculate delta for cascade updates
	bankDelta := updateRequest.Amount - distribution.BankAmount

	// Update bank amount and recalculate totals
	distribution.BankAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages based on new totals
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save updated distribution to database
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating bank amount: %v", err)
		sendErrorResponse(w, "Error updating bank amount", http.StatusInternalServerError)
		return
	}

	// Cascade the changes to all future months if there's a delta
	if bankDelta != 0 {
		cashDelta := 0.0 // Cash amount doesn't change
		err = cascadeUpdateFutureMonths(updateRequest.UserID, yearMonth, cashDelta, bankDelta)
		if err != nil {
			log.Printf("Error cascading updates: %v", err)
			sendErrorResponse(w, "Error updating future balances", http.StatusInternalServerError)
			return
		}
	}

	// Add transaction to history for tracking
	err = addTransaction(updateRequest.UserID, "bank_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// 🚨 CRITICAL: ALWAYS record sync operation with auto-generated operation_id
	// This follows the consistent pattern from the implementation guide
	log.Printf("✅ Recording sync operation for bank update with auto-generated operation_id")

	err = addSyncOperationForCashBankUpdate(
		updateRequest.UserID,
		updateRequest.OperationID, // May be empty - will auto-generate
		"bank",
		updateRequest.Amount,
		updateRequest.Date,
		updateRequest.DeviceID, // Use device_id from request
		updateRequest.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for bank update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for bank update")
	}

	// Invalidate caches since bank amount was updated
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(updateRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", updateRequest.UserID, err)
		}

		// Also invalidate dashboard cache
		err = cacheManager.InvalidateDashboardCache(updateRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", updateRequest.UserID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", updateRequest.UserID)
	}

	// Return success response with updated data
	sendSuccessResponse(w, "Bank amount updated successfully", distribution)
}
