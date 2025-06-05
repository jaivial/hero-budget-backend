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
type DashboardData struct {
	Period           string          `json:"period"`
	Date             string          `json:"date"`
	BudgetOverview   BudgetOverview  `json:"budget_overview"`
	SavingsOverview  SavingsOverview `json:"savings_overview"`
	CashDistribution CashBank        `json:"cash_distribution"`
	FinanceMetrics   FinanceMetrics  `json:"finance_metrics"`
	UpcomingBills    []Bill          `json:"upcoming_bills"`
}

type BudgetOverview struct {
	MoneyFlow       MoneyFlow `json:"money_flow"`
	RemainingAmount float64   `json:"remaining_amount"`
	TotalAmount     float64   `json:"total_amount"`
	SpentAmount     float64   `json:"spent_amount"`
	UpcomingAmount  float64   `json:"upcoming_amount"`
	CombinedExpense float64   `json:"combined_expense"`
	ExpensePercent  float64   `json:"expense_percent"`
	DailyRate       float64   `json:"daily_rate"`
	HighSpending    bool      `json:"high_spending"`
	TotalIncome     float64   `json:"total_income"`
}

type MoneyFlow struct {
	Percent      float64 `json:"percent"`
	FromPrevious float64 `json:"from_previous"`
}

type SavingsOverview struct {
	Percent     float64 `json:"percent"`
	Available   float64 `json:"available"`
	Goal        float64 `json:"goal"`
	Period      string  `json:"period"`
	NeedToSave  float64 `json:"need_to_save"`
	DailyTarget float64 `json:"daily_target"`
}

type CashBank struct {
	Month        string  `json:"month"`
	CashAmount   float64 `json:"cash_amount"`
	CashPercent  float64 `json:"cash_percent"`
	BankAmount   float64 `json:"bank_amount"`
	BankPercent  float64 `json:"bank_percent"`
	MonthlyTotal float64 `json:"monthly_total"`
}

type FinanceMetrics struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Bills    float64 `json:"bills"`
}

