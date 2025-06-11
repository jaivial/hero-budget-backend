package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// BudgetOverview represents the complete budget overview response
type BudgetOverview struct {
	RemainingAmount      float64              `json:"remaining_amount"`
	ExpensePercent       float64              `json:"expense_percent"`
	SpentAmount          float64              `json:"spent_amount"`
	UpcomingAmount       float64              `json:"upcoming_amount"`
	TotalAmount          float64              `json:"total_amount"`
	TotalBalance         float64              `json:"total_balance"`
	CombinedExpense      float64              `json:"combined_expense"`
	TotalIncome          float64              `json:"total_income"`
	DailyRate            float64              `json:"daily_rate"`
	HighSpending         bool                 `json:"high_spending"`
	IsNegativeBalance    bool                 `json:"is_negative_balance"`
	MoneyFlow            MoneyFlow            `json:"money_flow"`
	CashBankDistribution CashBankDistribution `json:"cash_bank_distribution"`
	SavingsData          SavingsData          `json:"savings_data"`
	AvailableBalance     float64              `json:"available_balance"`
}

// MoneyFlow represents money flow from previous period
type MoneyFlow struct {
	FromPrevious float64 `json:"from_previous"`
}

// BudgetOverviewRequest represents the request structure
type BudgetOverviewRequest struct {
	UserID    string `json:"user_id"`
	Period    string `json:"period"`               // daily, weekly, monthly, quarterly, semiannual, annual
	Date      string `json:"date"`                 // Format depends on period type
	StartDate string `json:"start_date,omitempty"` // For custom periods
	EndDate   string `json:"end_date,omitempty"`   // For custom periods
}

// ApiResponse represents the standard API response
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// BalanceData represents the balance data from database
type BalanceData struct {
	IncomeBankAmount     float64 `json:"income_bank_amount"`
	IncomeCashAmount     float64 `json:"income_cash_amount"`
	ExpenseBankAmount    float64 `json:"expense_bank_amount"`
	ExpenseCashAmount    float64 `json:"expense_cash_amount"`
	BillBankAmount       float64 `json:"bill_bank_amount"`
	BillCashAmount       float64 `json:"bill_cash_amount"`
	BankAmount           float64 `json:"bank_amount"`
	PreviousBankAmount   float64 `json:"previous_bank_amount"`
	CashAmount           float64 `json:"cash_amount"`
	PreviousCashAmount   float64 `json:"previous_cash_amount"`
	BalanceCashAmount    float64 `json:"balance_cash_amount"`
	BalanceBankAmount    float64 `json:"balance_bank_amount"`
	TotalPreviousBalance float64 `json:"total_previous_balance"`
	TotalBalance         float64 `json:"total_balance"`
}

// CashBankDistribution represents the cash and bank distribution
type CashBankDistribution struct {
	CashAmount  float64 `json:"cash_amount"`
	CashPercent float64 `json:"cash_percent"`
	BankAmount  float64 `json:"bank_amount"`
	BankPercent float64 `json:"bank_percent"`
	TotalAmount float64 `json:"total_amount"`
}

// SavingsData represents savings information
type SavingsData struct {
	Available   float64 `json:"available"`
	Goal        float64 `json:"goal"`
	Period      string  `json:"period"` // New field for period type
	Percent     float64 `json:"percent"`
	NeedToSave  float64 `json:"need_to_save"`
	DailyTarget float64 `json:"daily_target"`
}

// Transaction represents a unified transaction (income, expense, or bill)
type Transaction struct {
	ID            int     `json:"id"`
	Type          string  `json:"type"` // "income", "expense", "bill"
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description,omitempty"`
	Name          string  `json:"name,omitempty"`         // For bills
	Paid          *bool   `json:"paid,omitempty"`         // For bills (pointer to handle null)
	Overdue       *bool   `json:"overdue,omitempty"`      // For bills (pointer to handle null)
	OverdueDays   *int    `json:"overdue_days,omitempty"` // For bills (pointer to handle null)
	Recurring     *bool   `json:"recurring,omitempty"`    // For bills (pointer to handle null)
	Icon          string  `json:"icon,omitempty"`         // For bills
	PaymentDay    *int    `json:"payment_day,omitempty"`  // For bills (pointer to handle null)
}

// TransactionRequest represents the request structure for transaction queries
type TransactionRequest struct {
	UserID           string   `json:"user_id"`
	Period           string   `json:"period,omitempty"`            // daily, weekly, monthly, quarterly, semiannual, annual
	Date             string   `json:"date,omitempty"`              // Format depends on period type
	StartDate        string   `json:"start_date,omitempty"`        // YYYY-MM-DD
	EndDate          string   `json:"end_date,omitempty"`          // YYYY-MM-DD
	TransactionTypes []string `json:"transaction_types,omitempty"` // ["income", "expense", "bill"]
	PaymentMethods   []string `json:"payment_methods,omitempty"`   // ["cash", "bank"]
	Limit            int      `json:"limit,omitempty"`             // For pagination (default: 100)
	Offset           int      `json:"offset,omitempty"`            // For pagination (default: 0)
}

