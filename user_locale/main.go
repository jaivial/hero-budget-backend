package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/herobudget/backend/common"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
	ctx = context.Background()
	// Cache manager for Redis operations to improve performance
	cacheManager *common.CacheManager
)

type UserLocaleResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Locale  string `json:"locale,omitempty"`
}

// UserLocaleUpdateRequest represents a request to update user locale
// Includes sync operation parameters for incremental synchronization
type UserLocaleUpdateRequest struct {
	UserID string `json:"user_id"`
	Locale string `json:"locale"`
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
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

	// Initialize cache manager for improved performance
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	} else {
		log.Println("✅ Cache manager initialized successfully")
	}

	log.Println("User Locale service initialized successfully")
}


// addSyncOperation registra una operación de sincronización en la tabla sync_operations
// Implements timestamp adjustment and device_ids JSON array for multi-device sync
func addSyncOperation(userID, operationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, operation=%s, action=%s, table=%s, record=%s, device=%s", 
		userID, operationID, action, tableName, recordID, deviceID)
	
	// Serialize operation data to JSON for storage
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling sync operation data: %v", err)
		return err
	}
	
	// Prepare device_ids JSON array
	var deviceIDs []string
	if deviceID != "" {
		deviceIDs = []string{deviceID}
	} else {
		deviceIDs = []string{} // Empty array if no device ID provided
	}
	
	// Marshal device IDs to JSON
	deviceIDsJSON, err := json.Marshal(deviceIDs)
	if err != nil {
		log.Printf("Error marshaling device_ids: %v", err)
		return err
	}
	
	// Timestamp adjustment: check if client timestamp is older than latest timestamp
	var latestTimestamp int64
	err = db.QueryRow("SELECT MAX(created_at) FROM sync_operations WHERE user_id = ?", userID).Scan(&latestTimestamp)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking latest timestamp: %v", err)
		return err
	}
	
	// Adjust timestamp if necessary to maintain chronological ordering
	adjustedTimestamp := clientTimestamp
	if clientTimestamp <= latestTimestamp {
		adjustedTimestamp = latestTimestamp + 1
		log.Printf("Adjusted client timestamp from %d to %d (latest was %d)", 
			clientTimestamp, adjustedTimestamp, latestTimestamp)
	}
	
	// Use current server timestamp
	serverTimestamp := time.Now().Unix()
	
	// Insert sync operation record with device_ids JSON array
	// Use adjusted timestamp for created_at to maintain proper synchronization ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, action, table_name, record_id, data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	result, err := db.Exec(
		insertQuery,
		userID,
		operationID,
		action,
		tableName,
		recordID,
		string(dataJSON),
		string(deviceIDsJSON), // Store device IDs as JSON array
		clientTimestamp,
		serverTimestamp,
		adjustedTimestamp, // Use adjusted timestamp for created_at
	)
	
	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}
	
	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	log.Printf("Successfully added sync operation with ID: %d, device_ids: %v, adjusted timestamp: %d", 
		syncOpID, deviceIDs, adjustedTimestamp)
	
	return nil
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/user_locale/get", corsMiddleware(handleGetUserLocale))
	http.HandleFunc("/user_locale/update", corsMiddleware(handleUpdateUserLocale))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	port := 8099
	log.Printf("User Locale service started on :%d", port)
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

	// Try cache first for user locale data
	if cacheManager != nil {
		var cachedLocale string
		err := cacheManager.GetUserData(userID, &cachedLocale)
		if err == nil {
			log.Printf("✅ Cache HIT: user locale for user %s", userID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(UserLocaleResponse{
				Success: true,
				Message: "User locale retrieved from cache",
				Locale:  cachedLocale,
			})
			return
		}
		log.Printf("🔍 Cache MISS: user locale for user %s", userID)
	}

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

		// Cache the result for future requests
		if cacheManager != nil {
			err = cacheManager.CacheUserData(userID, userLocale)
			if err != nil {
				log.Printf("Warning: Failed to cache user locale for user %s: %v", userID, err)
			}
		}

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

func handleUpdateUserLocale(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UserLocaleUpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Locale == "" {
		log.Printf("Missing required fields: UserID=%s, Locale=%s", req.UserID, req.Locale)
		http.Error(w, "User ID and locale are required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating user locale for user ID: %s to locale: %s", req.UserID, req.Locale)

	// Update the user's locale in the database
	result, err := db.Exec(`UPDATE users SET locale = ? WHERE id = ?`, req.Locale, req.UserID)
	if err != nil {
		log.Printf("Database error updating user %s locale: %v", req.UserID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error checking rows affected for user %s: %v", req.UserID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		log.Printf("No user found with ID: %s", req.UserID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(UserLocaleResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}

	log.Printf("Successfully updated locale for user %s to %s", req.UserID, req.Locale)

	// Record sync operation if sync parameters are provided
	if req.OperationID != "" && req.DeviceID != "" && req.Timestamp > 0 {
		log.Printf("Recording sync operation for user locale update: operation_id=%s, device_id=%s, timestamp=%d", 
			req.OperationID, req.DeviceID, req.Timestamp)
		
		// Create sync operation data for user locale update
		syncData := map[string]interface{}{
			"user_id": req.UserID,
			"locale":  req.Locale,
			"action":  "update_locale",
		}
		
		// Add sync operation record to database
		err = addSyncOperation(
			req.UserID,
			req.OperationID,
			"update",
			"user_locale",
			fmt.Sprintf("%s", req.UserID),
			syncData,
			req.DeviceID,
			req.Timestamp,
		)
		
		if err != nil {
			log.Printf("Warning: Failed to record sync operation for user locale update: %v", err)
			// Don't fail the locale update for sync errors, just log warning
		} else {
			log.Printf("Successfully recorded sync operation for user locale update: user=%s, locale=%s", 
				req.UserID, req.Locale)
		}
	} else {
		log.Printf("Sync parameters not provided or incomplete, skipping sync operation recording")
	}

	// Invalidate cache after updating user locale
	if cacheManager != nil {
		err = cacheManager.InvalidateUserCache(req.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate user cache for user %s: %v", req.UserID, err)
		}
		log.Printf("✅ Cache invalidated for user: %s (locale update)", req.UserID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserLocaleResponse{
		Success: true,
		Message: "User locale updated successfully",
		Locale:  req.Locale,
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

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
