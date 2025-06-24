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
	ID               int       `json:"id"`
	GoogleID         string    `json:"google_id"`
	Email            string    `json:"email"`
	Password         string    `json:"-"` // Never send password to client
	Name             string    `json:"name"`
	GivenName        string    `json:"given_name"`
	FamilyName       string    `json:"family_name"`
	Picture          string    `json:"picture"`
	ProfileImageBlob string    `json:"profile_image_blob,omitempty"`
	Locale           string    `json:"locale"`
	VerifiedEmail    bool      `json:"verified_email"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	DisplayImage     string    `json:"display_image"`
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
	_, err = db.Exec("UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", user.ID)
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

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Note: Redis session management removed to improve API performance
// Signin operations are typically one-time actions that don't benefit from caching