type Bill struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Amount      float64 `json:"amount"`
	DueDate     string  `json:"due_date"`
	Paid        bool    `json:"paid"`
	Overdue     bool    `json:"overdue"`
	OverdueDays int     `json:"overdue_days"`
	Recurring   bool    `json:"recurring"`
	Category    string  `json:"category"`
	Icon        string  `json:"icon"`
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
	// Create budget table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS budget (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			period TEXT NOT NULL,
			date TEXT NOT NULL,
			total_amount REAL NOT NULL,
			remaining_amount REAL NOT NULL,
			spent_amount REAL NOT NULL,
			upcoming_amount REAL NOT NULL,
			from_previous REAL NOT NULL,
			percent REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create budget table: %v", err)
	}

	// Create savings table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS savings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			available REAL NOT NULL,
			goal REAL NOT NULL,
			period TEXT NOT NULL DEFAULT 'monthly',
			percent REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create savings table: %v", err)
	}

	// Add period column if it doesn't exist (for existing tables)
	_, err = db.Exec(`
		ALTER TABLE savings ADD COLUMN period TEXT NOT NULL DEFAULT 'monthly'
	`)
	if err != nil {
		// Column might already exist, which is fine
		log.Printf("Note: period column might already exist: %v", err)
	}

	// Create cash_bank table
	_, err = db.Exec(`
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

	// Create finance_metrics table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS finance_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			period TEXT NOT NULL,
			income REAL NOT NULL,
			expenses REAL NOT NULL,
			bills REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create finance_metrics table: %v", err)
	}

	// Create bills table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			amount REAL NOT NULL,
			due_date TEXT NOT NULL,
			paid BOOLEAN NOT NULL,
			overdue BOOLEAN NOT NULL,
			overdue_days INTEGER NOT NULL,
			recurring BOOLEAN NOT NULL,
			category TEXT NOT NULL,
			icon TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create bills table: %v", err)
	}

	// Insert mock data for testing
	insertMockDataIfEmpty()
}

func insertMockDataIfEmpty() {
	// Check if there is data in the budget table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM budget").Scan(&count)
	if err != nil {
		log.Printf("Error checking budget table: %v", err)
		return
	}

	// If there's no data, insert mock data
	if count == 0 {
		insertMockData()
	}
}

func insertMockData() {
	// Insert mock budget data
	_, err := db.Exec(`
		INSERT INTO budget (user_id, period, date, total_amount, remaining_amount, spent_amount, upcoming_amount, from_previous, percent)
		VALUES ('1', 'monthly', '2025-05-01', 975.00, 875.00, 0.00, 100.00, 975.00, 10.0)
	`)
	if err != nil {
		log.Printf("Error inserting mock budget data: %v", err)
	}

	// Insert mock savings data
	_, err = db.Exec(`
		INSERT INTO savings (user_id, available, goal, percent)
		VALUES ('1', 875.00, 1000.00, 88.0)
	`)
	if err != nil {
		log.Printf("Error inserting mock savings data: %v", err)
	}

	// Insert mock cash_bank data
	_, err = db.Exec(`
		INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
		VALUES ('1', 'mayo de 2025', 200.00, 100.0, 0.00, 0.0, 200.00)
	`)
	if err != nil {
		log.Printf("Error inserting mock cash_bank data: %v", err)
	}

	// Insert mock finance_metrics data
	_, err = db.Exec(`
		INSERT INTO finance_metrics (user_id, period, income, expenses, bills)
		VALUES ('1', 'monthly', 0.00, 0.00, 100.00)
	`)
	if err != nil {
		log.Printf("Error inserting mock finance_metrics data: %v", err)
	}

	// Insert mock bills data
	_, err = db.Exec(`
		INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon)
		VALUES ('1', 'Cash', 100.00, '2025-05-28', false, true, 8751, true, 'Rent', '🏠')
	`)
	if err != nil {
		log.Printf("Error inserting mock bills data: %v", err)
	}
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/dashboard/data", corsMiddleware(handleFetchDashboardData))

	port := 8087
	log.Printf("Dashboard Data service started on :%d", port)
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

func handleFetchDashboardData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get period from query parameter (default to 'monthly' if not provided)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	// Get dashboard data from database
	dashboardData, err := fetchDashboardData(userID, period)
	if err != nil {
		log.Printf("Error fetching dashboard data: %v", err)
		http.Error(w, "Error fetching dashboard data", http.StatusInternalServerError)
		return
	}

	// Return dashboard data as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboardData)
}

func fetchDashboardData(userID, period string) (DashboardData, error) {
	var dashboardData DashboardData
	dashboardData.Period = period

	// Get current date
	now := time.Now()
	dashboardData.Date = now.Format("2006-01-02")

	// Get budget overview
	budgetOverview, err := fetchBudgetOverview(userID, period)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.BudgetOverview = budgetOverview

	// Get savings overview
	savingsOverview, err := fetchSavingsOverview(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.SavingsOverview = savingsOverview

	// Get cash bank distribution
	cashBank, err := fetchCashBankDistribution(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.CashDistribution = cashBank

	// Get finance metrics
	financeMetrics, err := fetchFinanceMetrics(userID, period)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.FinanceMetrics = financeMetrics

	// Get upcoming bills
	upcomingBills, err := fetchUpcomingBills(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.UpcomingBills = upcomingBills

	return dashboardData, nil
}

func fetchBudgetOverview(userID, period string) (BudgetOverview, error) {
	var budgetOverview BudgetOverview

	// Query budget data from database
	err := db.QueryRow(`
		SELECT total_amount, remaining_amount, spent_amount, upcoming_amount, from_previous, percent
		FROM budget
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&budgetOverview.TotalAmount,
		&budgetOverview.RemainingAmount,
		&budgetOverview.SpentAmount,
		&budgetOverview.UpcomingAmount,
		&budgetOverview.MoneyFlow.FromPrevious,
		&budgetOverview.MoneyFlow.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		budgetOverview.TotalAmount = 0
		budgetOverview.RemainingAmount = 0
		budgetOverview.SpentAmount = 0
		budgetOverview.UpcomingAmount = 0
		budgetOverview.MoneyFlow.FromPrevious = 0
		budgetOverview.MoneyFlow.Percent = 0
	} else if err != nil {
		return budgetOverview, err
	}

	// Calculate the total income for the period
	// Fetch total income from incomes table for the specified period
	var startDate, endDate string
	now := time.Now()

	switch period {
	case "daily":
		startDate = now.Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	case "weekly":
		// Start of the week (Monday)
		startDate = now.AddDate(0, 0, -int(now.Weekday())+1).Format("2006-01-02")
		// End of the week (Sunday)
		endDate = now.AddDate(0, 0, 7-int(now.Weekday())).Format("2006-01-02")
	case "monthly":
		// Start of the month
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		// End of the month
		endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month())-1)/3 + 1
		startDate = time.Date(now.Year(), time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), time.Month(quarter*3+1), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "semiannual":
		halfYear := (int(now.Month())-1)/6 + 1
		startDate = time.Date(now.Year(), time.Month((halfYear-1)*6+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), time.Month(halfYear*6+1), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "annual":
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	default:
		// Default to monthly if period is not recognized
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}

	// Get total income for the period
	var totalIncome float64
	err = db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM incomes
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`, userID, startDate, endDate).Scan(&totalIncome)

	if err != nil {
		log.Printf("Error fetching total income: %v", err)
		totalIncome = 0 // Default to 0 if there's an error
	}

	budgetOverview.TotalIncome = totalIncome

	// Calculate combined expense and expense percent
	budgetOverview.CombinedExpense = budgetOverview.SpentAmount + budgetOverview.UpcomingAmount
	if budgetOverview.TotalAmount > 0 {
		budgetOverview.ExpensePercent = (budgetOverview.CombinedExpense / budgetOverview.TotalAmount) * 100
	}

	// Calculate daily rate
	daysInPeriod := 30 // Default for monthly
	switch period {
	case "daily":
		daysInPeriod = 1
	case "weekly":
		daysInPeriod = 7
	case "monthly":
		// Calculate actual days in the current month
		year, month, _ := time.Now().Date()
		lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Now().Location())
		daysInPeriod = lastDay.Day()
	case "quarterly":
		daysInPeriod = 90
	case "semiannual":
		daysInPeriod = 180
	case "annual":
		daysInPeriod = 365
	}

	if daysInPeriod > 0 {
		budgetOverview.DailyRate = budgetOverview.CombinedExpense / float64(daysInPeriod)
	}

	// Determine high spending warning
	// For example, if spent more than 50% of budget in first third of period
	currentDay := time.Now().Day()
	if period == "monthly" && currentDay <= 10 && budgetOverview.ExpensePercent > 50 {
		budgetOverview.HighSpending = true
	}

	return budgetOverview, nil
}

func fetchSavingsOverview(userID string) (SavingsOverview, error) {
	var savingsOverview SavingsOverview

	// Query savings data from database
	err := db.QueryRow(`
		SELECT available, goal, period, percent
		FROM savings
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&savingsOverview.Available,
		&savingsOverview.Goal,
		&savingsOverview.Period,
		&savingsOverview.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		savingsOverview.Available = 0
		savingsOverview.Goal = 0
		savingsOverview.Period = "monthly" // Default period
		savingsOverview.Percent = 0
		return savingsOverview, nil
	} else if err != nil {
		return savingsOverview, err
	}

	// Calculate need to save and daily target
	savingsOverview.NeedToSave = savingsOverview.Goal - savingsOverview.Available
	if savingsOverview.NeedToSave < 0 {
		savingsOverview.NeedToSave = 0
	}

	// Assuming goal needs to be achieved within a month (30 days)
	savingsOverview.DailyTarget = savingsOverview.NeedToSave / 30

	return savingsOverview, nil
}

