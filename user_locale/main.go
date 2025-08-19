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
	"regexp"
	"strconv"
	"strings"
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

	// Initialize/update sync operations schema on startup
	err = updateSyncOperationsSchema()
	if err != nil {
		log.Printf("Warning: Failed to update sync operations schema: %v", err)
	} else {
		log.Println("✅ Sync operations schema updated successfully")
	}

	log.Println("User Locale service initialized successfully")
}


// updateSyncOperationsSchema ensures sync_operations table has correct schema with proper constraints
// This function detects and fixes constraint violations by updating the CHECK constraint
func updateSyncOperationsSchema() error {
	log.Printf("Updating sync_operations schema to include user_locale operation types...")
	
	// Check if sync_operations table exists
	var tableExists bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='table' AND name='sync_operations'
	`).Scan(&tableExists)
	
	if err != nil {
		return fmt.Errorf("failed to check if sync_operations table exists: %v", err)
	}
	
	if !tableExists {
		log.Printf("Creating sync_operations table with proper schema...")
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS sync_operations (
				operation_id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete', 'pay', 'transfer', 'update_cash', 'update_bank')),
				entity_type TEXT NOT NULL,
				entity_id TEXT NOT NULL,
				operation_data TEXT NOT NULL,
				device_ids TEXT DEFAULT '[]',
				client_timestamp INTEGER DEFAULT 0,
				server_timestamp INTEGER DEFAULT 0
			);
			
			CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
				ON sync_operations(operation_id);
				
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
				ON sync_operations(user_id, operation_id);
				
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
				ON sync_operations(user_id, created_at);
				
			CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
				ON sync_operations(user_id, entity_type, entity_id);
		`)
		return err
	}
	
	// Test if current schema accepts user_locale operations by attempting an INSERT
	testOperationID := fmt.Sprintf("test_%d", time.Now().UnixMilli())
	testInsert := `
		INSERT INTO sync_operations (
			operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data
		) VALUES (?, 'test_user', ?, 'update', 'user_locale', 'test_id', '{}')
	`
	
	_, err = db.Exec(testInsert, testOperationID, time.Now().UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "CHECK constraint failed") {
			log.Printf("⚠️ Detected CHECK constraint violation - updating schema...")
			
			// Update schema to include user_locale operation types
			_, err = db.Exec(`
				-- Create new table with updated schema
				CREATE TABLE IF NOT EXISTS sync_operations_new (
					operation_id TEXT PRIMARY KEY,
					user_id TEXT NOT NULL,
					created_at INTEGER NOT NULL,
					operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete', 'pay', 'transfer', 'update_cash', 'update_bank')),
					entity_type TEXT NOT NULL,
					entity_id TEXT NOT NULL,
					operation_data TEXT NOT NULL,
					device_ids TEXT DEFAULT '[]',
					client_timestamp INTEGER DEFAULT 0,
					server_timestamp INTEGER DEFAULT 0
				);
				
				-- Copy existing data to new table
				INSERT OR IGNORE INTO sync_operations_new 
				SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, 
					   operation_data, device_ids, client_timestamp, server_timestamp 
				FROM sync_operations;
				
				-- Drop old table and rename new one
				DROP TABLE IF EXISTS sync_operations_old;
				ALTER TABLE sync_operations RENAME TO sync_operations_old;
				ALTER TABLE sync_operations_new RENAME TO sync_operations;
				DROP TABLE sync_operations_old;
			`)
			
			if err != nil {
				return fmt.Errorf("failed to update sync_operations schema: %v", err)
			}
			
			// Recreate indexes
			_, err = db.Exec(`
				CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
					ON sync_operations(operation_id);
					
				CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
					ON sync_operations(user_id, operation_id);
					
				CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
					ON sync_operations(user_id, created_at);
					
				CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
					ON sync_operations(user_id, entity_type, entity_id);
			`)
			
			log.Printf("✅ Successfully updated sync_operations schema")
		} else {
			return fmt.Errorf("failed to test sync_operations schema: %v", err)
		}
	} else {
		// Clean up test record
		db.Exec("DELETE FROM sync_operations WHERE operation_id = ?", testOperationID)
		log.Printf("✅ sync_operations schema is already compatible")
	}
	
	return nil
}

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number}
func isValidOperationId(operationId string) bool {
	if operationId == "" {
		return false
	}
	
	// Expected format: 1755209423000_001
	operationIdPattern := `^\\d{13}_\\d{3}$`
	matched, err := regexp.MatchString(operationIdPattern, operationId)
	if err != nil {
		log.Printf("Error validating operation ID pattern: %v", err)
		return false
	}
	
	return matched
}

// extractTimestampFromOperationId extracts timestamp from operation ID
func extractTimestampFromOperationId(operationId string) int64 {
	if !isValidOperationId(operationId) {
		return 0
	}
	
	parts := strings.Split(operationId, "_")
	if len(parts) != 2 {
		return 0
	}
	
	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		log.Printf("Error parsing timestamp from operation ID: %v", err)
		return 0
	}
	
	return timestamp
}

// getLastOperationIdForUser retrieves the last operation ID for a specific user
func getLastOperationIdForUser(userID string) (string, error) {
	var lastOperationId string
	err := db.QueryRow("SELECT operation_id FROM sync_operations WHERE user_id = ? ORDER BY operation_id DESC LIMIT 1", userID).Scan(&lastOperationId)
	
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No previous operations found for user: %s", userID)
			return "", nil
		}
		log.Printf("Error retrieving last operation ID for user %s: %v", userID, err)
		return "", err
	}
	
	log.Printf("Retrieved last operation ID for user %s: %s", userID, lastOperationId)
	return lastOperationId, nil
}

