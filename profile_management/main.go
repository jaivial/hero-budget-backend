package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
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
	"github.com/nfnt/resize"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// Definición de estructuras de datos
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

type ProfileUpdateRequest struct {
	UserID          int    `json:"user_id"`
	Name            string `json:"name,omitempty"`
	GivenName       string `json:"given_name,omitempty"`
	FamilyName      string `json:"family_name,omitempty"`
	ProfileImageB64 string `json:"profile_image_base64,omitempty"`
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

type PasswordUpdateRequest struct {
	UserID      int    `json:"user_id"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type LocaleUpdateRequest struct {
	UserID string `json:"user_id"`
	Locale string `json:"locale"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	// Database connection for profile data persistence
	db *sql.DB
	// Context for database operations
	ctx = context.Background()
	// Cache manager for Redis operations to improve performance
	cacheManager *common.CacheManager
)

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
		dbPath = getEnvOrDefault("DB_DEV_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to production database to match other services
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🔧 Running in DEFAULT mode - Database: %s", dbPath)
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

	log.Println("Profile Management service initialized successfully")

	// Initialize or update sync operations table schema
	if err := updateSyncOperationsSchema(); err != nil {
		log.Printf("Warning: Failed to initialize sync operations schema: %v", err)
	} else {
		log.Println("✅ Sync operations schema initialized successfully")
	}
}

// Operation ID generation utilities following the sync operations implementation guide

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number}
func isValidOperationId(operationId string) bool {
	if operationId == "" {
		return false
	}

	// Expected format: 1755209423000_001
	operationIdPattern := `^\d{13}_\d{3}$`
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

// updateSyncOperationsSchema ensures the sync_operations table exists with proper constraints
func updateSyncOperationsSchema() error {
	// Create the sync_operations table if it doesn't exist
	createTableSQL := `
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
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create sync_operations table: %v", err)
	}

	// Create necessary indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id ON sync_operations(operation_id);",
		"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation ON sync_operations(user_id, operation_id);",
		"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created ON sync_operations(user_id, created_at);",
		"CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity ON sync_operations(user_id, entity_type, entity_id);",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}

	return nil
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

	// Log all parameters before insertion
	log.Printf("DEBUG: About to insert sync operation with parameters:")
	log.Printf("  userID: %s", userID)
	log.Printf("  operationID: %s", operationID)
	log.Printf("  action: %s", action)
	log.Printf("  tableName: %s", tableName)
	log.Printf("  recordID: %s", recordID)
	log.Printf("  dataJSON: %s", string(dataJSON))
	log.Printf("  deviceIDsJSON: %s", string(deviceIDsJSON))
	log.Printf("  clientTimestampValue: %v", clientTimestampValue)
	log.Printf("  serverTimestamp: %d", serverTimestamp)
	log.Printf("  operationTimestamp: %d", operationTimestamp)

	// Insert sync operation record with operation_id-based ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, operation_type, entity_type, entity_id, operation_data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	log.Printf("DEBUG: Executing SQL query: %s", insertQuery)

	result, err := db.Exec(
		insertQuery,
		userID,
		operationID,
		action,                // operation_type (create, update, delete, pay)
		tableName,             // entity_type (profile_picture, profile_info)
		recordID,              // entity_id
		string(dataJSON),      // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or null
		clientTimestampValue,  // client_timestamp (original from client or null)
		serverTimestamp,       // server_timestamp (when processed)
		operationTimestamp,    // created_at (extracted from operation_id for ordering)
	)

	if err != nil {
		log.Printf("ERROR: Failed to insert sync operation: %v", err)
		log.Printf("ERROR: SQL was: %s", insertQuery)
		return err
	}

	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	rowsAffected, _ := result.RowsAffected()
	log.Printf("Successfully added sync operation with ID: %d, operation_id: %s, timestamp: %d, rows_affected: %d",
		syncOpID, operationID, operationTimestamp, rowsAffected)

	// Verify the record was actually inserted by querying it back
	var count int
	countQuery := "SELECT COUNT(*) FROM sync_operations WHERE operation_id = ?"
	err = db.QueryRow(countQuery, operationID).Scan(&count)
	if err != nil {
		log.Printf("WARNING: Failed to verify sync operation insertion: %v", err)
	} else {
		log.Printf("VERIFICATION: Found %d records with operation_id: %s", count, operationID)
		if count == 0 {
			log.Printf("ERROR: Sync operation was not found after insertion!")
		}
	}

	return nil
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/profile/update", corsMiddleware(handleProfileUpdate))
	http.HandleFunc("/profile/update-password", corsMiddleware(handlePasswordUpdate))
	http.HandleFunc("/profile/ping", corsMiddleware(handlePing))
	http.HandleFunc("/profile/test-image-update", corsMiddleware(handleTestImageUpdate))
	http.HandleFunc("/profile/editprofilepicture", corsMiddleware(handleEditProfilePicture))
	http.HandleFunc("/update/locale", corsMiddleware(handleLocaleUpdate))
	http.HandleFunc("/profile/delete-account", corsMiddleware(handleDeleteAccount))
	http.HandleFunc("/user/info", corsMiddleware(handleGetUserInfo))
	http.HandleFunc("/user/update", corsMiddleware(handleUpdateUser))

	port := 8092 // Asignamos el puerto 8092 para el servicio de profile_management
	log.Printf("Profile Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

// Endpoint para pruebas de actualización de imagen de perfil
func handleTestImageUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Test Image Update: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("TEST IMAGE UPDATE: Recibida solicitud para usuario ID: %d", req.UserID)
	log.Printf("TEST IMAGE UPDATE: Tamaño de imagen recibida: %d bytes", len(req.ProfileImageB64))

	// Verificar que el usuario existe
	var user User
	if err := getUserById(req.UserID, &user); err != nil {
		log.Printf("TEST IMAGE UPDATE: Usuario no encontrado: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Printf("TEST IMAGE UPDATE: Información de usuario antes de la actualización:")
	log.Printf("ID: %d, Email: %s, Nombre: %s", user.ID, user.Email, user.Name)
	if user.ProfileImageBlob != nil {
		log.Printf("ProfileImageBlob presente: %d bytes", len(*user.ProfileImageBlob))
	} else {
		log.Printf("ProfileImageBlob es NULL")
	}

	if user.Picture != nil {
		log.Printf("Picture presente: %s", *user.Picture)
	} else {
		log.Printf("Picture es NULL")
	}

	// Procesar la imagen
	if req.ProfileImageB64 != "" {
		processedImage, err := processImage(req.ProfileImageB64)
		if err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al procesar imagen: %v", err)
			http.Error(w, fmt.Sprintf("Error processing image: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("TEST IMAGE UPDATE: Imagen procesada: %d bytes", len(processedImage))

		// Actualizar directamente la imagen de perfil en la base de datos
		result, err := db.Exec("UPDATE users SET profile_image_blob = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			processedImage, req.UserID)

		if err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al actualizar imagen en la base de datos: %v", err)
			http.Error(w, "Database update failed", http.StatusInternalServerError)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("TEST IMAGE UPDATE: Actualización exitosa, filas afectadas: %d", rowsAffected)

		// Verificar el estado después de la actualización
		var updatedUser User
		if err := getUserById(req.UserID, &updatedUser); err != nil {
			log.Printf("TEST IMAGE UPDATE: Error al obtener usuario actualizado: %v", err)
		} else {
			if updatedUser.ProfileImageBlob != nil {
				log.Printf("TEST IMAGE UPDATE: Después de actualizar, ProfileImageBlob presente: %d bytes",
					len(*updatedUser.ProfileImageBlob))
			} else {
				log.Printf("TEST IMAGE UPDATE: Después de actualizar, ProfileImageBlob sigue siendo NULL")
			}
		}

		// Responder con éxito
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: true,
			Message: fmt.Sprintf("Test image update successful. Processed image size: %d bytes, Rows affected: %d",
				len(processedImage), rowsAffected),
			Data: updatedUser,
		})
		return
	}

	http.Error(w, "No image data provided", http.StatusBadRequest)
}

/**
 * handleEditProfilePicture - Dedicated endpoint for profile picture updates
 *
 * This endpoint is specifically designed to handle profile picture updates
 * with optimized processing and focused error handling for image operations.
 *
 * Request format:
 * POST /profile/editprofilepicture
 * {
 *   "user_id": 123,
 *   "profile_image_base64": "base64_encoded_image_data"
 * }
 *
 * Response format:
 * {
 *   "success": true,
 *   "message": "Profile picture updated successfully",
 *   "data": {updated_user_object}
 * }
 */
func handleEditProfilePicture(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("EDIT PROFILE PICTURE: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.UserID <= 0 {
		log.Printf("EDIT PROFILE PICTURE: Invalid user ID: %d", req.UserID)
		http.Error(w, "Valid user ID is required", http.StatusBadRequest)
		return
	}

	// Handle both profile picture updates and removals
	isRemoval := req.ProfileImageB64 == ""
	if isRemoval {
		log.Printf("🗑️ EDIT PROFILE PICTURE: Processing profile picture REMOVAL for user %d", req.UserID)
	} else {
		log.Printf("🖼️ EDIT PROFILE PICTURE: Processing profile picture UPDATE for user %d", req.UserID)
	}

	log.Printf("🖼️🔍 EDIT PROFILE PICTURE: DETAILED REQUEST PROCESSING:")
	log.Printf("  📋 User ID: %d (type: %T)", req.UserID, req.UserID)
	log.Printf("  📊 Image data length: %d bytes (%.2f KB)", len(req.ProfileImageB64), float64(len(req.ProfileImageB64))/1024)
	log.Printf("  🎯 Request timestamp: %s", time.Now().Format("2006-01-02 15:04:05.000 MST"))

	// Log first 100 characters of image data for debugging
	if len(req.ProfileImageB64) > 100 {
		log.Printf("  📸 Image data preview: %s...", req.ProfileImageB64[:100])
	} else {
		log.Printf("  📸 Full image data: %s", req.ProfileImageB64)
	}

	// Verify user exists in database
	var userExists int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&userExists)
	if err != nil {
		log.Printf("EDIT PROFILE PICTURE: Database error checking user: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if userExists == 0 {
		log.Printf("EDIT PROFILE PICTURE: User not found: %d", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var processedImage *string // Use pointer to handle NULL for removal

	if isRemoval {
		// Profile picture removal - set to NULL
		log.Printf("🗑️✅ EDIT PROFILE PICTURE: PROCESSING REMOVAL:")
		log.Printf("  🎯 Setting profile_image_blob to NULL for user %d", req.UserID)
		processedImage = nil
	} else {
		// Profile picture update - validate and use provided image
		log.Printf("🖼️✅ EDIT PROFILE PICTURE: USING PRE-PROCESSED IMAGE FROM FRONTEND:")
		log.Printf("  📥 Frontend processed image data: %d bytes (%.2f KB)", len(req.ProfileImageB64), float64(len(req.ProfileImageB64))/1024)
		log.Printf("  🎯 Skipping backend processing - frontend already optimized the image")
		log.Printf("  📸 Image data preview: %s...", req.ProfileImageB64[:min(50, len(req.ProfileImageB64))])

		// Basic validation of image data
		if len(req.ProfileImageB64) == 0 {
			log.Printf("❌📸 EDIT PROFILE PICTURE: IMAGE DATA IS EMPTY")
			http.Error(w, "Image data is empty", http.StatusBadRequest)
			return
		}

		// Basic validation that it's base64 data (removed format-specific restrictions)
		if len(strings.TrimSpace(req.ProfileImageB64)) < 10 {
			log.Printf("❌📸 EDIT PROFILE PICTURE: INVALID IMAGE DATA - TOO SHORT")
			log.Printf("  🔍 Image data preview: %s...", req.ProfileImageB64[:min(100, len(req.ProfileImageB64))])
			http.Error(w, "Invalid image data format", http.StatusBadRequest)
			return
		}

		// Use the frontend-processed image directly
		processedImage = &req.ProfileImageB64
		log.Printf("✅📸 EDIT PROFILE PICTURE: VALIDATED FRONTEND-PROCESSED IMAGE")
		log.Printf("  📊 Final image size: %d bytes (%.2f KB)", len(*processedImage), float64(len(*processedImage))/1024)
	}

	// Update profile_image_blob in database
	log.Printf("💾🔄 EDIT PROFILE PICTURE: STARTING DATABASE UPDATE...")
	log.Printf("  🎯 Updating user ID: %d", req.UserID)
	if processedImage != nil {
		log.Printf("  📊 Image data to store: %d bytes", len(*processedImage))
	} else {
		log.Printf("  🗑️ Setting profile image to NULL (removal)")
	}

	result, err := db.Exec(
		"UPDATE users SET profile_image_blob = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		processedImage, req.UserID,
	)
	if err != nil {
		log.Printf("❌💾 EDIT PROFILE PICTURE: DATABASE UPDATE FAILED:")
		log.Printf("  💥 Error: %v", err)
		log.Printf("  🆔 User ID: %d", req.UserID)
		if processedImage != nil {
			log.Printf("  📊 Data size: %d bytes", len(*processedImage))
		} else {
			log.Printf("  🗑️ Data: NULL (removal)")
		}
		log.Printf("  🕐 Failed at: %s", time.Now().Format("2006-01-02 15:04:05.000 MST"))
		http.Error(w, "Failed to update profile picture in database", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅💾 EDIT PROFILE PICTURE: DATABASE UPDATE SUCCESSFUL:")
	log.Printf("  📊 Rows affected: %d", rowsAffected)
	log.Printf("  🆔 User ID: %d", req.UserID)
	if processedImage != nil {
		log.Printf("  📸 Image size stored: %d bytes", len(*processedImage))
	} else {
		log.Printf("  🗑️ Profile image set to NULL (removed)")
	}
	log.Printf("  🕐 Updated at: %s", time.Now().Format("2006-01-02 15:04:05.000 MST"))

	// Retrieve updated user data
	log.Printf("📋🔍 EDIT PROFILE PICTURE: RETRIEVING UPDATED USER DATA...")
	var updatedUser User
	if err := getUserById(req.UserID, &updatedUser); err != nil {
		log.Printf("⚠️📋 EDIT PROFILE PICTURE: WARNING - FAILED TO RETRIEVE UPDATED USER:")
		log.Printf("  💥 Error: %v", err)
		log.Printf("  🆔 User ID: %d", req.UserID)

		// Still return success since the update worked
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: true,
			Message: "Profile picture updated successfully (verification failed)",
		})
		return
	}

	// Verify the update was successful by checking the profile_image_blob
	log.Printf("🔍✅ EDIT PROFILE PICTURE: VERIFYING UPDATE SUCCESS...")
	if updatedUser.ProfileImageBlob != nil && len(*updatedUser.ProfileImageBlob) > 0 {
		log.Printf("✅🎉 EDIT PROFILE PICTURE: UPDATE VERIFICATION SUCCESSFUL:")
		log.Printf("  📊 Profile image blob size: %d bytes (%.2f KB)", len(*updatedUser.ProfileImageBlob), float64(len(*updatedUser.ProfileImageBlob))/1024)
		log.Printf("  📸 Image data preview: %s...", (*updatedUser.ProfileImageBlob)[:min(50, len(*updatedUser.ProfileImageBlob))])
	} else {
		log.Printf("⚠️❌ EDIT PROFILE PICTURE: UPDATE VERIFICATION FAILED:")
		log.Printf("  💥 Profile image blob appears to be empty after update")
		log.Printf("  🔍 ProfileImageBlob is nil: %v", updatedUser.ProfileImageBlob == nil)
		if updatedUser.ProfileImageBlob != nil {
			log.Printf("  📊 ProfileImageBlob length: %d", len(*updatedUser.ProfileImageBlob))
		}
	}

	// Record sync operation with auto-generated operation_id if needed
	// Following the consistent sync recording pattern from implementation guide
	log.Printf("🔄 DEBUG: Starting sync operation recording for profile picture update")
	log.Printf("🔄 DEBUG: Request sync params - OperationID='%s', DeviceID='%s', Timestamp=%d",
		req.OperationID, req.DeviceID, req.Timestamp)

	// Prepare sync operation data for recording
	// IMPORTANT: Include the actual image data so other devices can sync the complete profile picture
	var syncOperationData map[string]interface{}

	if isRemoval {
		// Profile picture removal
		syncOperationData = map[string]interface{}{
			"user_id":              req.UserID,
			"action":               "update",
			"type":                 "profile_picture",
			"profile_image_base64": "", // Empty string indicates removal
			"image_size":           0,
			"processed_at":         time.Now().Format("2006-01-02 15:04:05"),
			"has_profile_image":    false,
		}
		log.Printf("🗑️ SYNC: Recording profile picture REMOVAL operation")
	} else {
		// Profile picture update
		syncOperationData = map[string]interface{}{
			"user_id":              req.UserID,
			"action":               "update",
			"type":                 "profile_picture",
			"profile_image_base64": *processedImage, // Include the actual image data for sync
			"image_size":           len(*processedImage),
			"processed_at":         time.Now().Format("2006-01-02 15:04:05"),
			"has_profile_image":    true,
		}
		log.Printf("🖼️ SYNC: Recording profile picture UPDATE operation (image size: %d bytes)", len(*processedImage))
	}

	log.Printf("🔄 DEBUG: Calling addSyncOperation with params: userID=%d, operationID='%s', deviceID='%s'",
		req.UserID, req.OperationID, req.DeviceID)

	// Record the sync operation with auto-generation if operation_id not provided
	err = addSyncOperation(
		fmt.Sprintf("%d", req.UserID),
		req.OperationID, // May be empty, will auto-generate if needed
		"update",
		"profile_picture",
		fmt.Sprintf("%d", req.UserID),
		syncOperationData,
		req.DeviceID,
		req.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for profile picture update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for profile picture update")
	}

	// Return success response with updated user data
	log.Printf("📤🎉 EDIT PROFILE PICTURE: SENDING SUCCESS RESPONSE...")
	w.Header().Set("Content-Type", "application/json")

	var message string
	if isRemoval {
		message = "Profile picture removed successfully"
	} else {
		message = fmt.Sprintf("Profile picture updated successfully. Image size: %d bytes", len(*processedImage))
	}

	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    updatedUser,
	}

	log.Printf("📋📤 EDIT PROFILE PICTURE: RESPONSE DETAILS:")
	log.Printf("  ✅ Success: true")
	log.Printf("  📝 Message: %s", message)
	log.Printf("  👤 User ID: %d", updatedUser.ID)
	log.Printf("  📧 Email: %s", updatedUser.Email)
	log.Printf("  👨 Name: %s", updatedUser.Name)
	log.Printf("  🖼️ Has profile image: %v", updatedUser.ProfileImageBlob != nil && len(*updatedUser.ProfileImageBlob) > 0)

	json.NewEncoder(w).Encode(response)
	log.Printf("🎉✅ EDIT PROFILE PICTURE: REQUEST COMPLETED SUCCESSFULLY FOR USER %d", req.UserID)
}

func handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProfileUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 {
		http.Error(w, "Valid user ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating profile for user ID: %d", req.UserID)

	// Verify user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&count)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		log.Printf("User not found: %d", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Process the image if provided
	var processedImageBase64 *string
	if req.ProfileImageB64 != "" {
		log.Printf("Received image data for processing (length: %d bytes)", len(req.ProfileImageB64))
		processedImage, err := processImage(req.ProfileImageB64)
		if err != nil {
			log.Printf("❌ CRITICAL ERROR: Failed to process image: %v", err)
			// Return error instead of continuing - this was causing the silent failure
			http.Error(w, fmt.Sprintf("Image processing failed: %v", err), http.StatusBadRequest)
			return
		} else {
			processedImageBase64 = &processedImage
			log.Printf("✅ Successfully processed and compressed profile image (processed length: %d bytes)", len(processedImage))

			// Verificamos que la imagen procesada no esté vacía
			if len(processedImage) == 0 {
				log.Printf("❌ CRITICAL ERROR: La imagen procesada está vacía")
				http.Error(w, "Processed image is empty", http.StatusBadRequest)
				return
			}
		}
	}

	// Build the update query dynamically based on provided fields
	updateQuery := "UPDATE users SET updated_at = CURRENT_TIMESTAMP"
	var params []interface{}

	if req.Name != "" {
		updateQuery += ", name = ?"
		params = append(params, req.Name)
	}

	if req.GivenName != "" {
		updateQuery += ", given_name = ?"
		params = append(params, req.GivenName)
	}

	if req.FamilyName != "" {
		updateQuery += ", family_name = ?"
		params = append(params, req.FamilyName)
	}

	if processedImageBase64 != nil {
		updateQuery += ", profile_image_blob = ?"
		params = append(params, *processedImageBase64)
	}

	if len(params) == 0 {
		// No fields to update
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: false,
			Message: "No fields to update",
		})
		return
	}

	// Add the WHERE clause and user ID parameter
	updateQuery += " WHERE id = ?"
	params = append(params, req.UserID)

	// Log debug information
	log.Printf("Update query: %s", updateQuery)
	log.Printf("Update params count: %d", len(params))

	// Debug log for image parameter
	for i, p := range params {
		if s, ok := p.(string); ok && len(s) > 100 {
			log.Printf("Param %d: %s... (length: %d)", i, s[:min(100, len(s))], len(s))
		} else if s, ok := p.(string); ok {
			log.Printf("Param %d: %s (length: %d)", i, s, len(s))
		} else {
			log.Printf("Param %d: %v", i, p)
		}
	}

	// Execute the update directly instead of using a prepared statement
	result, err := db.Exec(updateQuery, params...)
	if err != nil {
		log.Printf("Failed to execute update: %v", err)

		// Intentar una actualización separada solo para la imagen si es lo que falló
		if processedImageBase64 != nil {
			log.Printf("Attempting separate image update")

			// Intentar una actualización directa solo de la imagen
			imgResult, imgErr := db.Exec(
				"UPDATE users SET profile_image_blob = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
				*processedImageBase64, req.UserID,
			)

			if imgErr != nil {
				log.Printf("Separate image update also failed: %v", imgErr)
			} else {
				imgRows, _ := imgResult.RowsAffected()
				log.Printf("Separate image update succeeded! Rows affected: %d", imgRows)

				// Si la actualización de la imagen fue exitosa, actualizar el resto de los campos si es necesario
				if req.Name != "" || req.GivenName != "" || req.FamilyName != "" {
					otherFieldsQuery := "UPDATE users SET updated_at = CURRENT_TIMESTAMP"
					var otherParams []interface{}

					if req.Name != "" {
						otherFieldsQuery += ", name = ?"
						otherParams = append(otherParams, req.Name)
					}

					if req.GivenName != "" {
						otherFieldsQuery += ", given_name = ?"
						otherParams = append(otherParams, req.GivenName)
					}

					if req.FamilyName != "" {
						otherFieldsQuery += ", family_name = ?"
						otherParams = append(otherParams, req.FamilyName)
					}

					otherFieldsQuery += " WHERE id = ?"
					otherParams = append(otherParams, req.UserID)

					_, otherErr := db.Exec(otherFieldsQuery, otherParams...)
					if otherErr != nil {
						log.Printf("Error updating other fields: %v", otherErr)
					} else {
						log.Printf("Other fields updated successfully")
					}
				}

				// Get the updated user
				var user User
				err = getUserById(req.UserID, &user)
				if err == nil {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(ApiResponse{
						Success: true,
						Message: "Profile partially updated successfully (only image)",
						Data:    user,
					})
					return
				}
			}
		}

		http.Error(w, "Failed to update user profile", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Successfully updated user %d, %d rows affected", req.UserID, rowsAffected)

	// Record sync operation with auto-generated operation_id if needed
	// Following the consistent sync recording pattern from implementation guide
	log.Printf("🔄 DEBUG: Starting sync operation recording for profile info update")
	log.Printf("🔄 DEBUG: Request sync params - OperationID='%s', DeviceID='%s', Timestamp=%d",
		req.OperationID, req.DeviceID, req.Timestamp)

	// Prepare sync operation data for recording
	syncOperationData := map[string]interface{}{
		"user_id":      req.UserID,
		"name":         req.Name,
		"given_name":   req.GivenName,
		"family_name":  req.FamilyName,
		"action":       "update",
		"processed_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// Include profile image info if it was updated
	if req.ProfileImageB64 != "" {
		syncOperationData["has_profile_image"] = true
		syncOperationData["image_size"] = len(req.ProfileImageB64)
	}

	log.Printf("🔄 DEBUG: Calling addSyncOperation with params: userID=%d, operationID='%s', deviceID='%s'",
		req.UserID, req.OperationID, req.DeviceID)

	// Record the sync operation with auto-generation if operation_id not provided
	err = addSyncOperation(
		fmt.Sprintf("%d", req.UserID),
		req.OperationID, // May be empty, will auto-generate if needed
		"update",
		"profile_info",
		fmt.Sprintf("%d", req.UserID),
		syncOperationData,
		req.DeviceID,
		req.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for profile info update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for profile info update")
	}

	// Get the updated user to return in the response
	var user User
	err = getUserById(req.UserID, &user)
	if err != nil {
		log.Printf("Error retrieving updated user: %v", err)
		// Still return success even if we can't fetch the updated user
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: true,
			Message: "Profile updated successfully",
		})
		return
	}

	// Return the updated user info
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Profile updated successfully",
		Data:    user,
	})
}

func handlePasswordUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PasswordUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID <= 0 || req.OldPassword == "" || req.NewPassword == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating password for user ID: %d", req.UserID)

	// Verify old password
	var currentPassword string
	err := db.QueryRow("SELECT password FROM users WHERE id = ?", req.UserID).Scan(&currentPassword)

	if err == sql.ErrNoRows {
		log.Printf("User not found: %d", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Verify old password matches
	// In a real app, you would compare hashed passwords
	if currentPassword != req.OldPassword {
		log.Printf("Incorrect password for user ID: %d", req.UserID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ApiResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
		return
	}

	// Update password
	_, err = db.Exec("UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		req.NewPassword, req.UserID)

	if err != nil {
		log.Printf("Failed to update password: %v", err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	log.Printf("Password updated successfully for user ID: %d", req.UserID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: "Password updated successfully",
	})
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "Profile Management service is running",
	})
}

func handleLocaleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LocaleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Locale == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	log.Printf("Updating locale for user ID: %s", req.UserID)

	// Verify user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", req.UserID).Scan(&count)
	if err != nil {
		log.Printf("Database error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		log.Printf("User not found: %s", req.UserID)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Update locale
	_, err = db.Exec("UPDATE users SET locale = ? WHERE id = ?", req.Locale, req.UserID)
	if err != nil {
		log.Printf("Failed to update locale: %v", err)
		http.Error(w, "Failed to update locale", http.StatusInternalServerError)
		return
	}

	log.Printf("Locale updated successfully for user ID: %s", req.UserID)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	response := ApiResponse{
		Success: true,
		Message: "Locale updated successfully",
		Data:    req,
	}
	json.NewEncoder(w).Encode(response)
}

// DeleteAccountRequest structure for handling account deletion requests
type DeleteAccountRequest struct {
	UserID int `json:"user_id"`
}

// handleDeleteAccount handles the complete deletion of user account and all associated data
func handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Delete Account: Invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("DELETE ACCOUNT: Iniciando eliminación completa para usuario ID: %d", req.UserID)

	// Verificar que el usuario existe antes de eliminarlo
	var user User
	if err := getUserById(req.UserID, &user); err != nil {
		log.Printf("DELETE ACCOUNT: Usuario no encontrado: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	log.Printf("DELETE ACCOUNT: Usuario encontrado: %s (%d)", user.Email, user.ID)

	// Iniciar transacción para garantizar atomicidad
	tx, err := db.Begin()
	if err != nil {
		log.Printf("DELETE ACCOUNT: Error iniciando transacción: %v", err)
		http.Error(w, "Database transaction failed", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Lista de tablas a limpiar (orden importante por las foreign keys)
	tables := []string{
		"categories",
		"cash_bank_transactions",
		"cash_bank",
		"daily_balance",
		"weekly_balance",
		"monthly_balance",
		"daily_cash_bank_balance",
		"weekly_cash_bank_balance",
		"monthly_cash_bank_balance",
		"bills",
		"expenses",
		"incomes",
		"savings",
		"balances",
		"users",
	}

	userIDStr := fmt.Sprintf("%d", req.UserID)

	// Eliminar registros de todas las tablas
	for _, table := range tables {
		var query string

		// Para la tabla users, usar 'id' como campo
		if table == "users" {
			query = fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
		} else {
			// Para el resto de tablas, usar 'user_id' como campo
			query = fmt.Sprintf("DELETE FROM %s WHERE user_id = ?", table)
		}

		result, err := tx.Exec(query, userIDStr)
		if err != nil {
			log.Printf("DELETE ACCOUNT: Error eliminando de tabla %s: %v", table, err)
			http.Error(w, fmt.Sprintf("Failed to delete from %s", table), http.StatusInternalServerError)
			return
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("DELETE ACCOUNT: Eliminados %d registros de tabla %s", rowsAffected, table)
	}

	// Confirmar la transacción
	if err := tx.Commit(); err != nil {
		log.Printf("DELETE ACCOUNT: Error confirmando transacción: %v", err)
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	log.Printf("DELETE ACCOUNT: Eliminación completa exitosa para usuario %s (%d)", user.Email, user.ID)

	// Responder con éxito
	w.Header().Set("Content-Type", "application/json")
	response := ApiResponse{
		Success: true,
		Message: fmt.Sprintf("Cuenta y todos los datos del usuario %s eliminados exitosamente", user.Email),
	}
	json.NewEncoder(w).Encode(response)
}

func getUserById(userID int, user *User) error {
	err := db.QueryRow(`
		SELECT id, google_id, email, name, given_name, family_name, 
		picture, profile_image_blob, locale, verified_email, created_at, updated_at 
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
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return err
	}

	// Set the display image based on the user type
	if user.GoogleID != nil && *user.GoogleID != "" {
		// Google user - use Picture URL field
		if user.Picture != nil && *user.Picture != "" {
			user.DisplayImage = *user.Picture
		}
	} else {
		// Regular user - use ProfileImageBlob field
		if user.ProfileImageBlob != nil && *user.ProfileImageBlob != "" {
			user.DisplayImage = *user.ProfileImageBlob
		}
	}

	return nil
}

