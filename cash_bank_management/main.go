package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras de datos
type CashBankDistribution struct {
	UserID       string  `json:"user_id"`
	Month        string  `json:"month"`
	CashAmount   float64 `json:"cash_amount"`
	CashPercent  float64 `json:"cash_percent"`
	BankAmount   float64 `json:"bank_amount"`
	BankPercent  float64 `json:"bank_percent"`
	MonthlyTotal float64 `json:"monthly_total"`
}

type TransferRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date"`
}

type UpdateAmountRequest struct {
	UserID string  `json:"user_id"`
	Amount float64 `json:"amount"`
	Date   string  `json:"date"`
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

	log.Println("Database connection established successfully")
}

func createTablesIfNotExist() {
	// Create cash_bank table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			month TEXT NOT NULL,
			cash_amount REAL NOT NULL,
			cash_percent REAL NOT NULL,
			bank_amount REAL NOT NULL,
			bank_percent REAL NOT NULL,
			monthly_total REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Create transaction history table
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

	// Create monthly_cash_bank_balance table (requerida por fetchCashBankDistribution)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_month TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, year_month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create monthly_cash_bank_balance table: %v", err)
	}

	// Create indices for better performance
	db.Exec("CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_balance_user ON monthly_cash_bank_balance(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_balance_month ON monthly_cash_bank_balance(year_month)")

	// Create daily_cash_bank_balance table (requerida por updateAllPeriodTables)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, date)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_cash_bank_balance table: %v", err)
	}

	// Create indices for daily table
	db.Exec("CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_user ON daily_cash_bank_balance(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_date ON daily_cash_bank_balance(date)")

	// Create weekly_cash_bank_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, year_week)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_cash_bank_balance table: %v", err)
	}

	// Create quarterly_cash_bank_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, year_quarter)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_cash_bank_balance table: %v", err)
	}

	// Create semiannual_cash_bank_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_half TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, year_half)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_cash_bank_balance table: %v", err)
	}

	// Create annual_cash_bank_balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_bank_amount REAL DEFAULT 0,
			income_cash_amount REAL DEFAULT 0,
			expense_bank_amount REAL DEFAULT 0,
			expense_cash_amount REAL DEFAULT 0,
			bill_bank_amount REAL DEFAULT 0,
			bill_cash_amount REAL DEFAULT 0,
			bank_amount REAL DEFAULT 0,
			previous_bank_amount REAL DEFAULT 0,
			cash_amount REAL DEFAULT 0,
			previous_cash_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			balance_cash_amount REAL DEFAULT 0,
			balance_bank_amount REAL DEFAULT 0,
			total_previous_balance REAL DEFAULT 0,
			total_balance REAL DEFAULT 0,
			UNIQUE(user_id, year)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_cash_bank_balance table: %v", err)
	}
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/cash-bank/distribution", corsMiddleware(handleFetchDistribution))
	http.HandleFunc("/cash-bank/cash/update", corsMiddleware(handleUpdateCash))
	http.HandleFunc("/cash-bank/bank/update", corsMiddleware(handleUpdateBank))
	http.HandleFunc("/transfer/cash-to-bank", corsMiddleware(handleCashToBankTransfer))
	http.HandleFunc("/transfer/bank-to-cash", corsMiddleware(handleBankToCashTransfer))

	port := 8090
	log.Printf("Cash Bank Management service started on :%d", port)
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

func handleFetchDistribution(w http.ResponseWriter, r *http.Request) {
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

	// Get cash bank distribution data from database
	distribution, err := fetchCashBankDistribution(userID)
	if err != nil {
		log.Printf("Error fetching cash bank distribution: %v", err)
		sendErrorResponse(w, "Error fetching cash bank distribution", http.StatusInternalServerError)
		return
	}

	// Return cash bank distribution data as JSON
	sendSuccessResponse(w, "Cash bank distribution fetched successfully", distribution)
}

