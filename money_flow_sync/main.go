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
type SyncRequest struct {
	UserID string `json:"user_id"`
	Period string `json:"period"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type BudgetData struct {
	UserID          string  `json:"user_id"`
	Period          string  `json:"period"`
	Date            string  `json:"date"`
	TotalAmount     float64 `json:"total_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	SpentAmount     float64 `json:"spent_amount"`
	UpcomingAmount  float64 `json:"upcoming_amount"`
	FromPrevious    float64 `json:"from_previous"`
	Percent         float64 `json:"percent"`
	TotalIncome     float64 `json:"total_income"`
}

type Bill struct {
	Amount    float64 `json:"amount"`
	DueDate   string  `json:"due_date"`
	Paid      bool    `json:"paid"`
	Recurring bool    `json:"recurring"`
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

	log.Println("Database connection established successfully")
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/money-flow/sync", corsMiddleware(handleSyncMoneyFlow))
	http.HandleFunc("/money-flow/data", corsMiddleware(handleGetMoneyFlowData))

	port := 8097 // Puerto para el servicio de sincronización de money flow
	log.Printf("Money Flow Sync service started on :%d", port)
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

func handleSyncMoneyFlow(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var syncRequest SyncRequest
	err := json.NewDecoder(r.Body).Decode(&syncRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if syncRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if syncRequest.Period == "" {
		syncRequest.Period = "monthly" // Default to monthly
	}

	// Sync money flow data
	budget, err := syncMoneyFlow(syncRequest.UserID, syncRequest.Period)
	if err != nil {
		log.Printf("Error syncing money flow: %v", err)
		sendErrorResponse(w, "Error syncing money flow", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Money flow synced successfully", budget)
}

func handleGetMoneyFlowData(w http.ResponseWriter, r *http.Request) {
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

	// Get period from query parameter (default to monthly)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	log.Printf("Getting money flow data for user %s with period %s", userID, period)

	// Get money flow data
	budget, err := syncMoneyFlow(userID, period)
	if err != nil {
		log.Printf("Error getting money flow data: %v", err)
		sendErrorResponse(w, "Error getting money flow data", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Money flow data retrieved successfully", budget)
}

func syncMoneyFlow(userID, period string) (*BudgetData, error) {
	log.Printf("Syncing money flow for user %s with period %s", userID, period)

	// Get date range for the period
	startDate, endDate := getDateRangeForPeriod(period)
	log.Printf("Date range: %s to %s", startDate, endDate)

	// Get remaining amount from previous period
	previousPeriod, fromPrevious := getPreviousPeriodData(userID, period)
	log.Printf("Previous period: %s, fromPrevious: %.2f", previousPeriod, fromPrevious)

	// Get total income for the period
	totalIncome, err := getTotalIncomeForPeriod(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("error getting total income: %v", err)
	}
	log.Printf("Total income: %.2f", totalIncome)

	// Get spent amount for the period
	spentAmount, err := getSpentAmountForPeriod(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("error getting spent amount: %v", err)
	}
	log.Printf("Spent amount: %.2f", spentAmount)

	// Get upcoming bills amount
	upcomingAmount, err := getUpcomingBillsAmount(userID, startDate, endDate)
	if err != nil {
		log.Printf("Error getting upcoming bills amount: %v", err)
		return nil, fmt.Errorf("error getting upcoming bills amount: %v", err)
	}
	log.Printf("Upcoming amount: %.2f", upcomingAmount)

	// Calculate total and remaining amounts
	totalAmount := fromPrevious + totalIncome
	remainingAmount := totalAmount - spentAmount - upcomingAmount

	// Calculate percent
	var percent float64
	if totalAmount > 0 {
		percent = ((spentAmount + upcomingAmount) / totalAmount) * 100
	}

	log.Printf("Total amount: %.2f, Remaining amount: %.2f, Percent: %.2f", totalAmount, remainingAmount, percent)

	// Create budget data
	budget := &BudgetData{
		UserID:          userID,
		Period:          period,
		Date:            time.Now().Format("2006-01-02"),
		TotalAmount:     totalAmount,
		RemainingAmount: remainingAmount,
		SpentAmount:     spentAmount,
		UpcomingAmount:  upcomingAmount,
		FromPrevious:    fromPrevious,
		Percent:         percent,
		TotalIncome:     totalIncome,
	}

	// Update budget record in database
	err = updateBudgetData(budget)
	if err != nil {
		return nil, fmt.Errorf("error updating budget data: %v", err)
	}

	// Update finance metrics
	err = updateFinanceMetrics(userID, period, totalIncome, spentAmount, upcomingAmount)
	if err != nil {
		log.Printf("Warning: error updating finance metrics: %v", err)
		// Don't fail the entire operation if updating finance metrics fails
	}

	return budget, nil
}

func getDateRangeForPeriod(period string) (string, string) {
	now := time.Now()

	switch period {
	case "daily":
		// Current day
		return now.Format("2006-01-02"), now.Format("2006-01-02")
	case "weekly":
		// Start of the week (Monday)
		startDate := now.AddDate(0, 0, -int(now.Weekday())+1)
		if now.Weekday() == 0 { // If today is Sunday
			startDate = now.AddDate(0, 0, -6) // Go back to previous Monday
		}
		// End of the week (Sunday)
		endDate := startDate.AddDate(0, 0, 6)
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "monthly":
		// Start of the month
		startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		// End of the month
		endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month())-1)/3 + 1
		startDate := time.Date(now.Year(), time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), time.Month(quarter*3+1), 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "semiannual":
		halfYear := (int(now.Month())-1)/6 + 1
		startDate := time.Date(now.Year(), time.Month((halfYear-1)*6+1), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), time.Month(halfYear*6+1), 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "annual":
		startDate := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	default:
		// Default to monthly if period is not recognized
		startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	}
}

func getPreviousPeriodData(userID, currentPeriod string) (string, float64) {
	// Para el cálculo del flujo de dinero, necesitamos el previous_amount del MES ACTUAL
	// no del mes anterior. Esto es porque previous_amount ya contiene el balance heredado.

	now := time.Now()

	switch currentPeriod {
	case "monthly":
		// Obtener el año-mes actual
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts for user %s, month %s", userID, currentYearMonth)

		// Consultar la tabla monthly_cash_bank_balance para obtener los previous_amounts del mes actual
		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			// Si no hay registro, devolver 0 (no hay balance heredado)
			return "monthly", 0
		}

		log.Printf("📊 Found previous amounts for %s: total_previous=%.2f", currentYearMonth, totalPrevious)
		return "monthly", totalPrevious

	case "daily":
		// Para períodos diarios, también buscamos en monthly_cash_bank_balance
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts (daily) for user %s, month %s", userID, currentYearMonth)

		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			return "daily", 0
		}

		return "daily", totalPrevious

	case "weekly":
		// Para períodos semanales, también buscamos en monthly_cash_bank_balance del mes actual
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts (weekly) for user %s, month %s", userID, currentYearMonth)

		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			return "weekly", 0
		}

		return "weekly", totalPrevious

	default:
		// Para otros períodos, mantener la lógica original como fallback
		log.Printf("⚠️ Using fallback logic for period: %s", currentPeriod)
		return "", 0
	}
}

func getTotalIncomeForPeriod(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM incomes
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var totalIncome float64
	err := db.QueryRow(query, userID, startDate, endDate).Scan(&totalIncome)
	if err != nil {
		return 0, err
	}

	return totalIncome, nil
}

func getSpentAmountForPeriod(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var spentAmount float64
	err := db.QueryRow(query, userID, startDate, endDate).Scan(&spentAmount)
	if err != nil {
		return 0, err
	}

	return spentAmount, nil
}

func getUpcomingBillsAmount(userID, startDate, endDate string) (float64, error) {
	// Para calcular las facturas pendientes, necesitamos consultar la tabla bill_payments
	// y obtener las facturas que NO han sido pagadas en el período actual

	// Convertir las fechas del período al formato year_month para consultar bill_payments
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, fmt.Errorf("error parsing start date: %v", err)
	}

	// Para períodos mensuales, usar el año-mes del startDate
	yearMonth := startTime.Format("2006-01")

	// Consultar bill_payments para obtener facturas NO pagadas del mes actual
	query := `
		SELECT COALESCE(SUM(b.amount), 0)
		FROM bills b
		INNER JOIN bill_payments bp ON b.id = bp.bill_id
		WHERE bp.user_id = ? 
		AND bp.year_month = ? 
		AND bp.paid = 0
	`

	var upcomingAmount float64
	err = db.QueryRow(query, userID, yearMonth).Scan(&upcomingAmount)
	if err != nil {
		log.Printf("Error getting upcoming bills amount from bill_payments: %v", err)
		// Fallback a la lógica original si falla la nueva consulta
		return getUpcomingBillsAmountFallback(userID, startDate, endDate)
	}

	log.Printf("📋 Found upcoming bills for %s: amount=%.2f", yearMonth, upcomingAmount)
	return upcomingAmount, nil
}

