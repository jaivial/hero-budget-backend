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

var (
	db *sql.DB
)

type User struct {
	ID               int       `json:"id"`
	GoogleID         *string   `json:"google_id"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	GivenName        *string   `json:"given_name"`
	FamilyName       *string   `json:"family_name"`
	Picture          *string   `json:"picture"`
	ProfileImageBlob *string   `json:"profile_image_blob,omitempty"`
	Locale           string    `json:"locale"`
	VerifiedEmail    bool      `json:"verified_email"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	DisplayImage     string    `json:"display_image"`
}

type UserUpdateRequest struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Email      string `json:"email,omitempty"`
	GivenName  string `json:"given_name,omitempty"`
	FamilyName string `json:"family_name,omitempty"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
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

	log.Println("Database connection established successfully")
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/user/info", corsMiddleware(handleGetUserInfo))
	http.HandleFunc("/user/update", corsMiddleware(handleUpdateUser))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	port := 8085
	log.Printf("Fetch Dashboard service started on :%d", port)
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

func handleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userID string

	// Get user ID from query parameter (GET) or request body (POST)
	if r.Method == "GET" {
		userID = r.URL.Query().Get("id")
	} else { // POST
		var requestBody map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			log.Printf("Error parsing request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if uid, ok := requestBody["user_id"].(string); ok {
			userID = uid
		}
	}

	if userID == "" || userID == "null" {
		log.Printf("Error: User ID is empty or 'null' in request")
		http.Error(w, "Valid user ID is required", http.StatusBadRequest)
		return
	}

	// Log for debugging
	log.Printf("Getting user info for user ID: %s", userID)

	// Get user info from database
	var user User
	err := db.QueryRow(`
		SELECT id, google_id, email, name, given_name, family_name, 
		picture, profile_image_blob, locale, verified_email, created_at, updated_at 
		FROM users 
		WHERE id = ?
	`, userID).Scan(
		&user.ID,
		&user.GoogleID,
		&user.Email,
		&user.Name,
		&user.GivenName,
		&user.FamilyName,
		&user.Picture,
		&user.ProfileImageBlob,
		&user.Locale,
		&user.VerifiedEmail,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		log.Printf("User not found for ID: %s", userID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error for user ID %s: %v", userID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Set the display image based on the user type
	if user.GoogleID != nil && *user.GoogleID != "" {
		// Google user - use Picture URL field
		if user.Picture != nil && *user.Picture != "" {
			user.DisplayImage = *user.Picture
			log.Printf("Using Google profile picture URL for user %d", user.ID)
		}
	} else {
		// Regular user - use ProfileImageBlob field
		if user.ProfileImageBlob != nil && *user.ProfileImageBlob != "" {
			user.DisplayImage = *user.ProfileImageBlob
			log.Printf("Using profile image blob for user %d", user.ID)
		}
	}

	log.Printf("Successfully retrieved user %s: %s (%s)", userID, user.Name, user.Email)

	// Return user info as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UserUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Log for debugging
	log.Printf("Updating user info: %+v", updateRequest)

	// Update user info in database
	result, err := db.Exec(`
		UPDATE users 
		SET name = ?, email = ?, given_name = ?, family_name = ? 
		WHERE id = ?
	`, updateRequest.Name, updateRequest.Email, updateRequest.GivenName, updateRequest.FamilyName, updateRequest.ID)

	if err != nil {
		log.Printf("Database error for user ID %s: %v", updateRequest.ID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		log.Printf("User not found for ID: %s", updateRequest.ID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Printf("Successfully updated user %s", updateRequest.ID)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{Success: true, Message: "User updated successfully"})
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
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Fetch Dashboard service is healthy",
		Data: map[string]string{
			"status":    "healthy",
			"service":   "fetch_dashboard",
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		},
	})
}
