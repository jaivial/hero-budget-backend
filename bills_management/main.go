package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// Data structures
type Bill struct {
	ID             int     `json:"id"`
	UserID         string  `json:"user_id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	DueDate        string  `json:"due_date"`
	StartDate      string  `json:"start_date"`
	PaymentDay     int     `json:"payment_day"`
	DurationMonths int     `json:"duration_months"`
	Regularity     string  `json:"regularity"`
	Paid           bool    `json:"paid"`
	Overdue        bool    `json:"overdue"`
	OverdueDays    int     `json:"overdue_days"`
	Recurring      bool    `json:"recurring"`
	Category       string  `json:"category"`
	Icon           string  `json:"icon"`
	PaymentMethod  string  `json:"payment_method"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type UpdateBillRequest struct {
	UserID         string  `json:"user_id"`
	BillID         int     `json:"bill_id"`
	Name           string  `json:"name,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	StartDate      string  `json:"start_date,omitempty"`
	PaymentDay     int     `json:"payment_day,omitempty"`
	DurationMonths int     `json:"duration_months,omitempty"`
	Regularity     string  `json:"regularity,omitempty"`
	Category       string  `json:"category,omitempty"`
	Icon           string  `json:"icon,omitempty"`
	PaymentMethod  string  `json:"payment_method,omitempty"`
}

type DeleteBillRequest struct {
	UserID string `json:"user_id"`
	BillID int    `json:"bill_id"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type BillToExpenseRequest struct {
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description"`
}

func init() {
	var err error
	dbPath := "../google_auth/users.db"
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Using database at: %s\n", dbPath)
	createTablesIfNotExist()
	log.Println("Database connection established successfully")
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/bills", corsMiddleware(handleFetchBills))
	http.HandleFunc("/bills/add", corsMiddleware(handleAddBill))
	http.HandleFunc("/bills/pay", corsMiddleware(handlePayBill))
	http.HandleFunc("/bills/update", corsMiddleware(handleUpdateBill))
	http.HandleFunc("/bills/delete", corsMiddleware(handleDeleteBill))
	http.HandleFunc("/bills/upcoming", corsMiddleware(handleGetUpcomingBills))

	fmt.Println("Bills Management service started on :8091")
	log.Fatal(http.ListenAndServe(":8091", nil))
}

func createTablesIfNotExist() {
	// Create bills table
	createBillsTable := `
	CREATE TABLE IF NOT EXISTS bills (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		amount REAL NOT NULL,
		due_date TEXT,
		start_date TEXT NOT NULL,
		payment_day INTEGER NOT NULL,
		duration_months INTEGER NOT NULL,
		regularity TEXT NOT NULL DEFAULT 'monthly',
		paid BOOLEAN DEFAULT 0,
		overdue BOOLEAN DEFAULT 0,
		overdue_days INTEGER DEFAULT 0,
		recurring BOOLEAN DEFAULT 1,
		category TEXT DEFAULT 'general',
		icon TEXT DEFAULT '💳',
		payment_method TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(createBillsTable)
	if err != nil {
		log.Printf("Error creating bills table: %v", err)
	}

	// Create bill_payments table
	createBillPaymentsTable := `
	CREATE TABLE IF NOT EXISTS bill_payments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bill_id INTEGER NOT NULL,
		year_month TEXT NOT NULL,
		paid BOOLEAN DEFAULT 0,
		payment_date TEXT,
		payment_method TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (bill_id) REFERENCES bills (id) ON DELETE CASCADE,
		UNIQUE(bill_id, year_month)
	);`

	_, err = db.Exec(createBillPaymentsTable)
	if err != nil {
		log.Printf("Error creating bill_payments table: %v", err)
	}

	// Add bill_id column to expenses if it doesn't exist
	alterExpensesTable := `ALTER TABLE expenses ADD COLUMN bill_id INTEGER;`
	db.Exec(alterExpensesTable) // Ignore error if column already exists
}

// Basic handlers
func handleFetchBills(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	bills, err := fetchBills(userID)
	if err != nil {
		sendErrorResponse(w, "Error fetching bills", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Bills fetched successfully", bills)
}

// UpdateCascadeBalances recalcula los saldos en cascada desde startMonth
func updateCascadeBalances(db *sql.DB, userID string, startMonth string) error {
	// Obtener todos los meses posteriores o iguales a startMonth
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months: %v", err)
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return fmt.Errorf("error scanning month: %v", err)
		}
		months = append(months, month)
	}

	for i, month := range months {
		// Obtener el mes anterior (si existe)
		var previousMonth string
		if i > 0 {
			previousMonth = months[i-1]
		} else if month != startMonth {
			row := db.QueryRow(`
				SELECT year_month FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month < ? ORDER BY year_month DESC LIMIT 1
			`, userID, month)
			if err := row.Scan(&previousMonth); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous month: %v", err)
			}
		}

		// Obtener saldos previos
		var previousCashAmount, previousBankAmount, totalPreviousBalance float64
		if previousMonth != "" {
			err := db.QueryRow(`
				SELECT cash_amount, bank_amount, total_balance
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, previousMonth).Scan(&previousCashAmount, &previousBankAmount, &totalPreviousBalance)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous balances: %v", err)
			}
		}

		// Obtener movimientos del mes actual
		var incomeCash, incomeBank, expenseCash, expenseBank, billCash, billBank float64
		err := db.QueryRow(`
			SELECT income_cash_amount, income_bank_amount,
			       expense_cash_amount, expense_bank_amount,
			       bill_cash_amount, bill_bank_amount
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`, userID, month).Scan(&incomeCash, &incomeBank, &expenseCash, &expenseBank, &billCash, &billBank)
		if err != nil {
			return fmt.Errorf("error fetching current month data: %v", err)
		}

		// Calcular saldos del mes actual (AQUÍ ESTÁ EL ALGORITMO COMPLETO)
		cashAmount := previousCashAmount + incomeCash - expenseCash - billCash
		bankAmount := previousBankAmount + incomeBank - expenseBank - billBank
		balanceCashAmount := cashAmount
		balanceBankAmount := bankAmount
		totalBalance := balanceCashAmount + balanceBankAmount

		// Actualizar registro con todos los campos del algoritmo
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance
			SET cash_amount = ?,
			    bank_amount = ?,
			    balance_cash_amount = ?,
			    balance_bank_amount = ?,
			    total_balance = ?,
			    previous_cash_amount = ?,
			    previous_bank_amount = ?,
			    total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, cashAmount, bankAmount, balanceCashAmount, balanceBankAmount,
			totalBalance, previousCashAmount, previousBankAmount, totalPreviousBalance,
			userID, month)
		if err != nil {
			return fmt.Errorf("error updating balance for month %s: %v", month, err)
		}
	}

	return nil
}

