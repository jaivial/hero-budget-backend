package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// corsMiddleware maneja CORS para permitir requests desde el frontend
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for cross-origin resource sharing
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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

// handleAddIncome maneja requests para agregar nuevos ingresos
func handleAddIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body for income data
	var addRequest AddIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&addRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request parameters
	if addRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if addRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	if addRequest.Date == "" {
		// Use current date if not provided
		addRequest.Date = time.Now().Format("2006-01-02")
	}

	if addRequest.Category == "" {
		sendErrorResponse(w, "Category is required", http.StatusBadRequest)
		return
	}

	if addRequest.PaymentMethod == "" || (addRequest.PaymentMethod != "cash" && addRequest.PaymentMethod != "bank") {
		sendErrorResponse(w, "Valid payment method (cash or bank) is required", http.StatusBadRequest)
		return
	}

	// Create an income object with validated data
	income := Income{
		UserID:        addRequest.UserID,
		Amount:        addRequest.Amount,
		Date:          addRequest.Date,
		Category:      addRequest.Category,
		CategoryID:    addRequest.CategoryID, // Support for new category_id field
		PaymentMethod: addRequest.PaymentMethod,
		Description:   addRequest.Description,
	}

	// Add the income to the database
	incomeID, err := addIncome(income)
	if err != nil {
		log.Printf("Error adding income: %v", err)
		sendErrorResponse(w, "Error adding income", http.StatusInternalServerError)
		return
	}

	// Set the ID of the newly added income
	income.ID = incomeID

	// Record sync operation with auto-generated operation_id (following consistent pattern)
	// Critical: ALL handlers must follow the same pattern for sync operations
	log.Printf("Recording sync operation for income creation with auto-generated operation_id")

	// Create sync operation data matching the income structure
	syncData := map[string]interface{}{
		"id":             incomeID,
		"user_id":        income.UserID,
		"amount":         income.Amount,
		"date":           income.Date,
		"category":       income.Category,
		"payment_method": income.PaymentMethod,
		"description":    income.Description,
		"created_at":     time.Now().Format("2006-01-02 15:04:05"),
		"updated_at":     time.Now().Format("2006-01-02 15:04:05"),
	}

	// Add sync operation record to database with auto-generated operation_id
	err = addSyncOperation(
		income.UserID,
		"", // Empty operation_id triggers auto-generation
		"create",
		"incomes",
		strconv.Itoa(incomeID),
		syncData,
		addRequest.DeviceID, // Use device_id from request
		0,                   // Timestamp auto-generated
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for income creation: %v", err)
		// Don't fail the income creation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for income ID: %d", incomeID)
	}

	// Update cash or bank balance based on payment method
	if err := updateBalance(income.UserID, income.Amount, income.PaymentMethod); err != nil {
		log.Printf("Error updating balance: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Actualizar los balances por períodos
	if err := updateTimeBalances(income.UserID, income.Amount, income.Date); err != nil {
		log.Printf("Error updating time balances: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Balance recalculation - simplified logging
	log.Printf("Note: Balance recalculation triggered for userID: %s, date: %s", income.UserID, income.Date)

	// Invalidate income analytics cache after adding new income
	invalidateIncomeAnalytics(income.UserID)

	// Invalidate related cache entries if available
	if cacheManager != nil {
		err = cacheManager.InvalidateIncomeCache(income.UserID, "monthly", "daily", "weekly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate income cache: %v", err)
		}
		err = cacheManager.InvalidateDashboardCache(income.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response with income data
	sendSuccessResponse(w, "Income added successfully", income)
}

// handleListIncomes maneja requests para listar ingresos de un usuario
func handleListIncomes(w http.ResponseWriter, r *http.Request) {
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

	// Get optional filters from query parameters
	category := r.URL.Query().Get("category")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	paymentMethod := r.URL.Query().Get("payment_method")

	// Try to get data from cache first
	cacheKey := buildIncomeListCacheKey(userID, category, startDate, endDate, paymentMethod)
	if cacheManager != nil {
		var cachedIncomes []Income
		err := cacheManager.GetIncomeData(userID, cacheKey, &cachedIncomes)
		if err == nil {
			log.Printf("✓ Cache HIT: income list for user %s", userID)
			sendSuccessResponse(w, "Incomes retrieved from cache", cachedIncomes)
			return
		}
		log.Printf("🔍 Cache MISS: income list for user %s", userID)
	}

	// Get incomes from database with filters
	incomes, err := getIncomes(userID, category, startDate, endDate, paymentMethod)
	if err != nil {
		log.Printf("Error retrieving incomes: %v", err)
		sendErrorResponse(w, "Error retrieving incomes", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests
	if cacheManager != nil {
		err = cacheManager.CacheIncomeData(userID, cacheKey, incomes)
		if err != nil {
			log.Printf("Warning: Failed to cache income list: %v", err)
		}
	}

	// Return success response with incomes list
	sendSuccessResponse(w, "Incomes retrieved successfully", incomes)
}

// buildIncomeListCacheKey construye key de cache para lista de ingresos
func buildIncomeListCacheKey(userID, category, startDate, endDate, paymentMethod string) string {
	return "list_" + userID + "_" + category + "_" + startDate + "_" + endDate + "_" + paymentMethod
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
