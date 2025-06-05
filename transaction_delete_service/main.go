package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Transaction deletion request structure
type DeleteTransactionRequest struct {
	UserID          string `json:"user_id"`
	TransactionID   int    `json:"transaction_id"`
	TransactionType string `json:"transaction_type"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var db *sql.DB

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

	log.Println("Transaction Delete Service - Database connection established successfully")
}

func main() {
	// CORS middleware function
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		}
	}

	// Health check endpoint
	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		response := ApiResponse{
			Success: true,
			Message: "Transaction Delete Service is running",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))

	// Delete transaction endpoint
	http.HandleFunc("/transactions/delete", corsMiddleware(handleDeleteTransaction))

	port := "8095" // Unique port for transaction delete service
	log.Printf("Transaction Delete Service starting on port %s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest DeleteTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		log.Printf("Error decoding request body: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Invalid request format",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if deleteRequest.UserID == "" || deleteRequest.TransactionID <= 0 || deleteRequest.TransactionType == "" {
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: user_id, transaction_id, or transaction_type",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Deleting transaction ID %d of type %s for user %s",
		deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)

	// Get transaction details before deletion for balance recalculation
	transaction, err := getTransactionDetails(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching transaction details: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Transaction not found or access denied",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Delete the transaction
	err = deleteTransaction(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting transaction: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Failed to delete transaction",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Recalculate balances for all time periods
	err = recalculateAllBalances(deleteRequest.UserID, transaction.Date, transaction.Amount, transaction.PaymentMethod, deleteRequest.TransactionType)
	if err != nil {
		log.Printf("Error recalculating balances: %v", err)
		// Don't fail the request if balance recalculation fails, just log it
	}

	response := ApiResponse{
		Success: true,
		Message: "Transaction deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type TransactionDetails struct {
	ID            int     `json:"id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	PaymentMethod string  `json:"payment_method"`
}

func getTransactionDetails(transactionID int, transactionType, userID string) (*TransactionDetails, error) {
	var transaction TransactionDetails
	var query string

	switch strings.ToLower(transactionType) {
	case "expense":
		query = `SELECT id, user_id, amount, date, payment_method FROM expenses WHERE id = ? AND user_id = ?`
	case "income":
		query = `SELECT id, user_id, amount, date, payment_method FROM incomes WHERE id = ? AND user_id = ?`
	case "bill":
		query = `SELECT id, user_id, amount, due_date as date, 'bank' as payment_method FROM bills WHERE id = ? AND user_id = ?`
	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", transactionType)
	}

	row := db.QueryRow(query, transactionID, userID)
	err := row.Scan(&transaction.ID, &transaction.UserID, &transaction.Amount, &transaction.Date, &transaction.PaymentMethod)
	if err != nil {
		return nil, err
	}

	return &transaction, nil
}

func deleteTransaction(transactionID int, transactionType, userID string) error {
	var query string

	switch strings.ToLower(transactionType) {
	case "expense":
		query = `DELETE FROM expenses WHERE id = ? AND user_id = ?`
	case "income":
		query = `DELETE FROM incomes WHERE id = ? AND user_id = ?`
	case "bill":
		query = `DELETE FROM bills WHERE id = ? AND user_id = ?`
	default:
		return fmt.Errorf("unsupported transaction type: %s", transactionType)
	}

	result, err := db.Exec(query, transactionID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no transaction found with ID %d for user %s", transactionID, userID)
	}

	return nil
}

func recalculateAllBalances(userID, transactionDate string, amount float64, paymentMethod, transactionType string) error {
	log.Printf("Updating balances for user %s after deleting %s transaction (amount: %.2f, method: %s)",
		userID, transactionType, amount, paymentMethod)

	// Parse the transaction date
	date, err := time.Parse("2006-01-02", transactionDate[:10])
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	// Define all period types
	periods := []struct {
		name      string
		tableName string
	}{
		{"daily", "daily_cash_bank_balance"},
		{"weekly", "weekly_cash_bank_balance"},
		{"monthly", "monthly_cash_bank_balance"},
		{"quarterly", "quarterly_cash_bank_balance"},
		{"semiannual", "semiannual_cash_bank_balance"},
		{"annual", "annual_cash_bank_balance"},
	}

	for _, periodInfo := range periods {
		// Calculate the period identifier for the transaction date
		periodIdentifier := calculatePeriodIdentifier(date, periodInfo.name)

		// Update the specific period balance
		err = updatePeriodBalance(userID, periodInfo.tableName, periodIdentifier, amount, paymentMethod, transactionType)
		if err != nil {
			log.Printf("Error updating %s balance: %v", periodInfo.name, err)
			continue
		}

		// Update subsequent periods' previous balances
		err = updateSubsequentPeriods(userID, periodInfo.tableName, periodInfo.name, date)
		if err != nil {
			log.Printf("Error updating subsequent %s periods: %v", periodInfo.name, err)
		}
	}

	return nil
}

