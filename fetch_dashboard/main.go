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
	// Redis client for caching dashboard data and user info queries
	rdb *redis.Client
	// Context for Redis operations with timeout handling
	ctx = context.Background()
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

	// Initialize Redis connection for dashboard data caching
	initRedis()
}

// initRedis initializes Redis connection for dashboard data caching
// Provides distributed caching for user info queries and dashboard data aggregation
func initRedis() {
	// Redis connection configuration with environment variable support
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address for local development
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 5 // Use DB 5 for fetch_dashboard service to avoid conflicts

	// Create Redis client with connection pooling and timeout settings
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     10,                // Connection pool size for concurrent requests
		DialTimeout:  5 * time.Second,   // Connection establishment timeout
		ReadTimeout:  3 * time.Second,   // Read operation timeout
		WriteTimeout: 3 * time.Second,   // Write operation timeout
		ConnMaxIdleTime: 5 * time.Minute, // Idle connection timeout
	})

	// Test Redis connection with ping operation
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("Redis connection failed (continuing without cache): %v", err)
		rdb = nil // Disable Redis if connection fails
		return
	}

	log.Println("Redis connection established successfully for dashboard caching")
}

// getUserInfoFromCache retrieves cached user info data with JSON deserialization
// Returns cached user data if available, reducing database load for frequent requests
func getUserInfoFromCache(userID string) (*User, error) {
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	// Generate cache key with service namespace for data isolation
	cacheKey := fmt.Sprintf("fetch_dashboard:user_info:%s", userID)
	
	// Retrieve JSON data from Redis cache
	val, err := rdb.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		log.Printf("Cache miss for user info: %s", userID)
		return nil, fmt.Errorf("cache miss")
	} else if err != nil {
		log.Printf("Redis error retrieving user info cache for %s: %v", userID, err)
		return nil, err
	}

	// Deserialize JSON data to User struct
	var user User
	err = json.Unmarshal([]byte(val), &user)
	if err != nil {
		log.Printf("Error deserializing cached user info for %s: %v", userID, err)
		return nil, err
	}

	log.Printf("Cache hit for user info: %s", userID)
	return &user, nil
}

// cacheUserInfo stores user info data in Redis cache with TTL
// Implements 5-minute TTL for dashboard data to balance freshness and performance
func cacheUserInfo(userID string, user *User) {
	if rdb == nil {
		return
	}

	// Generate cache key with service namespace
	cacheKey := fmt.Sprintf("fetch_dashboard:user_info:%s", userID)
	
	// Serialize user data to JSON for storage
	userData, err := json.Marshal(user)
	if err != nil {
		log.Printf("Error serializing user info for cache: %v", err)
		return
	}

	// Store in Redis with 5-minute TTL (300 seconds)
	// Dashboard data changes infrequently but should stay reasonably fresh
	err = rdb.Set(ctx, cacheKey, userData, 5*time.Minute).Err()
	if err != nil {
		log.Printf("Error caching user info for %s: %v", userID, err)
		return
	}

	log.Printf("User info cached successfully for: %s", userID)
}

// invalidateUserCache removes cached user data when user info is updated
// Ensures cache consistency after user profile modifications
func invalidateUserCache(userID string) {
	if rdb == nil {
		return
	}

	// Generate cache key for deletion
	cacheKey := fmt.Sprintf("fetch_dashboard:user_info:%s", userID)
	
	// Remove cached data to force fresh fetch on next request
	err := rdb.Del(ctx, cacheKey).Err()
	if err != nil {
		log.Printf("Error invalidating user cache for %s: %v", userID, err)
	} else {
		log.Printf("User cache invalidated for: %s", userID)
	}
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

	// Try to get user info from cache first to reduce database load
	cachedUser, err := getUserInfoFromCache(userID)
	if err == nil && cachedUser != nil {
		log.Printf("Returning cached user info for user ID: %s", userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cachedUser)
		return
	}

	// Cache miss or error - fetch from database with proper null handling
	var user User
	var createdAtStr, updatedAtStr sql.NullString
	err = db.QueryRow(`
		SELECT id, google_id, email, name, given_name, family_name, 
		picture, profile_image_blob, locale, verified_email, 
		COALESCE(created_at, datetime('now')) as created_at,
		COALESCE(updated_at, datetime('now')) as updated_at
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
		&createdAtStr,
		&updatedAtStr,
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

	// Parse dates safely
	if createdAtStr.Valid {
		user.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr.String)
	} else {
		user.CreatedAt = time.Now()
	}

	if updatedAtStr.Valid {
		user.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAtStr.String)
	} else {
		user.UpdatedAt = time.Now()
	}

	// Debug user type information
	log.Printf("User %d type info - GoogleID: %v, ProfileImageBlob present: %v",
		user.ID,
		user.GoogleID != nil && *user.GoogleID != "",
		user.ProfileImageBlob != nil && *user.ProfileImageBlob != "")

	// Set the display image based on the user type
	if user.GoogleID != nil && *user.GoogleID != "" {
		// Google user - use Picture URL field
		if user.Picture != nil && *user.Picture != "" {
			user.DisplayImage = *user.Picture
			log.Printf("Using Google profile picture URL for user %d", user.ID)
		} else {
			log.Printf("Google user %d has no picture URL", user.ID)
		}
	} else {
		// Regular/Email user - use ProfileImageBlob field
		if user.ProfileImageBlob != nil && *user.ProfileImageBlob != "" {
			user.DisplayImage = *user.ProfileImageBlob
			log.Printf("Using profile image blob for email user %d (blob size: %d bytes)", user.ID, len(*user.ProfileImageBlob))
		} else {
			log.Printf("Email user %d has no profile image blob", user.ID)
		}
	}

	log.Printf("Successfully retrieved user %s: %s (%s) - DisplayImage set: %v",
		userID, user.Name, user.Email, user.DisplayImage != "")

	// Cache the user info for future requests to improve performance
	go cacheUserInfo(userID, &user)

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

	// Invalidate cache after successful update to ensure data consistency
	go invalidateUserCache(updateRequest.ID)

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

	// Test Redis connection if available
	redisStatus := "disabled"
	if rdb != nil {
		_, err := rdb.Ping(ctx).Result()
		if err != nil {
			redisStatus = "error"
			log.Printf("Health check - Redis connection error: %v", err)
		} else {
			redisStatus = "healthy"
		}
	}

	// Return success response with Redis status
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Fetch Dashboard service is healthy",
		Data: map[string]string{
			"status":      "healthy",
			"service":     "fetch_dashboard",
			"redis":       redisStatus,
			"timestamp":   fmt.Sprintf("%d", time.Now().Unix()),
		},
	})
}
