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

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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
	http.HandleFunc("/bills/payment-status", corsMiddleware(handleGetPaymentStatus))
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

	// Obtener parámetros opcionales de período y fecha
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")

	log.Printf("🔍 handleFetchBills: userID=%s, period=%s, date=%s", userID, period, date)

	// Si se proporcionan parámetros de período, usar la nueva lógica
	if period != "" && date != "" {
		billsWithStatus, err := fetchBillsForPeriod(userID, period, date)
		if err != nil {
			log.Printf("❌ Error fetching bills for period: %v", err)
			sendErrorResponse(w, "Error fetching bills for period", http.StatusInternalServerError)
			return
		}

		// Convertir a formato Bill para compatibilidad
		var bills []Bill
		for _, billWithStatus := range billsWithStatus {
			bills = append(bills, convertBillWithPeriodStatusToBill(billWithStatus))
		}

		log.Printf("✅ Returning %d bills for period %s", len(bills), date)
		sendSuccessResponse(w, "Bills fetched successfully", bills)
		return
	}

	// Fallback: usar la lógica original si no se proporcionan parámetros de período
	bills, err := fetchBills(userID)
	if err != nil {
		sendErrorResponse(w, "Error fetching bills", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Bills fetched successfully", bills)
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

	// Insert into database
	result, err := db.Exec(`
		INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method)
		VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?)
	`, addRequest.UserID, addRequest.Name, addRequest.Amount, addRequest.DueDate, addRequest.Category, addRequest.Icon, addRequest.StartDate, addRequest.PaymentDay, addRequest.DurationMonths, addRequest.Regularity, addRequest.PaymentMethod)

	if err != nil {
		log.Printf("Error adding bill: %v", err)
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}

	// Get the ID of the newly created bill
	billID, err := result.LastInsertId()
	if err != nil {
		log.Printf("Error getting bill ID: %v", err)
		sendErrorResponse(w, "Error getting bill ID", http.StatusInternalServerError)
		return
	}

	// Add the new bill to monthly_cash_bank_balance
	err = addNewBillToMonthlyBalance(db, addRequest.UserID, addRequest.Amount,
		addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)
	if err != nil {
		log.Printf("Error adding bill to monthly balance: %v", err)
		// Note: We don't return error here as the bill was created successfully
		// The balance issue can be fixed later with reconciliation
	}

	// Create bill payment records for tracking individual payments
	err = createBillPaymentRecords(db, int(billID), addRequest.UserID,
		addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)
	if err != nil {
		log.Printf("Error creating bill payment records: %v", err)
		// Note: We don't return error here as the bill was created successfully
		// The payment records can be created later manually if needed
	}

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
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payRequest PayBillRequest
	err := json.NewDecoder(r.Body).Decode(&payRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	err = validatePayBillRequest(payRequest)
	if err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set default payment date if not provided
	if payRequest.PaymentDate == "" {
		payRequest.PaymentDate = time.Now().Format("2006-01-02")
	}

	// Process the payment
	response, err := markBillPaid(db, payRequest.BillID, payRequest.UserID,
		payRequest.YearMonth, payRequest.PaymentDate)
	if err != nil {
		log.Printf("Error processing bill payment: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error processing payment: %v", err), http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Bill payment processed successfully", response)
}

func handleGetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	billIDStr := r.URL.Query().Get("bill_id")
	userID := r.URL.Query().Get("user_id")

	if billIDStr == "" || userID == "" {
		sendErrorResponse(w, "Bill ID and User ID are required", http.StatusBadRequest)
		return
	}

	billID := 0
	_, err := fmt.Sscanf(billIDStr, "%d", &billID)
	if err != nil || billID <= 0 {
		sendErrorResponse(w, "Valid bill ID is required", http.StatusBadRequest)
		return
	}

	status, err := getBillPaymentStatus(db, billID, userID)
	if err != nil {
		log.Printf("Error getting payment status: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error getting payment status: %v", err), http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Payment status retrieved successfully", status)
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

	// 1. Obtener datos antiguos del bill
	oldBillData, err := getBillOldData(db, updateRequest.BillID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching old bill data: %v", err)
		sendErrorResponse(w, "Bill not found", http.StatusNotFound)
		return
	}

	// 2. Actualizar tabla bills con los nuevos datos
	err = updateBillInDatabase(db, updateRequest)
	if err != nil {
		log.Printf("Error updating bill in database: %v", err)
		sendErrorResponse(w, "Error updating bill", http.StatusInternalServerError)
		return
	}

	// 3. Preparar datos para la lógica de actualización
	updateData := BillUpdateData{
		BillID:            updateRequest.BillID,
		UserID:            updateRequest.UserID,
		OldAmount:         oldBillData.Amount,
		NewAmount:         getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths,
		NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate:      oldBillData.StartDate,
		NewStartDate:      getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod:  oldBillData.PaymentMethod,
		NewPaymentMethod:  getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
	}

	// 4. Ejecutar lógica de actualización de importes
	err = updateBillAmountLogic(db, updateData)
	if err != nil {
		log.Printf("Error in amount update logic: %v", err)
		sendErrorResponse(w, "Error updating bill amounts", http.StatusInternalServerError)
		return
	}

	// 5. Ejecutar lógica de actualización de duración
	err = updateBillDurationLogic(db, updateData)
	if err != nil {
		log.Printf("Error in duration update logic: %v", err)
		sendErrorResponse(w, "Error updating bill duration", http.StatusInternalServerError)
		return
	}

	// 6. Recalcular saldos en cascada usando el algoritmo existente
	startDate := updateData.NewStartDate
	parsedDate, err := time.Parse("2006-01-02", startDate)
	if err == nil {
		firstMonth := parsedDate.Format("2006-01")
		err = updateCascadeBalances(db, updateData.UserID, firstMonth)
		if err != nil {
			log.Printf("Error updating cascade balances: %v", err)
		}
	}

	sendSuccessResponse(w, "Bill updated successfully", map[string]interface{}{
		"bill_id": updateRequest.BillID,
		"user_id": updateRequest.UserID,
		"status":  "updated",
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

// getBillOldData obtiene los datos antiguos de un bill antes de actualizarlo
func getBillOldData(db *sql.DB, billID int, userID string) (*Bill, error) {
	query := `
		SELECT id, user_id, name, amount, COALESCE(due_date, start_date), start_date, payment_day, 
		       duration_months, regularity, paid, overdue, overdue_days, 
		       recurring, category, icon, COALESCE(payment_method, 'cash'), 
		       COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM bills 
		WHERE id = ? AND user_id = ?
	`

	var bill Bill
	err := db.QueryRow(query, billID, userID).Scan(
		&bill.ID, &bill.UserID, &bill.Name, &bill.Amount, &bill.DueDate,
		&bill.StartDate, &bill.PaymentDay, &bill.DurationMonths, &bill.Regularity,
		&bill.Paid, &bill.Overdue, &bill.OverdueDays, &bill.Recurring,
		&bill.Category, &bill.Icon, &bill.PaymentMethod, &bill.CreatedAt, &bill.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &bill, nil
}

// updateBillInDatabase actualiza los campos del bill en la base de datos
func updateBillInDatabase(db *sql.DB, updateRequest UpdateBillRequest) error {
	// Construir query dinámicamente según los campos que se proporcionan
	setParts := []string{}
	args := []interface{}{}

	if updateRequest.Name != "" {
		setParts = append(setParts, "name = ?")
		args = append(args, updateRequest.Name)
	}
	if updateRequest.Amount > 0 {
		setParts = append(setParts, "amount = ?")
		args = append(args, updateRequest.Amount)
	}
	if updateRequest.StartDate != "" {
		setParts = append(setParts, "start_date = ?")
		args = append(args, updateRequest.StartDate)
	}
	if updateRequest.PaymentDay > 0 {
		setParts = append(setParts, "payment_day = ?")
		args = append(args, updateRequest.PaymentDay)
	}
	if updateRequest.DurationMonths > 0 {
		setParts = append(setParts, "duration_months = ?")
		args = append(args, updateRequest.DurationMonths)
	}
	if updateRequest.Regularity != "" {
		setParts = append(setParts, "regularity = ?")
		args = append(args, updateRequest.Regularity)
	}
	if updateRequest.Category != "" {
		setParts = append(setParts, "category = ?")
		args = append(args, updateRequest.Category)
	}
	if updateRequest.Icon != "" {
		setParts = append(setParts, "icon = ?")
		args = append(args, updateRequest.Icon)
	}
	if updateRequest.PaymentMethod != "" {
		setParts = append(setParts, "payment_method = ?")
		args = append(args, updateRequest.PaymentMethod)
	}

	if len(setParts) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Añadir updated_at
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")

	// Construir query final
	query := fmt.Sprintf("UPDATE bills SET %s WHERE id = ? AND user_id = ?",
		fmt.Sprintf("%s", setParts[0]))
	for i := 1; i < len(setParts); i++ {
		query = fmt.Sprintf("%s, %s", query, setParts[i])
	}

	// Añadir parámetros de WHERE
	args = append(args, updateRequest.BillID, updateRequest.UserID)

	_, err := db.Exec(query, args...)
	return err
}

// Funciones auxiliares para manejar valores opcionales
func getValueOrDefault(value, defaultValue float64) float64 {
	if value > 0 {
		return value
	}
	return defaultValue
}

func getIntValueOrDefault(value, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}

func getStringValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// updateCascadeBalances recalcula los saldos en cascada desde startMonth
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

		// Calcular saldos del mes actual
		cashAmount := previousCashAmount + incomeCash - expenseCash - billCash
		bankAmount := previousBankAmount + incomeBank - expenseBank - billBank
		balanceCashAmount := cashAmount
		balanceBankAmount := bankAmount
		totalBalance := balanceCashAmount + balanceBankAmount

		// Actualizar registro
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
