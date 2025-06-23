package main

import (
	"encoding/json"
	"log"
	"net/http"
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
		"user_id":          deleteRequest.UserID,
	})
}