package main

import (
	"database/sql"
	"encoding/json"
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
	http.HandleFunc("/auth/apple", corsMiddleware(handleAppleAuth))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	log.Println("Apple Auth service started on :8100")
	log.Fatal(http.ListenAndServe(":8100", nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the actual handler
		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"service":   "apple-auth",
		"timestamp": time.Now().UTC(),
		"port":      "8100",
	})
}

func handleAppleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AppleSignInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.IdentityToken == "" {
		sendErrorResponse(w, "Identity token is required", http.StatusBadRequest)
		return
	}

	// Parse and validate the Apple JWT token
	claims, err := validateAppleToken(req.IdentityToken)
	if err != nil {
		log.Printf("Failed to validate Apple token: %v", err)
		sendErrorResponse(w, "Invalid Apple token", http.StatusUnauthorized)
		return
	}

	// Extract user information from claims and request
	user := extractUserFromRequest(req, claims)

	log.Printf("Processing Apple Sign-In for user: %s (Apple ID: %s)", user.Email, user.AppleID)

	// Check if user exists in database
	existingUser, err := getUserByAppleID(user.AppleID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Database error checking user: %v", err)
		sendErrorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err == sql.ErrNoRows {
		// Check if user exists with same email (different auth method)
		existingEmailUser, emailErr := getUserByEmail(user.Email)
		if emailErr != nil && emailErr != sql.ErrNoRows {
			log.Printf("Database error checking email: %v", emailErr)
			sendErrorResponse(w, "Database error", http.StatusInternalServerError)
			return
		}

		if emailErr == nil {
			// User exists with same email, link Apple ID to existing account
			err = linkAppleIDToUser(existingEmailUser.ID, user.AppleID)
			if err != nil {
				log.Printf("Failed to link Apple ID to existing user: %v", err)
				sendErrorResponse(w, "Failed to link account", http.StatusInternalServerError)
				return
			}

			// Update user info and return
			existingEmailUser.AppleID = user.AppleID
			updateUserLastLogin(existingEmailUser.ID)
			sendSuccessResponse(w, "Account linked successfully", existingEmailUser)
			return
		}

		// Create new user
		newUser, err := createAppleUser(user)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			sendErrorResponse(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		log.Printf("Created new Apple user: %s", newUser.Email)
		sendSuccessResponse(w, "User created successfully", newUser)
		return
	}

	// User exists, update last login and return user data
	updateUserLastLogin(existingUser.ID)
	log.Printf("Apple user logged in: %s", existingUser.Email)
	sendSuccessResponse(w, "Login successful", existingUser)
}

func sendSuccessResponse(w http.ResponseWriter, message string, user *User) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AppleSignInResponse{
		Success: true,
		Message: message,
		User:    user,
	})
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(AppleSignInResponse{
		Success: false,
		Message: message,
	})
}