// Process image: resize, compress, and convert to WebP
func processImage(base64Image string) (string, error) {
	log.Printf("🔄 IMAGE PROCESSING: Starting image processing, input length: %d", len(base64Image))

	// Debug: Log first 100 characters of input
	if len(base64Image) > 100 {
		log.Printf("🔍 IMAGE PROCESSING: Input preview: %s...", base64Image[:100])
	} else {
		log.Printf("🔍 IMAGE PROCESSING: Full input: %s", base64Image)
	}

	// Extract the actual base64 content from the data URL
	base64Data := base64Image
	if idx := strings.Index(base64Image, ";base64,"); idx > 0 {
		base64Data = base64Image[idx+8:]
		log.Printf("Extracted base64 data after prefix, new length: %d", len(base64Data))
	}

	// Check if the base64 string is valid
	if len(base64Data) == 0 {
		return "", fmt.Errorf("empty base64 image data")
	}

	// Clean the base64 string - remove any whitespace or newlines
	base64Data = strings.ReplaceAll(base64Data, "\n", "")
	base64Data = strings.ReplaceAll(base64Data, "\r", "")
	base64Data = strings.ReplaceAll(base64Data, " ", "")

	// Ensure padding is correct (base64 string length must be multiple of 4)
	for len(base64Data)%4 != 0 {
		base64Data += "="
	}

	log.Printf("Cleaned base64 data, final length: %d", len(base64Data))

	// Decode base64 image
	log.Printf("🔄 IMAGE PROCESSING: Attempting base64 decode...")
	imgData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		log.Printf("❌ IMAGE PROCESSING: Base64 decode error: %v", err)
		// If standard decoding fails, try URL-safe decoding
		log.Printf("🔄 IMAGE PROCESSING: Trying URL-safe base64 decoding...")
		imgData, err = base64.URLEncoding.DecodeString(base64Data)
		if err != nil {
			log.Printf("❌ IMAGE PROCESSING: URL-safe base64 decode also failed: %v", err)
			return "", fmt.Errorf("failed to decode base64 image (tried standard and URL-safe): %v", err)
		}
		log.Printf("✅ IMAGE PROCESSING: Successfully decoded using URL-safe base64")
	} else {
		log.Printf("✅ IMAGE PROCESSING: Successfully decoded using standard base64")
	}

	log.Printf("🔍 IMAGE PROCESSING: Decoded image data size: %d bytes", len(imgData))

	// Determine image format and decode
	log.Printf("🔄 IMAGE PROCESSING: Attempting image decode...")
	imgReader := bytes.NewReader(imgData)
	img, format, err := image.Decode(imgReader)
	if err != nil {
		log.Printf("❌ IMAGE PROCESSING: Generic image decode failed: %v", err)
		// Try to handle JPEG specifically if the generic decode fails
		log.Printf("🔄 IMAGE PROCESSING: Trying JPEG decode...")
		imgReader.Seek(0, 0) // Reset reader
		img, err = jpeg.Decode(imgReader)
		if err != nil {
			log.Printf("❌ IMAGE PROCESSING: JPEG decode failed: %v", err)
			// Try to handle PNG specifically if JPEG decode also fails
			log.Printf("🔄 IMAGE PROCESSING: Trying PNG decode...")
			imgReader.Seek(0, 0) // Reset reader
			img, err = png.Decode(imgReader)
			if err != nil {
				log.Printf("❌ IMAGE PROCESSING: PNG decode also failed: %v", err)
				return "", fmt.Errorf("failed to decode image (tried generic, JPEG, and PNG formats): %v", err)
			}
			format = "png"
			log.Printf("✅ IMAGE PROCESSING: PNG decode successful")
		} else {
			format = "jpeg"
			log.Printf("✅ IMAGE PROCESSING: JPEG decode successful")
		}
	} else {
		log.Printf("✅ IMAGE PROCESSING: Generic image decode successful")
	}

	log.Printf("📊 IMAGE PROCESSING: Image format: %s, size: %d KB", format, len(imgData)/1024)

	// Resize the image if it's too large
	// Calculate resize dimensions while maintaining aspect ratio
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var maxWidth uint = 800
	var maxHeight uint = 800

	if width > height && width > int(maxWidth) {
		img = resize.Resize(maxWidth, 0, img, resize.Lanczos3)
	} else if height > int(maxHeight) {
		img = resize.Resize(0, maxHeight, img, resize.Lanczos3)
	}

	// Instead of WebP (which might have compatibility issues), use standard JPEG for better compatibility
	var jpegBuf bytes.Buffer
	err = jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 80})
	if err != nil {
		log.Printf("Failed to encode JPEG: %v, falling back to PNG", err)
		// If JPEG encoding fails, try PNG as fallback
		var pngBuf bytes.Buffer
		err = png.Encode(&pngBuf, img)
		if err != nil {
			return "", fmt.Errorf("failed to encode both JPEG and PNG: %v", err)
		}
		log.Printf("Successfully encoded as PNG, size: %d KB", pngBuf.Len()/1024)
		// Convert back to base64
		encodedImage := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
		log.Printf("Final encoded PNG image size: %d bytes", len(encodedImage))
		return encodedImage, nil
	}

	// Check if the compressed image is still too large (>100KB)
	compressedSize := jpegBuf.Len()
	log.Printf("Compressed JPEG size: %d KB", compressedSize/1024)

	// If still too large, compress more
	if compressedSize > 100*1024 {
		jpegBuf.Reset()
		quality := 70
		for compressedSize > 100*1024 && quality > 30 {
			jpegBuf.Reset()
			err = jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: quality})
			if err != nil {
				return "", fmt.Errorf("failed to encode JPEG with quality %d: %v", quality, err)
			}
			compressedSize = jpegBuf.Len()
			quality -= 10
			log.Printf("Recompressed JPEG size: %d KB (quality: %d)", compressedSize/1024, quality)
		}
	}

	// Convert back to base64 with JPEG prefix
	encodedImage := base64.StdEncoding.EncodeToString(jpegBuf.Bytes())
	log.Printf("🎉 IMAGE PROCESSING: Final encoded JPEG image size: %d bytes", len(encodedImage))
	log.Printf("✅ IMAGE PROCESSING: Image processing completed successfully")
	return encodedImage, nil
}

