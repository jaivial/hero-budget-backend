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

	"github.com/redis/go-redis/v9"
	_ "github.com/mattn/go-sqlite3"
)

var (
	// Database connection for user data persistence
	db *sql.DB
	// Redis client for caching JWT tokens and session management
	rdb *redis.Client
	// Context for Redis operations with timeout handling
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

	// Initialize Redis connection for JWT token caching and session management
	initRedis()
}

// initRedis initializes Redis connection for JWT token caching and session management
// Provides distributed caching for authentication tokens and user sessions
func initRedis() {
	// Redis connection configuration with environment variable support
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address for local development
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0 // Default Redis database index

	// Create Redis client with connection pooling and automatic failover
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           redisDB,
		DialTimeout:  5 * time.Second,  // Connection timeout for Redis dial
		ReadTimeout:  3 * time.Second,  // Read timeout for Redis operations
		WriteTimeout: 3 * time.Second,  // Write timeout for Redis operations
		PoolSize:     10,               // Connection pool size for concurrent operations
		MinIdleConns: 2,                // Minimum idle connections in pool
	})

	// Test Redis connection with ping command
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("⚠️ Redis connection failed: %v (continuing without cache)", err)
		return
	}

	log.Printf("✅ Redis connected successfully: %s", pong)
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

	// Check if token is blacklisted in Redis cache
	if isTokenBlacklisted(req.IdentityToken) {
		log.Printf("Blacklisted token used: %s", req.IdentityToken[:20]+"...")
		sendErrorResponse(w, "Token has been revoked", http.StatusUnauthorized)
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

		// Cache new user session in Redis for immediate access
		cacheUserSession(newUser, req.IdentityToken)

		log.Printf("Created new Apple user: %s with type 'apple'", newUser.Email)
		sendSuccessResponse(w, "User created successfully", newUser)
		return
	}

	// User exists, update last login and return user data
	updateUserLastLogin(existingUser.ID)

	// Cache user session in Redis for quick access and session management
	cacheUserSession(existingUser, req.IdentityToken)

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

// isTokenBlacklisted checks if a JWT token is blacklisted in Redis cache
// Returns true if token is blacklisted, false otherwise
func isTokenBlacklisted(token string) bool {
	if rdb == nil {
		return false // Redis not available, skip blacklist check
	}

	// Check if token exists in blacklist set
	blacklistKey := fmt.Sprintf("blacklist:token:%s", token)
	exists, err := rdb.Exists(ctx, blacklistKey).Result()
	if err != nil {
		log.Printf("Redis error checking blacklist: %v", err)
		return false // On error, allow token (fail open)
	}

	return exists > 0
}

// cacheUserSession stores user session data in Redis for quick access
// Caches user information and authentication state for performance optimization
func cacheUserSession(user *User, token string) {
	if rdb == nil {
		return // Redis not available, skip caching
	}

	// Create session data structure for caching
	sessionData := map[string]interface{}{
		"user_id":    user.ID,
		"email":      user.Email,
		"login_time": time.Now().Unix(),
		"auth_type":  "apple",
	}

	if user.Name.Valid {
		sessionData["name"] = user.Name.String
	}
	if user.AppleID.Valid {
		sessionData["apple_id"] = user.AppleID.String
	}

	// Serialize session data to JSON for Redis storage
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		log.Printf("Failed to marshal session data: %v", err)
		return
	}

	// Cache session with 24-hour expiration for security and performance
	sessionKey := fmt.Sprintf("session:apple:%d", user.ID)
	err = rdb.Set(ctx, sessionKey, sessionJSON, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Failed to cache user session: %v", err)
		return
	}

	// Cache token to user ID mapping for quick lookups
	tokenKey := fmt.Sprintf("token:apple:%s", token[:32]) // Use first 32 chars as key
	err = rdb.Set(ctx, tokenKey, user.ID, 24*time.Hour).Err()
	if err != nil {
		log.Printf("Failed to cache token mapping: %v", err)
	}

	log.Printf("✅ Cached session for user %d (Apple)", user.ID)
}