// generateNextOperationId generates the next operation ID for a user
// Gets the last operation ID and adds +1 millisecond time unit
func generateNextOperationId(userID string) (string, error) {
	log.Printf("Generating next operation ID for user: %s", userID)
	
	// Get the last operation ID for this user
	lastOperationId, err := getLastOperationIdForUser(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get last operation ID: %v", err)
	}
	
	var nextTimestamp int64
	var sequenceNumber int = 1
	
	if lastOperationId == "" {
		// No previous operations, start with current timestamp
		nextTimestamp = time.Now().UnixMilli()
		log.Printf("No previous operations, starting with timestamp: %d", nextTimestamp)
	} else {
		// Extract timestamp from last operation ID
		lastTimestamp := extractTimestampFromOperationId(lastOperationId)
		if lastTimestamp == 0 {
			// Invalid last operation ID format, use current timestamp
			nextTimestamp = time.Now().UnixMilli()
			log.Printf("Invalid last operation ID format, using current timestamp: %d", nextTimestamp)
		} else {
			// Add 1 millisecond to ensure chronological ordering
			nextTimestamp = lastTimestamp + 1
			log.Printf("Incremented timestamp from %d to %d", lastTimestamp, nextTimestamp)
		}
	}
	
	// Format as {timestamp_ms}_{sequence_number}
	operationId := fmt.Sprintf("%d_%03d", nextTimestamp, sequenceNumber)
	
	log.Printf("Generated operation ID: %s", operationId)
	return operationId, nil
}

// addSyncOperation records a sync operation in the sync_operations table
// Uses the new operation_id system with timestamp-based format and automatic generation
func addSyncOperation(userID, providedOperationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, provided_operation=%s, action=%s, table=%s, record=%s, device=%s", 
		userID, providedOperationID, action, tableName, recordID, deviceID)
	
	// Generate operation ID if not provided or if provided ID is not valid timestamp format
	var operationID string
	var err error
	
	if providedOperationID != "" && isValidOperationId(providedOperationID) {
		// Use provided operation ID if it's valid
		operationID = providedOperationID
		log.Printf("Using provided operation ID: %s", operationID)
	} else {
		// Generate new timestamp-based operation ID
		operationID, err = generateNextOperationId(userID)
		if err != nil {
			log.Printf("Error generating operation ID: %v", err)
			return fmt.Errorf("failed to generate operation ID: %v", err)
		}
		log.Printf("Generated new operation ID: %s (provided was: %s)", operationID, providedOperationID)
	}
	
	// Validate that we have a valid operation ID
	if !isValidOperationId(operationID) {
		return fmt.Errorf("invalid operation ID format: %s", operationID)
	}
	
	// Serialize operation data to JSON for storage
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling sync operation data: %v", err)
		return err
	}
	
	// Prepare device_ids JSON array - store null if deviceID is empty
	var deviceIDsJSON []byte
	if deviceID != "" {
		deviceIDs := []string{deviceID}
		deviceIDsJSON, err = json.Marshal(deviceIDs)
		if err != nil {
			log.Printf("Error marshaling device_ids: %v", err)
			return err
		}
	} else {
		deviceIDsJSON = []byte("null")
		log.Printf("Device ID empty, storing null in device_ids column")
	}
	
	// Extract timestamp from operation ID for created_at field
	operationTimestamp := extractTimestampFromOperationId(operationID)
	if operationTimestamp == 0 {
		operationTimestamp = time.Now().UnixMilli()
		log.Printf("Warning: Could not extract timestamp from operation ID, using current timestamp: %d", operationTimestamp)
	}
	
	// Use current server timestamp
	serverTimestamp := time.Now().UnixMilli()
	
	// Handle client timestamp - use null if 0
	var clientTimestampValue interface{}
	if clientTimestamp == 0 {
		clientTimestampValue = nil
		log.Printf("Client timestamp is 0, storing null in client_timestamp column")
	} else {
		clientTimestampValue = clientTimestamp
	}
	
	// Insert sync operation record with operation_id-based ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, operation_type, entity_type, entity_id, operation_data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	result, err := db.Exec(
		insertQuery,
		userID,
		operationID,
		action,            // operation_type (create, update, delete, pay)
		tableName,         // entity_type (user_locale)
		recordID,          // entity_id
		string(dataJSON),  // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or null
		clientTimestampValue,  // client_timestamp (original from client or null)
		serverTimestamp,   // server_timestamp (when processed)
		operationTimestamp, // created_at (extracted from operation_id for ordering)
	)
	
	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}
	
	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	log.Printf("Successfully added sync operation with ID: %d, operation_id: %s, timestamp: %d", 
		syncOpID, operationID, operationTimestamp)
	
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

	// Record sync operation with auto-generated operation_id for consistency
	// Following the implementation guide: ALL handlers must use same pattern
	log.Printf("✅ Recording sync operation for user locale update with auto-generated operation_id")
	
	// Create sync operation data for user locale update
	syncData := map[string]interface{}{
		"user_id": req.UserID,
		"locale":  req.Locale,
		"action":  "update_locale",
		"processed_at": time.Now().Format("2006-01-02 15:04:05"),
	}
	
	// Add sync operation record to database with auto-generated operation_id
	err = addSyncOperation(
		req.UserID,
		"", // Empty operation_id triggers auto-generation
		"update",
		"user_locale",
		fmt.Sprintf("%s", req.UserID),
		syncData,
		req.DeviceID, // Use device_id from request
		0, // Timestamp auto-generated
	)
	
	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for user locale update: %v", err)
		// Don't fail the locale update for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for user locale update: user=%s, locale=%s", 
			req.UserID, req.Locale)
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