func updatePeriodBalance(userID, tableName, periodIdentifier string, amount float64, paymentMethod, transactionType string) error {
	// Determine the correct column name for the period based on table name
	var periodColumn string
	switch {
	case strings.Contains(tableName, "daily"):
		periodColumn = "date"
	case strings.Contains(tableName, "weekly"):
		periodColumn = "year_week"
	case strings.Contains(tableName, "monthly"):
		periodColumn = "year_month"
	case strings.Contains(tableName, "quarterly"):
		periodColumn = "year_quarter"
	case strings.Contains(tableName, "semiannual"):
		periodColumn = "year_half"
	case strings.Contains(tableName, "annual"):
		periodColumn = "year"
	default:
		periodColumn = "period"
	}

	// Check if the period record exists
	var exists bool
	checkQuery := fmt.Sprintf(`SELECT COUNT(*) > 0 FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)
	err := db.QueryRow(checkQuery, userID, periodIdentifier).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking period existence: %v", err)
	}

	if !exists {
		// If period doesn't exist, log and skip (this shouldn't happen for deletion)
		log.Printf("Period %s not found in %s for user %s", periodIdentifier, tableName, userID)
		return nil
	}

	// Determine which columns to update and calculate the changes
	var updates []string
	var params []interface{}

	switch strings.ToLower(transactionType) {
	case "income":
		if paymentMethod == "bank" {
			updates = append(updates, "income_bank_amount = income_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount - ?")
			updates = append(updates, "balance_bank_amount = balance_bank_amount - ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "income_cash_amount = income_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount - ?")
			updates = append(updates, "balance_cash_amount = balance_cash_amount - ?")
			params = append(params, amount, amount, amount)
		}
	case "expense":
		if paymentMethod == "bank" {
			updates = append(updates, "expense_bank_amount = expense_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount + ?") // Adding back the expense
			updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "expense_cash_amount = expense_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount + ?") // Adding back the expense
			updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
			params = append(params, amount, amount, amount)
		}
	case "bill":
		if paymentMethod == "bank" {
			updates = append(updates, "bill_bank_amount = bill_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount + ?") // Adding back the bill
			updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "bill_cash_amount = bill_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount + ?") // Adding back the bill
			updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
			params = append(params, amount, amount, amount)
		}
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	// Build and execute the first update query (without total_balance)
	updateQuery := fmt.Sprintf("UPDATE %s SET %s WHERE user_id = ? AND %s = ?",
		tableName, strings.Join(updates, ", "), periodColumn)

	// Add WHERE clause parameters
	params = append(params, userID, periodIdentifier)

	_, err = db.Exec(updateQuery, params...)
	if err != nil {
		return fmt.Errorf("error updating period balance: %v", err)
	}

	// Now update total_balance separately to ensure it uses the updated values
	totalBalanceQuery := fmt.Sprintf(`
		UPDATE %s 
		SET total_balance = (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(totalBalanceQuery, userID, periodIdentifier)
	if err != nil {
		return fmt.Errorf("error updating total balance: %v", err)
	}

	log.Printf("Updated %s balance for period %s (amount change: %.2f %s, type: %s)",
		tableName, periodIdentifier, amount, paymentMethod, transactionType)

	return nil
}

func updateSubsequentPeriods(userID, tableName, periodType string, transactionDate time.Time) error {
	// Get all periods after the transaction date to update their previous balances
	// For simplicity, we'll recalculate previous amounts for periods after the deleted transaction
	// This ensures cascade effect is properly maintained

	var nextPeriods []string

	switch periodType {
	case "monthly":
		// Get next months
		for i := 1; i <= 12; i++ { // Check next 12 months
			nextDate := transactionDate.AddDate(0, i, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "quarterly":
		// Get next quarters
		for i := 1; i <= 4; i++ { // Check next 4 quarters
			nextDate := transactionDate.AddDate(0, i*3, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "annual":
		// Get next years
		for i := 1; i <= 5; i++ { // Check next 5 years
			nextDate := transactionDate.AddDate(i, 0, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	}

	// Update previous balance amounts for subsequent periods
	for _, nextPeriod := range nextPeriods {
		err := updatePreviousBalanceForPeriod(userID, tableName, nextPeriod, periodType)
		if err != nil {
			log.Printf("Error updating previous balance for period %s: %v", nextPeriod, err)
		}
	}

	return nil
}

func updatePreviousBalanceForPeriod(userID, tableName, period, periodType string) error {
	// Get the previous period identifier
	currentTime, err := parsePeriodIdentifier(period, periodType)
	if err != nil {
		return err
	}

	var previousTime time.Time
	switch periodType {
	case "monthly":
		previousTime = currentTime.AddDate(0, -1, 0)
	case "quarterly":
		previousTime = currentTime.AddDate(0, -3, 0)
	case "annual":
		previousTime = currentTime.AddDate(-1, 0, 0)
	default:
		return nil // Don't update for daily/weekly
	}

	previousPeriod := calculatePeriodIdentifier(previousTime, periodType)

	// Determine the correct column name for the period based on table name
	var periodColumn string
	switch {
	case strings.Contains(tableName, "daily"):
		periodColumn = "date"
	case strings.Contains(tableName, "weekly"):
		periodColumn = "year_week"
	case strings.Contains(tableName, "monthly"):
		periodColumn = "year_month"
	case strings.Contains(tableName, "quarterly"):
		periodColumn = "year_quarter"
	case strings.Contains(tableName, "semiannual"):
		periodColumn = "year_half"
	case strings.Contains(tableName, "annual"):
		periodColumn = "year"
	default:
		periodColumn = "period"
	}

	// Get previous period's balance amounts
	var prevBankAmount, prevCashAmount, prevTotalBalance float64
	selectQuery := fmt.Sprintf(`
		SELECT COALESCE(balance_bank_amount, 0), COALESCE(balance_cash_amount, 0), COALESCE(total_balance, 0)
		FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	err = db.QueryRow(selectQuery, userID, previousPeriod).Scan(&prevBankAmount, &prevCashAmount, &prevTotalBalance)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting previous period balance: %v", err)
	}

	// Update current period's previous balance fields
	updateQuery := fmt.Sprintf(`
		UPDATE %s 
		SET previous_bank_amount = ?,
		    previous_cash_amount = ?,
		    total_previous_balance = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(updateQuery, prevBankAmount, prevCashAmount, prevTotalBalance, userID, period)
	if err != nil {
		return fmt.Errorf("error updating previous balance: %v", err)
	}

	return nil
}

func parsePeriodIdentifier(period, periodType string) (time.Time, error) {
	switch periodType {
	case "monthly":
		return time.Parse("2006-01", period)
	case "quarterly":
		// Parse format like "2025-Q1"
		var year, quarter int
		_, err := fmt.Sscanf(period, "%d-Q%d", &year, &quarter)
		if err != nil {
			return time.Time{}, err
		}
		month := (quarter-1)*3 + 1
		return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
	case "annual":
		// Parse format like "2025"
		var year int
		_, err := fmt.Sscanf(period, "%d", &year)
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Parse("2006-01-02", period)
	}
}

func calculatePeriodIdentifier(date time.Time, period string) string {
	switch period {
	case "daily":
		return date.Format("2006-01-02")
	case "weekly":
		year, week := date.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "monthly":
		return date.Format("2006-01")
	case "quarterly":
		quarter := (date.Month()-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", date.Year(), quarter)
	case "semiannual":
		half := 1
		if date.Month() > 6 {
			half = 2
		}
		return fmt.Sprintf("%d-H%d", date.Year(), half)
	case "annual":
		return fmt.Sprintf("%d", date.Year())
	default:
		return date.Format("2006-01")
	}
}
