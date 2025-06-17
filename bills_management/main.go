package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

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
	http.HandleFunc("/bills", corsMiddleware(handleFetchBills))
	http.HandleFunc("/bills/add", corsMiddleware(handleAddBill))
	http.HandleFunc("/bills/pay", corsMiddleware(handleGenericEndpoint))
	http.HandleFunc("/bills/payment-status", corsMiddleware(handleGenericEndpoint))
	http.HandleFunc("/bills/update", corsMiddleware(handleUpdateBill))
	http.HandleFunc("/bills/delete", corsMiddleware(handleDeleteBill))
	http.HandleFunc("/bills/upcoming", corsMiddleware(handleGenericEndpoint))
	http.HandleFunc("/bills/debug-add", corsMiddleware(handleDebugAddBill))
	fmt.Println("Bills Management service started on :8091")
	log.Fatal(http.ListenAndServe(":8091", nil))
}
func createTablesIfNotExist() {
	db.Exec(`CREATE TABLE IF NOT EXISTS bills (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT NOT NULL, name TEXT NOT NULL, amount REAL NOT NULL, due_date TEXT, start_date TEXT NOT NULL, payment_day INTEGER NOT NULL, duration_months INTEGER NOT NULL, regularity TEXT NOT NULL DEFAULT 'monthly', paid BOOLEAN DEFAULT 0, overdue BOOLEAN DEFAULT 0, overdue_days INTEGER DEFAULT 0, recurring BOOLEAN DEFAULT 1, category TEXT DEFAULT 'general', icon TEXT DEFAULT '💳', payment_method TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);`)
	db.Exec(`CREATE TABLE IF NOT EXISTS bill_payments (id INTEGER PRIMARY KEY AUTOINCREMENT, bill_id INTEGER NOT NULL, year_month TEXT NOT NULL, paid BOOLEAN DEFAULT 0, payment_date TEXT, payment_method TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY (bill_id) REFERENCES bills (id) ON DELETE CASCADE, UNIQUE(bill_id, year_month));`)
	db.Exec("ALTER TABLE expenses ADD COLUMN bill_id INTEGER;")
}
func handleFetchBills(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")
	if period != "" && date != "" {
		billsWithStatus, err := fetchBillsForPeriod(userID, period, date)
		if err != nil {
			sendErrorResponse(w, "Error fetching bills for period", http.StatusInternalServerError)
			return
		}
		var bills []Bill
		for _, billWithStatus := range billsWithStatus {
			bills = append(bills, convertBillWithPeriodStatusToBill(billWithStatus))
		}
		sendSuccessResponse(w, "Bills fetched successfully", bills)
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
	var req struct {
		UserID         string  `json:"user_id"`
		Name           string  `json:"name"`
		Amount         float64 `json:"amount"`
		DueDate        string  `json:"due_date"`
		Category       string  `json:"category"`
		Icon           string  `json:"icon"`
		StartDate      string  `json:"start_date"`
		PaymentDay     int     `json:"payment_day"`
		DurationMonths int     `json:"duration_months"`
		Regularity     string  `json:"regularity"`
		PaymentMethod  string  `json:"payment_method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Name == "" || req.Amount <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?)", req.UserID, req.Name, req.Amount, req.DueDate, req.Category, req.Icon, req.StartDate, req.PaymentDay, req.DurationMonths, req.Regularity, req.PaymentMethod)
	if err != nil {
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}
	billID, _ := result.LastInsertId()
	addNewBillToMonthlyBalance(db, req.UserID, req.Amount, req.StartDate, req.DurationMonths, req.PaymentMethod)
	createBillPaymentRecords(db, int(billID), req.UserID, req.StartDate, req.DurationMonths, req.PaymentMethod)
	sendSuccessResponse(w, "Bill added successfully", map[string]interface{}{"id": billID, "user_id": req.UserID, "name": req.Name, "amount": req.Amount})
}
func handleUpdateBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var updateRequest UpdateBillRequest
	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil || updateRequest.UserID == "" || updateRequest.BillID <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	oldBillData, err := getBillOldData(db, updateRequest.BillID, updateRequest.UserID)
	if err != nil {
		sendErrorResponse(w, "Bill not found", http.StatusNotFound)
		return
	}
	if err = updateBillInDatabase(db, updateRequest); err != nil {
		sendErrorResponse(w, "Error updating bill", http.StatusInternalServerError)
		return
	}
	updateData := BillUpdateData{
		BillID: updateRequest.BillID, UserID: updateRequest.UserID,
		OldAmount: oldBillData.Amount, NewAmount: getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths, NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate: oldBillData.StartDate, NewStartDate: getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod: oldBillData.PaymentMethod, NewPaymentMethod: getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
	}
	if err = updateBillAmountLogic(db, updateData); err != nil {
		sendErrorResponse(w, "Error updating bill amounts", http.StatusInternalServerError)
		return
	}
	if err = updateBillDurationLogic(db, updateData); err != nil {
		sendErrorResponse(w, "Error updating bill duration", http.StatusInternalServerError)
		return
	}
	sendSuccessResponse(w, "Bill updated successfully", map[string]interface{}{"bill_id": updateRequest.BillID, "status": "updated"})
}
func handleGenericEndpoint(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Endpoint available", map[string]string{"status": "available"})
}
func handleDebugAddBill(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔥 DEBUG: handleDebugAddBill llamada")

	// Parámetros fijos para depuración
	userID := "14"
	amount := 100.0
	startDate := "2025-01-15T00:00:00Z"
	durationMonths := 6
	paymentMethod := "bank"

	log.Printf("🔥 DEBUG: Llamando a addNewBillToMonthlyBalance directamente...")
	err := addNewBillToMonthlyBalance(db, userID, amount, startDate, durationMonths, paymentMethod)

	if err != nil {
		log.Printf("🔥 DEBUG: Error en addNewBillToMonthlyBalance: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("🔥 DEBUG: addNewBillToMonthlyBalance completada sin errores")
	sendSuccessResponse(w, "Debug completed", map[string]interface{}{
		"user_id":         userID,
		"amount":          amount,
		"duration_months": durationMonths,
		"payment_method":  paymentMethod,
	})
}
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{Success: false, Message: message})
}
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{Success: true, Message: message, Data: data})
}
func fetchBills(userID string) ([]Bill, error) {
	query := "SELECT id, user_id, name, amount, COALESCE(due_date, start_date), start_date, payment_day, duration_months, regularity, paid, overdue, overdue_days, recurring, category, icon, COALESCE(payment_method, 'cash'), COALESCE(created_at, ''), COALESCE(updated_at, '') FROM bills WHERE user_id = ? ORDER BY id ASC"
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var bills []Bill
	for rows.Next() {
		var bill Bill
		if err := rows.Scan(&bill.ID, &bill.UserID, &bill.Name, &bill.Amount, &bill.DueDate, &bill.StartDate, &bill.PaymentDay, &bill.DurationMonths, &bill.Regularity, &bill.Paid, &bill.Overdue, &bill.OverdueDays, &bill.Recurring, &bill.Category, &bill.Icon, &bill.PaymentMethod, &bill.CreatedAt, &bill.UpdatedAt); err == nil {
			bills = append(bills, bill)
		}
	}
	return bills, nil
}
func getBillOldData(db *sql.DB, billID int, userID string) (*Bill, error) {
	query := "SELECT id, user_id, name, amount, COALESCE(due_date, start_date), start_date, payment_day, duration_months, regularity, paid, overdue, overdue_days, recurring, category, icon, COALESCE(payment_method, 'cash'), COALESCE(created_at, ''), COALESCE(updated_at, '') FROM bills WHERE id = ? AND user_id = ?"
	var bill Bill
	err := db.QueryRow(query, billID, userID).Scan(&bill.ID, &bill.UserID, &bill.Name, &bill.Amount, &bill.DueDate, &bill.StartDate, &bill.PaymentDay, &bill.DurationMonths, &bill.Regularity, &bill.Paid, &bill.Overdue, &bill.OverdueDays, &bill.Recurring, &bill.Category, &bill.Icon, &bill.PaymentMethod, &bill.CreatedAt, &bill.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &bill, nil
}
func updateBillInDatabase(db *sql.DB, updateRequest UpdateBillRequest) error {
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
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")
	setClause := strings.Join(setParts, ", ")
	query := fmt.Sprintf("UPDATE bills SET %s WHERE id = ? AND user_id = ?", setClause)
	args = append(args, updateRequest.BillID, updateRequest.UserID)
	_, err := db.Exec(query, args...)
	return err
}
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
