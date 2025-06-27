package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/herobudget/backend/common"
	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras de datos
type Expense struct {
	ID            int     `json:"id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"` // "cash" o "bank"
	Description   string  `json:"description,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

type AddExpenseRequest struct {
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description,omitempty"`
}

type UpdateExpenseRequest struct {
	UserID        string  `json:"user_id"`
	ExpenseID     int     `json:"expense_id"`
	Amount        float64 `json:"amount,omitempty"`
	Date          string  `json:"date,omitempty"`
	Category      string  `json:"category,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	Description   string  `json:"description,omitempty"`
}

type DeleteExpenseRequest struct {
	UserID    string `json:"user_id"`
	ExpenseID int    `json:"expense_id"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	// Database connection for financial data persistence
	db *sql.DB
	// Context for database operations with proper timeout handling
	ctx = context.Background()
	// Cache manager for Redis operations to improve performance
	cacheManager *common.CacheManager
)

func init() {
	// Parse command line flags
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Determine database path based on environment flag
	var dbPath string
	if *prodMode {
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)
	} else if *devMode {
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to development mode if no flag specified
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)
	}

	var err error
	// Open the database connection with SQLite driver
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection to ensure database is accessible
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	// CENTRALIZED SCHEMA: DDL operations moved to database_schema.sql
	// Tables are now created by centralized database initialization
	log.Println("✅ Using centralized database schema - no local DDL operations")
	
	// Initialize cache manager for Redis operations
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	}

	log.Println("Database connection established successfully")
	log.Println("Expense Management service initialized successfully")
}

func main() {
	// Set up CORS middleware and routes for expense management endpoints
	http.HandleFunc("/expenses/add", corsMiddleware(handleAddExpense))
	http.HandleFunc("/expenses/update", corsMiddleware(handleUpdateExpense))
	http.HandleFunc("/expenses/delete", corsMiddleware(handleDeleteExpense))
	http.HandleFunc("/expenses/fetch", corsMiddleware(handleFetchExpenses))
	
	// CRITICAL: Add the /expenses endpoint that Flutter expects  
	http.HandleFunc("/expenses", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔥 NEW ENDPOINT /expenses called with method: %s, query: %s", r.Method, r.URL.RawQuery)
		handleFetchExpenses(w, r)
	}))
	http.HandleFunc("/expenses/analytics/daily", corsMiddleware(handleDailyAnalytics))
	http.HandleFunc("/expenses/analytics/weekly", corsMiddleware(handleWeeklyAnalytics))
	http.HandleFunc("/expenses/analytics/monthly", corsMiddleware(handleMonthlyAnalytics))
	http.HandleFunc("/expenses/analytics/quarterly", corsMiddleware(handleQuarterlyAnalytics))
	http.HandleFunc("/expenses/analytics/semiannual", corsMiddleware(handleSemiannualAnalytics))
	http.HandleFunc("/expenses/analytics/annual", corsMiddleware(handleAnnualAnalytics))
	http.HandleFunc("/balance/fetch", corsMiddleware(handleFetchBalance))
	http.HandleFunc("/balance/update-cash", corsMiddleware(handleUpdateCashBalance))
	http.HandleFunc("/balance/update-bank", corsMiddleware(handleUpdateBankBalance))

	// Rutas de sincronización offline para manejo de operaciones offline
	http.HandleFunc("/sync/batch", corsMiddleware(handleSyncBatch))
	http.HandleFunc("/sync/changes", corsMiddleware(handleSyncChanges))
	http.HandleFunc("/sync/resolve-conflicts", corsMiddleware(handleResolveConflicts))
	http.HandleFunc("/sync/health", corsMiddleware(handleSyncHealth))
	http.HandleFunc("/sync/stats", corsMiddleware(handleSyncStats))
	http.HandleFunc("/sync/config", corsMiddleware(handleSyncConfig))

	// Start the HTTP server on port 8086
	port := 8094
	log.Printf("Expense Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers for cross-origin requests from frontend
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight OPTIONS requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler in the chain
		next(w, r)
	}
}

// Cache invalidation function for expense-related operations
func invalidateExpenseCache(userID string) {
	if cacheManager != nil {
		// Invalidate all periods for expense data
		err := cacheManager.InvalidateExpenseCache(userID, "daily", "weekly", "monthly", "quarterly", "semiannual", "annual")
		if err != nil {
			log.Printf("Warning: Failed to invalidate expense cache for user %s: %v", userID, err)
		}
		
		// Also invalidate dashboard cache since expenses affect dashboard
		err = cacheManager.InvalidateDashboardCache(userID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", userID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (expenses and dashboard)", userID)
	}
}

func handleAddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body to extract expense data
	var expense AddExpenseRequest
	err := json.NewDecoder(r.Body).Decode(&expense)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields for expense creation
	if expense.UserID == "" || expense.Amount <= 0 || expense.Date == "" || expense.Category == "" {
		sendErrorResponse(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Validate payment method to ensure it's either cash or bank
	if expense.PaymentMethod != "cash" && expense.PaymentMethod != "bank" {
		sendErrorResponse(w, "Payment method must be 'cash' or 'bank'", http.StatusBadRequest)
		return
	}

	// Add the expense to the database
	expenseID, err := addExpenseToDatabase(expense)
	if err != nil {
		log.Printf("Error adding expense: %v", err)
		sendErrorResponse(w, "Error adding expense", http.StatusInternalServerError)
		return
	}

	// Update all balance tables with the new expense
	err = updateAllBalanceTablesExpense(expense.UserID, expense.Date, expense.Amount, expense.PaymentMethod)
	if err != nil {
		log.Printf("Error updating balance tables: %v", err)
		// Continue despite error since expense was added successfully
	}

	// Update time-based balances including monthly cash bank balance with cascade effects
	err = updateTimeBalances(expense.UserID, expense.Amount, expense.Date)
	if err != nil {
		log.Printf("Error updating time balances: %v", err)
		// Continue despite error since expense was added successfully
	}

	// Invalidate cache since expense data was modified
	invalidateExpenseCache(expense.UserID)

	// Return success response with the new expense ID
	sendSuccessResponse(w, "Expense added successfully", map[string]interface{}{
		"expense_id": expenseID,
		"expense":    expense,
	})
}

func handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body to extract update data
	var updateRequest UpdateExpenseRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields for expense update
	if updateRequest.UserID == "" || updateRequest.ExpenseID <= 0 {
		sendErrorResponse(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Get the current expense data to calculate balance changes
	currentExpense, err := getExpenseByID(updateRequest.ExpenseID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current expense: %v", err)
		sendErrorResponse(w, "Error fetching expense data", http.StatusInternalServerError)
		return
	}

	// Update the expense in the database
	err = updateExpenseInDatabase(updateRequest)
	if err != nil {
		log.Printf("Error updating expense: %v", err)
		sendErrorResponse(w, "Error updating expense", http.StatusInternalServerError)
		return
	}

	// Reverse the old expense effect and apply the new one to balance tables
	err = reverseExpenseEffect(updateRequest.UserID, currentExpense.Date, currentExpense.Amount, currentExpense.PaymentMethod)
	if err != nil {
		log.Printf("Error reversing old expense effect: %v", err)
	}

	// Get updated expense data to apply new effect
	updatedExpense, err := getExpenseByID(updateRequest.ExpenseID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching updated expense: %v", err)
	} else {
		err = updateAllBalanceTablesExpense(updateRequest.UserID, updatedExpense.Date, updatedExpense.Amount, updatedExpense.PaymentMethod)
		if err != nil {
			log.Printf("Error updating balance tables with new expense: %v", err)
		}
	}

	// Invalidate cache since expense data was modified
	invalidateExpenseCache(updateRequest.UserID)

	// Return success response
	sendSuccessResponse(w, "Expense updated successfully", nil)
}

func handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body to extract deletion data
	var deleteRequest DeleteExpenseRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields for expense deletion
	if deleteRequest.UserID == "" || deleteRequest.ExpenseID <= 0 {
		sendErrorResponse(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Get the expense data before deletion to reverse its effect on balances
	expense, err := getExpenseByID(deleteRequest.ExpenseID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching expense for deletion: %v", err)
		sendErrorResponse(w, "Error fetching expense data", http.StatusInternalServerError)
		return
	}

	// Delete the expense from the database
	err = deleteExpenseFromDatabase(deleteRequest.ExpenseID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting expense: %v", err)
		sendErrorResponse(w, "Error deleting expense", http.StatusInternalServerError)
		return
	}

	// Reverse the expense effect on balance tables
	err = reverseExpenseEffect(deleteRequest.UserID, expense.Date, expense.Amount, expense.PaymentMethod)
	if err != nil {
		log.Printf("Error reversing expense effect: %v", err)
		// Continue despite error since expense was deleted successfully
	}

	// Invalidate cache since expense data was modified
	invalidateExpenseCache(deleteRequest.UserID)

	// Return success response
	sendSuccessResponse(w, "Expense deleted successfully", nil)
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}