// Función auxiliar para encontrar el mínimo de dos enteros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// UserUpdateRequest structure for user update requests
type UserUpdateRequest struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Email      string `json:"email,omitempty"`
	GivenName  string `json:"given_name,omitempty"`
	FamilyName string `json:"family_name,omitempty"`
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

// handleGetUserInfo handles GET requests for user information
func handleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var userID string

	// Get user ID from query parameter (GET) or request body (POST)
	if r.Method == "GET" {
		userID = r.URL.Query().Get("user_id")
		if userID == "" {
			userID = r.URL.Query().Get("id")
		}
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

	// Convert string userID to int
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		log.Printf("Error converting user ID to int: %v", err)
		http.Error(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	// Log for debugging
	log.Printf("Getting user info for user ID: %s", userID)

	// Get user info from database
	var user User
	if err := getUserById(userIDInt, &user); err != nil {
		if err == sql.ErrNoRows {
			log.Printf("User not found for ID: %s", userID)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		log.Printf("Database error for user ID %s: %v", userID, err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully retrieved user %s: %s (%s)", userID, user.Name, user.Email)

	// Return user info as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// handleUpdateUser handles POST requests for updating user information
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

	// Convert string ID to int
	userIDInt, err := strconv.Atoi(updateRequest.ID)
	if err != nil {
		log.Printf("Error converting user ID to int: %v", err)
		http.Error(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	// Log for debugging
	log.Printf("Updating user info: %+v", updateRequest)

	// Update user info in database
	result, err := db.Exec(`
		UPDATE users 
		SET name = ?, email = ?, given_name = ?, family_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, updateRequest.Name, updateRequest.Email, updateRequest.GivenName, updateRequest.FamilyName, userIDInt)

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

	// Record sync operation with auto-generated operation_id if needed
	// Following the consistent sync recording pattern from implementation guide
	log.Printf("Recording sync operation for user info update with auto-generated operation_id")

	// Prepare sync operation data for recording
	syncOperationData := map[string]interface{}{
		"user_id":      userIDInt,
		"name":         updateRequest.Name,
		"email":        updateRequest.Email,
		"given_name":   updateRequest.GivenName,
		"family_name":  updateRequest.FamilyName,
		"action":       "update",
		"processed_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// Record the sync operation with auto-generation if operation_id not provided
	err = addSyncOperation(
		updateRequest.ID,
		updateRequest.OperationID, // May be empty, will auto-generate if needed
		"update",
		"profile_info",
		updateRequest.ID,
		syncOperationData,
		updateRequest.DeviceID,
		updateRequest.Timestamp,
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for user info update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for user info update")
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ApiResponse{Success: true, Message: "User updated successfully"})
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
