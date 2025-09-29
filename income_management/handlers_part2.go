package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// handleUpdateIncome maneja requests para actualizar ingresos existentes
func handleUpdateIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for update data
	var updateRequest UpdateIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request parameters
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.IncomeID <= 0 {
		sendErrorResponse(w, "Valid income ID is required", http.StatusBadRequest)
		return
	}

	// Get the current income to compare changes
	currentIncome, err := getIncomeByID(updateRequest.UserID, updateRequest.IncomeID)
	if err != nil {
		log.Printf("Error retrieving current income: %v", err)
		sendErrorResponse(w, "Income not found", http.StatusNotFound)
		return
	}

	// Prepare updated income with new values or keep existing ones
	updatedIncome := currentIncome

	if updateRequest.Amount > 0 {
		updatedIncome.Amount = updateRequest.Amount
	}
	if updateRequest.Date != "" {
		updatedIncome.Date = updateRequest.Date
	}
	if updateRequest.Category != "" {
		updatedIncome.Category = updateRequest.Category
	}
	if updateRequest.CategoryID != nil {
		updatedIncome.CategoryID = updateRequest.CategoryID
	}
	if updateRequest.PaymentMethod != "" {
		if updateRequest.PaymentMethod != "cash" && updateRequest.PaymentMethod != "bank" {
			sendErrorResponse(w, "Valid payment method (cash or bank) is required", http.StatusBadRequest)
			return
		}
		updatedIncome.PaymentMethod = updateRequest.PaymentMethod
	}
	if updateRequest.Description != "" {
		updatedIncome.Description = updateRequest.Description
	}

	// Update the income in the database
	err = updateIncome(updatedIncome)
	if err != nil {
		log.Printf("Error updating income: %v", err)
		sendErrorResponse(w, "Error updating income", http.StatusInternalServerError)
		return
	}

	// Record sync operation with auto-generated operation_id (following consistent pattern)
	// Critical: ALL handlers must follow the same pattern for sync operations
	log.Printf("Recording sync operation for income update with auto-generated operation_id")

	// Create sync operation data matching the updated income structure
	syncData := map[string]interface{}{
		"id":             updatedIncome.ID,
		"user_id":        updatedIncome.UserID,
		"amount":         updatedIncome.Amount,
		"date":           updatedIncome.Date,
		"category":       updatedIncome.Category,
		"payment_method": updatedIncome.PaymentMethod,
		"description":    updatedIncome.Description,
		"updated_at":     time.Now().Format("2006-01-02 15:04:05"),
	}

	// Add sync operation record to database with auto-generated operation_id
	err = addSyncOperation(
		updatedIncome.UserID,
		"", // Empty operation_id triggers auto-generation
		"update",
		"incomes",
		strconv.Itoa(updatedIncome.ID),
		syncData,
		updateRequest.DeviceID, // Use device_id from request
		0,                      // Timestamp auto-generated
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for income update: %v", err)
		// Don't fail the income update for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for income ID: %d", updatedIncome.ID)
	}

	// Update balance changes if amount or payment method changed
	if currentIncome.Amount != updatedIncome.Amount || currentIncome.PaymentMethod != updatedIncome.PaymentMethod {
		// Reverse the old balance effect
		reverseAmount := -currentIncome.Amount
		if err := updateBalance(currentIncome.UserID, reverseAmount, currentIncome.PaymentMethod); err != nil {
			log.Printf("Error reversing old balance: %v", err)
		}

		// Apply the new balance effect
		if err := updateBalance(updatedIncome.UserID, updatedIncome.Amount, updatedIncome.PaymentMethod); err != nil {
			log.Printf("Error updating new balance: %v", err)
		}

		// Balance recalculation - simplified logging
		log.Printf("Note: Balance recalculation for update - userID: %s, date: %s", updatedIncome.UserID, updatedIncome.Date)
	}

	// Invalidate income analytics cache after updating
	invalidateIncomeAnalytics(updatedIncome.UserID)

	// Invalidate related cache entries if available
	if cacheManager != nil {
		err = cacheManager.InvalidateIncomeCache(updatedIncome.UserID, "monthly", "daily", "weekly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate income cache: %v", err)
		}
		err = cacheManager.InvalidateDashboardCache(updatedIncome.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response with updated income
	sendSuccessResponse(w, "Income updated successfully", updatedIncome)
}

// handleDeleteIncome maneja requests para eliminar ingresos
func handleDeleteIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for delete data
	var deleteRequest DeleteIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request parameters
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if deleteRequest.IncomeID <= 0 {
		sendErrorResponse(w, "Valid income ID is required", http.StatusBadRequest)
		return
	}

	// Get the income to be deleted for balance adjustment
	incomeToDelete, err := getIncomeByID(deleteRequest.UserID, deleteRequest.IncomeID)
	if err != nil {
		log.Printf("Error retrieving income to delete: %v", err)
		sendErrorResponse(w, "Income not found", http.StatusNotFound)
		return
	}

	// Delete the income from the database
	err = deleteIncome(deleteRequest.IncomeID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting income: %v", err)
		sendErrorResponse(w, "Error deleting income", http.StatusInternalServerError)
		return
	}

	// Record sync operation with auto-generated operation_id (following consistent pattern)
	// Critical: ALL handlers must follow the same pattern for sync operations
	log.Printf("Recording sync operation for income deletion with auto-generated operation_id")

	// Create sync operation data matching the deleted income structure
	syncData := map[string]interface{}{
		"id":             incomeToDelete.ID,
		"user_id":        incomeToDelete.UserID,
		"amount":         incomeToDelete.Amount,
		"date":           incomeToDelete.Date,
		"category":       incomeToDelete.Category,
		"payment_method": incomeToDelete.PaymentMethod,
		"description":    incomeToDelete.Description,
		"deleted_at":     time.Now().Format("2006-01-02 15:04:05"),
	}

	// Add sync operation record to database with auto-generated operation_id
	err = addSyncOperation(
		incomeToDelete.UserID,
		"", // Empty operation_id triggers auto-generation
		"delete",
		"incomes",
		strconv.Itoa(incomeToDelete.ID),
		syncData,
		deleteRequest.DeviceID, // Use device_id from request
		0,                      // Timestamp auto-generated
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for income deletion: %v", err)
		// Don't fail the income deletion for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for deleted income ID: %d", incomeToDelete.ID)
	}

	// Reverse the balance effect of the deleted income
	reverseAmount := -incomeToDelete.Amount
	if err := updateBalance(incomeToDelete.UserID, reverseAmount, incomeToDelete.PaymentMethod); err != nil {
		log.Printf("Error reversing balance for deleted income: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Balance recalculation after deletion - simplified logging
	log.Printf("Note: Balance recalculation after deletion - userID: %s, date: %s", incomeToDelete.UserID, incomeToDelete.Date)

	// Invalidate income analytics cache after deletion
	invalidateIncomeAnalytics(incomeToDelete.UserID)

	// Invalidate related cache entries if available
	if cacheManager != nil {
		err = cacheManager.InvalidateIncomeCache(incomeToDelete.UserID, "monthly", "daily", "weekly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate income cache: %v", err)
		}
		err = cacheManager.InvalidateDashboardCache(incomeToDelete.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Income deleted successfully", map[string]interface{}{
		"deleted_income_id": deleteRequest.IncomeID,
		"user_id":           deleteRequest.UserID,
	})
}