func handleUpdateCash(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UpdateAmountRequest
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

	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Get current distribution
	distribution, err := fetchCashBankDistribution(updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Update cash amount
	distribution.CashAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save the updated distribution
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating cash amount: %v", err)
		sendErrorResponse(w, "Error updating cash amount", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	err = addTransaction(updateRequest.UserID, "cash_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// Return success response
	sendSuccessResponse(w, "Cash amount updated successfully", distribution)
}

func handleUpdateBank(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UpdateAmountRequest
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

	if updateRequest.Amount < 0 {
		sendErrorResponse(w, "Amount must be greater than or equal to 0", http.StatusBadRequest)
		return
	}

	// Get current distribution
	distribution, err := fetchCashBankDistribution(updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current distribution: %v", err)
		sendErrorResponse(w, "Error fetching current distribution", http.StatusInternalServerError)
		return
	}

	// Update bank amount
	distribution.BankAmount = updateRequest.Amount
	distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

	// Recalculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	// Save the updated distribution
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating bank amount: %v", err)
		sendErrorResponse(w, "Error updating bank amount", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	err = addTransaction(updateRequest.UserID, "bank_update", updateRequest.Amount, updateRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// Return success response
	sendSuccessResponse(w, "Bank amount updated successfully", distribution)
}

func handleCashToBankTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Get current distribution
	distribution, err := fetchCashBankDistribution(transferRequest.UserID)
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

	// Update amounts
	distribution.CashAmount -= transferRequest.Amount
	distribution.BankAmount += transferRequest.Amount

	// Recalculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Save the updated distribution
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating distribution after transfer: %v", err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	err = addTransaction(transferRequest.UserID, "cash_to_bank", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// Return success response
	sendSuccessResponse(w, "Cash to bank transfer successful", distribution)
}

func handleBankToCashTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var transferRequest TransferRequest
	err := json.NewDecoder(r.Body).Decode(&transferRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if transferRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if transferRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	// Get current distribution
	distribution, err := fetchCashBankDistribution(transferRequest.UserID)
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

	// Update amounts
	distribution.BankAmount -= transferRequest.Amount
	distribution.CashAmount += transferRequest.Amount

	// Recalculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	}

	// Save the updated distribution
	err = updateCashBankDistribution(distribution)
	if err != nil {
		log.Printf("Error updating distribution after transfer: %v", err)
		sendErrorResponse(w, "Error processing transfer", http.StatusInternalServerError)
		return
	}

	// Add transaction to history
	err = addTransaction(transferRequest.UserID, "bank_to_cash", transferRequest.Amount, transferRequest.Date)
	if err != nil {
		log.Printf("Error adding transaction to history: %v", err)
		// Continue despite the error
	}

	// Return success response
	sendSuccessResponse(w, "Bank to cash transfer successful", distribution)
}

func fetchCashBankDistribution(userID string) (CashBankDistribution, error) {
	var distribution CashBankDistribution
	distribution.UserID = userID

	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")

	// Query monthly_cash_bank_balance data from database for current month
	err := db.QueryRow(`
		SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, userID, currentMonth).Scan(
		&distribution.Month,
		&distribution.CashAmount,
		&distribution.BankAmount,
		&distribution.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// If no data for current month, try to get the most recent month
		err = db.QueryRow(`
			SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
			FROM monthly_cash_bank_balance
			WHERE user_id = ?
			ORDER BY year_month DESC, updated_at DESC
			LIMIT 1
		`, userID).Scan(
			&distribution.Month,
			&distribution.CashAmount,
			&distribution.BankAmount,
			&distribution.MonthlyTotal,
		)

		if err == sql.ErrNoRows {
			// Return default values if no data found
			now := time.Now()
			distribution.Month = now.Format("January 2006")
			distribution.CashAmount = 0
			distribution.CashPercent = 0
			distribution.BankAmount = 0
			distribution.BankPercent = 0
			distribution.MonthlyTotal = 0
			return distribution, nil
		} else if err != nil {
			return distribution, err
		}
	} else if err != nil {
		return distribution, err
	}

	// Calculate percentages
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	return distribution, nil
}

func updateCashBankDistribution(distribution CashBankDistribution) error {
	// Get current date and time periods
	now := time.Now()
	currentDate := now.Format("2006-01-02")
	currentMonth := now.Format("2006-01")
	currentWeek := getWeekPeriod(now)
	currentQuarter := getQuarterPeriod(now)
	currentSemiannual := getSemiannualPeriod(now)
	currentYear := now.Format("2006")

	// Update all period tables
	err := updateAllPeriodTables(distribution, currentDate, currentMonth, currentWeek, currentQuarter, currentSemiannual, currentYear)
	if err != nil {
		return err
	}

	// Also update the legacy cash_bank table for backward compatibility
	var legacyCount int
	err2 := db.QueryRow(`
		SELECT COUNT(*) 
		FROM cash_bank 
		WHERE user_id = ?
	`, distribution.UserID).Scan(&legacyCount)

	if err2 == nil {
		if legacyCount > 0 {
			// Update existing cash_bank entry
			db.Exec(`
				UPDATE cash_bank
				SET month = ?,
					cash_amount = ?,
					cash_percent = ?,
					bank_amount = ?,
					bank_percent = ?,
					monthly_total = ?,
					updated_at = CURRENT_TIMESTAMP
				WHERE user_id = ?
			`,
				distribution.Month,
				distribution.CashAmount,
				distribution.CashPercent,
				distribution.BankAmount,
				distribution.BankPercent,
				distribution.MonthlyTotal,
				distribution.UserID,
			)
		} else {
			// Insert new cash_bank entry
			db.Exec(`
				INSERT INTO cash_bank (
					user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total
				) VALUES (?, ?, ?, ?, ?, ?, ?)
			`,
				distribution.UserID,
				distribution.Month,
				distribution.CashAmount,
				distribution.CashPercent,
				distribution.BankAmount,
				distribution.BankPercent,
				distribution.MonthlyTotal,
			)
		}
	}

	return err
}

// Helper functions for period calculations
func getWeekPeriod(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-%02d", year, week)
}

func getQuarterPeriod(t time.Time) string {
	quarter := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d-%d", t.Year(), quarter)
}

func getSemiannualPeriod(t time.Time) string {
	semiannual := (int(t.Month())-1)/6 + 1
	return fmt.Sprintf("%d-%d", t.Year(), semiannual)
}

// Update all period tables with the new cash/bank distribution
func updateAllPeriodTables(distribution CashBankDistribution, currentDate, currentMonth, currentWeek, currentQuarter, currentSemiannual, currentYear string) error {
	// Update daily_cash_bank_balance
	err := updatePeriodTable("daily_cash_bank_balance", "date", currentDate, distribution)
	if err != nil {
		log.Printf("Error updating daily_cash_bank_balance: %v", err)
		return err
	}

	// Update weekly_cash_bank_balance
	err = updatePeriodTable("weekly_cash_bank_balance", "year_week", currentWeek, distribution)
	if err != nil {
		log.Printf("Error updating weekly_cash_bank_balance: %v", err)
		return err
	}

	// Update monthly_cash_bank_balance
	err = updatePeriodTable("monthly_cash_bank_balance", "year_month", currentMonth, distribution)
	if err != nil {
		log.Printf("Error updating monthly_cash_bank_balance: %v", err)
		return err
	}

	// Update quarterly_cash_bank_balance
	err = updatePeriodTable("quarterly_cash_bank_balance", "year_quarter", currentQuarter, distribution)
	if err != nil {
		log.Printf("Error updating quarterly_cash_bank_balance: %v", err)
		return err
	}

	// Update semiannual_cash_bank_balance
	err = updatePeriodTable("semiannual_cash_bank_balance", "year_half", currentSemiannual, distribution)
	if err != nil {
		log.Printf("Error updating semiannual_cash_bank_balance: %v", err)
		return err
	}

	// Update annual_cash_bank_balance
	err = updatePeriodTable("annual_cash_bank_balance", "year", currentYear, distribution)
	if err != nil {
		log.Printf("Error updating annual_cash_bank_balance: %v", err)
		return err
	}

	return nil
}

// Generic function to update any period table
func updatePeriodTable(tableName, periodColumn, periodValue string, distribution CashBankDistribution) error {
	// Check if entry exists
	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)
	err := db.QueryRow(query, distribution.UserID, periodValue).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing entry
		updateQuery := fmt.Sprintf(`
			UPDATE %s
			SET cash_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
				total_balance = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND %s = ?
		`, tableName, periodColumn)

		_, err = db.Exec(updateQuery,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
			distribution.UserID,
			periodValue,
		)
	} else {
		// Insert new entry
		insertQuery := fmt.Sprintf(`
			INSERT INTO %s (
				user_id, %s, cash_amount, bank_amount, balance_cash_amount, balance_bank_amount, total_balance
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, tableName, periodColumn)

		_, err = db.Exec(insertQuery,
			distribution.UserID,
			periodValue,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
		)
	}

	return err
}

func addTransaction(userID, transactionType string, amount float64, date string) error {
	_, err := db.Exec(`
		INSERT INTO cash_bank_transactions (
			user_id, transaction_type, amount, date
		) VALUES (?, ?, ?, ?)
	`,
		userID,
		transactionType,
		amount,
		date,
	)

	return err
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}
