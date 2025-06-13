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

type User struct {
	ID            int       `json:"id"`
	GoogleID      string    `json:"google_id"`
	Email         string    `json:"email"`
	Password      string    `json:"-"` // Never send password to client
	Name          string    `json:"name"`
	GivenName     string    `json:"given_name"`
	FamilyName    string    `json:"family_name"`
	Picture       string    `json:"picture"`
	Locale        string    `json:"locale"`
	VerifiedEmail bool      `json:"verified_email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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

	// Check if email exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", req.Email).Scan(&exists)
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

	// Check if user exists and password is correct
	var user User
	var storedPassword sql.NullString // Use NullString to handle NULL values safely
	var name sql.NullString
	var givenName sql.NullString
	var familyName sql.NullString
	var picture sql.NullString
	var locale sql.NullString

	err := db.QueryRow(`
		SELECT id, email, password, name, given_name, family_name, 
		picture, locale, verified_email, created_at, updated_at 
		FROM users 
		WHERE email = ?
	`, req.Email).Scan(
		&user.ID,
		&user.Email,
		&storedPassword, // This now handles NULL values properly
		&name,
		&givenName,
		&familyName,
		&picture,
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
	user.Locale = locale.String

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

	// Return user data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SignInResponse{
		Success: true,
		User:    user,
	})

	log.Printf("User %s logged in successfully", user.Email)
}
