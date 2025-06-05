package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	db *sql.DB
)

func init() {
	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create tables if they don't exist
	createTablesIfNotExist()

	// Add cash_amount and bank_amount columns to all balance tables if needed
	addCashBankColumnsToAllTables()

	log.Println("Database connection established successfully")
}

func createTablesIfNotExist() {
	// Create expenses table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			category TEXT NOT NULL,
			payment_method TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create expenses table: %v", err)
	}

	// Create balances table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS balances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT UNIQUE NOT NULL,
			cash_balance REAL NOT NULL DEFAULT 0,
			bank_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create balances table: %v", err)
	}

	// Ensure cash_bank table exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			month TEXT NOT NULL,
			cash_amount REAL NOT NULL DEFAULT 0,
			cash_percent REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			bank_percent REAL NOT NULL DEFAULT 0,
			monthly_total REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Ensure cash_bank_transactions table exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			transaction_type TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank_transactions table: %v", err)
	}

	// Create daily_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, date)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_balance table: %v", err)
	}

	// Create indices for daily_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_user ON daily_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_date ON daily_balance(date)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}

	// Create weekly_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_week)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_balance table: %v", err)
	}

	// Create indices for weekly_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_user ON weekly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_week ON weekly_balance(year_week)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}

	// Create monthly_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_month TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create monthly_balance table: %v", err)
	}

	// Create indices for monthly_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_balance_user ON monthly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_balance_month ON monthly_balance(year_month)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_balance: %v", err)
	}

	// Create quarterly_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_quarter)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_balance table: %v", err)
	}

	// Create indices for quarterly_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_balance_user ON quarterly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_balance_quarter ON quarterly_balance(year_quarter)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_balance: %v", err)
	}

	// Create semiannual_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_half TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_half)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_balance table: %v", err)
	}

	// Create indices for semiannual_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_balance_user ON semiannual_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_balance_half ON semiannual_balance(year_half)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_balance: %v", err)
	}

	// Create annual_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_balance table: %v", err)
	}

	// Create indices for annual_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_balance_user ON annual_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_balance_year ON annual_balance(year)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_balance: %v", err)
	}

	// Add cash_amount and bank_amount columns to existing tables if they don't exist
	// For daily_balance
	alterTableSafely("daily_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For weekly_balance
	alterTableSafely("weekly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For monthly_balance
	alterTableSafely("monthly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For quarterly_balance
	alterTableSafely("quarterly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For semiannual_balance
	alterTableSafely("semiannual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For annual_balance
	alterTableSafely("annual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "total_balance", "REAL NOT NULL DEFAULT 0")
}

// Helper function to safely alter a table by adding a column if it doesn't exist
func alterTableSafely(tableName, columnName, columnType string) {
	// Check if the column exists
	var existsQuery string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&existsQuery); err != nil {
		log.Printf("Error checking table %s: %v", tableName, err)
		return
	}

	// If the column doesn't exist in the schema, add it
	if !strings.Contains(existsQuery, columnName) {
		_, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, columnName, columnType))
		if err != nil {
			log.Printf("Error adding column %s to table %s: %v", columnName, tableName, err)
		} else {
			log.Printf("Added column %s to table %s successfully", columnName, tableName)
		}
	}
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/expenses", corsMiddleware(handleFetchExpenses))
	http.HandleFunc("/expenses/add", corsMiddleware(handleAddExpense))
	http.HandleFunc("/expenses/update", corsMiddleware(handleUpdateExpense))
	http.HandleFunc("/expenses/delete", corsMiddleware(handleDeleteExpense))

	port := 8094 // Puerto para el servicio de gastos
	log.Printf("Expense Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's OPTIONS, return with just the headers (preflight request)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

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

	// Get expenses from database
	expenses, err := fetchExpenses(userID)
	if err != nil {
		log.Printf("Error fetching expenses: %v", err)
		sendErrorResponse(w, "Error fetching expenses", http.StatusInternalServerError)
		return
	}

	// Return expenses as JSON
	sendSuccessResponse(w, "Expenses fetched successfully", expenses)
}

func handleAddExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var expense Expense
	err := json.NewDecoder(r.Body).Decode(&expense)
	if err != nil {
		log.Printf("Error parsing request body: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the expense
	if expense.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if expense.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	if expense.Date == "" {
		// Default to current date if not provided
		expense.Date = time.Now().Format("2006-01-02")
	}

	if expense.Category == "" {
		sendErrorResponse(w, "Category is required", http.StatusBadRequest)
		return
	}

	if expense.PaymentMethod == "" || (expense.PaymentMethod != "cash" && expense.PaymentMethod != "bank") {
		sendErrorResponse(w, "Valid payment method (cash or bank) is required", http.StatusBadRequest)
		return
	}

	// Log the expense details
	log.Printf("Adding expense: UserID=%s, Amount=%.2f, Date=%s, Category=%s, PaymentMethod=%s",
		expense.UserID, expense.Amount, expense.Date, expense.Category, expense.PaymentMethod)

	// Add the expense to the database
	expenseID, err := addExpense(expense)
	if err != nil {
		log.Printf("Error adding expense: %v", err)
		sendErrorResponse(w, "Failed to add expense", http.StatusInternalServerError)
		return
	}

	// Set the ID of the newly added expense
	expense.ID = expenseID

	// Update balance based on payment method
	// Need to pass a negative amount since this is an expense (reduces balance)
	if err := updateBalance(expense.UserID, -expense.Amount, expense.PaymentMethod); err != nil {
		log.Printf("Error updating balance: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Actualizar los balances por periodos
	if err := updateTimeBalances(expense.UserID, expense.Amount, expense.Date); err != nil {
		log.Printf("Error updating time balances: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Recalcular todos los balances para asegurar que previous_xxx_amount y balance_xxx_amount se actualicen en cascada
	if err := recalculateAllBalances(expense.UserID, expense.Date); err != nil {
		log.Printf("Error recalculating balances: %v", err)
		// Continue despite the error
	}

	// Return success response
	sendSuccessResponse(w, "Expense added successfully", expense)
}

func handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UpdateExpenseRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.ExpenseID <= 0 {
		sendErrorResponse(w, "Expense ID is required", http.StatusBadRequest)
		return
	}

	// Fetch the expense to update
	origExpense, err := fetchExpenseByID(updateRequest.ExpenseID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching expense: %v", err)
		sendErrorResponse(w, "Expense not found", http.StatusNotFound)
		return
	}

	// Calculate the difference in amount for balance update
	amountDifference := 0.0
	if updateRequest.Amount > 0 {
		amountDifference = origExpense.Amount - updateRequest.Amount
	}

	// Update expense object with new values
	expense := Expense{
		ID:            updateRequest.ExpenseID,
		UserID:        updateRequest.UserID,
		Amount:        updateRequest.Amount,
		Date:          updateRequest.Date,
		Category:      updateRequest.Category,
		PaymentMethod: updateRequest.PaymentMethod,
		Description:   updateRequest.Description,
	}

	// If fields are not provided, use original values
	if updateRequest.Amount <= 0 {
		expense.Amount = origExpense.Amount
	}

	if updateRequest.Date == "" {
		expense.Date = origExpense.Date
	}

	if updateRequest.Category == "" {
		expense.Category = origExpense.Category
	}

	if updateRequest.PaymentMethod == "" {
		expense.PaymentMethod = origExpense.PaymentMethod
	}

	if updateRequest.Description == "" {
		expense.Description = origExpense.Description
	}

	// Update expense in database
	err = updateExpense(expense)
	if err != nil {
		log.Printf("Error updating expense: %v", err)
		sendErrorResponse(w, "Error updating expense", http.StatusInternalServerError)
		return
	}

	// Check if amount, date, or payment method changed
	amountChanged := updateRequest.Amount > 0 && origExpense.Amount != expense.Amount
	dateChanged := updateRequest.Date != "" && origExpense.Date != expense.Date
	paymentMethodChanged := updateRequest.PaymentMethod != "" && origExpense.PaymentMethod != expense.PaymentMethod

	// Update user's balance if amount changed
	if amountDifference != 0 {
		err = updateBalance(expense.UserID, amountDifference, expense.PaymentMethod)
		if err != nil {
			log.Printf("Error updating balance: %v", err)
			// Continue since the expense was already updated
		}
	}

	// Update time balances if necessary
	if amountChanged || dateChanged || paymentMethodChanged {
		// First remove the old expense from time balances
		if err := updateTimeBalances(expense.UserID, -origExpense.Amount, origExpense.Date); err != nil {
			log.Printf("Error removing old expense from time balances: %v", err)
		}

		// Then add the new expense to time balances
		if err := updateTimeBalances(expense.UserID, expense.Amount, expense.Date); err != nil {
			log.Printf("Error adding new expense to time balances: %v", err)
		}

		// Recalcular todos los balances para asegurar que previous_xxx_amount y balance_xxx_amount se actualicen en cascada
		if err := recalculateAllBalances(expense.UserID, expense.Date); err != nil {
			log.Printf("Error recalculating balances: %v", err)
			// Continue despite the error
		}
	}

	// Fetch the updated expense
	updatedExpense, err := fetchExpenseByID(expense.ID, expense.UserID)
	if err != nil {
		log.Printf("Error fetching updated expense: %v", err)
		// Return the updated expense without timestamps
		sendSuccessResponse(w, "Expense updated successfully", expense)
		return
	}

	// Return the updated expense
	sendSuccessResponse(w, "Expense updated successfully", updatedExpense)
}

func handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var deleteRequest DeleteExpenseRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if deleteRequest.ExpenseID <= 0 {
		sendErrorResponse(w, "Expense ID is required", http.StatusBadRequest)
		return
	}

	// Fetch the expense to delete
	expense, err := fetchExpenseByID(deleteRequest.ExpenseID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching expense: %v", err)
		sendErrorResponse(w, "Expense not found", http.StatusNotFound)
		return
	}

	// Delete expense from database
	err = deleteExpense(deleteRequest.ExpenseID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting expense: %v", err)
		sendErrorResponse(w, "Error deleting expense", http.StatusInternalServerError)
		return
	}

	// Update user's balance (add the amount back)
	err = updateBalance(deleteRequest.UserID, expense.Amount, expense.PaymentMethod)
	if err != nil {
		log.Printf("Error updating balance: %v", err)
		// Continue since the expense was already deleted
	}

	// Remove expense from time balances
	if err := updateTimeBalances(deleteRequest.UserID, -expense.Amount, expense.Date); err != nil {
		log.Printf("Error removing expense from time balances: %v", err)
		// Continue despite the error
	}

	// Recalcular todos los balances para asegurar que previous_xxx_amount y balance_xxx_amount se actualicen en cascada
	if err := recalculateAllBalances(deleteRequest.UserID, expense.Date); err != nil {
		log.Printf("Error recalculating balances: %v", err)
		// Continue despite the error
	}

	// Return success
	sendSuccessResponse(w, "Expense deleted successfully", nil)
}

func fetchExpenses(userID string) ([]Expense, error) {
	// SQL query to fetch all expenses for a user, ordered by most recent
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM expenses
		WHERE user_id = ?
		ORDER BY date DESC, id DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse rows into expense objects
	expenses := []Expense{}
	for rows.Next() {
		var expense Expense
		err := rows.Scan(
			&expense.ID,
			&expense.UserID,
			&expense.Amount,
			&expense.Date,
			&expense.Category,
			&expense.PaymentMethod,
			&expense.Description,
			&expense.CreatedAt,
			&expense.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

func fetchExpenseByID(expenseID int, userID string) (*Expense, error) {
	// SQL query to fetch a specific expense by ID and user ID
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM expenses
		WHERE id = ? AND user_id = ?
	`

	row := db.QueryRow(query, expenseID, userID)

	var expense Expense
	err := row.Scan(
		&expense.ID,
		&expense.UserID,
		&expense.Amount,
		&expense.Date,
		&expense.Category,
		&expense.PaymentMethod,
		&expense.Description,
		&expense.CreatedAt,
		&expense.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &expense, nil
}

func addExpense(expense Expense) (int, error) {
	// SQL query to insert a new expense
	query := `
		INSERT INTO expenses (user_id, amount, date, category, payment_method, description)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		expense.UserID,
		expense.Amount,
		expense.Date,
		expense.Category,
		expense.PaymentMethod,
		expense.Description,
	)
	if err != nil {
		return 0, err
	}

	// Get the ID of the inserted expense
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func updateExpense(expense Expense) error {
	// SQL query to update an existing expense
	query := `
		UPDATE expenses
		SET amount = ?, date = ?, category = ?, payment_method = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(
		query,
		expense.Amount,
		expense.Date,
		expense.Category,
		expense.PaymentMethod,
		expense.Description,
		expense.ID,
		expense.UserID,
	)
	if err != nil {
		return err
	}

	return nil
}

func deleteExpense(expenseID int, userID string) error {
	// SQL query to delete an expense
	query := `
		DELETE FROM expenses
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(query, expenseID, userID)
	if err != nil {
		return err
	}

	return nil
}

func updateBalance(userID string, amount float64, paymentMethod string) error {
	log.Printf("updateBalance called with userID: %s, amount: %.2f, paymentMethod: %s", userID, amount, paymentMethod)

	// SQL query to check if user exists in the balances table
	checkQuery := `
		SELECT COUNT(*)
		FROM balances
		WHERE user_id = ?
	`

	var count int
	err := db.QueryRow(checkQuery, userID).Scan(&count)
	if err != nil {
		log.Printf("Error checking balances table: %v", err)
		return err
	}

	log.Printf("Found %d records in balances table for user %s", count, userID)
	var query string

	// If user doesn't exist in balances table, insert a new record
	if count == 0 {
		query = `
			INSERT INTO balances (user_id, cash_balance, bank_balance)
			VALUES (?, ?, ?)
		`
		cashAmount := 0.0
		bankAmount := 0.0

		if paymentMethod == "cash" {
			cashAmount = amount
		} else {
			bankAmount = amount
		}

		log.Printf("Inserting new balance record with cash: %.2f, bank: %.2f", cashAmount, bankAmount)
		_, err = db.Exec(query, userID, cashAmount, bankAmount)
	} else {
		// Update existing balance
		if paymentMethod == "cash" {
			query = `
				UPDATE balances
				SET cash_balance = cash_balance + ?
				WHERE user_id = ?
			`
		} else {
			query = `
				UPDATE balances
				SET bank_balance = bank_balance + ?
				WHERE user_id = ?
			`
		}

		log.Printf("Updating existing balance with amount: %.2f for method: %s", amount, paymentMethod)
		_, err = db.Exec(query, amount, userID)
	}

	if err != nil {
		log.Printf("Error updating balances table: %v", err)
		return err
	} else {
		log.Printf("Successfully updated balances table")
	}

	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")
	log.Printf("Processing cash_bank for month: %s", currentMonth)

	// Fetch current cash-bank distribution
	var distribution struct {
		CashAmount   float64
		BankAmount   float64
		MonthlyTotal float64
		Exists       bool
	}

	// Check if a record exists for the current month
	cashBankCheckQuery := `
		SELECT 1
		FROM cash_bank
		WHERE user_id = ? AND month = ?
	`
	var exists bool
	err = db.QueryRow(cashBankCheckQuery, userID, currentMonth).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking cash_bank: %v", err)
		return err
	}

	distribution.Exists = err != sql.ErrNoRows
	log.Printf("Record exists in cash_bank for user %s and month %s: %v", userID, currentMonth, distribution.Exists)

	if distribution.Exists {
		// Get current values
		getQuery := `
			SELECT cash_amount, bank_amount, monthly_total
			FROM cash_bank
			WHERE user_id = ? AND month = ?
		`
		err := db.QueryRow(getQuery, userID, currentMonth).Scan(
			&distribution.CashAmount,
			&distribution.BankAmount,
			&distribution.MonthlyTotal,
		)
		if err != nil {
			log.Printf("Error fetching cash_bank data: %v", err)
			return err
		}

		log.Printf("Current cash_bank values - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Update the appropriate amount based on payment method
		if paymentMethod == "cash" {
			distribution.CashAmount += amount
		} else if paymentMethod == "bank" {
			distribution.BankAmount += amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		log.Printf("Updated cash_bank values - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		log.Printf("Calculated cash_bank percentages - cash: %.2f%%, bank: %.2f%%",
			cashPercent, bankPercent)

		// Update the record
		updateQuery := `
			UPDATE cash_bank
			SET cash_amount = ?, cash_percent = ?, bank_amount = ?, bank_percent = ?, monthly_total = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(
			updateQuery,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
			userID,
			currentMonth,
		)
		if err != nil {
			log.Printf("Error updating cash_bank: %v", err)
			return err
		}
		log.Printf("Successfully updated cash_bank record")
	} else {
		// Create a new record with initial values
		if paymentMethod == "cash" {
			distribution.CashAmount = amount
			distribution.BankAmount = 0
		} else if paymentMethod == "bank" {
			distribution.CashAmount = 0
			distribution.BankAmount = amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		log.Printf("Creating new cash_bank record - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		log.Printf("Calculated cash_bank percentages for new record - cash: %.2f%%, bank: %.2f%%",
			cashPercent, bankPercent)

		// Insert the new record
		insertQuery := `
			INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(
			insertQuery,
			userID,
			currentMonth,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
		)
		if err != nil {
			log.Printf("Error inserting new cash_bank record: %v", err)
			return err
		}
		log.Printf("Successfully inserted new cash_bank record")
	}

	// Add transaction record for the expense (negative amount)
	// Note: For expenses, we record a negative transaction
	transactionQuery := `
		INSERT INTO cash_bank_transactions (user_id, transaction_type, amount, date)
		VALUES (?, ?, ?, ?)
	`
	transactionType := "expense_" + paymentMethod
	transactionAmount := -math.Abs(amount) // Ensure amount is negative for expenses

	log.Printf("Recording transaction - type: %s, amount: %.2f", transactionType, transactionAmount)

	_, err = db.Exec(
		transactionQuery,
		userID,
		transactionType,
		transactionAmount,
		time.Now().Format("2006-01-02"),
	)
	if err != nil {
		log.Printf("Error recording cash_bank_transaction: %v", err)
		return err
	}
	log.Printf("Successfully recorded cash_bank_transaction")

	log.Printf("updateBalance completed successfully")
	return nil
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
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

// Función para actualizar los balances por periodos al añadir un gasto
func updateTimeBalances(userID string, amount float64, dateStr string) error {
	// Parse la fecha del gasto
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Obtener la información del gasto para determinar si fue cash o bank
	var paymentMethod string
	err = db.QueryRow(`
		SELECT payment_method FROM expenses
		WHERE user_id = ? AND date = ? AND amount = ?
		ORDER BY created_at DESC LIMIT 1
	`, userID, dateStr, amount).Scan(&paymentMethod)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error fetching payment method: %v", err)
	}

	if err == sql.ErrNoRows {
		// Si no se encuentra, asumimos bank por defecto
		paymentMethod = "bank"
	}

	// Calculamos los montos de cash y bank según el método de pago
	var cashAmount, bankAmount float64
	if paymentMethod == "cash" {
		cashAmount = amount
		bankAmount = 0
	} else {
		cashAmount = 0
		bankAmount = amount
	}

	// Actualizar balance diario
	if err := updateDailyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating daily balance: %v", err)
	}

	// Actualizar balance semanal
	if err := updateWeeklyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating weekly balance: %v", err)
	}

	// Actualizar balance mensual
	if err := updateMonthlyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating monthly balance: %v", err)
	}

	// Actualizar balance trimestral
	if err := updateQuarterlyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating quarterly balance: %v", err)
	}

	// Actualizar balance semestral
	if err := updateSemiannualBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating semiannual balance: %v", err)
	}

	// Actualizar balance anual
	if err := updateAnnualBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating annual balance: %v", err)
	}

	return nil
}

func updateDailyBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	dateStr := date.Format("2006-01-02")

	// Obtener el balance del día anterior para calcular el balance previo
	prevDate := date.AddDate(0, 0, -1)
	prevDateStr := prevDate.Format("2006-01-02")

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Buscar el balance del día anterior
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM daily_balance 
		WHERE user_id = ? AND date = ?
	`, userID, prevDateStr).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro del día anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calcular el balance como el balance previo + ingresos - gastos - facturas
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Verificar si ya existe un registro para esta fecha
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM daily_balance
		WHERE user_id = ? AND date = ?
	`, userID, dateStr).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, insertar uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO daily_balance (user_id, date, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, dateStr, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Actualizar registro existente
		// Calculamos los nuevos totales sumando los valores existentes
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Actualizar los montos de cash y bank sumando los nuevos valores a los existentes
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalcular el balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE daily_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, existingCash, existingBank, prevCashAmount, prevBankAmount, userID, dateStr)
	}

	if err != nil {
		return err
	}

	// Actualizar todos los días posteriores en cascada
	return updateSubsequentDailyBalances(userID, date.AddDate(0, 0, 1))
}

// Función para actualizar días posteriores en cascada
func updateSubsequentDailyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a un año para evitar bucles infinitos
	endDate := startDate.AddDate(1, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		currentDateStr := currentDate.Format("2006-01-02")

		// Verificar si existe un registro para esta fecha
		var exists bool
		var incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_amount, expense_amount, bills_amount, cash_amount, bank_amount FROM daily_balance
			WHERE user_id = ? AND date = ?
		`, userID, currentDateStr).Scan(&exists, &incomeAmount, &expenseAmount, &billsAmount, &cashAmount, &bankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance del día anterior
		prevDate := currentDate.AddDate(0, 0, -1)
		prevDateStr := prevDate.Format("2006-01-02")

		var previousBalance float64
		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT balance, cash_amount, bank_amount FROM daily_balance 
			WHERE user_id = ? AND date = ?
		`, userID, prevDateStr).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			previousBalance = 0
			prevCashAmount = 0
			prevBankAmount = 0
		}

		// Actualizar el balance con el nuevo balance previo
		balance := previousBalance + incomeAmount - expenseAmount - billsAmount

		// CAMBIO EN LA LÓGICA: Siempre acumulamos los valores del día anterior
		// independientemente de si hay transacciones en este día o no
		hasTransactions := incomeAmount != 0 || expenseAmount != 0 || billsAmount != 0

		// Inicializar con los valores del día anterior
		newCashAmount := prevCashAmount
		newBankAmount := prevBankAmount

		// Si hay transacciones propias en este día, las sumamos a lo heredado
		if hasTransactions {
			// Agregamos las transacciones propias de este día
			newCashAmount += cashAmount
			newBankAmount += bankAmount
		}

		_, err = db.Exec(`
			UPDATE daily_balance
			SET previous_balance = ?,
				balance = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, previousBalance, balance, newCashAmount, newBankAmount, prevCashAmount, prevBankAmount, userID, currentDateStr)

		if err != nil {
			return err
		}

		// Pasar al siguiente día
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil
}

func updateWeeklyBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el año e ISO semana
	year, week := date.ISOWeek()
	yearWeek := fmt.Sprintf("%d-W%02d", year, week)

	// Calcular fecha de inicio y fin de la semana
	// El día 0 de una semana es domingo, necesitamos ajustar para obtener lunes (día 1)
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Convertir domingo (0) a 7 para restar correctamente
	}
	startDate := date.AddDate(0, 0, -(dayOfWeek - 1))
	endDate := startDate.AddDate(0, 0, 6)

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Calcular la semana anterior
	prevWeekStart := startDate.AddDate(0, 0, -7)
	prevYearWeek := fmt.Sprintf("%d-W%02d", prevWeekStart.Year(), func() int {
		year, week := prevWeekStart.ISOWeek()
		if prevWeekStart.Year() != year {
			// Ajustar para casos especiales al final/inicio de año
			return 53 // Última semana del año anterior
		}
		return week
	}())

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Buscar el balance de la semana anterior
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM weekly_balance 
		WHERE user_id = ? AND year_week = ?
	`, userID, prevYearWeek).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro de la semana anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calcular el balance como: balance previo + ingresos - gastos - facturas
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Verificar si ya existe un registro para esta semana
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM weekly_balance
		WHERE user_id = ? AND year_week = ?
	`, userID, yearWeek).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, insertar uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO weekly_balance (user_id, year_week, start_date, end_date, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearWeek, startDateStr, endDateStr, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Actualizar registro existente
		// Actualizar los montos sumando los nuevos valores
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Actualizar los montos de cash y bank sumando los nuevos valores a los existentes
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalcular el balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE weekly_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_week = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, existingCash, existingBank, prevCashAmount, prevBankAmount, userID, yearWeek)
	}

	if err != nil {
		return err
	}

	// Actualizar todas las semanas posteriores en cascada
	return updateSubsequentWeeklyBalances(userID, startDate.AddDate(0, 0, 7))
}

// Función para actualizar semanas posteriores en cascada
func updateSubsequentWeeklyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a un año para evitar bucles infinitos
	endDate := startDate.AddDate(1, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		year, week := currentDate.ISOWeek()
		currentYearWeek := fmt.Sprintf("%d-W%02d", year, week)

		// Verificar si existe un registro para esta semana
		var exists bool
		var incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_amount, expense_amount, bills_amount, cash_amount, bank_amount FROM weekly_balance
			WHERE user_id = ? AND year_week = ?
		`, userID, currentYearWeek).Scan(&exists, &incomeAmount, &expenseAmount, &billsAmount, &cashAmount, &bankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance de la semana anterior
		prevWeekStart := currentDate.AddDate(0, 0, -7)
		prevYear, prevWeek := prevWeekStart.ISOWeek()
		prevYearWeek := fmt.Sprintf("%d-W%02d", prevYear, prevWeek)

		var previousBalance float64
		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT balance, cash_amount, bank_amount FROM weekly_balance 
			WHERE user_id = ? AND year_week = ?
		`, userID, prevYearWeek).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			previousBalance = 0
			prevCashAmount = 0
			prevBankAmount = 0
		}

		// Actualizar el balance con el nuevo balance previo
		balance := previousBalance + incomeAmount - expenseAmount - billsAmount

		// CAMBIO EN LA LÓGICA: Siempre acumulamos los valores de la semana anterior
		// independientemente de si hay transacciones en esta semana o no
		hasTransactions := incomeAmount != 0 || expenseAmount != 0 || billsAmount != 0

		// Inicializar con los valores de la semana anterior
		newCashAmount := prevCashAmount
		newBankAmount := prevBankAmount

		// Si hay transacciones propias en esta semana, las sumamos a lo heredado
		if hasTransactions {
			// Agregamos las transacciones propias de esta semana
			newCashAmount += cashAmount
			newBankAmount += bankAmount
		}

		// Calcular los valores de balance para cash y bank
		balanceCashAmount := prevCashAmount + cashAmount
		balanceBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE weekly_balance
			SET previous_balance = ?,
				balance = ?,
				cash_amount = ?,
				bank_amount = ?,
                previous_cash_amount = ?,
                previous_bank_amount = ?,
                balance_cash_amount = ?,
                balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_week = ?
		`, previousBalance, balance, newCashAmount, newBankAmount, prevCashAmount, prevBankAmount, balanceCashAmount, balanceBankAmount, userID, currentYearWeek)

		if err != nil {
			return err
		}

		// Pasar a la siguiente semana
		currentDate = currentDate.AddDate(0, 0, 7)
	}

	return nil
}

func updateMonthlyBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el año y mes
	yearMonth := date.Format("2006-01")

	// Calcular el mes anterior
	prevMonth := date.AddDate(0, -1, 0)
	prevYearMonth := prevMonth.Format("2006-01")

	var prevCashAmount, prevBankAmount float64

	// Buscar los valores del mes anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month = ?
	`, userID, prevYearMonth).Scan(&prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro del mes anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		// Si no hay un mes inmediatamente anterior, buscar el último mes anterior disponible
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month < ?
			ORDER BY year_month DESC LIMIT 1
		`, userID, yearMonth).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún mes anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			prevCashAmount = 0
			prevBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para este mes
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
		expense_cash_amount, expense_bank_amount, 
		bill_cash_amount, bill_bank_amount,
		cash_amount, bank_amount
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
	`, userID, yearMonth).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount - cashAmount
		totalBankAmount := prevBankAmount - bankAmount

		_, err = db.Exec(`
			INSERT INTO monthly_cash_bank_balance (
				user_id, year_month,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearMonth,
			0, 0,
			cashAmount, bankAmount,
			0, 0,
			totalCashAmount, prevCashAmount,
			totalBankAmount, prevBankAmount,
			totalCashAmount, totalBankAmount)
	} else {
		// Actualizar registro existente
		// Calculamos los nuevos totales sumando los valores existentes

		// Actualizar los montos de cash y bank restando los nuevos gastos a los existentes
		newCashAmount := existingCashAmount - cashAmount
		newBankAmount := existingBankAmount - bankAmount

		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance SET
				expense_cash_amount = expense_cash_amount + ?,
				expense_bank_amount = expense_bank_amount + ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`, cashAmount, bankAmount,
			newCashAmount, newBankAmount,
			prevCashAmount, prevBankAmount,
			newCashAmount, newBankAmount,
			userID, yearMonth)
	}

	if err != nil {
		return err
	}

	// Actualizar meses posteriores para ajustar los balances en cascada
	return updateSubsequentMonthlyBalances(userID, date.AddDate(0, 1, 0))
}

func updateQuarterlyBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el trimestre (1-4)
	quarter := (int(date.Month())-1)/3 + 1
	yearQuarter := fmt.Sprintf("%d-Q%d", date.Year(), quarter)

	// Calcular fecha de inicio y fin del trimestre
	startMonth := (quarter-1)*3 + 1
	startDate := time.Date(date.Year(), time.Month(startMonth), 1, 0, 0, 0, 0, date.Location())
	endDate := startDate.AddDate(0, 3, -1)

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Calcular el trimestre anterior
	prevQuarterDate := startDate.AddDate(0, -3, 0)
	prevQuarter := (int(prevQuarterDate.Month())-1)/3 + 1
	prevYearQuarter := fmt.Sprintf("%d-Q%d", prevQuarterDate.Year(), prevQuarter)

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Buscar el balance del trimestre anterior
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM quarterly_balance 
		WHERE user_id = ? AND year_quarter = ?
	`, userID, prevYearQuarter).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro del trimestre anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calcular el balance actual
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Verificar si ya existe un registro para este trimestre
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM quarterly_balance
		WHERE user_id = ? AND year_quarter = ?
	`, userID, yearQuarter).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, insertar uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO quarterly_balance (user_id, year_quarter, start_date, end_date, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearQuarter, startDateStr, endDateStr, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Actualizar registro existente
		// Calculamos los nuevos totales sumando los valores existentes
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Actualizar los montos de cash y bank sumando los nuevos valores a los existentes
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalcular el balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE quarterly_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_quarter = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, existingCash, existingBank, prevCashAmount, prevBankAmount, userID, yearQuarter)
	}

	if err != nil {
		return err
	}

	// Actualizar todos los trimestres posteriores en cascada
	nextQuarterDate := startDate.AddDate(0, 3, 0)
	return updateSubsequentQuarterlyBalances(userID, nextQuarterDate)
}

func updateSemiannualBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el semestre (1-2)
	half := (int(date.Month())-1)/6 + 1
	yearHalf := fmt.Sprintf("%d-H%d", date.Year(), half)

	// Calcular fecha de inicio y fin del semestre
	startMonth := (half-1)*6 + 1
	startDate := time.Date(date.Year(), time.Month(startMonth), 1, 0, 0, 0, 0, date.Location())
	endDate := startDate.AddDate(0, 6, -1)

	startDateStr := startDate.Format("2006-01-02")
	endDateStr := endDate.Format("2006-01-02")

	// Calcular el semestre anterior
	prevHalfDate := startDate.AddDate(0, -6, 0)
	prevHalf := (int(prevHalfDate.Month())-1)/6 + 1
	prevYearHalf := fmt.Sprintf("%d-H%d", prevHalfDate.Year(), prevHalf)

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Buscar el balance del semestre anterior
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM semiannual_balance 
		WHERE user_id = ? AND year_half = ?
	`, userID, prevYearHalf).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro del semestre anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calcular el balance actual
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Verificar si ya existe un registro para este semestre
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM semiannual_balance
		WHERE user_id = ? AND year_half = ?
	`, userID, yearHalf).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, insertar uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO semiannual_balance (user_id, year_half, start_date, end_date, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearHalf, startDateStr, endDateStr, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Actualizar registro existente
		// Calculamos los nuevos totales sumando los valores existentes
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Actualizar los montos de cash y bank sumando los nuevos valores a los existentes
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalcular el balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE semiannual_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_half = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, existingCash, existingBank, prevCashAmount, prevBankAmount, userID, yearHalf)
	}

	if err != nil {
		return err
	}

	// Actualizar todos los semestres posteriores en cascada
	nextHalfDate := startDate.AddDate(0, 6, 0)
	return updateSubsequentSemiannualBalances(userID, nextHalfDate)
}

func updateAnnualBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el año
	year := strconv.Itoa(date.Year())

	// Calcular el año anterior
	prevYear := strconv.Itoa(date.Year() - 1)

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Buscar el balance del año anterior
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM annual_balance 
		WHERE user_id = ? AND year = ?
	`, userID, prevYear).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// Si no existe registro del año anterior, el balance previo es 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calcular el balance actual
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Verificar si ya existe un registro para este año
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM annual_balance
		WHERE user_id = ? AND year = ?
	`, userID, year).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, insertar uno nuevo
		// Los montos de efectivo y banco deben acumularse del período anterior
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO annual_balance (user_id, year, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, year, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Actualizar registro existente
		// Calculamos los nuevos totales sumando los valores existentes
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Actualizar los montos de cash y bank sumando los nuevos valores a los existentes
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalcular el balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE annual_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, existingCash, existingBank, prevCashAmount, prevBankAmount, userID, year)
	}

	if err != nil {
		return err
	}

	// Actualizar años posteriores en cascada
	nextYear := date.AddDate(1, 0, 0)
	return updateSubsequentAnnualBalances(userID, nextYear)
}

// Add cash_amount and bank_amount columns to all balance tables if they don't exist
func addCashBankColumnsToAllTables() {
	alterTableSafely("daily_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")

	alterTableSafely("weekly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")

	alterTableSafely("monthly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("monthly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")

	alterTableSafely("quarterly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")

	alterTableSafely("semiannual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")

	alterTableSafely("annual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
}

// Función para actualizar trimestres posteriores en cascada
func updateSubsequentQuarterlyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	// Variable currentDate no se usa, la eliminamos

	// Mantener un registro de los trimestres procesados
	processedQuarters := make(map[string]struct{})

	// Encontrar el trimestre anterior al trimestre de inicio para saber el valor de partida
	var lastPrevCashAmount, lastPrevBankAmount float64

	// Calcular el trimestre anterior
	quarter := (int(startDate.Month()) - 1) / 3
	year := startDate.Year()
	var prevYear, prevQuarter int
	if quarter == 0 {
		prevYear = year - 1
		prevQuarter = 3 // Q4 del año anterior
	} else {
		prevYear = year
		prevQuarter = quarter - 1
	}
	prevToStartYearQuarter := fmt.Sprintf("%d-Q%d", prevYear, prevQuarter+1)

	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
		WHERE user_id = ? AND year_quarter = ?
	`, userID, prevToStartYearQuarter).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// Si no hay un trimestre inmediatamente anterior, buscar el último trimestre anterior disponible
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
			WHERE user_id = ? AND year_quarter < ?
			ORDER BY year_quarter DESC LIMIT 1
		`, userID, fmt.Sprintf("%d-Q%d", year, quarter+1)).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún trimestre anterior, usar valores en cero
		if err == sql.ErrNoRows {
			lastPrevCashAmount = 0
			lastPrevBankAmount = 0
		}
	}

	// Obtener todos los trimestres existentes desde la fecha de inicio
	rows, err := db.Query(`
		SELECT year_quarter FROM quarterly_cash_bank_balance
		WHERE user_id = ? AND year_quarter >= ?
		ORDER BY year_quarter ASC
	`, userID, fmt.Sprintf("%d-Q%d", year, quarter+1))

	if err != nil {
		return err
	}
	defer rows.Close()

	var existingQuarters []string
	for rows.Next() {
		var yearQuarter string
		if err := rows.Scan(&yearQuarter); err != nil {
			return err
		}
		existingQuarters = append(existingQuarters, yearQuarter)
	}

	// Si no hay trimestres existentes, no hay nada que procesar
	if len(existingQuarters) == 0 {
		return nil
	}

	// Determinar el último trimestre que necesitamos procesar
	lastQuarterStr := existingQuarters[len(existingQuarters)-1]
	quarterRegex := regexp.MustCompile(`(\d+)-Q(\d+)`)
	matches := quarterRegex.FindStringSubmatch(lastQuarterStr)
	if len(matches) != 3 {
		return fmt.Errorf("invalid quarter format: %s", lastQuarterStr)
	}
	lastYear, _ := strconv.Atoi(matches[1])
	lastQuarter, _ := strconv.Atoi(matches[2])
	lastQuarter-- // Convertir de Q1-Q4 a 0-3

	// Crear un mapa para acceso rápido a los trimestres existentes
	existingQuartersMap := make(map[string]bool)
	for _, q := range existingQuarters {
		existingQuartersMap[q] = true
	}

	// Variables para mantener los valores acumulados
	currentCashAmount := lastPrevCashAmount
	currentBankAmount := lastPrevBankAmount

	// Iterar trimestre por trimestre desde la fecha de inicio hasta el último trimestre existente
	currentYear := year
	currentQuarter := quarter

	for {
		// Avanzar al trimestre siguiente (ya que estamos empezando desde el anterior a startDate)
		if currentQuarter == 3 {
			currentYear++
			currentQuarter = 0
		} else {
			currentQuarter++
		}

		currentYearQuarter := fmt.Sprintf("%d-Q%d", currentYear, currentQuarter+1)

		// Si hemos superado el último trimestre, salimos
		lastYearQuarter := fmt.Sprintf("%d-Q%d", lastYear, lastQuarter+1)
		if compareQuarters(currentYearQuarter, lastYearQuarter) > 0 {
			break
		}

		if existingQuartersMap[currentYearQuarter] {
			// Si el trimestre existe, actualizarlo con los valores correctos
			var incomeCashAmount, incomeBankAmount float64
			var expenseCashAmount, expenseBankAmount float64
			var billCashAmount, billBankAmount float64

			err := db.QueryRow(`
				SELECT income_cash_amount, income_bank_amount,
				       expense_cash_amount, expense_bank_amount,
				       bill_cash_amount, bill_bank_amount
				FROM quarterly_cash_bank_balance
				WHERE user_id = ? AND year_quarter = ?
			`, userID, currentYearQuarter).Scan(
				&incomeCashAmount, &incomeBankAmount,
				&expenseCashAmount, &expenseBankAmount,
				&billCashAmount, &billBankAmount)

			if err != nil {
				return err
			}

			// Calcular nuevos montos considerando los ingresos, gastos y facturas del trimestre actual
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			// Actualizar los balances con el nuevo balance previo y las transacciones del trimestre
			currentCashAmount = prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
			currentBankAmount = prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

			// Actualizar el registro
			_, err = db.Exec(`
				UPDATE quarterly_cash_bank_balance
				SET cash_amount = ?,
				    bank_amount = ?,
				    previous_cash_amount = ?,
				    previous_bank_amount = ?,
				    balance_cash_amount = ?,
				    balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				    updated_at = CURRENT_TIMESTAMP
				WHERE user_id = ? AND year_quarter = ?
			`, currentCashAmount, currentBankAmount,
				prevCashAmount, prevBankAmount,
				currentCashAmount, currentBankAmount,
				userID, currentYearQuarter)

			if err != nil {
				return err
			}
		} else {
			// Si el trimestre no existe, crear un registro para él con valores en 0 para ingresos/gastos/facturas
			// pero con los valores correctos basados en el trimestre anterior
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			// Los montos no cambian ya que no hay transacciones
			// Sin embargo, creamos el registro para mantener la continuidad
			_, err = db.Exec(`
				INSERT INTO quarterly_cash_bank_balance (
					user_id, year_quarter,
					income_cash_amount, income_bank_amount,
					expense_cash_amount, expense_bank_amount,
					bill_cash_amount, bill_bank_amount,
					cash_amount, previous_cash_amount,
					bank_amount, previous_bank_amount,
					balance_cash_amount, balance_bank_amount
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, userID, currentYearQuarter,
				0, 0,
				0, 0,
				0, 0,
				prevCashAmount, prevCashAmount,
				prevBankAmount, prevBankAmount,
				prevCashAmount, prevBankAmount)

			if err != nil {
				// Si el error es por duplicado, ignorarlo
				if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
					return err
				}
			}
		}

		// Marcar este trimestre como procesado
		processedQuarters[currentYearQuarter] = struct{}{}
	}

	return nil
}

