package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