// TransactionHistoryResponse represents the response for transaction history
type TransactionHistoryResponse struct {
	Transactions []Transaction `json:"transactions"`
	Total        int           `json:"total"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	Period       string        `json:"period,omitempty"`
	StartDate    string        `json:"start_date,omitempty"`
	EndDate      string        `json:"end_date,omitempty"`
}

// UpcomingBillsResponse represents the response for upcoming bills
type UpcomingBillsResponse struct {
	Bills     []Transaction `json:"bills"`
	Total     int           `json:"total"`
	Overdue   int           `json:"overdue"`
	Upcoming  int           `json:"upcoming"`
	ThisWeek  int           `json:"this_week"`
	ThisMonth int           `json:"this_month"`
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
	// Set up HTTP routes
	http.HandleFunc("/budget-overview", corsMiddleware(handleBudgetOverview))
	http.HandleFunc("/transactions/history", corsMiddleware(handleTransactionHistory))
	http.HandleFunc("/transactions/upcoming-bills", corsMiddleware(handleUpcomingBills))
	http.HandleFunc("/upcoming-bills", corsMiddleware(handleUpcomingBills))
	http.HandleFunc("/health", corsMiddleware(handleHealth))
	http.HandleFunc("/budget-overview/health", corsMiddleware(handleBudgetOverviewHealth))

	// Start server on port 8098
	port := "8098"
	log.Printf("Budget Overview Fetch service starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// corsMiddleware adds CORS headers to allow cross-origin requests
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

// handleHealth provides a health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sendSuccessResponse(w, "Service is healthy", map[string]string{
		"service": "budget_overview_fetch",
		"status":  "active",
		"port":    "8098",
	})
}

// handleBudgetOverview handles the budget overview requests
func handleBudgetOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request BudgetOverviewRequest

	// Handle both GET and POST methods
	if r.Method == http.MethodGet {
		// Get parameters from query string
		request.UserID = r.URL.Query().Get("user_id")
		request.Period = r.URL.Query().Get("period")
		request.Date = r.URL.Query().Get("date")
		request.StartDate = r.URL.Query().Get("start_date")
		request.EndDate = r.URL.Query().Get("end_date")
	} else {
		// POST method - decode JSON body
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			log.Printf("Error decoding request: %v", err)
			sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
			return
		}
	}

	// Validate required fields
	if request.UserID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if request.Period == "" {
		request.Period = "monthly" // Default to monthly
	}

	if request.Date == "" {
		// Default to current date/period
		request.Date = formatDateForPeriod(time.Now(), request.Period)
	}

	// Fetch budget overview data
	overview, err := fetchBudgetOverview(request)
	if err != nil {
		log.Printf("Error fetching budget overview: %v", err)
		sendErrorResponse(w, "Failed to fetch budget overview", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Budget overview fetched successfully", overview)
}

// fetchBudgetOverview retrieves budget overview data for the specified period
func fetchBudgetOverview(request BudgetOverviewRequest) (*BudgetOverview, error) {
	// Get the appropriate table name and date condition
	tableName, dateCondition := getTableAndCondition(request.Period, request.Date)

	// Fetch balance data from the appropriate table
	balanceData, err := fetchBalanceData(tableName, request.UserID, dateCondition)
	if err != nil {
		log.Printf("Error fetching balance data: %v", err)
		return nil, fmt.Errorf("failed to fetch balance data: %v", err)
	}

	// Calculate budget overview from balance data, passing the date
	overview := calculateBudgetOverview(balanceData, request.Period, request.Date, request.UserID)

	return overview, nil
}

// getTableAndCondition returns the table name and WHERE condition for the given period
func getTableAndCondition(period, date string) (string, string) {
	switch period {
	case "daily":
		return "daily_cash_bank_balance", fmt.Sprintf("date = '%s'", date)
	case "weekly":
		return "weekly_cash_bank_balance", fmt.Sprintf("year_week = '%s'", date)
	case "monthly":
		return "monthly_cash_bank_balance", fmt.Sprintf("year_month = '%s'", date)
	case "quarterly":
		return "quarterly_cash_bank_balance", fmt.Sprintf("year_quarter = '%s'", date)
	case "semiannual":
		return "semiannual_cash_bank_balance", fmt.Sprintf("year_half = '%s'", date)
	case "annual":
		return "annual_cash_bank_balance", fmt.Sprintf("year = '%s'", date)
	default:
		// Default to monthly
		return "monthly_cash_bank_balance", fmt.Sprintf("year_month = '%s'", date)
	}
}

// fetchBalanceData retrieves balance data from the specified table with data inheritance
func fetchBalanceData(tableName, userID, condition string) (*BalanceData, error) {
	query := fmt.Sprintf(`
		SELECT 
			COALESCE(income_bank_amount, 0) as income_bank_amount,
			COALESCE(income_cash_amount, 0) as income_cash_amount,
			COALESCE(expense_bank_amount, 0) as expense_bank_amount,
			COALESCE(expense_cash_amount, 0) as expense_cash_amount,
			COALESCE(bill_bank_amount, 0) as bill_bank_amount,
			COALESCE(bill_cash_amount, 0) as bill_cash_amount,
			COALESCE(bank_amount, 0) as bank_amount,
			COALESCE(previous_bank_amount, 0) as previous_bank_amount,
			COALESCE(cash_amount, 0) as cash_amount,
			COALESCE(previous_cash_amount, 0) as previous_cash_amount,
			COALESCE(balance_cash_amount, 0) as balance_cash_amount,
			COALESCE(balance_bank_amount, 0) as balance_bank_amount,
			COALESCE(total_previous_balance, 0) as total_previous_balance,
			COALESCE(total_balance, 0) as total_balance
		FROM %s 
		WHERE user_id = ? AND %s
	`, tableName, condition)

	row := db.QueryRow(query, userID)

	var data BalanceData
	err := row.Scan(
		&data.IncomeBankAmount,
		&data.IncomeCashAmount,
		&data.ExpenseBankAmount,
		&data.ExpenseCashAmount,
		&data.BillBankAmount,
		&data.BillCashAmount,
		&data.BankAmount,
		&data.PreviousBankAmount,
		&data.CashAmount,
		&data.PreviousCashAmount,
		&data.BalanceCashAmount,
		&data.BalanceBankAmount,
		&data.TotalPreviousBalance,
		&data.TotalBalance,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// No data found for the requested period, try to inherit from previous periods
			log.Printf("🔍 No data found for current period, searching for historical data to inherit...")
			return fetchBalanceDataWithInheritance(tableName, userID, condition)
		}
		return nil, err
	}

	log.Printf("📊 Balance data found: IncomeBank=%.2f, IncomeCash=%.2f, ExpenseBank=%.2f, ExpenseCash=%.2f, BillBank=%.2f, BillCash=%.2f, TotalBalance=%.2f",
		data.IncomeBankAmount, data.IncomeCashAmount, data.ExpenseBankAmount, data.ExpenseCashAmount,
		data.BillBankAmount, data.BillCashAmount, data.TotalBalance)

	return &data, nil
}

// fetchBalanceDataWithInheritance handles data inheritance when no data exists for the requested period
func fetchBalanceDataWithInheritance(tableName, userID, condition string) (*BalanceData, error) {
	// Extract period and date from the condition to search backwards
	period, date := extractPeriodAndDateFromCondition(tableName, condition)

	if period == "" || date == "" {
		log.Printf("⚠️  Could not extract period/date from condition: %s", condition)
		return &BalanceData{}, nil
	}

	// Search for the last available period with data
	inheritedData, err := findLastAvailablePeriod(tableName, userID, date, period)
	if err != nil {
		log.Printf("Error searching for historical data: %v", err)
		return &BalanceData{}, nil
	}

	return inheritedData, nil
}

// extractPeriodAndDateFromCondition extracts period type and date from table name and condition
func extractPeriodAndDateFromCondition(tableName, condition string) (string, string) {
	// Determine period from table name
	var period string
	switch tableName {
	case "daily_cash_bank_balance":
		period = "daily"
	case "weekly_cash_bank_balance":
		period = "weekly"
	case "monthly_cash_bank_balance":
		period = "monthly"
	case "quarterly_cash_bank_balance":
		period = "quarterly"
	case "semiannual_cash_bank_balance":
		period = "semiannual"
	case "annual_cash_bank_balance":
		period = "annual"
	default:
		return "", ""
	}

	// Extract date from condition using string manipulation
	// Conditions are in format: "date = '2024-01-15'" or "year_month = '2024-01'" etc.
	parts := strings.Split(condition, "'")
	if len(parts) >= 2 {
		return period, parts[1]
	}

	return "", ""
}

// calculateBudgetOverview calculates the budget overview from balance data
func calculateBudgetOverview(data *BalanceData, period, date, userID string) *BudgetOverview {
	// Calculate income from separate cash and bank income amounts
	totalIncome := data.IncomeBankAmount + data.IncomeCashAmount

	// Calculate spent amount from actual expenses only (not including bills)
	spentAmount := data.ExpenseBankAmount + data.ExpenseCashAmount

	// Calculate combined expense including both expenses and bills
	combinedExpense := data.ExpenseBankAmount + data.ExpenseCashAmount + data.BillBankAmount + data.BillCashAmount

	// Calculate available balance
	availableBalance := totalIncome - combinedExpense

	// Calculate upcoming bills separately for clarity (bills that haven't been paid yet)
	upcomingAmount := data.BillBankAmount + data.BillCashAmount

	// Log the calculation breakdown for transparency
	log.Printf("🧮 Budget calculation breakdown for period %s, date %s:", period, date)
	log.Printf("   💰 Total Income: %.2f (Bank: %.2f + Cash: %.2f)",
		totalIncome, data.IncomeBankAmount, data.IncomeCashAmount)
	log.Printf("   💸 Spent Amount (expenses only): %.2f (Bank: %.2f + Cash: %.2f)",
		spentAmount, data.ExpenseBankAmount, data.ExpenseCashAmount)
	log.Printf("   🏷️ Bills Amount: %.2f (Bank: %.2f + Cash: %.2f)",
		data.BillBankAmount+data.BillCashAmount, data.BillBankAmount, data.BillCashAmount)
	log.Printf("   📊 Combined Expense (expenses + bills): %.2f", combinedExpense)
	log.Printf("   💵 Available Balance: %.2f (Income: %.2f - Combined Expenses: %.2f)",
		availableBalance, totalIncome, combinedExpense)
	log.Printf("   📋 Upcoming Bills: %.2f", upcomingAmount)

	// Calculate remaining amount (should show real balance, including negative values)
	remainingAmount := availableBalance

	// Calculate expense percentage
	var expensePercent float64
	if totalIncome > 0 {
		expensePercent = (combinedExpense / totalIncome) * 100
		if expensePercent > 100 {
			expensePercent = 100
		}
	}

	// Calculate total amount (could be total income or total available including previous balance)
	totalAmount := totalIncome + data.TotalPreviousBalance

	// Use the total balance from balance data
	totalBalance := data.TotalBalance

	// Calculate daily rate
	dailyRate := calculateDailyRate(spentAmount, period)

	// Determine if spending is high (more than 80% of income)
	highSpending := expensePercent > 80

	// Determine if balance is negative
	isNegativeBalance := availableBalance < 0

	// Calculate money flow from previous period
	moneyFlow := MoneyFlow{
		FromPrevious: data.TotalPreviousBalance,
	}

	// Calculate cash/bank distribution based on current balances
	cashBankDistribution := calculateCashBankDistribution(data)

	// Get savings data
	savingsData := getSavingsDataFromDB(userID, remainingAmount, period)

	return &BudgetOverview{
		RemainingAmount:      remainingAmount,
		ExpensePercent:       expensePercent,
		SpentAmount:          spentAmount,
		UpcomingAmount:       upcomingAmount,
		TotalAmount:          totalAmount,
		TotalBalance:         totalBalance,
		CombinedExpense:      combinedExpense,
		TotalIncome:          totalIncome,
		DailyRate:            dailyRate,
		HighSpending:         highSpending,
		IsNegativeBalance:    isNegativeBalance,
		MoneyFlow:            moneyFlow,
		CashBankDistribution: cashBankDistribution,
		SavingsData:          savingsData,
		AvailableBalance:     availableBalance,
	}
}

// calculateCashBankDistribution calculates the distribution between cash and bank amounts
func calculateCashBankDistribution(data *BalanceData) CashBankDistribution {
	// Calculate total available amounts (balance amounts represent current available funds)
	totalCashAmount := data.BalanceCashAmount
	totalBankAmount := data.BalanceBankAmount
	totalAmount := totalCashAmount + totalBankAmount

	var cashPercent, bankPercent float64

	if totalAmount > 0 {
		cashPercent = (totalCashAmount / totalAmount) * 100
		bankPercent = (totalBankAmount / totalAmount) * 100
	}

	log.Printf("💳 Cash/Bank Distribution: Cash=%.2f (%.1f%%), Bank=%.2f (%.1f%%), Total=%.2f",
		totalCashAmount, cashPercent, totalBankAmount, bankPercent, totalAmount)

	return CashBankDistribution{
		CashAmount:  totalCashAmount,
		CashPercent: cashPercent,
		BankAmount:  totalBankAmount,
		BankPercent: bankPercent,
		TotalAmount: totalAmount,
	}
}

// calculateDailyRate calculates the daily spending rate based on the period
func calculateDailyRate(spentAmount float64, period string) float64 {
	var days float64

	switch period {
	case "daily":
		days = 1
	case "weekly":
		days = 7
	case "monthly":
		days = 30 // Average month
	case "quarterly":
		days = 90 // 3 months
	case "semiannual":
		days = 180 // 6 months
	case "annual":
		days = 365
	default:
		days = 30 // Default to monthly
	}

	if days > 0 {
		return spentAmount / days
	}

	return 0
}

// formatDateForPeriod formats the current date according to the period type
func formatDateForPeriod(date time.Time, period string) string {
	switch period {
	case "daily":
		return date.Format("2006-01-02")
	case "weekly":
		year, week := date.ISOWeek()
		// Return format without 'W' to match database format (e.g., "2025-22")
		return fmt.Sprintf("%d-%02d", year, week)
	case "monthly":
		return date.Format("2006-01")
	case "quarterly":
		quarter := ((date.Month() - 1) / 3) + 1
		return fmt.Sprintf("%d-Q%d", date.Year(), quarter)
	case "semiannual":
		half := 1
		if date.Month() > 6 {
			half = 2
		}
		return fmt.Sprintf("%d-H%d", date.Year(), half)
	case "annual":
		return strconv.Itoa(date.Year())
	default:
		return date.Format("2006-01")
	}
}

// sendSuccessResponse sends a successful JSON response
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends an error JSON response
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ApiResponse{
		Success: false,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}

// getSavingsDataFromDB retrieves savings data from the database
func getSavingsDataFromDB(userID string, remainingAmount float64, period string) SavingsData {
	// First try to get existing savings goal from database
	var goal float64
	var goalPeriod string

	query := `SELECT goal, period FROM savings WHERE user_id = ? LIMIT 1`
	row := db.QueryRow(query, userID)

	err := row.Scan(&goal, &goalPeriod)
	if err != nil {
		// No savings goal found in database, return zero values
		fmt.Printf("No savings goal found for user %s: %v\n", userID, err)
		return SavingsData{
			Available:   remainingAmount,
			Goal:        0,
			Period:      period,
			Percent:     0,
			NeedToSave:  0,
			DailyTarget: 0,
		}
	}

	// Calculate percentage of goal achieved
	var savingsPercent float64
	if goal > 0 {
		savingsPercent = (remainingAmount / goal) * 100
		if savingsPercent > 100 {
			savingsPercent = 100
		}
	}

	// Calculate need to save and daily target
	needToSave := goal - remainingAmount
	if needToSave < 0 {
		needToSave = 0
	}

	// Calculate daily target based on period
	var dailyTarget float64
	var periodDays float64

	switch period {
	case "daily":
		periodDays = 1
	case "weekly":
		periodDays = 7
	case "monthly":
		periodDays = 30 // Average month
	case "quarterly":
		periodDays = 90 // 3 months
	case "semiannual":
		periodDays = 180 // 6 months
	case "annual":
		periodDays = 365
	default:
		periodDays = 30 // Default to monthly
	}

	if periodDays > 0 && needToSave > 0 {
		dailyTarget = needToSave / periodDays
	}

	return SavingsData{
		Available:   remainingAmount,
		Goal:        goal,
		Period:      goalPeriod, // Use the period from the database
		Percent:     savingsPercent,
		NeedToSave:  needToSave,
		DailyTarget: dailyTarget,
	}
}

// parseDateString parses a date string based on the period type and returns a time.Time
func parseDateString(dateStr, period string) (time.Time, error) {
	switch period {
	case "daily":
		return time.Parse("2006-01-02", dateStr)
	case "weekly":
		// Handle both formats: "2024-W03" and "2024-03"
		// The database stores the format without 'W' (e.g., "2024-03")
		var parts []string
		if strings.Contains(dateStr, "-W") {
			parts = strings.Split(dateStr, "-W")
		} else {
			// Already in the database format "2025-22"
			parts = strings.Split(dateStr, "-")
		}

		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("invalid weekly date format: %s", dateStr)
		}

		year, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid year in weekly date: %s", parts[0])
		}
		week, err := strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid week in weekly date: %s", parts[1])
		}

		// Calculate first day of week (Monday) using proper ISO week calculation
		// Find the first Thursday of the year (it's always in week 1)
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)

		// Find Monday of week 1 (the week containing January 4th)
		mondayOfWeek1 := jan4.AddDate(0, 0, -(int(jan4.Weekday()) - 1))
		if jan4.Weekday() == time.Sunday {
			mondayOfWeek1 = jan4.AddDate(0, 0, -6)
		}

		// Calculate the target week's Monday
		targetWeekMonday := mondayOfWeek1.AddDate(0, 0, (week-1)*7)

		return targetWeekMonday, nil
	case "monthly":
		return time.Parse("2006-01-02", dateStr+"-01")
	case "quarterly":
		// Parse format like "2024-Q1"
		parts := strings.Split(dateStr, "-Q")
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("invalid quarterly date format: %s", dateStr)
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid year in quarterly date: %s", parts[0])
		}
		quarter, err := strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid quarter in quarterly date: %s", parts[1])
		}
		month := (quarter-1)*3 + 1
		return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
	case "semiannual":
		// Parse format like "2024-H1"
		parts := strings.Split(dateStr, "-H")
		if len(parts) != 2 {
			return time.Time{}, fmt.Errorf("invalid semiannual date format: %s", dateStr)
		}
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid year in semiannual date: %s", parts[0])
		}
		half, err := strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid half in semiannual date: %s", parts[1])
		}
		month := 1
		if half == 2 {
			month = 7
		}
		return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
	case "annual":
		year, err := strconv.Atoi(dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid annual date format: %s", dateStr)
		}
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), nil
	default:
		return time.Parse("2006-01-02", dateStr+"-01")
	}
}

// getPreviousPeriodDate calculates the previous period date string
func getPreviousPeriodDate(dateStr, period string) (string, error) {
	currentDate, err := parseDateString(dateStr, period)
	if err != nil {
		return "", err
	}

	var previousDate time.Time
	switch period {
	case "daily":
		previousDate = currentDate.AddDate(0, 0, -1)
	case "weekly":
		previousDate = currentDate.AddDate(0, 0, -7)
	case "monthly":
		previousDate = currentDate.AddDate(0, -1, 0)
	case "quarterly":
		previousDate = currentDate.AddDate(0, -3, 0)
	case "semiannual":
		previousDate = currentDate.AddDate(0, -6, 0)
	case "annual":
		previousDate = currentDate.AddDate(-1, 0, 0)
	default:
		previousDate = currentDate.AddDate(0, -1, 0)
	}

	return formatDateForPeriod(previousDate, period), nil
}

// findLastAvailablePeriod searches backwards for the most recent period with data
func findLastAvailablePeriod(tableName, userID, originalDate, period string) (*BalanceData, error) {
	const maxSearchPeriods = 24 // Limit search to avoid infinite loops

	currentDate := originalDate

	for i := 0; i < maxSearchPeriods; i++ {
		// Get previous period date
		previousDate, err := getPreviousPeriodDate(currentDate, period)
		if err != nil {
			log.Printf("Error calculating previous period date: %v", err)
			break
		}

		// Get table condition for the previous period
		_, condition := getTableAndCondition(period, previousDate)

		// Fetch all balance data needed for inheritance calculation
		query := fmt.Sprintf(`
			SELECT 
				COALESCE(total_previous_balance, 0) as total_previous_balance,
				COALESCE(total_balance, 0) as total_balance,
				COALESCE(income_bank_amount, 0) as income_bank_amount,
				COALESCE(income_cash_amount, 0) as income_cash_amount,
				COALESCE(expense_bank_amount, 0) as expense_bank_amount,
				COALESCE(expense_cash_amount, 0) as expense_cash_amount,
				COALESCE(bill_bank_amount, 0) as bill_bank_amount,
				COALESCE(bill_cash_amount, 0) as bill_cash_amount
			FROM %s 
			WHERE user_id = ? AND %s
		`, tableName, condition)

		row := db.QueryRow(query, userID)

		var totalPreviousBalance, totalBalance, incomeBankAmount, incomeCashAmount float64
		var expenseBankAmount, expenseCashAmount, billBankAmount, billCashAmount float64
		err = row.Scan(&totalPreviousBalance, &totalBalance, &incomeBankAmount, &incomeCashAmount,
			&expenseBankAmount, &expenseCashAmount, &billBankAmount, &billCashAmount)

		if err == nil {
			// Found data! Use the total_balance from the last available period
			// For future periods without data, inherit the exact total_balance from the last available period
			inheritedTotalBalance := totalBalance

			// Create a clean BalanceData with only the inherited balance as both total_previous_balance and total_balance
			// All other fields remain 0 for the future period
			data := &BalanceData{
				IncomeBankAmount:     0,
				IncomeCashAmount:     0,
				ExpenseBankAmount:    0,
				ExpenseCashAmount:    0,
				BillBankAmount:       0,
				BillCashAmount:       0,
				BankAmount:           0,
				PreviousBankAmount:   0,
				CashAmount:           0,
				PreviousCashAmount:   0,
				BalanceCashAmount:    0,
				BalanceBankAmount:    0,
				TotalPreviousBalance: inheritedTotalBalance,
				TotalBalance:         inheritedTotalBalance, // Use the last available total_balance
			}

			log.Printf("📊 Balance inheritance: Using total_balance %.2f from %s as total_balance for requested period %s (user: %s)",
				inheritedTotalBalance, previousDate, originalDate, userID)
			return data, nil
		}

		if err != sql.ErrNoRows {
			// Some other error occurred
			log.Printf("Error querying period %s: %v", previousDate, err)
			break
		}

		// No data found for this period either, continue searching backwards
		currentDate = previousDate
	}

	// No data found in any of the searched periods
	log.Printf("⚠️  No historical data found for user %s in table %s after searching %d periods backwards from %s",
		userID, tableName, maxSearchPeriods, originalDate)
	return &BalanceData{}, nil
}

// handleTransactionHistory handles requests for transaction history
func handleTransactionHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request TransactionRequest

	// Handle both GET and POST methods
	if r.Method == http.MethodGet {
		// Get parameters from query string
		request.UserID = r.URL.Query().Get("user_id")
		request.Period = r.URL.Query().Get("period")
		request.Date = r.URL.Query().Get("date")
		request.StartDate = r.URL.Query().Get("start_date")
		request.EndDate = r.URL.Query().Get("end_date")

		// Parse arrays from comma-separated values
		if transactionTypes := r.URL.Query().Get("transaction_types"); transactionTypes != "" {
			request.TransactionTypes = strings.Split(transactionTypes, ",")
		}
		if paymentMethods := r.URL.Query().Get("payment_methods"); paymentMethods != "" {
			request.PaymentMethods = strings.Split(paymentMethods, ",")
		}

		// Parse integers
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				request.Limit = limit
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				request.Offset = offset
			}
		}
	} else {
		// POST method - decode JSON body
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			log.Printf("Error decoding transaction request: %v", err)
			sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
			return
		}
	}

	// Validate required fields
	if request.UserID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Set defaults
	if request.Limit <= 0 {
		request.Limit = 100
	}
	if request.Limit > 1000 {
		request.Limit = 1000 // Maximum limit
	}

	// Calculate date range if period is specified
	if request.Period != "" && request.StartDate == "" && request.EndDate == "" {
		startDate, endDate, err := calculatePeriodDateRangeWithBase(request.Period, request.Date)
		if err != nil {
			log.Printf("Error calculating period date range: %v", err)
			sendErrorResponse(w, "Invalid period specified", http.StatusBadRequest)
			return
		}
		request.StartDate = startDate
		request.EndDate = endDate
	}

	// Fetch transaction history
	response, err := fetchTransactionHistory(request)
	if err != nil {
		log.Printf("Error fetching transaction history: %v", err)
		sendErrorResponse(w, "Failed to fetch transaction history", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Transaction history fetched successfully", response)
}

// handleUpcomingBills handles requests for upcoming bills
func handleUpcomingBills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request TransactionRequest

	// Handle both GET and POST methods
	if r.Method == http.MethodGet {
		// Get parameters from query string
		request.UserID = r.URL.Query().Get("user_id")
		request.Period = r.URL.Query().Get("period")
		request.Date = r.URL.Query().Get("date")
		request.StartDate = r.URL.Query().Get("start_date")
		request.EndDate = r.URL.Query().Get("end_date")

		// Parse transaction types and payment methods if provided
		if transactionTypesStr := r.URL.Query().Get("transaction_types"); transactionTypesStr != "" {
			request.TransactionTypes = strings.Split(transactionTypesStr, ",")
		}
		if paymentMethodsStr := r.URL.Query().Get("payment_methods"); paymentMethodsStr != "" {
			request.PaymentMethods = strings.Split(paymentMethodsStr, ",")
		}

		// Parse limit and offset
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				request.Limit = limit
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				request.Offset = offset
			}
		}
	} else {
		// POST method - decode JSON body
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			log.Printf("Error decoding upcoming bills request: %v", err)
			sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
			return
		}
	}

	// Validate required fields
	if request.UserID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Calculate date range if period is specified
	if request.Period != "" && request.StartDate == "" && request.EndDate == "" {
		startDate, endDate, err := calculatePeriodDateRangeWithBase(request.Period, request.Date)
		if err != nil {
			log.Printf("Error calculating period date range: %v", err)
			sendErrorResponse(w, "Invalid period specified", http.StatusBadRequest)
			return
		}
		request.StartDate = startDate
		request.EndDate = endDate
	}

	// Fetch upcoming bills
	response, err := fetchUpcomingBills(request)
	if err != nil {
		log.Printf("Error fetching upcoming bills: %v", err)
		sendErrorResponse(w, "Failed to fetch upcoming bills", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Upcoming bills fetched successfully", response)
}

// calculatePeriodDateRange calculates start and end dates for a given period
func calculatePeriodDateRange(period string) (string, string, error) {
	return calculatePeriodDateRangeWithBase(period, "")
}

// calculatePeriodDateRangeWithBase calculates start and end dates for a given period with optional base date
func calculatePeriodDateRangeWithBase(period, baseDate string) (string, string, error) {
	var baseTime time.Time
	var err error

	// Use provided base date or current time
	if baseDate != "" {
		baseTime, err = parseDateString(baseDate, period)
		if err != nil {
			log.Printf("Error parsing base date %s for period %s: %v", baseDate, period, err)
			baseTime = time.Now()
		}
	} else {
		baseTime = time.Now()
	}

	var startDate, endDate time.Time

	switch period {
	case "daily":
		startDate = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), 0, 0, 0, 0, baseTime.Location())
		endDate = startDate.AddDate(0, 0, 1).Add(-time.Second)
	case "weekly":
		// Start of week (Monday) for the base date
		weekday := int(baseTime.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}
		startDate = baseTime.AddDate(0, 0, -(weekday - 1))
		startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
		endDate = startDate.AddDate(0, 0, 7).Add(-time.Second)
	case "monthly":
		startDate = time.Date(baseTime.Year(), baseTime.Month(), 1, 0, 0, 0, 0, baseTime.Location())
		endDate = startDate.AddDate(0, 1, 0).Add(-time.Second)
	case "quarterly":
		quarter := ((baseTime.Month() - 1) / 3) + 1
		startMonth := (quarter-1)*3 + 1
		startDate = time.Date(baseTime.Year(), time.Month(startMonth), 1, 0, 0, 0, 0, baseTime.Location())
		endDate = startDate.AddDate(0, 3, 0).Add(-time.Second)
	case "semiannual":
		var startMonth time.Month
		if baseTime.Month() <= 6 {
			startMonth = 1
		} else {
			startMonth = 7
		}
		startDate = time.Date(baseTime.Year(), startMonth, 1, 0, 0, 0, 0, baseTime.Location())
		endDate = startDate.AddDate(0, 6, 0).Add(-time.Second)
	case "annual":
		startDate = time.Date(baseTime.Year(), 1, 1, 0, 0, 0, 0, baseTime.Location())
		endDate = startDate.AddDate(1, 0, 0).Add(-time.Second)
	default:
		return "", "", fmt.Errorf("invalid period: %s", period)
	}

	return startDate.Format("2006-01-02"), endDate.Format("2006-01-02"), nil
}

// fetchTransactionHistory retrieves transaction history from the database
func fetchTransactionHistory(request TransactionRequest) (*TransactionHistoryResponse, error) {
	var transactions []Transaction
	var totalCount int

	// Build the WHERE clause for filtering
	whereConditions := []string{"user_id = ?"}
	args := []interface{}{request.UserID}

	// Add date range filter
	if request.StartDate != "" && request.EndDate != "" {
		whereConditions = append(whereConditions, "date BETWEEN ? AND ?")
		args = append(args, request.StartDate, request.EndDate)
	} else if request.StartDate != "" {
		whereConditions = append(whereConditions, "date >= ?")
		args = append(args, request.StartDate)
	} else if request.EndDate != "" {
		whereConditions = append(whereConditions, "date <= ?")
		args = append(args, request.EndDate)
	}

	// Build payment method filter
	var paymentMethodFilter string
	if len(request.PaymentMethods) > 0 {
		placeholders := make([]string, len(request.PaymentMethods))
		for i, method := range request.PaymentMethods {
			placeholders[i] = "?"
			args = append(args, method)
		}
		paymentMethodFilter = fmt.Sprintf("payment_method IN (%s)", strings.Join(placeholders, ","))
	}

	// Build transaction type filter - Only include incomes and expenses, no bills
	var queries []string
	includeIncomes := len(request.TransactionTypes) == 0 || contains(request.TransactionTypes, "income")
	includeExpenses := len(request.TransactionTypes) == 0 || contains(request.TransactionTypes, "expense")
	// Bills are excluded from transaction history - they are handled separately in upcoming bills

	// Income query
	if includeIncomes {
		incomeWhere := strings.Join(whereConditions, " AND ")
		if paymentMethodFilter != "" {
			incomeWhere += " AND " + paymentMethodFilter
		}

		incomeQuery := fmt.Sprintf(`
			SELECT 
				id, 'income' as type, amount, date, category, payment_method, description,
				NULL as name, NULL as paid, NULL as overdue, NULL as overdue_days,
				NULL as recurring, NULL as icon
			FROM incomes 
			WHERE %s`, incomeWhere)
		queries = append(queries, incomeQuery)
	}

	// Expense query
	if includeExpenses {
		expenseWhere := strings.Join(whereConditions, " AND ")
		if paymentMethodFilter != "" {
			expenseWhere += " AND " + paymentMethodFilter
		}

		expenseQuery := fmt.Sprintf(`
			SELECT 
				id, 'expense' as type, amount, date, category, payment_method, description,
				NULL as name, NULL as paid, NULL as overdue, NULL as overdue_days,
				NULL as recurring, NULL as icon
			FROM expenses 
			WHERE %s`, expenseWhere)
		queries = append(queries, expenseQuery)
	}

	// Note: Bills are intentionally excluded from transaction history
	// Bills represent future obligations and appear only in the upcoming bills endpoint
	// When bills are paid, they create expense records which appear in this history

	if len(queries) == 0 {
		return &TransactionHistoryResponse{
			Transactions: []Transaction{},
			Total:        0,
			Limit:        request.Limit,
			Offset:       request.Offset,
			StartDate:    request.StartDate,
			EndDate:      request.EndDate,
			Period:       request.Period,
		}, nil
	}

	// Combine queries with UNION ALL
	unionQuery := strings.Join(queries, " UNION ALL ")
	finalQuery := fmt.Sprintf(`
		SELECT * FROM (%s) 
		ORDER BY date DESC 
		LIMIT ? OFFSET ?`, unionQuery)

	// Add limit and offset to args
	finalArgs := make([]interface{}, 0)
	for range queries {
		finalArgs = append(finalArgs, args...)
	}
	finalArgs = append(finalArgs, request.Limit, request.Offset)

	// Execute query
	rows, err := db.Query(finalQuery, finalArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %v", err)
	}
	defer rows.Close()

	// Scan results
	for rows.Next() {
		var t Transaction
		var paid, overdue, recurring sql.NullBool
		var overdueDays sql.NullInt64
		var name, description, icon sql.NullString

		err := rows.Scan(
			&t.ID, &t.Type, &t.Amount, &t.Date, &t.Category, &t.PaymentMethod,
			&description, &name, &paid, &overdue, &overdueDays, &recurring, &icon,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %v", err)
		}

		// Handle nullable fields
		if description.Valid {
			t.Description = description.String
		}
		if name.Valid {
			t.Name = name.String
		}
		if icon.Valid {
			t.Icon = icon.String
		}
		if paid.Valid {
			t.Paid = &paid.Bool
		}
		if overdue.Valid {
			t.Overdue = &overdue.Bool
		}
		if recurring.Valid {
			t.Recurring = &recurring.Bool
		}
		if overdueDays.Valid {
			days := int(overdueDays.Int64)
			t.OverdueDays = &days
		}

		transactions = append(transactions, t)
	}

	// Get total count (without limit/offset)
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s)", unionQuery)
	countArgs := make([]interface{}, 0)
	for range queries {
		countArgs = append(countArgs, args...)
	}

	err = db.QueryRow(countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		log.Printf("Warning: failed to get total count: %v", err)
		totalCount = len(transactions)
	}

	log.Printf("📊 Transaction history retrieved: %d transactions (incomes + expenses only, bills excluded)", totalCount)

	return &TransactionHistoryResponse{
		Transactions: transactions,
		Total:        totalCount,
		Limit:        request.Limit,
		Offset:       request.Offset,
		StartDate:    request.StartDate,
		EndDate:      request.EndDate,
		Period:       request.Period,
	}, nil
}

// fetchUpcomingBills retrieves all bills with payment status for specific month from bill_payments
func fetchUpcomingBills(request TransactionRequest) (*UpcomingBillsResponse, error) {
	// Extract year-month from the date range for bill_payments lookup
	var targetYearMonth string
	if request.StartDate != "" {
		if len(request.StartDate) >= 7 {
			targetYearMonth = request.StartDate[:7] // Extract YYYY-MM from YYYY-MM-DD
		}
	}

	// Build the WHERE clause for filtering
	whereConditions := []string{"b.user_id = ?"}
	args := []interface{}{request.UserID}

	// Add date range filter - Support for recurring bills with duration
	if request.StartDate != "" && request.EndDate != "" {
		// For recurring bills, check if target month falls within the bill's duration
		whereConditions = append(whereConditions, `(
			(b.recurring = 1 AND 
			 strftime('%Y-%m', b.start_date) <= ? AND 
			 strftime('%Y-%m', date(b.start_date, '+' || (b.duration_months - 1) || ' months')) >= ?) OR
			(b.recurring = 0 AND strftime('%Y-%m', b.due_date) = ?)
		)`)
		args = append(args, targetYearMonth, targetYearMonth, targetYearMonth)
	} else if targetYearMonth != "" {
		// For recurring bills, check if target month falls within the bill's duration
		whereConditions = append(whereConditions, `(
			(b.recurring = 1 AND 
			 strftime('%Y-%m', b.start_date) <= ? AND 
			 strftime('%Y-%m', date(b.start_date, '+' || (b.duration_months - 1) || ' months')) >= ?) OR
			(b.recurring = 0 AND strftime('%Y-%m', b.due_date) = ?)
		)`)
		args = append(args, targetYearMonth, targetYearMonth, targetYearMonth)
	}

	// Build the query with LEFT JOIN to bill_payments to get payment status for specific months
	var query string
	if targetYearMonth != "" {
		// For specific month queries, check payment status for that month ONLY from bill_payments
		query = fmt.Sprintf(`
			SELECT 
				b.id, b.name, b.amount, 
				CASE 
					WHEN b.recurring = 1 THEN 
						strftime('%%Y-%%m-%%d', 
							date('%s-01', '+' || (b.payment_day - 1) || ' days'))
					ELSE b.due_date 
				END as calculated_due_date,
				COALESCE(bp.paid, 0) as month_paid, 
				b.overdue, b.overdue_days, b.recurring, b.category, b.icon,
				b.start_date, b.duration_months, b.payment_day
			FROM bills b 
			LEFT JOIN bill_payments bp ON b.id = bp.bill_id AND bp.year_month = '%s'
			WHERE %s 
			ORDER BY COALESCE(bp.paid, 0) ASC, calculated_due_date ASC`, targetYearMonth, targetYearMonth, strings.Join(whereConditions, " AND "))
	} else {
		// For general queries without specific month, show only unpaid bills
		whereConditions = append(whereConditions, "b.paid = 0")
		query = fmt.Sprintf(`
			SELECT 
				b.id, b.name, b.amount, b.due_date as calculated_due_date, 
				0 as month_paid, 
				b.overdue, b.overdue_days, b.recurring, b.category, b.icon,
				b.start_date, b.duration_months, b.payment_day
			FROM bills b 
			WHERE %s 
			ORDER BY calculated_due_date ASC`, strings.Join(whereConditions, " AND "))
	}

	log.Printf("🔍 fetchUpcomingBills for month: %s", targetYearMonth)
	log.Printf("🔍 Query: %s", query)
	log.Printf("🔍 Args: %v", args)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming bills: %v", err)
	}
	defer rows.Close()

	var bills []Transaction
	var overdue, upcoming, thisWeek, thisMonth int
	now := time.Now()
	weekFromNow := now.AddDate(0, 0, 7)
	monthFromNow := now.AddDate(0, 1, 0)

	for rows.Next() {
		var t Transaction
		var monthPaid, overdueFlag, recurring bool
		var overdueDays int
		var startDate sql.NullString
		var paymentDay, durationMonths sql.NullInt64

		err := rows.Scan(
			&t.ID, &t.Name, &t.Amount, &t.Date, &monthPaid, &overdueFlag, &overdueDays,
			&recurring, &t.Category, &t.Icon, &startDate, &durationMonths, &paymentDay,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bill: %v", err)
		}

		// Set transaction type and bill-specific fields
		t.Type = "bill"
		t.PaymentMethod = "cash" // Default value since bills table doesn't have payment_method
		t.Paid = &monthPaid      // Use month-specific payment status
		t.Recurring = &recurring

		// Set payment_day if available
		if paymentDay.Valid {
			dayValue := int(paymentDay.Int64)
			t.PaymentDay = &dayValue
		}

		log.Printf("🏠 Bill ID=%d, Name=%s, Month=%s, Paid=%t", t.ID, t.Name, targetYearMonth, monthPaid)

		// Parse due date for categorization
		dueDate, err := time.Parse("2006-01-02", t.Date)
		if err != nil {
			log.Printf("Warning: failed to parse due date %s: %v", t.Date, err)
			continue
		}

		// Calculate overdue status dynamically based on current date
		currentOverdue := false
		currentOverdueDays := 0

		if !monthPaid && dueDate.Before(now) {
			currentOverdue = true
			currentOverdueDays = int(now.Sub(dueDate).Hours() / 24)
			log.Printf("📅 Bill ID=%d is overdue by %d days (due: %s, now: %s)",
				t.ID, currentOverdueDays, t.Date, now.Format("2006-01-02"))
		}

		// Set the calculated overdue values
		t.Overdue = &currentOverdue
		t.OverdueDays = &currentOverdueDays

		// Categorize bills - only count unpaid bills in statistics
		if !monthPaid {
			if currentOverdue {
				overdue++
			} else {
				upcoming++
				if dueDate.Before(weekFromNow) {
					thisWeek++
				}
				if dueDate.Before(monthFromNow) {
					thisMonth++
				}
			}
		}

		bills = append(bills, t)
	}

	return &UpcomingBillsResponse{
		Bills:     bills,
		Total:     len(bills),
		Overdue:   overdue,
		Upcoming:  upcoming,
		ThisWeek:  thisWeek,
		ThisMonth: thisMonth,
	}, nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// fetchPaidBillsAmount retrieves the total amount of paid bills for a specific period and date
func fetchPaidBillsAmount(userID, period, date string) (float64, float64, error) {
	var bankAmount, cashAmount float64
	var dateCondition string

	// Build date condition based on period type
	switch period {
	case "daily":
		dateCondition = "due_date = ?"
	case "weekly":
		// For weekly, we need to check if due_date falls within the week
		dateCondition = "strftime('%Y-%W', due_date) = ?"
	case "monthly":
		dateCondition = "strftime('%Y-%m', due_date) = ?"
	case "quarterly":
		// For quarterly, extract year and quarter from date (format: YYYY-Q)
		dateCondition = "strftime('%Y', due_date) || '-' || CASE WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 1 AND 3 THEN '1' WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 4 AND 6 THEN '2' WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 7 AND 9 THEN '3' ELSE '4' END = ?"
	case "semiannual":
		// For semiannual, extract year and half from date (format: YYYY-H)
		dateCondition = "strftime('%Y', due_date) || '-' || CASE WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 1 AND 6 THEN '1' ELSE '2' END = ?"
	case "annual":
		dateCondition = "strftime('%Y', due_date) = ?"
	default:
		dateCondition = "strftime('%Y-%m', due_date) = ?"
	}

	query := fmt.Sprintf(`
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_amount,
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_amount
		FROM bills 
		WHERE user_id = ? AND paid = 1 AND %s
	`, dateCondition)

	row := db.QueryRow(query, userID, date)
	err := row.Scan(&bankAmount, &cashAmount)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to fetch paid bills: %v", err)
	}

	log.Printf("💳 Paid bills for %s %s: Bank=%.2f, Cash=%.2f", period, date, bankAmount, cashAmount)
	return bankAmount, cashAmount, nil
}

// fetchUnpaidBillsAmount retrieves the total amount of unpaid bills for a specific period and date
func fetchUnpaidBillsAmount(userID, period, date string) (float64, float64, error) {
	var bankAmount, cashAmount float64
	var dateCondition string

	// Build date condition based on period type (same logic as paid bills)
	switch period {
	case "daily":
		dateCondition = "due_date = ?"
	case "weekly":
		dateCondition = "strftime('%Y-%W', due_date) = ?"
	case "monthly":
		dateCondition = "strftime('%Y-%m', due_date) = ?"
	case "quarterly":
		dateCondition = "strftime('%Y', due_date) || '-' || CASE WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 1 AND 3 THEN '1' WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 4 AND 6 THEN '2' WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 7 AND 9 THEN '3' ELSE '4' END = ?"
	case "semiannual":
		dateCondition = "strftime('%Y', due_date) || '-' || CASE WHEN CAST(strftime('%m', due_date) AS INTEGER) BETWEEN 1 AND 6 THEN '1' ELSE '2' END = ?"
	case "annual":
		dateCondition = "strftime('%Y', due_date) = ?"
	default:
		dateCondition = "strftime('%Y-%m', due_date) = ?"
	}

	query := fmt.Sprintf(`
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_amount,
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_amount
		FROM bills 
		WHERE user_id = ? AND paid = 0 AND %s
	`, dateCondition)

	row := db.QueryRow(query, userID, date)
	err := row.Scan(&bankAmount, &cashAmount)

	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to fetch unpaid bills: %v", err)
	}

	log.Printf("⏳ Unpaid bills for %s %s: Bank=%.2f, Cash=%.2f", period, date, bankAmount, cashAmount)
	return bankAmount, cashAmount, nil
}

func handleBudgetOverviewHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Budget Overview service is healthy", map[string]string{
		"service":   "budget_overview_fetch",
		"status":    "healthy",
		"port":      "8098",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}
