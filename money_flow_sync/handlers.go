package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// corsMiddleware maneja CORS para permitir requests desde el frontend
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for cross-origin resource sharing
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's OPTIONS request, return with just the headers (preflight request)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler in the chain
		next(w, r)
	}
}

// handleSyncMoneyFlow maneja requests POST para sincronizar money flow con cache invalidation
func handleSyncMoneyFlow(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for sync parameters
	var syncRequest SyncRequest
	err := json.NewDecoder(r.Body).Decode(&syncRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request parameters
	if syncRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if syncRequest.Period == "" {
		syncRequest.Period = "monthly" // Default to monthly period
	}

	// Sync money flow data using core business logic
	budget, err := syncMoneyFlow(syncRequest.UserID, syncRequest.Period)
	if err != nil {
		log.Printf("Error syncing money flow: %v", err)
		sendErrorResponse(w, "Error syncing money flow", http.StatusInternalServerError)
		return
	}

	// Invalidate cache after syncing money flow data
	invalidateMoneyFlowCache(syncRequest.UserID)

	// Return success response with synchronized data
	sendSuccessResponse(w, "Money flow synced successfully", budget)
}

// handleGetMoneyFlowData maneja requests GET para obtener datos de money flow con Redis caching
func handleGetMoneyFlowData(w http.ResponseWriter, r *http.Request) {
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

	// Get period from query parameter (default to monthly)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	log.Printf("Getting money flow data for user %s with period %s", userID, period)

	// Try cache first for money flow data
	if cacheManager != nil {
		var cachedBudget BudgetData
		cacheKey := fmt.Sprintf("money_flow_%s", period)
		err := cacheManager.GetDashboardData(userID, cacheKey, &cachedBudget)
		if err == nil {
			log.Printf("✅ Cache HIT: money flow data for user %s, period %s", userID, period)
			sendSuccessResponse(w, "Money flow data retrieved from cache", cachedBudget)
			return
		}
		log.Printf("🔍 Cache MISS: money flow data for user %s, period %s", userID, period)
	}

	// Get money flow data using synchronization logic
	budget, err := syncMoneyFlow(userID, period)
	if err != nil {
		log.Printf("Error getting money flow data: %v", err)
		sendErrorResponse(w, "Error getting money flow data", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		cacheKey := fmt.Sprintf("money_flow_%s", period)
		err = cacheManager.CacheDashboardData(userID, cacheKey, budget)
		if err != nil {
			log.Printf("Warning: Failed to cache money flow data for user %s: %v", userID, err)
		}
	}

	// Return success response with retrieved data
	sendSuccessResponse(w, "Money flow data retrieved successfully", budget)
}

// syncMoneyFlow lógica principal para sincronizar datos de flujo de dinero
func syncMoneyFlow(userID, period string) (*BudgetData, error) {
	log.Printf("Syncing money flow for user %s with period %s", userID, period)

	// Get date range for the specified period
	startDate, endDate := getDateRangeForPeriod(period)
	log.Printf("Date range: %s to %s", startDate, endDate)

	// Get remaining amount from previous period for carry-over calculation
	previousPeriod, fromPrevious := getPreviousPeriodData(userID, period)
	log.Printf("Previous period: %s, fromPrevious: %.2f", previousPeriod, fromPrevious)

	// Get total income for the period from database
	totalIncome, err := getTotalIncomeForPeriod(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("error getting total income: %v", err)
	}
	log.Printf("Total income: %.2f", totalIncome)

	// Get spent amount for the period from expenses table
	spentAmount, err := getSpentAmountForPeriod(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("error getting spent amount: %v", err)
	}
	log.Printf("Spent amount: %.2f", spentAmount)

	// Get upcoming bills amount from bills table
	upcomingAmount, err := getUpcomingBillsAmount(userID, startDate, endDate)
	if err != nil {
		log.Printf("Error getting upcoming bills amount: %v", err)
		return nil, fmt.Errorf("error getting upcoming bills amount: %v", err)
	}
	log.Printf("Upcoming amount: %.2f", upcomingAmount)

	// Calculate total and remaining amounts using business logic
	totalAmount := fromPrevious + totalIncome
	remainingAmount := totalAmount - spentAmount - upcomingAmount

	// Calculate percentage of budget used
	var percent float64
	if totalAmount > 0 {
		percent = ((spentAmount + upcomingAmount) / totalAmount) * 100
	}

	log.Printf("Total amount: %.2f, Remaining amount: %.2f, Percent: %.2f", totalAmount, remainingAmount, percent)

	// Create budget data structure with calculated values
	budget := &BudgetData{
		UserID:          userID,
		Period:          period,
		Date:            time.Now().Format("2006-01-02"),
		TotalAmount:     totalAmount,
		RemainingAmount: remainingAmount,
		SpentAmount:     spentAmount,
		UpcomingAmount:  upcomingAmount,
		FromPrevious:    fromPrevious,
		Percent:         percent,
		TotalIncome:     totalIncome,
	}

	// Update budget record in database with new calculations
	err = updateBudgetData(budget)
	if err != nil {
		return nil, fmt.Errorf("error updating budget data: %v", err)
	}

	// Update finance metrics for reporting and analysis
	err = updateFinanceMetrics(userID, period, totalIncome, spentAmount, upcomingAmount)
	if err != nil {
		log.Printf("Warning: error updating finance metrics: %v", err)
		// Don't fail the entire operation if updating finance metrics fails
	}

	// Invalidate related cache entries after successful data update
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(userID, period)
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
		err = cacheManager.InvalidateIncomeCache(userID, period)
		if err != nil {
			log.Printf("Warning: Failed to invalidate income cache: %v", err)
		}
	}

	return budget, nil
}

// invalidateMoneyFlowCache invalidates money flow related cache for a user
func invalidateMoneyFlowCache(userID string) {
	if cacheManager != nil {
		// Invalidate dashboard cache for all periods
		err := cacheManager.InvalidateDashboardCache(userID, "daily", "weekly", "monthly", "quarterly", "semiannual", "annual")
		if err != nil {
			log.Printf("Warning: Failed to invalidate money flow cache for user %s: %v", userID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (money flow sync)", userID)
	}
}

// sendSuccessResponse envía respuesta exitosa con datos
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse envía respuesta de error con código de estado
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ApiResponse{
		Success: false,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}