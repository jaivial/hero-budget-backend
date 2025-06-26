package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TopTransactionItem represents a single top transaction item
type TopTransactionItem struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Category      string  `json:"category"`
	Amount        float64 `json:"amount"`
	Type          string  `json:"type"` // "income", "expense", "bill"
	Date          string  `json:"date"`
	IsPaid        *bool   `json:"is_paid,omitempty"`
	Icon          string  `json:"icon,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
}

// TopTransactionsResponse represents the response for top transactions
type TopTransactionsResponse struct {
	TopExpenses []TopTransactionItem `json:"top_expenses"`
	TopIncomes  []TopTransactionItem `json:"top_incomes"`
	TopBills    []TopTransactionItem `json:"top_bills"`
	Month       string               `json:"month"`
	Total       int                  `json:"total"`
}

// CategoryStatsItem represents statistics for a category
type CategoryStatsItem struct {
	Category         string  `json:"category"`
	DisplayName      string  `json:"display_name"`
	TotalAmount      float64 `json:"total_amount"`
	Percentage       float64 `json:"percentage"`
	Type             string  `json:"type"` // "income" or "expense"
	Icon             string  `json:"icon,omitempty"`
	TransactionCount int     `json:"transaction_count"`
}

// CategoryStatsResponse represents the response for category statistics
type CategoryStatsResponse struct {
	Categories []CategoryStatsItem `json:"categories"`
	Type       string              `json:"type"` // "income" or "expense"
	Month      string              `json:"month"`
	Total      int                 `json:"total"`
}

// TopTransactionsRequest represents the request for top transactions
type TopTransactionsRequest struct {
	UserID string `json:"user_id"`
	Month  string `json:"month,omitempty"` // Format: YYYY-MM
	Limit  int    `json:"limit,omitempty"` // Default: 4
}

// handleTopExpenses returns the top expenses for a given month
func handleTopExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	limitStr := r.URL.Query().Get("limit")

	if userID == "" {
		sendErrorResponse(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	// Default to current month if not provided
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Default limit to 4
	limit := 4
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch top expenses
	topExpenses, err := fetchTopExpenses(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top expenses: %v", err)
		sendErrorResponse(w, "Failed to fetch top expenses", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Success     bool                 `json:"success"`
		Message     string               `json:"message"`
		Data        []TopTransactionItem `json:"data"`
		Month       string               `json:"month"`
		Total       int                  `json:"total"`
		Limit       int                  `json:"limit"`
		Type        string               `json:"type"`
	}{
		Success: true,
		Message: "Top expenses fetched successfully",
		Data:    topExpenses,
		Month:   month,
		Total:   len(topExpenses),
		Limit:   limit,
		Type:    "expense",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTopIncomes returns the top incomes for a given month
func handleTopIncomes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	limitStr := r.URL.Query().Get("limit")

	if userID == "" {
		sendErrorResponse(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	// Default to current month if not provided
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Default limit to 4
	limit := 4
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch top incomes
	topIncomes, err := fetchTopIncomes(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top incomes: %v", err)
		sendErrorResponse(w, "Failed to fetch top incomes", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Success     bool                 `json:"success"`
		Message     string               `json:"message"`
		Data        []TopTransactionItem `json:"data"`
		Month       string               `json:"month"`
		Total       int                  `json:"total"`
		Limit       int                  `json:"limit"`
		Type        string               `json:"type"`
	}{
		Success: true,
		Message: "Top incomes fetched successfully",
		Data:    topIncomes,
		Month:   month,
		Total:   len(topIncomes),
		Limit:   limit,
		Type:    "income",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTopBills returns the top unpaid bills for a given month
func handleTopBills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	limitStr := r.URL.Query().Get("limit")

	if userID == "" {
		sendErrorResponse(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	// Default to current month if not provided
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Default limit to 4
	limit := 4
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch top bills
	topBills, err := fetchTopBills(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top bills: %v", err)
		sendErrorResponse(w, "Failed to fetch top bills", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Success     bool                 `json:"success"`
		Message     string               `json:"message"`
		Data        []TopTransactionItem `json:"data"`
		Month       string               `json:"month"`
		Total       int                  `json:"total"`
		Limit       int                  `json:"limit"`
		Type        string               `json:"type"`
	}{
		Success: true,
		Message: "Top bills fetched successfully",
		Data:    topBills,
		Month:   month,
		Total:   len(topBills),
		Limit:   limit,
		Type:    "bill",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCategoryStats returns category statistics for a given type and month
func handleCategoryStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	categoryType := r.URL.Query().Get("type") // "income" or "expense"
	limitStr := r.URL.Query().Get("limit")

	if userID == "" {
		sendErrorResponse(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	if categoryType == "" {
		sendErrorResponse(w, "Missing type parameter (income or expense)", http.StatusBadRequest)
		return
	}

	if categoryType != "income" && categoryType != "expense" {
		sendErrorResponse(w, "Invalid type parameter. Must be 'income' or 'expense'", http.StatusBadRequest)
		return
	}

	// Default to current month if not provided
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Default limit to 5
	limit := 5
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch category statistics
	categoryStats, err := fetchCategoryStats(userID, month, categoryType, limit)
	if err != nil {
		log.Printf("Error fetching category stats: %v", err)
		sendErrorResponse(w, "Failed to fetch category statistics", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Success    bool                `json:"success"`
		Message    string              `json:"message"`
		Data       []CategoryStatsItem `json:"data"`
		Month      string              `json:"month"`
		Type       string              `json:"type"`
		Total      int                 `json:"total"`
		Limit      int                 `json:"limit"`
	}{
		Success: true,
		Message: fmt.Sprintf("Category statistics for %s fetched successfully", categoryType),
		Data:    categoryStats,
		Month:   month,
		Type:    categoryType,
		Total:   len(categoryStats),
		Limit:   limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTopTransactions returns combined top transactions (expenses, incomes, bills)
func handleTopTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	limitStr := r.URL.Query().Get("limit")

	if userID == "" {
		sendErrorResponse(w, "Missing user_id parameter", http.StatusBadRequest)
		return
	}

	// Default to current month if not provided
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	// Default limit to 4
	limit := 4
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Fetch all top transactions
	topExpenses, err := fetchTopExpenses(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top expenses: %v", err)
		sendErrorResponse(w, "Failed to fetch top expenses", http.StatusInternalServerError)
		return
	}

	topIncomes, err := fetchTopIncomes(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top incomes: %v", err)
		sendErrorResponse(w, "Failed to fetch top incomes", http.StatusInternalServerError)
		return
	}

	topBills, err := fetchTopBills(userID, month, limit)
	if err != nil {
		log.Printf("Error fetching top bills: %v", err)
		sendErrorResponse(w, "Failed to fetch top bills", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := struct {
		Success     bool                 `json:"success"`
		Message     string               `json:"message"`
		Data        TopTransactionsResponse `json:"data"`
		Month       string               `json:"month"`
		Limit       int                  `json:"limit"`
	}{
		Success: true,
		Message: "Top transactions fetched successfully",
		Data: TopTransactionsResponse{
			TopExpenses: topExpenses,
			TopIncomes:  topIncomes,
			TopBills:    topBills,
			Month:       month,
			Total:       len(topExpenses) + len(topIncomes) + len(topBills),
		},
		Month: month,
		Limit: limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}