// Función auxiliar para comparar trimestres
func compareQuarters(q1, q2 string) int {
	q1Parts := strings.Split(q1, "-Q")
	q2Parts := strings.Split(q2, "-Q")

	if len(q1Parts) != 2 || len(q2Parts) != 2 {
		return 0 // Error formato
	}

	year1, _ := strconv.Atoi(q1Parts[0])
	quarter1, _ := strconv.Atoi(q1Parts[1])
	year2, _ := strconv.Atoi(q2Parts[0])
	quarter2, _ := strconv.Atoi(q2Parts[1])

	if year1 != year2 {
		return year1 - year2
	}
	return quarter1 - quarter2
}

// Función para actualizar semestres posteriores en cascada
func updateSubsequentSemiannualBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	endDate := startDate.AddDate(5, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		// Determinar el semestre actual
		half := (int(currentDate.Month())-1)/6 + 1
		currentYearHalf := fmt.Sprintf("%d-H%d", currentDate.Year(), half)

		// Verificar si existe un registro para este semestre
		var exists bool
		var incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_amount, expense_amount, bills_amount, cash_amount, bank_amount FROM semiannual_balance
			WHERE user_id = ? AND year_half = ?
		`, userID, currentYearHalf).Scan(&exists, &incomeAmount, &expenseAmount, &billsAmount, &cashAmount, &bankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance del semestre anterior
		prevHalfDate := currentDate.AddDate(0, -6, 0)
		prevHalf := (int(prevHalfDate.Month())-1)/6 + 1
		prevYearHalf := fmt.Sprintf("%d-H%d", prevHalfDate.Year(), prevHalf)

		var previousBalance float64
		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT balance, cash_amount, bank_amount FROM semiannual_balance 
			WHERE user_id = ? AND year_half = ?
		`, userID, prevYearHalf).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			previousBalance = 0
			prevCashAmount = 0
			prevBankAmount = 0
		}

		// Actualizar el balance con el nuevo balance previo
		balance := previousBalance + incomeAmount - expenseAmount - billsAmount

		// CAMBIO EN LA LÓGICA: Siempre acumulamos los valores del semestre anterior
		// independientemente de si hay transacciones en este semestre o no
		hasTransactions := incomeAmount != 0 || expenseAmount != 0 || billsAmount != 0

		// Inicializar con los valores del semestre anterior
		newCashAmount := prevCashAmount
		newBankAmount := prevBankAmount

		// Si hay transacciones propias en este semestre, las sumamos a lo heredado
		if hasTransactions {
			// Agregamos las transacciones propias de este semestre
			newCashAmount += cashAmount
			newBankAmount += bankAmount
		}

		_, err = db.Exec(`
			UPDATE semiannual_balance
			SET previous_balance = ?,
				balance = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_half = ?
		`, previousBalance, balance, newCashAmount, newBankAmount, prevCashAmount, prevBankAmount, userID, currentYearHalf)

		if err != nil {
			return err
		}

		// Pasar al siguiente semestre
		currentDate = currentDate.AddDate(0, 6, 0)
	}

	return nil
}