// Función de fallback para mantener compatibilidad con la lógica original
func getUpcomingBillsAmountFallback(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT amount, due_date, paid, recurring
		FROM bills
		WHERE user_id = ? AND due_date BETWEEN ? AND ? AND paid = 0
	`

	rows, err := db.Query(query, userID, startDate, endDate)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var upcomingAmount float64
	for rows.Next() {
		var bill Bill
		err := rows.Scan(&bill.Amount, &bill.DueDate, &bill.Paid, &bill.Recurring)
		if err != nil {
			return 0, err
		}

		// Only count bills that haven't been paid yet
		// (This check is now redundant since we filter in SQL, but keeping for safety)
		if !bill.Paid {
			upcomingAmount += bill.Amount
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	log.Printf("📋 Fallback upcoming bills: amount=%.2f", upcomingAmount)
	return upcomingAmount, nil
}

func updateBudgetData(budget *BudgetData) error {
	// Check if a budget entry already exists for this user and period
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM budget 
		WHERE user_id = ? AND period = ?
	`, budget.UserID, budget.Period).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing budget entry
		_, err = db.Exec(`
			UPDATE budget
			SET total_amount = ?,
				remaining_amount = ?,
				spent_amount = ?,
				upcoming_amount = ?,
				from_previous = ?,
				percent = ?,
				total_income = ?,
				date = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND period = ?
		`,
			budget.TotalAmount,
			budget.RemainingAmount,
			budget.SpentAmount,
			budget.UpcomingAmount,
			budget.FromPrevious,
			budget.Percent,
			budget.TotalIncome,
			budget.Date,
			budget.UserID,
			budget.Period,
		)
	} else {
		// Insert new budget entry
		_, err = db.Exec(`
			INSERT INTO budget (
				user_id, period, date, total_amount, remaining_amount, 
				spent_amount, upcoming_amount, from_previous, percent, total_income
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			budget.UserID,
			budget.Period,
			budget.Date,
			budget.TotalAmount,
			budget.RemainingAmount,
			budget.SpentAmount,
			budget.UpcomingAmount,
			budget.FromPrevious,
			budget.Percent,
			budget.TotalIncome,
		)
	}

	return err
}

func updateFinanceMetrics(userID, period string, income, expenses, bills float64) error {
	// Check if a finance metrics entry already exists for this user and period
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM finance_metrics 
		WHERE user_id = ? AND period = ?
	`, userID, period).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing entry
		_, err = db.Exec(`
			UPDATE finance_metrics
			SET income = ?,
				expenses = ?,
				bills = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND period = ?
		`,
			income,
			expenses,
			bills,
			userID,
			period,
		)
	} else {
		// Insert new entry
		_, err = db.Exec(`
			INSERT INTO finance_metrics (
				user_id, period, income, expenses, bills
			) VALUES (?, ?, ?, ?, ?)
		`,
			userID,
			period,
			income,
			expenses,
			bills,
		)
	}

	return err
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
