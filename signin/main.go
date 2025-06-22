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
	"github.com/redis/go-redis/v9"
)

var (
	db *sql.DB
	// Redis client and context for session management
	rdb *redis.Client
	ctx = context.Background()
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
	// Initialize Redis connection for session management
	initRedis()
	
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

	// Check cache for user session first
	cacheKey := fmt.Sprintf("signin_session:%s", req.Email)
	if cachedUser, found := getSigninSession(cacheKey); found {
		log.Printf("Signin session cache hit for user: %s", req.Email)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SignInResponse{
			Success: true,
			Message: "User signed in successfully (cached)",
			User:    cachedUser,
		})
		return
	}

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

	// Cache signin session for 24 hours
	setSigninSession(cacheKey, user, 24*time.Hour)

	// Return user data
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SignInResponse{
		Success: true,
		User:    user,
	})

	log.Printf("User %s logged in successfully", user.Email)
}

// initRedis initializes Redis client connection for session management
func initRedis() {
	// Redis configuration for signin session caching
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",    // Redis server address (localhost on VPS)
		Password: "Jva-Mvc-5171",      // Redis AUTH password
		DB:       3,                   // Use DB 3 for signin sessions
	})

	// Test Redis connection
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		log.Println("Continuing without Redis caching...")
		rdb = nil
	} else {
		log.Println("Successfully connected to Redis for signin session management")
	}
}

// getSigninSession retrieves signin session data from Redis cache
func getSigninSession(key string) (User, bool) {
	if rdb == nil {
		return User{}, false
	}

	// Get cached signin session data
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return User{}, false // Cache miss
	} else if err != nil {
		log.Printf("Redis error getting signin session %s: %v", key, err)
		return User{}, false
	}

	// Deserialize cached user data
	var user User
	err = json.Unmarshal([]byte(val), &user)
	if err != nil {
		log.Printf("Error deserializing signin session %s: %v", key, err)
		return User{}, false
	}

	return user, true
}

// setSigninSession stores signin session data in Redis cache with TTL
func setSigninSession(key string, user User, ttl time.Duration) {
	if rdb == nil {
		return // Redis not available
	}

	// Serialize user data for caching
	userBytes, err := json.Marshal(user)
	if err != nil {
		log.Printf("Error serializing signin session %s: %v", key, err)
		return
	}

	// Store in Redis with TTL (24 hours for signin sessions)
	err = rdb.Set(ctx, key, userBytes, ttl).Err()
	if err != nil {
		log.Printf("Error storing signin session %s: %v", key, err)
	} else {
		log.Printf("Signin session cached successfully: %s (TTL: %v)", key, ttl)
	}
}