// AddBill registra una factura y actualiza los balances mensuales
func addBillWithBalanceUpdate(db *sql.DB, userID, name string, amount float64, startDate string, paymentDay, durationMonths int, paymentMethod, category, icon, regularity string) (int, error) {
	if amount <= 0 || durationMonths < 1 || paymentDay < 1 || paymentDay > 28 || (paymentMethod != "cash" && paymentMethod != "bank") {
		return 0, fmt.Errorf("invalid bill data")
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Registrar factura
	result, err := tx.Exec(`
		INSERT INTO bills (user_id, name, amount, due_date, start_date, payment_day, duration_months, regularity, payment_method, category, icon, paid, overdue, overdue_days, recurring, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, name, amount, startDate, startDate, paymentDay, durationMonths, regularity, paymentMethod, category, icon)
	if err != nil {
		return 0, fmt.Errorf("error inserting bill: %v", err)
	}

	billID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting bill ID: %v", err)
	}

	// Calcular meses afectados
	currentDate, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, fmt.Errorf("invalid start date format: %v", err)
	}

	var firstMonth string
	for i := 0; i < durationMonths; i++ {
		monthDate := currentDate.AddDate(0, i, 0)
		month := monthDate.Format("2006-01")

		if i == 0 {
			firstMonth = month
		}

		// Crear registro en bill_payments si la tabla existe
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO bill_payments (bill_id, year_month, paid, payment_date, payment_method, created_at)
			VALUES (?, ?, 0, NULL, ?, CURRENT_TIMESTAMP)
		`, billID, month, paymentMethod)
		if err != nil {
			log.Printf("Warning: Could not create bill_payments record: %v", err)
		}

		// Crear o actualizar registro mensual en monthly_cash_bank_balance
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (
				user_id, year_month, 
				income_cash_amount, income_bank_amount,
				expense_cash_amount, expense_bank_amount,
				bill_cash_amount, bill_bank_amount,
				cash_amount, bank_amount,
				previous_cash_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount,
				total_previous_balance, total_balance
			) VALUES (?, ?, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
		`, userID, month)
		if err != nil {
			log.Printf("Warning: Could not create monthly_cash_bank_balance record: %v", err)
		}

		// PASO 1: Sumar el importe a las columnas bill_bank_amount o bill_cash_amount
		if paymentMethod == "cash" {
			_, err = tx.Exec(`
				UPDATE monthly_cash_bank_balance
				SET bill_cash_amount = COALESCE(bill_cash_amount, 0) + ?
				WHERE user_id = ? AND year_month = ?
			`, amount, userID, month)
		} else {
			_, err = tx.Exec(`
				UPDATE monthly_cash_bank_balance
				SET bill_bank_amount = COALESCE(bill_bank_amount, 0) + ?
				WHERE user_id = ? AND year_month = ?
			`, amount, userID, month)
		}
		if err != nil {
			return 0, fmt.Errorf("error updating bill amount for month %s: %v", month, err)
		}

		log.Printf("Updated projection in monthly_cash_bank_balance for bill %d", billID)
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	// PASOS 2-4: Recalcular saldos en cascada (esto hace automáticamente todos los pasos restantes del algoritmo)
	if err = updateCascadeBalances(db, userID, firstMonth); err != nil {
		return int(billID), fmt.Errorf("error updating cascade balances: %v", err)
	}

	log.Printf("Successfully recalculated all balances for user %s from date %s", userID, firstMonth)

	return int(billID), nil
}

func handleAddBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var addRequest struct {
		UserID         string  `json:"user_id"`
		Name           string  `json:"name"`
		Amount         float64 `json:"amount"`
		DueDate        string  `json:"due_date"`
		StartDate      string  `json:"start_date"`
		PaymentDay     int     `json:"payment_day"`
		DurationMonths int     `json:"duration_months"`
		Regularity     string  `json:"regularity"`
		Category       string  `json:"category"`
		Icon           string  `json:"icon"`
		PaymentMethod  string  `json:"payment_method"`
	}

	err := json.NewDecoder(r.Body).Decode(&addRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if addRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}
	if addRequest.Name == "" {
		sendErrorResponse(w, "Name is required", http.StatusBadRequest)
		return
	}
	if addRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}
	if addRequest.DueDate == "" {
		sendErrorResponse(w, "Due date is required", http.StatusBadRequest)
		return
	}
	if addRequest.Category == "" {
		sendErrorResponse(w, "Category is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if addRequest.Icon == "" {
		addRequest.Icon = "💳"
	}
	if addRequest.PaymentMethod == "" {
		addRequest.PaymentMethod = "bank"
	}
	if addRequest.StartDate == "" {
		addRequest.StartDate = addRequest.DueDate
	}
	if addRequest.PaymentDay == 0 {
		addRequest.PaymentDay = 1
	}
	if addRequest.DurationMonths == 0 {
		addRequest.DurationMonths = 1
	}
	if addRequest.Regularity == "" {
		addRequest.Regularity = "monthly"
	}

	// Validate payment method
	if addRequest.PaymentMethod != "cash" && addRequest.PaymentMethod != "bank" {
		sendErrorResponse(w, "Payment method must be 'cash' or 'bank'", http.StatusBadRequest)
		return
	}

	// Use the AddBill function to properly handle balance updates
	billID, err := addBillWithBalanceUpdate(
		db,
		addRequest.UserID,
		addRequest.Name,
		addRequest.Amount,
		addRequest.StartDate,
		addRequest.PaymentDay,
		addRequest.DurationMonths,
		addRequest.PaymentMethod,
		addRequest.Category,
		addRequest.Icon,
		addRequest.Regularity,
	)

	if err != nil {
		log.Printf("Error adding bill with balance updates: %v", err)
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully added bill %d with balance updates for user %s", billID, addRequest.UserID)

	// Return success response with the new bill data
	billData := map[string]interface{}{
		"id":              billID,
		"user_id":         addRequest.UserID,
		"name":            addRequest.Name,
		"amount":          addRequest.Amount,
		"due_date":        addRequest.DueDate,
		"start_date":      addRequest.StartDate,
		"payment_day":     addRequest.PaymentDay,
		"duration_months": addRequest.DurationMonths,
		"regularity":      addRequest.Regularity,
		"category":        addRequest.Category,
		"icon":            addRequest.Icon,
		"payment_method":  addRequest.PaymentMethod,
		"paid":            false,
		"overdue":         false,
		"overdue_days":    0,
		"recurring":       true,
	}

	sendSuccessResponse(w, "Bill added successfully", billData)
}

func handlePayBill(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Pay bill endpoint available", map[string]string{"status": "available"})
}

func handleUpdateBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var updateRequest UpdateBillRequest
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
	if updateRequest.BillID <= 0 {
		sendErrorResponse(w, "Valid bill ID is required", http.StatusBadRequest)
		return
	}

	sendSuccessResponse(w, "Bill update endpoint working", map[string]interface{}{
		"bill_id": updateRequest.BillID,
		"user_id": updateRequest.UserID,
		"status":  "endpoint_active",
	})
}

func handleDeleteBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest DeleteBillRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}
	if deleteRequest.BillID <= 0 {
		sendErrorResponse(w, "Valid bill ID is required", http.StatusBadRequest)
		return
	}

	// Check if bill exists and belongs to the user
	var existingBillID int
	checkQuery := "SELECT id FROM bills WHERE id = ? AND user_id = ?"
	err = db.QueryRow(checkQuery, deleteRequest.BillID, deleteRequest.UserID).Scan(&existingBillID)

	if err == sql.ErrNoRows {
		sendErrorResponse(w, "Bill not found or you don't have permission to delete it", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("Error checking bill existence: %v", err)
		sendErrorResponse(w, "Error checking bill", http.StatusInternalServerError)
		return
	}

	// Delete related bill_payments first
	deletePaymentsQuery := "DELETE FROM bill_payments WHERE bill_id = ?"
	_, err = db.Exec(deletePaymentsQuery, deleteRequest.BillID)
	if err != nil {
		log.Printf("Error deleting bill payments: %v", err)
		sendErrorResponse(w, "Error deleting bill payments", http.StatusInternalServerError)
		return
	}

	// Delete the bill
	deleteBillQuery := "DELETE FROM bills WHERE id = ? AND user_id = ?"
	result, err := db.Exec(deleteBillQuery, deleteRequest.BillID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting bill: %v", err)
		sendErrorResponse(w, "Error deleting bill", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		sendErrorResponse(w, "Error verifying deletion", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		sendErrorResponse(w, "Bill not found or already deleted", http.StatusNotFound)
		return
	}

	sendSuccessResponse(w, "Bill deleted successfully", map[string]interface{}{
		"bill_id": deleteRequest.BillID,
		"user_id": deleteRequest.UserID,
		"status":  "deleted",
	})
}

func handleGetUpcomingBills(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Upcoming bills endpoint available", map[string]string{"status": "available"})
}

// Helper functions
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := ApiResponse{
		Success: false,
		Message: message,
	}
	json.NewEncoder(w).Encode(response)
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	json.NewEncoder(w).Encode(response)
}

func fetchBills(userID string) ([]Bill, error) {
	query := `
		SELECT id, user_id, name, amount, COALESCE(due_date, start_date), start_date, payment_day, 
		       duration_months, regularity, paid, overdue, overdue_days, 
		       recurring, category, icon, COALESCE(payment_method, 'cash'), 
		       COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM bills 
		WHERE user_id = ? 
		ORDER BY id ASC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bills []Bill
	for rows.Next() {
		var bill Bill
		err := rows.Scan(
			&bill.ID, &bill.UserID, &bill.Name, &bill.Amount, &bill.DueDate,
			&bill.StartDate, &bill.PaymentDay, &bill.DurationMonths, &bill.Regularity,
			&bill.Paid, &bill.Overdue, &bill.OverdueDays, &bill.Recurring,
			&bill.Category, &bill.Icon, &bill.PaymentMethod, &bill.CreatedAt, &bill.UpdatedAt,
		)
		if err != nil {
			log.Printf("Error scanning bill: %v", err)
			continue
		}
		bills = append(bills, bill)
	}

	return bills, nil
}

// addBill function for test compatibility
func addBill(bill Bill) (int, error) {
	// Use the new addBillWithBalanceUpdate function
	return addBillWithBalanceUpdate(
		db,
		bill.UserID,
		bill.Name,
		bill.Amount,
		bill.DueDate,
		bill.PaymentDay,
		bill.DurationMonths,
		bill.PaymentMethod,
		bill.Category,
		bill.Icon,
		bill.Regularity,
	)
}

// createExpenseFromBill function for test compatibility
func createExpenseFromBill(req BillToExpenseRequest) error {
	log.Printf("Creating expense from bill: %+v", req)

	// Insert expense into database
	_, err := db.Exec(`
		INSERT INTO expenses (user_id, amount, date, category, payment_method, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, req.UserID, req.Amount, req.Date, req.Category, req.PaymentMethod, req.Description)

	if err != nil {
		return fmt.Errorf("error creating expense: %v", err)
	}

	log.Printf("Successfully created expense for user %s", req.UserID)
	return nil
}
