package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// HTTP handlers for expense fetch operations with Redis caching

func handleFetchExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Try cache first for expense list
	if cacheManager != nil {
		var cachedExpenses []Expense
		err := cacheManager.GetExpenseData(userID, "all", &cachedExpenses)
		if err == nil {
			log.Printf("✅ Cache HIT: expense list for user %s", userID)
			sendSuccessResponse(w, "Expenses fetched successfully from cache", cachedExpenses)
			return
		}
		log.Printf("🔍 Cache MISS: expense list for user %s", userID)
	}

	// Get expenses from database
	expenses, err := fetchExpensesFromDatabase(userID)
	if err != nil {
		log.Printf("Error fetching expenses: %v", err)
		sendErrorResponse(w, "Error fetching expenses", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, "all", expenses)
		if err != nil {
			log.Printf("Warning: Failed to cache expense data for user %s: %v", userID, err)
		}
	}

	// Return expenses as JSON
	sendSuccessResponse(w, "Expenses fetched successfully", expenses)
}

func handleDailyAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get date from query parameter (optional)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	// Try cache first for daily analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("daily_%s", dateStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: daily analytics for user %s, date %s", userID, dateStr)
			sendSuccessResponse(w, "Daily analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: daily analytics for user %s, date %s", userID, dateStr)
	}

	// Fetch daily analytics from database
	analytics, err := fetchDailyAnalytics(userID, dateStr)
	if err != nil {
		log.Printf("Error fetching daily analytics: %v", err)
		sendErrorResponse(w, "Error fetching daily analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("daily_%s", dateStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache daily analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Daily analytics fetched successfully", analytics)
}

func handleWeeklyAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get week from query parameter (optional)
	weekStr := r.URL.Query().Get("week")
	if weekStr == "" {
		year, week := time.Now().ISOWeek()
		weekStr = fmt.Sprintf("%d-W%02d", year, week)
	}

	// Try cache first for weekly analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("weekly_%s", weekStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: weekly analytics for user %s, week %s", userID, weekStr)
			sendSuccessResponse(w, "Weekly analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: weekly analytics for user %s, week %s", userID, weekStr)
	}

	// Fetch weekly analytics from database
	analytics, err := fetchWeeklyAnalytics(userID, weekStr)
	if err != nil {
		log.Printf("Error fetching weekly analytics: %v", err)
		sendErrorResponse(w, "Error fetching weekly analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("weekly_%s", weekStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache weekly analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Weekly analytics fetched successfully", analytics)
}

func handleMonthlyAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get month from query parameter (optional)
	monthStr := r.URL.Query().Get("month")
	if monthStr == "" {
		monthStr = time.Now().Format("2006-01")
	}

	// Try cache first for monthly analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("monthly_%s", monthStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: monthly analytics for user %s, month %s", userID, monthStr)
			sendSuccessResponse(w, "Monthly analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: monthly analytics for user %s, month %s", userID, monthStr)
	}

	// Fetch monthly analytics from database
	analytics, err := fetchMonthlyAnalytics(userID, monthStr)
	if err != nil {
		log.Printf("Error fetching monthly analytics: %v", err)
		sendErrorResponse(w, "Error fetching monthly analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("monthly_%s", monthStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache monthly analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Monthly analytics fetched successfully", analytics)
}

func handleQuarterlyAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get quarter from query parameter (optional)
	quarterStr := r.URL.Query().Get("quarter")
	if quarterStr == "" {
		now := time.Now()
		quarter := (int(now.Month())-1)/3 + 1
		quarterStr = fmt.Sprintf("%d-Q%d", now.Year(), quarter)
	}

	// Try cache first for quarterly analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("quarterly_%s", quarterStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: quarterly analytics for user %s, quarter %s", userID, quarterStr)
			sendSuccessResponse(w, "Quarterly analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: quarterly analytics for user %s, quarter %s", userID, quarterStr)
	}

	// Fetch quarterly analytics from database
	analytics, err := fetchQuarterlyAnalytics(userID, quarterStr)
	if err != nil {
		log.Printf("Error fetching quarterly analytics: %v", err)
		sendErrorResponse(w, "Error fetching quarterly analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("quarterly_%s", quarterStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache quarterly analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Quarterly analytics fetched successfully", analytics)
}

func handleSemiannualAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get half from query parameter (optional)
	halfStr := r.URL.Query().Get("half")
	if halfStr == "" {
		now := time.Now()
		half := (int(now.Month())-1)/6 + 1
		halfStr = fmt.Sprintf("%d-H%d", now.Year(), half)
	}

	// Try cache first for semiannual analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("semiannual_%s", halfStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: semiannual analytics for user %s, half %s", userID, halfStr)
			sendSuccessResponse(w, "Semiannual analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: semiannual analytics for user %s, half %s", userID, halfStr)
	}

	// Fetch semiannual analytics from database
	analytics, err := fetchSemiannualAnalytics(userID, halfStr)
	if err != nil {
		log.Printf("Error fetching semiannual analytics: %v", err)
		sendErrorResponse(w, "Error fetching semiannual analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("semiannual_%s", halfStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache semiannual analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Semiannual analytics fetched successfully", analytics)
}

func handleAnnualAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get year from query parameter (optional)
	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		yearStr = time.Now().Format("2006")
	}

	// Try cache first for annual analytics
	if cacheManager != nil {
		var cachedAnalytics interface{}
		err := cacheManager.GetExpenseData(userID, fmt.Sprintf("annual_%s", yearStr), &cachedAnalytics)
		if err == nil {
			log.Printf("✅ Cache HIT: annual analytics for user %s, year %s", userID, yearStr)
			sendSuccessResponse(w, "Annual analytics fetched successfully from cache", cachedAnalytics)
			return
		}
		log.Printf("🔍 Cache MISS: annual analytics for user %s, year %s", userID, yearStr)
	}

	// Fetch annual analytics from database
	analytics, err := fetchAnnualAnalytics(userID, yearStr)
	if err != nil {
		log.Printf("Error fetching annual analytics: %v", err)
		sendErrorResponse(w, "Error fetching annual analytics", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, fmt.Sprintf("annual_%s", yearStr), analytics)
		if err != nil {
			log.Printf("Warning: Failed to cache annual analytics for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Annual analytics fetched successfully", analytics)
}

func handleFetchBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Try cache first for balance data
	if cacheManager != nil {
		var cachedBalance interface{}
		err := cacheManager.GetExpenseData(userID, "balance", &cachedBalance)
		if err == nil {
			log.Printf("✅ Cache HIT: balance data for user %s", userID)
			sendSuccessResponse(w, "Balance fetched successfully from cache", cachedBalance)
			return
		}
		log.Printf("🔍 Cache MISS: balance data for user %s", userID)
	}

	// Fetch balance from database
	balance, err := fetchUserBalance(userID)
	if err != nil {
		log.Printf("Error fetching balance: %v", err)
		sendErrorResponse(w, "Error fetching balance", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheExpenseData(userID, "balance", balance)
		if err != nil {
			log.Printf("Warning: Failed to cache balance data for user %s: %v", userID, err)
		}
	}

	sendSuccessResponse(w, "Balance fetched successfully", balance)
}

func handleUpdateCashBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for balance update
	var updateRequest struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
	}
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

	// Update cash balance in database
	err = updateUserCashBalance(updateRequest.UserID, updateRequest.Amount)
	if err != nil {
		log.Printf("Error updating cash balance: %v", err)
		sendErrorResponse(w, "Error updating cash balance", http.StatusInternalServerError)
		return
	}

	// Invalidate cache since balance was updated
	invalidateExpenseCache(updateRequest.UserID)

	sendSuccessResponse(w, "Cash balance updated successfully", nil)
}

func handleUpdateBankBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for balance update
	var updateRequest struct {
		UserID string  `json:"user_id"`
		Amount float64 `json:"amount"`
	}
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

	// Update bank balance in database
	err = updateUserBankBalance(updateRequest.UserID, updateRequest.Amount)
	if err != nil {
		log.Printf("Error updating bank balance: %v", err)
		sendErrorResponse(w, "Error updating bank balance", http.StatusInternalServerError)
		return
	}

	// Invalidate cache since balance was updated
	invalidateExpenseCache(updateRequest.UserID)

	sendSuccessResponse(w, "Bank balance updated successfully", nil)
}

// Utility functions for API responses
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ApiResponse{
		Success: false,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
