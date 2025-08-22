package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
)

type User struct {
	ID                int       `json:"id"`
	GoogleID          string    `json:"google_id"`
	AppleID           string    `json:"apple_id"`
	Email             string    `json:"email"`
	Password          string    `json:"password"` // Password field included but will be cleared before response
	Name              string    `json:"name"`
	GivenName         string    `json:"given_name"`
	FamilyName        string    `json:"family_name"`
	Picture           string    `json:"picture"`
	ProfileImageBlob  string    `json:"profile_image_blob"`
	Locale            string    `json:"locale"`
	VerifiedEmail     bool      `json:"verified_email"`
	VerificationCode  string    `json:"verification_code,omitempty"`
	Type              string    `json:"type"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ResetToken        string    `json:"reset_token"`
	ResetExpires      string    `json:"reset_expires"`
	DisplayImage      string    `json:"display_image"`
}

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignInResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	User    interface{} `json:"user,omitempty"`
}

type FetchUserDataRequest struct {
	UserID int `json:"user_id"`
}

type FetchUserDataResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    *User  `json:"data,omitempty"`
}

func init() {
	// Parse command line flags
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Determine database path based on environment flag
	var dbPath string
	if *prodMode {
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)
	} else if *devMode {
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to development mode if no flag specified
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)
	}

	var err error
	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	log.Println("Database connection established successfully")
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/signin", corsMiddleware(handleSignIn))
	http.HandleFunc("/signin/check-email", corsMiddleware(handleCheckEmail))
	http.HandleFunc("/signin/fetch-user-data", corsMiddleware(handleFetchUserData))

	log.Println("SignIn service started on :8084")
	log.Fatal(http.ListenAndServe(":8084", nil))
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

type EmailCheckRequest struct {
	Email string `json:"email"`
}

type EmailCheckResponse struct {
	Exists bool `json:"exists"`
}

func handleCheckEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req EmailCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if email exists with type='email' (only email users, not Google/Apple)
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND type = 'email')", req.Email).Scan(&exists)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EmailCheckResponse{Exists: exists})
}

func handleSignIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SignInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Email and password are required",
		})
		return
	}

	// Note: Redis session caching removed to improve API speed

	// Check if user exists and password is correct - ONLY for email users (type='email')
	var user User
	var storedPassword sql.NullString // Use NullString to handle NULL values safely
	var name sql.NullString
	var givenName sql.NullString
	var familyName sql.NullString
	var picture sql.NullString
	var profileImageBlob sql.NullString
	var locale sql.NullString

	// Log signin attempt for debugging
	log.Printf("Sign-in attempt for email: %s", req.Email)

	err := db.QueryRow(`
		SELECT id, email, password, name, given_name, family_name, 
		picture, profile_image_blob, locale, verified_email, created_at, updated_at 
		FROM users 
		WHERE email = ? AND type = 'email'
	`, req.Email).Scan(
		&user.ID,
		&user.Email,
		&storedPassword, // This now handles NULL values properly
		&name,
		&givenName,
		&familyName,
		&picture,
		&profileImageBlob,
		&locale,
		&user.VerifiedEmail,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	// Convert NullString values to regular strings for User struct
	user.Name = name.String
	user.GivenName = givenName.String
	user.FamilyName = familyName.String
	user.Picture = picture.String
	user.ProfileImageBlob = profileImageBlob.String
	user.Locale = locale.String

	// Set display_image based on user type (email users use profile_image_blob)
	if user.ProfileImageBlob != "" {
		user.DisplayImage = user.ProfileImageBlob
		log.Printf("Using profile image blob for email user %d (blob size: %d bytes)", user.ID, len(user.ProfileImageBlob))
	} else {
		log.Printf("Email user %d has no profile image blob", user.ID)
	}

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		// Return proper JSON error instead of plain text
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Database error occurred",
		})
		return
	}

	// Handle NULL or empty password field
	if !storedPassword.Valid || storedPassword.String == "" {
		log.Printf("User %s has no password set (NULL or empty)", req.Email)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	// In a production app, you would use a secure password comparison
	// This is a simple string comparison for demonstration
	if storedPassword.String != req.Password {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Invalid email or password",
		})
		return
	}

	// Update last login time
	_, err = db.Exec("UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ? AND type = 'email'", user.ID)
	if err != nil {
		log.Printf("Failed to update last login time: %v", err)
		// Continue anyway, not critical
	}

	// Check if email is verified
	if !user.VerifiedEmail {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(SignInResponse{
			Success: false,
			Message: "Email not verified. Please check your inbox for verification email.",
			User:    user,
		})
		return
	}

	// Note: Redis session caching removed to improve API speed

	// Return user data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SignInResponse{
		Success: true,
		User:    user,
	})

	log.Printf("User %s logged in successfully", user.Email)
}

/**
 * handleFetchUserData - Fetches complete user data by user ID
 * 
 * Purpose: Provides complete user record from database for loginService synchronization
 * Used by mobile app to sync server user data with local database after successful authentication
 * 
 * Endpoint: POST /signin/fetch-user-data
 * Request: { "user_id": <integer> }
 * Response: { "success": <boolean>, "data": <User>, "message": <string> }
 * 
 * Security: No authentication required as this is called immediately after successful login
 * Database: Queries users table with all available columns
 * 
 * Algorithm:
 * 1. Validate POST method and parse JSON request body
 * 2. Validate user_id parameter is provided and valid
 * 3. Query database for complete user record by ID
 * 4. Handle nullable database fields safely using sql.NullString
 * 5. Convert database types to Go struct with proper field mapping
 * 6. Set display_image based on user type and available profile data
 * 7. Return complete user data as JSON response
 */
func handleFetchUserData(w http.ResponseWriter, r *http.Request) {
	// Step 1: Validate HTTP method
	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(FetchUserDataResponse{
			Success: false,
			Message: "Method not allowed - use POST",
		})
		return
	}

	// Step 2: Parse and validate request body
	var req FetchUserDataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error parsing request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FetchUserDataResponse{
			Success: false,
			Message: "Invalid request body - expected JSON with user_id",
		})
		return
	}

	// Step 3: Validate user_id parameter
	if req.UserID <= 0 {
		log.Printf("Invalid user_id provided: %d", req.UserID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(FetchUserDataResponse{
			Success: false,
			Message: "Invalid user_id - must be a positive integer",
		})
		return
	}

	log.Printf("Fetching complete user data for user ID: %d", req.UserID)

	// Step 4: Query database for complete user record
	// Use sql.NullString for nullable fields to handle NULL values safely
	var user User
	var googleID sql.NullString
	var appleID sql.NullString
	var password sql.NullString
	var name sql.NullString
	var givenName sql.NullString
	var familyName sql.NullString
	var picture sql.NullString
	var profileImageBlob sql.NullString
	var locale sql.NullString
	var verificationCode sql.NullString
	var userType sql.NullString
	var resetToken sql.NullString
	var resetExpires sql.NullString

	// Step 5: Execute comprehensive database query with all user table columns
	err := db.QueryRow(`
		SELECT 
			id, google_id, apple_id, email, password, name, given_name, family_name,
			picture, profile_image_blob, locale, verified_email, verification_code,
			type, created_at, updated_at, reset_token, reset_expires
		FROM users 
		WHERE id = ?
	`, req.UserID).Scan(
		&user.ID,
		&googleID,
		&appleID,
		&user.Email,
		&password, // Password is scanned but never returned to client (json:"-" tag)
		&name,
		&givenName,
		&familyName,
		&picture,
		&profileImageBlob,
		&locale,
		&user.VerifiedEmail,
		&verificationCode,
		&userType,
		&user.CreatedAt,
		&user.UpdatedAt,
		&resetToken,
		&resetExpires,
	)

	// Step 6: Handle query results and errors
	if err == sql.ErrNoRows {
		log.Printf("User not found with ID: %d", req.UserID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(FetchUserDataResponse{
			Success: false,
			Message: "User not found",
		})
		return
	} else if err != nil {
		log.Printf("Database error fetching user %d: %v", req.UserID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(FetchUserDataResponse{
			Success: false,
			Message: "Database error occurred",
		})
		return
	}

	// Step 7: Convert sql.NullString values to regular strings for User struct
	// This ensures safe handling of NULL database values
	user.GoogleID = googleID.String
	user.AppleID = appleID.String
	user.Name = name.String
	user.GivenName = givenName.String
	user.FamilyName = familyName.String
	user.Picture = picture.String
	user.ProfileImageBlob = profileImageBlob.String
	user.Locale = locale.String
	user.VerificationCode = verificationCode.String
	user.Type = userType.String
	user.ResetToken = resetToken.String
	user.ResetExpires = resetExpires.String

	// Step 8: Set display_image based on user type and available profile data
	// Priority: profile_image_blob > picture > empty string
	if user.ProfileImageBlob != "" {
		user.DisplayImage = user.ProfileImageBlob
		log.Printf("Set display_image from profile_image_blob for user %d (size: %d bytes)", 
			user.ID, len(user.ProfileImageBlob))
	} else if user.Picture != "" {
		user.DisplayImage = user.Picture
		log.Printf("Set display_image from picture URL for user %d", user.ID)
	} else {
		log.Printf("No profile image available for user %d", user.ID)
	}

	// Step 9: Clear sensitive fields before sending response
	// Security: Never send password field to client, even if empty
	user.Password = ""

	// Step 10: Log successful data retrieval for debugging
	log.Printf("Successfully fetched complete user data for user %d: email=%s, type=%s, verified=%t", 
		user.ID, user.Email, user.Type, user.VerifiedEmail)

	// Step 11: Return complete user data as JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(FetchUserDataResponse{
		Success: true,
		Data:    &user,
		Message: "User data retrieved successfully",
	})

	log.Printf("Complete user data response sent for user %d", user.ID)
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Note: Redis session management removed to improve API performance
// Signin operations are typically one-time actions that don't benefit from caching