// Función para actualizar años posteriores en cascada
func updateSubsequentAnnualBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	endDate := startDate.AddDate(5, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		currentYear := currentDate.Format("2006")

		// Verificar si existe un registro para este año
		var exists bool
		var incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_amount, expense_amount, bills_amount, cash_amount, bank_amount FROM annual_balance
			WHERE user_id = ? AND year = ?
		`, userID, currentYear).Scan(&exists, &incomeAmount, &expenseAmount, &billsAmount, &cashAmount, &bankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance del año anterior
		prevYear := currentDate.AddDate(-1, 0, 0)
		prevYearStr := prevYear.Format("2006")

		var previousBalance float64
		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT balance, cash_amount, bank_amount FROM annual_balance 
			WHERE user_id = ? AND year = ?
		`, userID, prevYearStr).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			previousBalance = 0
			prevCashAmount = 0
			prevBankAmount = 0
		}

		// Actualizar el balance con el nuevo balance previo
		balance := previousBalance + incomeAmount - expenseAmount - billsAmount

		// CAMBIO EN LA LÓGICA: Siempre acumulamos los valores del año anterior
		hasTransactions := incomeAmount != 0 || expenseAmount != 0 || billsAmount != 0

		// Inicializar con los valores del año anterior
		newCashAmount := prevCashAmount
		newBankAmount := prevBankAmount

		// Si hay transacciones propias en este año, las sumamos a lo heredado
		if hasTransactions {
			// Agregamos las transacciones propias de este año
			newCashAmount += cashAmount
			newBankAmount += bankAmount
		}

		// Calcular los valores de balance para cash y bank
		balanceCashAmount := prevCashAmount + cashAmount
		balanceBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE annual_balance
			SET previous_balance = ?,
				balance = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`, previousBalance, balance, newCashAmount, newBankAmount, prevCashAmount, prevBankAmount, balanceCashAmount, balanceBankAmount, userID, currentYear)

		if err != nil {
			return err
		}

		// Pasar al siguiente año
		currentDate = currentDate.AddDate(1, 0, 0)
	}

	return nil
}

// Nueva función para recalcular todos los balances en cascada
func recalculateAllBalances(userID string, dateStr string) error {
	// Parse la fecha de la transacción
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Recalcular balances diarios en cascada desde la fecha de la transacción
	if err := updateSubsequentDailyBalances(userID, date); err != nil {
		return fmt.Errorf("error updating daily balances: %v", err)
	}

	// Calcular el inicio de la semana que contiene la fecha
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Convertir domingo (0) a 7
	}
	startOfWeek := date.AddDate(0, 0, -(dayOfWeek - 1))

	// Recalcular balances semanales en cascada desde la semana que contiene la transacción
	if err := updateSubsequentWeeklyBalances(userID, startOfWeek); err != nil {
		return fmt.Errorf("error updating weekly balances: %v", err)
	}

	// Calcular el inicio del mes que contiene la fecha
	startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances mensuales en cascada desde el mes que contiene la transacción
	if err := updateSubsequentMonthlyBalances(userID, startOfMonth); err != nil {
		return fmt.Errorf("error updating monthly balances: %v", err)
	}

	// Calcular el inicio del trimestre que contiene la fecha
	quarter := (int(date.Month()) - 1) / 3
	startOfQuarter := time.Date(date.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances trimestrales en cascada desde el trimestre que contiene la transacción
	if err := updateSubsequentQuarterlyBalances(userID, startOfQuarter); err != nil {
		return fmt.Errorf("error updating quarterly balances: %v", err)
	}

	// Calcular el inicio del semestre que contiene la fecha
	halfYear := (int(date.Month()) - 1) / 6
	startOfHalfYear := time.Date(date.Year(), time.Month(halfYear*6+1), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances semestrales en cascada desde el semestre que contiene la transacción
	if err := updateSubsequentSemiannualBalances(userID, startOfHalfYear); err != nil {
		return fmt.Errorf("error updating semiannual balances: %v", err)
	}

	// Calcular el inicio del año que contiene la fecha
	startOfYear := time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances anuales en cascada desde el año que contiene la transacción
	if err := updateSubsequentAnnualBalances(userID, startOfYear); err != nil {
		return fmt.Errorf("error updating annual balances: %v", err)
	}

	return nil
}

// Función para actualizar meses posteriores en cascada
func updateSubsequentMonthlyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	currentDate := startDate

	// Mantener un registro de los meses procesados
	processedMonths := make(map[string]struct{})

	// Encontrar el mes anterior al mes de inicio para saber el valor de partida
	var lastPrevCashAmount, lastPrevBankAmount float64
	prevToStartDate := startDate.AddDate(0, -1, 0)
	prevToStartYearMonth := prevToStartDate.Format("2006-01")

	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month = ?
	`, userID, prevToStartYearMonth).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// Si no hay un mes inmediatamente anterior, buscar el último mes anterior disponible
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month < ?
			ORDER BY year_month DESC LIMIT 1
		`, userID, startDate.Format("2006-01")).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún mes anterior, usar valores en cero
		if err == sql.ErrNoRows {
			lastPrevCashAmount = 0
			lastPrevBankAmount = 0
		}
	}

	// Obtener todos los meses existentes desde la fecha de inicio hasta 5 años después
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month ASC
	`, userID, startDate.Format("2006-01"))

	if err != nil {
		return err
	}
	defer rows.Close()

	var existingMonths []string
	for rows.Next() {
		var yearMonth string
		if err := rows.Scan(&yearMonth); err != nil {
			return err
		}
		existingMonths = append(existingMonths, yearMonth)
	}

	// Si no hay meses existentes, no hay nada que procesar
	if len(existingMonths) == 0 {
		return nil
	}

	// Determinar el último mes que necesitamos procesar
	lastMonthStr := existingMonths[len(existingMonths)-1]
	lastMonthParts := strings.Split(lastMonthStr, "-")
	if len(lastMonthParts) != 2 {
		return fmt.Errorf("invalid month format: %s", lastMonthStr)
	}
	lastYear, _ := strconv.Atoi(lastMonthParts[0])
	lastMonth, _ := strconv.Atoi(lastMonthParts[1])
	lastDate := time.Date(lastYear, time.Month(lastMonth), 1, 0, 0, 0, 0, time.UTC)

	// Crear un mapa para acceso rápido a los meses existentes
	existingMonthsMap := make(map[string]bool)
	for _, m := range existingMonths {
		existingMonthsMap[m] = true
	}

	// Variables para mantener los valores acumulados
	currentCashAmount := lastPrevCashAmount
	currentBankAmount := lastPrevBankAmount

	// Iterar mes por mes desde la fecha de inicio hasta el último mes existente
	for currentDate.Before(lastDate.AddDate(0, 1, 0)) {
		currentYearMonth := currentDate.Format("2006-01")

		if existingMonthsMap[currentYearMonth] {
			// Si el mes existe, actualizarlo con los valores correctos
			var incomeCashAmount, incomeBankAmount float64
			var expenseCashAmount, expenseBankAmount float64
			var billCashAmount, billBankAmount float64

			err := db.QueryRow(`
				SELECT income_cash_amount, income_bank_amount,
				       expense_cash_amount, expense_bank_amount,
				       bill_cash_amount, bill_bank_amount
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, currentYearMonth).Scan(
				&incomeCashAmount, &incomeBankAmount,
				&expenseCashAmount, &expenseBankAmount,
				&billCashAmount, &billBankAmount)

			if err != nil {
				return err
			}

			// Calcular nuevos montos considerando los ingresos, gastos y facturas del mes actual
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			// Actualizar los balances con el nuevo balance previo y las transacciones del mes
			currentCashAmount = prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
			currentBankAmount = prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

			// Actualizar el registro
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance
				SET cash_amount = ?,
				    bank_amount = ?,
				    previous_cash_amount = ?,
				    previous_bank_amount = ?,
				    balance_cash_amount = ?,
				    balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				    updated_at = CURRENT_TIMESTAMP
				WHERE user_id = ? AND year_month = ?
			`, currentCashAmount, currentBankAmount,
				prevCashAmount, prevBankAmount,
				currentCashAmount, currentBankAmount,
				userID, currentYearMonth)

			if err != nil {
				return err
			}
		} else {
			// Si el mes no existe, crear un registro para él con valores en 0 para ingresos/gastos/facturas
			// pero con los valores correctos basados en el mes anterior
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			// Los montos no cambian ya que no hay transacciones
			// Sin embargo, creamos el registro para mantener la continuidad
			_, err = db.Exec(`
				INSERT INTO monthly_cash_bank_balance (
					user_id, year_month,
					income_cash_amount, income_bank_amount,
					expense_cash_amount, expense_bank_amount,
					bill_cash_amount, bill_bank_amount,
					cash_amount, previous_cash_amount,
					bank_amount, previous_bank_amount,
					balance_cash_amount, balance_bank_amount
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, userID, currentYearMonth,
				0, 0,
				0, 0,
				0, 0,
				prevCashAmount, prevCashAmount,
				prevBankAmount, prevBankAmount,
				prevCashAmount, prevBankAmount)

			if err != nil {
				// Si el error es por duplicado, ignorarlo
				if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
					return err
				}
			}
		}

		// Marcar este mes como procesado
		processedMonths[currentYearMonth] = struct{}{}

		// Avanzar al mes siguiente
		currentDate = currentDate.AddDate(0, 1, 0)
	}

	return nil
}
