package main

import (
	"context"
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
	// Database connection for user data persistence
	db *sql.DB
	// Context for database operations
	ctx = context.Background()
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

	// Add type column if it doesn't exist (for existing databases)
	_, err = db.Exec(`ALTER TABLE users ADD COLUMN type TEXT DEFAULT 'apple'`)
	if err != nil {
		// Ignore error if column already exists
		log.Printf("Column 'type' may already exist: %v", err)
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

	log.Printf("Processing Apple Sign-In for user: %s (Apple ID: %s)", user.Email, user.AppleID.String)

	// Check if user exists in database
	existingUser, err := getUserByAppleID(user.AppleID.String)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Database error checking user: %v", err)
		sendErrorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}

	if err == sql.ErrNoRows {
		// Check if user exists with same email and type 'apple'
		var existingAppleUser User
		appleErr := db.QueryRow(`
			SELECT id, apple_id, google_id, email, name, given_name, family_name, 
			picture, profile_image_blob, locale, verified_email, COALESCE(type, 'apple') as type, created_at, updated_at 
			FROM users WHERE email = ? AND type = 'apple'`, user.Email).Scan(
			&existingAppleUser.ID,
			&existingAppleUser.AppleID,
			&existingAppleUser.GoogleID,
			&existingAppleUser.Email,
			&existingAppleUser.Name,
			&existingAppleUser.GivenName,
			&existingAppleUser.FamilyName,
			&existingAppleUser.Picture,
			&existingAppleUser.ProfileImageBlob,
			&existingAppleUser.Locale,
			&existingAppleUser.VerifiedEmail,
			&existingAppleUser.Type,
			&existingAppleUser.CreatedAt,
			&existingAppleUser.UpdatedAt,
		)

		if appleErr == nil {
			log.Printf("User with email %s already exists with type 'apple'", user.Email)
			sendErrorResponse(w, "User with this email already exists for Apple Sign-In", http.StatusConflict)
			return
		} else if appleErr != sql.ErrNoRows {
			log.Printf("Database error checking Apple user: %v", appleErr)
			sendErrorResponse(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Check if user exists with same email but different type (for logging purposes)
		existingEmailUser, emailErr := getUserByEmail(user.Email)
		if emailErr == nil {
			log.Printf("User with email %s exists with type '%s', creating Apple user anyway", user.Email, existingEmailUser.Type.String)
		}

		// Create new user with type 'apple'
		newUser, err := createAppleUser(user)
		if err != nil {
			log.Printf("Failed to create user: %v", err)
			sendErrorResponse(w, "Failed to create user", http.StatusInternalServerError)
			return
		}


		log.Printf("Created new Apple user: %s with type 'apple'", newUser.Email)
		sendSuccessResponse(w, "User created successfully", newUser)
		return
	}

	// User exists, update last login and return user data
	updateUserLastLogin(existingUser.ID)

	log.Printf("Apple user logged in: %s", existingUser.Email)
	sendSuccessResponse(w, "Login successful", existingUser)
}

// userToJSON converts a User with sql.NullString fields to a JSON-friendly format
func userToJSON(user *User) map[string]interface{} {
	result := map[string]interface{}{
		"id":             user.ID,
		"email":          user.Email,
		"verified_email": user.VerifiedEmail,
		"created_at":     user.CreatedAt,
		"updated_at":     user.UpdatedAt,
	}

	if user.AppleID.Valid {
		result["apple_id"] = user.AppleID.String
	}
	if user.GoogleID.Valid {
		result["google_id"] = user.GoogleID.String
	}
	if user.Name.Valid {
		result["name"] = user.Name.String
	}
	if user.GivenName.Valid {
		result["given_name"] = user.GivenName.String
	}
	if user.FamilyName.Valid {
		result["family_name"] = user.FamilyName.String
	}
	if user.Picture.Valid {
		result["picture"] = user.Picture.String
	}
	if user.ProfileImageBlob.Valid {
		result["profile_image_blob"] = user.ProfileImageBlob.String
	}
	if user.Locale.Valid {
		result["locale"] = user.Locale.String
	}
	if user.Type.Valid {
		result["type"] = user.Type.String
	}

	return result
}

func sendSuccessResponse(w http.ResponseWriter, message string, user *User) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AppleSignInResponse{
		Success: true,
		Message: message,
		User:    userToJSON(user),
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

