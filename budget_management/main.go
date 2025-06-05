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

type BudgetUpdateRequest struct {
	UserID         string  `json:"user_id"`
	Period         string  `json:"period"`
	TotalAmount    float64 `json:"total_amount"`
	SpentAmount    float64 `json:"spent_amount"`
	UpcomingAmount float64 `json:"upcoming_amount"`
	FromPrevious   float64 `json:"from_previous"`
	TotalIncome    float64 `json:"total_income"`
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
	// Create budget table with new total_income column
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
			total_income REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create budget table: %v", err)
	}

	// Check if total_income column exists and add it if it doesn't
	var exists int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('budget') WHERE name='total_income'
	`).Scan(&exists)

	if err != nil {
		log.Printf("Error checking for total_income column: %v", err)
	} else if exists == 0 {
		// Add the column if it doesn't exist
		_, err = db.Exec(`ALTER TABLE budget ADD COLUMN total_income REAL NOT NULL DEFAULT 0`)
		if err != nil {
			log.Printf("Error adding total_income column: %v", err)
		} else {
			log.Println("Added total_income column to budget table")
		}
	}
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/budget/fetch", corsMiddleware(handleFetchBudget))
	http.HandleFunc("/budget/update", corsMiddleware(handleUpdateBudget))

	port := 8088
	log.Printf("Budget Management service started on :%d", port)
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

func handleFetchBudget(w http.ResponseWriter, r *http.Request) {
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

	// Get period from query parameter (default to 'monthly' if not provided)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	// Get budget data from database
	budget, err := fetchBudgetData(userID, period)
	if err != nil {
		log.Printf("Error fetching budget data: %v", err)
		sendErrorResponse(w, "Error fetching budget data", http.StatusInternalServerError)
		return
	}

	// Return budget data as JSON
	sendSuccessResponse(w, "Budget data fetched successfully", budget)
}

func handleUpdateBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest BudgetUpdateRequest
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

	if updateRequest.Period == "" {
		updateRequest.Period = "monthly"
	}

	// Calculate the remaining amount (include income in the calculation)
	remainingAmount := updateRequest.FromPrevious + updateRequest.TotalIncome - updateRequest.SpentAmount - updateRequest.UpcomingAmount

	// Calculate the percent (what percentage of the budget is used/upcoming)
	var percent float64
	totalAvailable := updateRequest.FromPrevious + updateRequest.TotalIncome
	if totalAvailable > 0 {
		percent = ((updateRequest.SpentAmount + updateRequest.UpcomingAmount) / totalAvailable) * 100
	}

	// Insert or update the budget
	budget := BudgetData{
		UserID:          updateRequest.UserID,
		Period:          updateRequest.Period,
		Date:            time.Now().Format("2006-01-02"),
		TotalAmount:     totalAvailable,
		RemainingAmount: remainingAmount,
		SpentAmount:     updateRequest.SpentAmount,
		UpcomingAmount:  updateRequest.UpcomingAmount,
		FromPrevious:    updateRequest.FromPrevious,
		Percent:         percent,
		TotalIncome:     updateRequest.TotalIncome,
	}

	err = updateBudgetData(budget)
	if err != nil {
		log.Printf("Error updating budget data: %v", err)
		sendErrorResponse(w, "Error updating budget data", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Budget updated successfully", budget)
}

func fetchBudgetData(userID, period string) (BudgetData, error) {
	var budget BudgetData

	// Query budget data from database
	err := db.QueryRow(`
		SELECT user_id, period, date, total_amount, remaining_amount, spent_amount, 
		       upcoming_amount, from_previous, percent, COALESCE(total_income, 0)
		FROM budget
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&budget.UserID,
		&budget.Period,
		&budget.Date,
		&budget.TotalAmount,
		&budget.RemainingAmount,
		&budget.SpentAmount,
		&budget.UpcomingAmount,
		&budget.FromPrevious,
		&budget.Percent,
		&budget.TotalIncome,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		budget.UserID = userID
		budget.Period = period
		budget.Date = time.Now().Format("2006-01-02")
		budget.TotalAmount = 0
		budget.RemainingAmount = 0
		budget.SpentAmount = 0
		budget.UpcomingAmount = 0
		budget.FromPrevious = 0
		budget.Percent = 0
		budget.TotalIncome = 0

		// Check if there's a previous period to inherit from
		previousPeriod, previousAmount := getPreviousPeriodData(userID, period)
		if previousAmount > 0 {
			budget.FromPrevious = previousAmount
			budget.TotalAmount = previousAmount
			budget.RemainingAmount = previousAmount

			// Log the inheritance
			log.Printf("Inheriting %f from previous period %s for user %s in period %s",
				previousAmount, previousPeriod, userID, period)
		}

		return budget, nil
	} else if err != nil {
		return budget, err
	}

	return budget, nil
}

// Get data from previous time periods to inherit the remaining amount
func getPreviousPeriodData(userID, currentPeriod string) (string, float64) {
	// Define the previous period based on the current period
	var previousPeriod string
	var queryDateCondition string

	now := time.Now()

	switch currentPeriod {
	case "daily":
		// Previous day
		previousPeriod = "daily"
		previousDate := now.AddDate(0, 0, -1).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date = '%s'", previousDate)
	case "weekly":
		// Previous week
		previousPeriod = "weekly"
		// Get the date for the previous week (7 days ago)
		previousWeekStart := now.AddDate(0, 0, -7).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousWeekStart)
	case "monthly":
		// Previous month
		previousPeriod = "monthly"
		// Get the date for the previous month
		previousMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousMonthStart)
	case "quarterly":
		// Previous quarter
		previousPeriod = "quarterly"
		// Calculate the start of the previous quarter
		currentQuarter := (int(now.Month())-1)/3 + 1
		previousQuarter := currentQuarter - 1
		var year int
		if previousQuarter <= 0 {
			previousQuarter = 4
			year = now.Year() - 1
		} else {
			year = now.Year()
		}
		previousQuarterStart := time.Date(year, time.Month((previousQuarter-1)*3+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousQuarterStart)
	case "semiannual":
		// Previous half-year
		previousPeriod = "semiannual"
		// Calculate the start of the previous half year
		currentHalf := (int(now.Month())-1)/6 + 1
		previousHalf := currentHalf - 1
		var year int
		if previousHalf <= 0 {
			previousHalf = 2
			year = now.Year() - 1
		} else {
			year = now.Year()
		}
		previousHalfStart := time.Date(year, time.Month((previousHalf-1)*6+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousHalfStart)
	case "annual":
		// Previous year
		previousPeriod = "annual"
		previousYearStart := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousYearStart)
	default:
		// If the period is not recognized, don't try to get previous data
		return "", 0
	}

	// Query to get the most recent budget entry for the previous period
	query := fmt.Sprintf(`
		SELECT remaining_amount FROM budget 
		WHERE user_id = ? AND period = ? %s
		LIMIT 1
	`, queryDateCondition)

	var remainingAmount float64
	err := db.QueryRow(query, userID, previousPeriod).Scan(&remainingAmount)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error getting previous period data: %v", err)
		}
		return "", 0
	}

	return previousPeriod, remainingAmount
}

func updateBudgetData(budget BudgetData) error {
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
