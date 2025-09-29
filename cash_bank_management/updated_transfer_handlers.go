package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleCashToBankTransferWithSync maneja transferencias de efectivo a banco con sync consistente
// SIEMPRE graba operación de sync con auto-generación de operation_id
// Sigue el patrón consistente del implementation guide
func handleCashToBankTransferWithSync(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST allowed for transfers
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse and validate transfer request JSON with sync parameters
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields in transfer request
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Validate transfer amount is positive
	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Extract year-month from transfer date for monthly tracking
	transferYearMonth := transferRequest.Date[:7] // "2025-05-01" -> "2025-05"

	// Get distribution for the specific month to check cash availability
	distribution, err := fetchCashBankDistribution(transferRequest.UserID, transferYearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Check if there's enough cash to transfer
	if transferRequest.Amount > distribution.CashAmount {
		sendErrorResponse(w, "Not enough cash to transfer", http.StatusBadRequest)
		return
	}

	// Calculate deltas for cascade updates
	cashDelta := -transferRequest.Amount // Cash decreases
	bankDelta := +transferRequest.Amount // Bank increases

	// Update amounts atomically - subtract from cash, add to bank
	distribution.CashAmount -= transferRequest.Amount
	distribution.BankAmount += transferRequest.Amount

	// Recalculate percentages after transfer
	// Total permanece igual, solo cambia la distribución
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Update the specific month in database
	_, err = db.Exec(`
		UPDATE monthly_cash_bank_balance 
		SET balance_cash_amount = ?, balance_bank_amount = ?, total_balance = ?, updated_at = datetime('now')
		WHERE user_id = ? AND year_month = ?
	`, distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal, transferRequest.UserID, transferYearMonth)

	if err != nil {
		log.Printf("Error updating month %s balance: %v", transferYearMonth, err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Cascade the changes to all future months
	err = cascadeUpdateFutureMonths(transferRequest.UserID, transferYearMonth, cashDelta, bankDelta)
	if err != nil {
		log.Printf("Error cascading updates: %v", err)
		sendErrorResponse(w, "Error updating future balances", http.StatusInternalServerError)
		return
	}

	// Add transaction to history for audit trail
	err = addTransaction(transferRequest.UserID, "cash_to_bank", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error - no bloquea operación principal
	}

	// 🚨 CRITICAL: ALWAYS record sync operation with auto-generated operation_id
	// This follows the consistent pattern from the implementation guide
	log.Printf("✅ Recording sync operation for cash-to-bank transfer with auto-generated operation_id")

	err = addSyncOperationForCashBankTransfer(
		transferRequest.UserID,
		transferRequest.OperationID, // May be empty - will auto-generate
		"cash_to_bank",
		transferRequest.Amount,
		transferRequest.Date,
		transferRequest.DeviceID, // Use device_id from request
		transferRequest.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for cash-to-bank transfer: %v", err)
		// Don't fail the transfer for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for cash-to-bank transfer: user=%s, amount=%.2f",
			transferRequest.UserID, transferRequest.Amount)
	}

	// Invalidate caches since transfer affects distribution
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(transferRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", transferRequest.UserID, err)
		}

		// Also invalidate dashboard cache
		err = cacheManager.InvalidateDashboardCache(transferRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", transferRequest.UserID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", transferRequest.UserID)
	}

	// Return success response with updated distribution
	sendSuccessResponse(w, "Cash to bank transfer successful", distribution)
}

// handleBankToCashTransferWithSync maneja transferencias de banco a efectivo con sync consistente
// SIEMPRE graba operación de sync con auto-generación de operation_id
// Sigue el patrón consistente del implementation guide
func handleBankToCashTransferWithSync(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse transfer request with sync parameters
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Extract year-month from transfer date for monthly tracking
	transferYearMonth := transferRequest.Date[:7] // "2025-05-01" -> "2025-05"

	// Get distribution for the specific month to check bank balance
	distribution, err := fetchCashBankDistribution(transferRequest.UserID, transferYearMonth)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Check if there's enough bank balance to transfer
	if transferRequest.Amount > distribution.BankAmount {
		sendErrorResponse(w, "Not enough bank balance to transfer", http.StatusBadRequest)
		return
	}

	// Calculate deltas for cascade updates
	cashDelta := +transferRequest.Amount // Cash increases
	bankDelta := -transferRequest.Amount // Bank decreases

	// Update amounts - subtract from bank, add to cash
	distribution.BankAmount -= transferRequest.Amount
	distribution.CashAmount += transferRequest.Amount

	// Recalculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Update the specific month in database
	_, err = db.Exec(`
		UPDATE monthly_cash_bank_balance 
		SET balance_cash_amount = ?, balance_bank_amount = ?, total_balance = ?, updated_at = datetime('now')
		WHERE user_id = ? AND year_month = ?
	`, distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal, transferRequest.UserID, transferYearMonth)

	if err != nil {
		log.Printf("Error updating month %s balance: %v", transferYearMonth, err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Cascade the changes to all future months
	err = cascadeUpdateFutureMonths(transferRequest.UserID, transferYearMonth, cashDelta, bankDelta)
	if err != nil {
		log.Printf("Error cascading updates: %v", err)
		sendErrorResponse(w, "Error updating future balances", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	err = addTransaction(transferRequest.UserID, "bank_to_cash", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
	}

	// 🚨 CRITICAL: ALWAYS record sync operation with auto-generated operation_id
	// This follows the consistent pattern from the implementation guide
	log.Printf("✅ Recording sync operation for bank-to-cash transfer with auto-generated operation_id")

	err = addSyncOperationForCashBankTransfer(
		transferRequest.UserID,
		transferRequest.OperationID, // May be empty - will auto-generate
		"bank_to_cash",
		transferRequest.Amount,
		transferRequest.Date,
		transferRequest.DeviceID, // Use device_id from request
		transferRequest.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for bank-to-cash transfer: %v", err)
		// Don't fail the transfer for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for bank-to-cash transfer: user=%s, amount=%.2f",
			transferRequest.UserID, transferRequest.Amount)
	}

	// Invalidate caches
	if cacheManager != nil {
		err = cacheManager.InvalidateCashBankCache(transferRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate cash bank cache for user %s: %v", transferRequest.UserID, err)
		}

		err = cacheManager.InvalidateDashboardCache(transferRequest.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", transferRequest.UserID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (cash/bank and dashboard)", transferRequest.UserID)
	}

	// Return success response
	sendSuccessResponse(w, "Bank to cash transfer successful", distribution)
}
