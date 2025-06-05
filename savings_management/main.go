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
type SavingsData struct {
	UserID      string  `json:"user_id"`
	Available   float64 `json:"available"`
	Goal        float64 `json:"goal"`
	Period      string  `json:"period"` // New field for period type
	Percent     float64 `json:"percent"`
	NeedToSave  float64 `json:"need_to_save"`
	DailyTarget float64 `json:"daily_target"`
}

type SavingsUpdateRequest struct {
	UserID    string  `json:"user_id"`
	Available float64 `json:"available,omitempty"`
	Goal      float64 `json:"goal,omitempty"`
	Period    string  `json:"period,omitempty"` // New field for period type
}

type SavingsDeleteRequest struct {
	UserID string `json:"user_id"`
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
	// Create savings table
	_, err := db.Exec(`
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
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/savings/fetch", corsMiddleware(handleFetchSavings))
	http.HandleFunc("/savings/update", corsMiddleware(handleUpdateSavings))
	http.HandleFunc("/savings/delete", corsMiddleware(handleDeleteSavings))
	http.HandleFunc("/health", corsMiddleware(handleHealth))
	http.HandleFunc("/savings/health", corsMiddleware(handleSavingsHealth))

	port := 8089
	log.Printf("Savings Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

func handleFetchSavings(w http.ResponseWriter, r *http.Request) {
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

	// Get savings data from database
	savings, err := fetchSavingsData(userID)
	if err != nil {
		log.Printf("Error fetching savings data: %v", err)
		sendErrorResponse(w, "Error fetching savings data", http.StatusInternalServerError)
		return
	}

	// Return savings data as JSON
	sendSuccessResponse(w, "Savings data fetched successfully", savings)
}

func handleUpdateSavings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest SavingsUpdateRequest
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

	// Get current savings data to update only the fields that were provided
	currentSavings, err := fetchSavingsData(updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching current savings data: %v", err)
		sendErrorResponse(w, "Error fetching current savings data", http.StatusInternalServerError)
		return
	}

	// Update only the fields that were provided
	if updateRequest.Available > 0 {
		currentSavings.Available = updateRequest.Available
	}
	if updateRequest.Goal > 0 {
		currentSavings.Goal = updateRequest.Goal
	}
	if updateRequest.Period != "" {
		currentSavings.Period = updateRequest.Period
	}

	// Calculate the percentage
	if currentSavings.Goal > 0 {
		currentSavings.Percent = (currentSavings.Available / currentSavings.Goal) * 100
	} else {
		currentSavings.Percent = 0
	}

	// Calculate need to save and daily target
	currentSavings.NeedToSave = currentSavings.Goal - currentSavings.Available
	if currentSavings.NeedToSave < 0 {
		currentSavings.NeedToSave = 0
	}
	// Assuming goal needs to be achieved within a month (30 days)
	currentSavings.DailyTarget = currentSavings.NeedToSave / 30

	// Save the updated savings data
	err = updateSavingsData(currentSavings)
	if err != nil {
		log.Printf("Error updating savings data: %v", err)
		sendErrorResponse(w, "Error updating savings data", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Savings updated successfully", currentSavings)
}

func handleDeleteSavings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var deleteRequest SavingsDeleteRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Delete the savings data
	err = deleteSavingsData(deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting savings data: %v", err)
		sendErrorResponse(w, "Error deleting savings data", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Savings goal deleted successfully", nil)
}

func fetchSavingsData(userID string) (SavingsData, error) {
	var savings SavingsData

	// Query savings data from database
	err := db.QueryRow(`
		SELECT user_id, available, goal, period, percent
		FROM savings
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&savings.UserID,
		&savings.Available,
		&savings.Goal,
		&savings.Period,
		&savings.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no data found
		savings.UserID = userID
		savings.Available = 0
		savings.Goal = 0
		savings.Period = "monthly" // Default period
		savings.Percent = 0
		savings.NeedToSave = 0
		savings.DailyTarget = 0
		return savings, nil
	} else if err != nil {
		return savings, err
	}

	// Calculate need to save and daily target
	savings.NeedToSave = savings.Goal - savings.Available
	if savings.NeedToSave < 0 {
		savings.NeedToSave = 0
	}
	// Assuming goal needs to be achieved within a month (30 days)
	savings.DailyTarget = savings.NeedToSave / 30

	return savings, nil
}

func updateSavingsData(savings SavingsData) error {
	// Check if a savings entry already exists for this user
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM savings 
		WHERE user_id = ?
	`, savings.UserID).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing savings entry
		_, err = db.Exec(`
			UPDATE savings
			SET available = ?,
				goal = ?,
				period = ?,
				percent = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?
		`,
			savings.Available,
			savings.Goal,
			savings.Period,
			savings.Percent,
			savings.UserID,
		)
	} else {
		// Insert new savings entry
		_, err = db.Exec(`
			INSERT INTO savings (
				user_id, available, goal, period, percent
			) VALUES (?, ?, ?, ?, ?)
		`,
			savings.UserID,
			savings.Available,
			savings.Goal,
			savings.Period,
			savings.Percent,
		)
	}

	return err
}

func deleteSavingsData(userID string) error {
	// Execute delete query
	result, err := db.Exec(`
		DELETE FROM savings 
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return err
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no savings goal found for user %s", userID)
	}

	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Savings Management service is healthy", map[string]string{
		"status":    "healthy",
		"service":   "savings_management",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

func handleSavingsHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Savings Management service is healthy", map[string]string{
		"status":    "healthy",
		"service":   "savings_management",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
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