func fetchCashBankDistribution(userID string) (CashBank, error) {
	var cashBank CashBank

	// Query cash_bank data from database
	err := db.QueryRow(`
		SELECT month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total
		FROM cash_bank
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&cashBank.Month,
		&cashBank.CashAmount,
		&cashBank.CashPercent,
		&cashBank.BankAmount,
		&cashBank.BankPercent,
		&cashBank.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		cashBank.Month = time.Now().Format("January 2006")
		cashBank.CashAmount = 0
		cashBank.CashPercent = 0
		cashBank.BankAmount = 0
		cashBank.BankPercent = 0
		cashBank.MonthlyTotal = 0
		return cashBank, nil
	} else if err != nil {
		return cashBank, err
	}

	return cashBank, nil
}

func fetchFinanceMetrics(userID, period string) (FinanceMetrics, error) {
	var financeMetrics FinanceMetrics

	// Query finance_metrics data from database
	err := db.QueryRow(`
		SELECT income, expenses, bills
		FROM finance_metrics
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&financeMetrics.Income,
		&financeMetrics.Expenses,
		&financeMetrics.Bills,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		financeMetrics.Income = 0
		financeMetrics.Expenses = 0
		financeMetrics.Bills = 0
		return financeMetrics, nil
	} else if err != nil {
		return financeMetrics, err
	}

	return financeMetrics, nil
}

func fetchUpcomingBills(userID string) ([]Bill, error) {
	var bills []Bill

	// Get the current date
	currentDate := time.Now().Format("2006-01-02")

	// Query bills that are not paid and due in the future, or recurring bills (but still not paid)
	rows, err := db.Query(`
		SELECT id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon
		FROM bills
		WHERE user_id = ? AND ((due_date >= ? AND paid = 0) OR recurring = 1) AND paid = 0
		ORDER BY due_date ASC
		LIMIT 10
	`, userID, currentDate)

	if err != nil {
		return bills, err
	}
	defer rows.Close()

	// Iterate through rows and append to bills slice
	for rows.Next() {
		var bill Bill
		err := rows.Scan(
			&bill.ID,
			&bill.Name,
			&bill.Amount,
			&bill.DueDate,
			&bill.Paid,
			&bill.Overdue,
			&bill.OverdueDays,
			&bill.Recurring,
			&bill.Category,
			&bill.Icon,
		)
		if err != nil {
			return bills, err
		}
		bills = append(bills, bill)
	}

	return bills, nil
}
