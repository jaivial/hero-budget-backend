package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleUpdateCashBankBalance maneja requests para actualizar balance cash/bank
func handleUpdateCashBankBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for cash/bank balance update
	var request struct {
		UserID     string  `json:"user_id"`
		Month      string  `json:"month"`
		CashAmount float64 `json:"cash_amount"`
		BankAmount float64 `json:"bank_amount"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request parameters
	if request.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if request.Month == "" {
		sendErrorResponse(w, "Month is required", http.StatusBadRequest)
		return
	}

	// Update cash/bank balance in database
	err = updateCashBankBalance(request.UserID, request.Month, request.CashAmount, request.BankAmount)
	if err != nil {
		log.Printf("Error updating cash/bank balance: %v", err)
		sendErrorResponse(w, "Error updating cash/bank balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Cash/Bank balance updated successfully", map[string]interface{}{
		"user_id":     request.UserID,
		"month":       request.Month,
		"cash_amount": request.CashAmount,
		"bank_amount": request.BankAmount,
	})
}

// handleGetCashBankBalance maneja requests para obtener balance cash/bank
func handleGetCashBankBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from query string
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")

	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if month == "" {
		sendErrorResponse(w, "Month is required", http.StatusBadRequest)
		return
	}

	// Get cash/bank balance from database
	balance, err := getCashBankBalance(userID, month)
	if err != nil {
		log.Printf("Error retrieving cash/bank balance: %v", err)
		sendErrorResponse(w, "Error retrieving cash/bank balance", http.StatusInternalServerError)
		return
	}

	// Return success response with balance data
	sendSuccessResponse(w, "Cash/Bank balance retrieved successfully", balance)
}

// handleGetBalance maneja requests genéricos para obtener balance
func handleGetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from query string
	userID := r.URL.Query().Get("user_id")
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")

	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Default to daily if no period specified
	if period == "" {
		period = "daily"
	}

	// Get balance based on period type
	var balance interface{}
	var err error

	switch period {
	case "daily":
		balance, err = getDailyBalance(userID, date)
	case "weekly":
		balance, err = getWeeklyBalance(userID, date)
	case "monthly":
		balance, err = getMonthlyBalance(userID, date)
	case "quarterly":
		balance, err = getQuarterlyBalance(userID, date)
	case "semiannual":
		balance, err = getSemiannualBalance(userID, date)
	case "annual":
		balance, err = getAnnualBalance(userID, date)
	default:
		sendErrorResponse(w, "Invalid period type", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Error retrieving %s balance: %v", period, err)
		sendErrorResponse(w, "Error retrieving balance", http.StatusInternalServerError)
		return
	}

	// Return success response with balance data
	sendSuccessResponse(w, "Balance retrieved successfully", balance)
}
