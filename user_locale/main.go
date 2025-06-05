package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
)

type UserLocaleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Locale  string `json:"locale,omitempty"`
}

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

	log.Println("User Locale service - Database connection established successfully")
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/user_locale/get", corsMiddleware(handleGetUserLocale))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	port := 8099
	log.Printf("User Locale service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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

func handleGetUserLocale(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" || userID == "null" {
		log.Printf("Error: User ID is empty or 'null' in request")
		http.Error(w, "Valid user ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Getting user locale for user ID: %s", userID)

	// Get only the locale from the database
	var locale sql.NullString
	err := db.QueryRow(`
		SELECT locale 
		FROM users 
		WHERE id = ?
	`, userID).Scan(&locale)

	if err == sql.ErrNoRows {
		log.Printf("User not found for ID: %s", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(UserLocaleResponse{
			Success: false,
			Message: "User not found",
		})
		return
	} else if err != nil {
		log.Printf("Database error for user ID %s: %v", userID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if locale is valid
	var userLocale string
	if locale.Valid && locale.String != "" {
		userLocale = locale.String
		log.Printf("Successfully retrieved locale for user %s: %s", userID, userLocale)
	} else {
		log.Printf("No locale set for user %s", userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(UserLocaleResponse{
			Success: false,
			Message: "No locale set for user",
		})
		return
	}

	// Return user locale as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserLocaleResponse{
		Success: true,
		Message: "User locale retrieved successfully",
		Locale:  userLocale,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		http.Error(w, "Database connection failed", http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserLocaleResponse{
		Success: true,
		Message: "User Locale service is healthy",
	})
}
