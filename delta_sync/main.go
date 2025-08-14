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
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// Variable global para la conexión a la base de datos
var db *sql.DB

// Context for database operations
var ctx = context.Background()

// SyncOperation estructura que representa una operación de sincronización
// Almacena todas las operaciones de datos para funcionalidad de sincronización incremental
// DeviceIDs ahora es un array JSON para soporte multi-dispositivo
type SyncOperation struct {
	OperationID   string   `json:"operation_id"`
	UserID        string   `json:"user_id"`
	CreatedAt     int64    `json:"created_at"`
	OperationType string   `json:"operation_type"` // create, update, delete
	EntityType    string   `json:"entity_type"`
	EntityID      string   `json:"entity_id"`
	OperationData string   `json:"operation_data"`
	DeviceIDs     []string `json:"device_ids,omitempty"` // Array of device IDs as JSON
}

// AddSyncOperationRequest estructura para solicitudes de adición de operaciones
// Contiene todos los campos necesarios para registrar una nueva operación de sincronización
// Acepta tanto device_id individual como device_ids array para compatibilidad
type AddSyncOperationRequest struct {
	OperationID   string   `json:"operation_id"`
	UserID        string   `json:"user_id"`
	OperationType string   `json:"operation_type"`
	EntityType    string   `json:"entity_type"`
	EntityID      string   `json:"entity_id"`
	OperationData string   `json:"operation_data"`
	DeviceID      string   `json:"device_id,omitempty"`      // Single device ID (backward compatibility)
	DeviceIDs     []string `json:"device_ids,omitempty"`     // Array of device IDs (new format)
}

// FetchSyncOperationsRequest estructura para solicitudes de obtención de operaciones
// Permite filtrar por usuario y timestamp para sincronización incremental
type FetchSyncOperationsRequest struct {
	UserID    string `json:"user_id"`
	Timestamp int64  `json:"timestamp"`
}

// ApiResponse estructura estándar para respuestas de la API
// Proporciona formato consistente para todas las respuestas
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// init inicializa la conexión a la base de datos y crea las tablas necesarias
// Se ejecuta automáticamente al importar el paquete
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
		log.Printf("🔧 Running in DEFAULT DEVELOPMENT mode - Database: %s", dbPath)
	}

	// Conectar a la base de datos SQLite
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	// Verificar la conexión
	if err := db.Ping(); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	log.Println("Successfully connected to SQLite database")

	// Crear las tablas necesarias
	if err := createTables(); err != nil {
		log.Fatalf("Error creating tables: %v", err)
	}

	log.Println("Delta sync service initialized successfully")
}

// getEnvOrDefault obtiene una variable de entorno o devuelve un valor por defecto
// Utilizado para configuración flexible entre entornos de desarrollo y producción
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createTables crea la tabla sync_operations si no existe
// Implementa el esquema especificado en la migración de base de datos
func createTables() error {
	log.Println("Creating sync_operations table if not exists...")

	// Migration: Add sync_operations table and related indexes
	// Date: 2025-01-12 (Updated: 2025-01-14 for device_ids JSON array support)
	// Purpose: Enable incremental sync functionality for efficient data synchronization

	// Create sync_operations table for tracking all data operations
	// device_ids column stores JSON array of device IDs for multi-device support
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sync_operations (
		operation_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete')),
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		operation_data TEXT NOT NULL,
		device_ids TEXT DEFAULT '[]'
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("error creating sync_operations table: %v", err)
	}

	// Migration: Convert existing device_id column to device_ids JSON array
	err = migrateDeviceIdToArray()
	if err != nil {
		return fmt.Errorf("error migrating device_id to device_ids: %v", err)
	}

	// Create indexes for efficient sync operation queries
	indexUserCreatedSQL := `
	CREATE INDEX IF NOT EXISTS idx_sync_operations_user_created 
	ON sync_operations (user_id, created_at);`

	_, err = db.Exec(indexUserCreatedSQL)
	if err != nil {
		return fmt.Errorf("error creating user_created index: %v", err)
	}

	indexUserEntitySQL := `
	CREATE INDEX IF NOT EXISTS idx_sync_operations_user_entity 
	ON sync_operations (user_id, entity_type, entity_id);`

	_, err = db.Exec(indexUserEntitySQL)
	if err != nil {
		return fmt.Errorf("error creating user_entity index: %v", err)
	}

	// Verify sync_operations table was created
	var tableName string
	checkTableSQL := "SELECT name FROM sqlite_master WHERE type='table' AND name='sync_operations';"
	err = db.QueryRow(checkTableSQL).Scan(&tableName)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error verifying table creation: %v", err)
	}

	if tableName == "sync_operations" {
		log.Println("✅ sync_operations table created and verified successfully")
	} else {
		log.Println("⚠️ sync_operations table verification failed")
	}

	return nil
}

// migrateDeviceIdToArray migra device_id individual a device_ids JSON array
// Maneja la migración de datos existentes para compatibilidad hacia atrás
func migrateDeviceIdToArray() error {
	log.Println("Checking if device_id to device_ids migration is needed...")

	// Check if old device_id column exists
	var columnExists bool
	checkColumnSQL := `
	SELECT COUNT(*) > 0 FROM pragma_table_info('sync_operations') 
	WHERE name = 'device_id' AND name != 'device_ids';`
	
	err := db.QueryRow(checkColumnSQL).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("error checking for device_id column: %v", err)
	}

	if !columnExists {
		log.Println("✅ No device_id column found, migration not needed")
		return nil
	}

	log.Println("🔄 Migrating device_id column to device_ids JSON array...")

	// Check if device_ids column exists, if not add it
	var deviceIdsExists bool
	checkDeviceIdsSQL := `
	SELECT COUNT(*) > 0 FROM pragma_table_info('sync_operations') 
	WHERE name = 'device_ids';`
	
	err = db.QueryRow(checkDeviceIdsSQL).Scan(&deviceIdsExists)
	if err != nil {
		return fmt.Errorf("error checking for device_ids column: %v", err)
	}

	if !deviceIdsExists {
		// Add device_ids column
		addColumnSQL := `ALTER TABLE sync_operations ADD COLUMN device_ids TEXT DEFAULT '[]';`
		_, err = db.Exec(addColumnSQL)
		if err != nil {
			return fmt.Errorf("error adding device_ids column: %v", err)
		}
		log.Println("✅ Added device_ids column")
	}

	// Migrate existing device_id values to device_ids JSON array
	migrateDataSQL := `
	UPDATE sync_operations 
	SET device_ids = CASE 
		WHEN device_id IS NULL OR device_id = '' THEN '[]'
		ELSE json_array(device_id)
	END
	WHERE device_ids = '[]' OR device_ids IS NULL;`

	result, err := db.Exec(migrateDataSQL)
	if err != nil {
		return fmt.Errorf("error migrating device_id data: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ Migrated %d rows from device_id to device_ids", rowsAffected)

	// Drop old device_id column (optional - uncomment if you want to remove it)
	// dropColumnSQL := `ALTER TABLE sync_operations DROP COLUMN device_id;`
	// _, err = db.Exec(dropColumnSQL)
	// if err != nil {
	//     log.Printf("Warning: Could not drop device_id column: %v", err)
	// }

	log.Println("✅ Device ID migration completed successfully")
	return nil
}

// addDeviceToOperation añade un device_id a la lista de devices de una operación existente
// Útil para cuando múltiples dispositivos procesan la misma operación
func addDeviceToOperation(operationID, deviceID string) error {
	if operationID == "" || deviceID == "" {
		return fmt.Errorf("operation_id and device_id are required")
	}

	// Get current device_ids array
	var currentDeviceIdsJSON string
	getQuery := "SELECT COALESCE(device_ids, '[]') FROM sync_operations WHERE operation_id = ?"
	err := db.QueryRow(getQuery, operationID).Scan(&currentDeviceIdsJSON)
	if err != nil {
		return fmt.Errorf("error getting current device_ids: %v", err)
	}

	// Parse current device IDs
	var deviceIds []string
	err = json.Unmarshal([]byte(currentDeviceIdsJSON), &deviceIds)
	if err != nil {
		return fmt.Errorf("error parsing current device_ids JSON: %v", err)
	}

	// Check if device ID already exists
	for _, existingDeviceID := range deviceIds {
		if existingDeviceID == deviceID {
			log.Printf("Device ID %s already exists in operation %s", deviceID, operationID)
			return nil // Already exists, no need to add
		}
	}

	// Add new device ID
	deviceIds = append(deviceIds, deviceID)

	// Marshal back to JSON
	newDeviceIdsJSON, err := json.Marshal(deviceIds)
	if err != nil {
		return fmt.Errorf("error marshaling updated device_ids: %v", err)
	}

	// Update in database
	updateQuery := "UPDATE sync_operations SET device_ids = ? WHERE operation_id = ?"
	_, err = db.Exec(updateQuery, string(newDeviceIdsJSON), operationID)
	if err != nil {
		return fmt.Errorf("error updating device_ids: %v", err)
	}

	log.Printf("✅ Added device %s to operation %s. Updated devices: %v", deviceID, operationID, deviceIds)
	return nil
}

// corsMiddleware añade headers CORS a las respuestas HTTP
// Permite solicitudes desde diferentes orígenes para la aplicación móvil
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// handleFetchSyncOperations maneja las solicitudes GET para obtener operaciones de sincronización
// Retorna todas las operaciones más recientes que el timestamp proporcionado
func handleFetchSyncOperations(w http.ResponseWriter, r *http.Request) {
	log.Printf("📥 Received request: %s %s", r.Method, r.URL.Path)

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from query string
	userID := r.URL.Query().Get("user_id")
	timestampStr := r.URL.Query().Get("timestamp")

	if userID == "" || timestampStr == "" {
		response := ApiResponse{
			Success: false,
			Message: "user_id and timestamp parameters are required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse timestamp
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		response := ApiResponse{
			Success: false,
			Message: "Invalid timestamp format",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Query database for sync operations with device_ids JSON array
	query := `
	SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data, COALESCE(device_ids, '[]')
	FROM sync_operations 
	WHERE user_id = ? AND created_at > ?
	ORDER BY created_at ASC`

	rows, err := db.Query(query, userID, timestamp)
	if err != nil {
		log.Printf("❌ Database query error: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Database error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer rows.Close()

	// Collect results
	var operations []SyncOperation
	for rows.Next() {
		var op SyncOperation
		var deviceIdsJSON string
		
		err := rows.Scan(&op.OperationID, &op.UserID, &op.CreatedAt, &op.OperationType, 
			&op.EntityType, &op.EntityID, &op.OperationData, &deviceIdsJSON)
		if err != nil {
			log.Printf("❌ Row scan error: %v", err)
			continue
		}

		// Parse device_ids JSON array
		if deviceIdsJSON != "" && deviceIdsJSON != "[]" {
			err = json.Unmarshal([]byte(deviceIdsJSON), &op.DeviceIDs)
			if err != nil {
				log.Printf("⚠️ Error parsing device_ids JSON for operation %s: %v", op.OperationID, err)
				op.DeviceIDs = []string{} // Default to empty array on parse error
			}
		} else {
			op.DeviceIDs = []string{} // Default to empty array
		}
		
		operations = append(operations, op)
	}

	if err = rows.Err(); err != nil {
		log.Printf("❌ Rows iteration error: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Database error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("✅ Retrieved %d sync operations for user %s since timestamp %d", len(operations), userID, timestamp)

	response := ApiResponse{
		Success: true,
		Message: fmt.Sprintf("Retrieved %d sync operations", len(operations)),
		Data:    operations,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleAddSyncOperation maneja las solicitudes POST para añadir nuevas operaciones de sincronización
// Registra una nueva operación en la tabla sync_operations
func handleAddSyncOperation(w http.ResponseWriter, r *http.Request) {
	log.Printf("📥 Received request: %s %s", r.Method, r.URL.Path)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req AddSyncOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ JSON decode error: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Invalid JSON format",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if req.OperationID == "" || req.UserID == "" || req.OperationType == "" || 
		req.EntityType == "" || req.EntityID == "" || req.OperationData == "" {
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: operation_id, user_id, operation_type, entity_type, entity_id, operation_data",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate operation_type
	validTypes := []string{"create", "update", "delete"}
	validType := false
	for _, vt := range validTypes {
		if req.OperationType == vt {
			validType = true
			break
		}
	}
	if !validType {
		response := ApiResponse{
			Success: false,
			Message: "Invalid operation_type. Must be 'create', 'update', or 'delete'",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Prepare device_ids JSON array for storage
	var deviceIdsJSON string
	var deviceIdsToStore []string

	// Handle backward compatibility: convert single device_id to array
	if len(req.DeviceIDs) > 0 {
		deviceIdsToStore = req.DeviceIDs
	} else if req.DeviceID != "" {
		deviceIdsToStore = []string{req.DeviceID}
	} else {
		deviceIdsToStore = []string{} // Empty array
	}

	// Marshal device IDs to JSON
	deviceIdsBytes, err := json.Marshal(deviceIdsToStore)
	if err != nil {
		log.Printf("❌ Error marshaling device_ids: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Error processing device IDs",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	deviceIdsJSON = string(deviceIdsBytes)

	// Insert into database
	currentTime := time.Now().Unix()
	insertSQL := `
	INSERT INTO sync_operations (operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data, device_ids)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.Exec(insertSQL, req.OperationID, req.UserID, currentTime, req.OperationType, 
		req.EntityType, req.EntityID, req.OperationData, deviceIdsJSON)
	if err != nil {
		log.Printf("❌ Database insert error: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Database error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("✅ Added sync operation: %s for user %s (type: %s, entity: %s/%s, devices: %v)", 
		req.OperationID, req.UserID, req.OperationType, req.EntityType, req.EntityID, deviceIdsToStore)

	response := ApiResponse{
		Success: true,
		Message: "Sync operation added successfully",
		Data: map[string]interface{}{
			"operation_id": req.OperationID,
			"created_at":   currentTime,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleAddDeviceToOperation maneja las solicitudes POST para añadir un device_id a una operación existente
// Permite que múltiples dispositivos marquen que han procesado una operación
func handleAddDeviceToOperation(w http.ResponseWriter, r *http.Request) {
	log.Printf("📥 Received request: %s %s", r.Method, r.URL.Path)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		OperationID string `json:"operation_id"`
		DeviceID    string `json:"device_id"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ JSON decode error: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Invalid JSON format",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if req.OperationID == "" || req.DeviceID == "" {
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: operation_id and device_id",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Add device to operation
	err := addDeviceToOperation(req.OperationID, req.DeviceID)
	if err != nil {
		log.Printf("❌ Error adding device to operation: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Error updating operation",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("✅ Added device %s to operation %s", req.DeviceID, req.OperationID)

	response := ApiResponse{
		Success: true,
		Message: "Device added to operation successfully",
		Data: map[string]interface{}{
			"operation_id": req.OperationID,
			"device_id":    req.DeviceID,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// BatchUpdateDeviceRequest estructura para solicitudes de actualización batch de device_ids
// Permite marcar múltiples operaciones como procesadas por un dispositivo específico
type BatchUpdateDeviceRequest struct {
	OperationIDs []string `json:"operation_ids"` // Array of operation IDs to update
	DeviceID     string   `json:"device_id"`     // Device ID to add to each operation
}

// handleBatchUpdateDeviceOperations maneja las solicitudes POST para añadir un device_id a múltiples operaciones
// Se ejecuta después de procesar exitosamente todas las operaciones en incrementalSyncService
func handleBatchUpdateDeviceOperations(w http.ResponseWriter, r *http.Request) {
	requestStartTime := time.Now()
	log.Printf("📥 ================ BATCH DEVICE UPDATE REQUEST ================")
	log.Printf("📥 Received batch device update request: %s %s", r.Method, r.URL.Path)
	log.Printf("📥 Remote address: %s", r.RemoteAddr)
	log.Printf("📥 User-Agent: %s", r.UserAgent())
	log.Printf("📥 Content-Type: %s", r.Header.Get("Content-Type"))
	log.Printf("📥 Content-Length: %s", r.Header.Get("Content-Length"))

	if r.Method != "POST" {
		log.Printf("❌ Method not allowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req BatchUpdateDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ JSON decode error: %v", err)
		log.Printf("❌ Request body parsing failed after %v", time.Since(requestStartTime))
		response := ApiResponse{
			Success: false,
			Message: "Invalid JSON format",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if len(req.OperationIDs) == 0 || req.DeviceID == "" {
		log.Printf("❌ Validation failed - Missing required fields:")
		log.Printf("   - Operation IDs count: %d", len(req.OperationIDs))
		log.Printf("   - Device ID: '%s' (length: %d)", req.DeviceID, len(req.DeviceID))
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: operation_ids array and device_id are required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("🔄 ============== BATCH UPDATE PROCESSING ==============")
	log.Printf("🔄 Request validation passed after %v", time.Since(requestStartTime))
	log.Printf("🔄 Device ID to add: '%s' (length: %d chars)", req.DeviceID, len(req.DeviceID))
	log.Printf("🔄 Operation IDs count: %d", len(req.OperationIDs))
	log.Printf("🔄 Operation IDs: %v", req.OperationIDs)
	log.Printf("🔄 Starting batch processing...")

	batchProcessingStartTime := time.Now()

	// Track successful and failed updates
	var successfulUpdates []string
	var failedUpdates []string

	// Process each operation ID
	for i, operationID := range req.OperationIDs {
		operationStartTime := time.Now()
		log.Printf("🔄 Processing operation %d/%d: %s", i+1, len(req.OperationIDs), operationID)
		
		err := addDeviceToOperation(operationID, req.DeviceID)
		operationDuration := time.Since(operationStartTime)
		
		if err != nil {
			log.Printf("❌ FAILED to add device '%s' to operation '%s' in %v", req.DeviceID, operationID, operationDuration)
			log.Printf("   - Error: %v", err)
			log.Printf("   - Progress: %d/%d operations processed (%d successful, %d failed)", i+1, len(req.OperationIDs), len(successfulUpdates), len(failedUpdates)+1)
			failedUpdates = append(failedUpdates, operationID)
		} else {
			log.Printf("✅ SUCCESS added device '%s' to operation '%s' in %v", req.DeviceID, operationID, operationDuration)
			log.Printf("   - Progress: %d/%d operations processed (%d successful, %d failed)", i+1, len(req.OperationIDs), len(successfulUpdates)+1, len(failedUpdates))
			successfulUpdates = append(successfulUpdates, operationID)
		}
	}
	
	batchProcessingDuration := time.Since(batchProcessingStartTime)

	// Log detailed results
	totalRequestDuration := time.Since(requestStartTime)
	log.Printf("🔄 ============= BATCH UPDATE RESULTS =============")
	log.Printf("   - Total operations: %d", len(req.OperationIDs))
	log.Printf("   - Successful updates: %d", len(successfulUpdates))
	log.Printf("   - Failed updates: %d", len(failedUpdates))
	log.Printf("   - Success rate: %.1f%%", float64(len(successfulUpdates))/float64(len(req.OperationIDs))*100)
	log.Printf("   - Batch processing time: %v", batchProcessingDuration)
	log.Printf("   - Total request time: %v", totalRequestDuration)
	log.Printf("   - Average time per operation: %v", batchProcessingDuration/time.Duration(len(req.OperationIDs)))
	log.Printf("   - Device ID added: '%s'", req.DeviceID)
	
	if len(successfulUpdates) > 0 {
		log.Printf("✅ Successfully updated operation IDs: %v", successfulUpdates)
	}
	if len(failedUpdates) > 0 {
		log.Printf("❌ Failed to update operation IDs: %v", failedUpdates)
	}
	
	// Determine response status
	allSuccessful := len(failedUpdates) == 0
	statusCode := http.StatusOK
	message := fmt.Sprintf("Batch update completed: %d successful, %d failed", len(successfulUpdates), len(failedUpdates))
	
	if !allSuccessful && len(successfulUpdates) == 0 {
		// All failed
		statusCode = http.StatusInternalServerError
		message = "All batch updates failed"
		log.Printf("❌ COMPLETE FAILURE - All %d operations failed to update", len(req.OperationIDs))
	} else if !allSuccessful {
		// Partial success
		statusCode = http.StatusPartialContent
		message = "Batch update partially successful"
		log.Printf("⚠️ PARTIAL SUCCESS - %d of %d operations updated successfully", len(successfulUpdates), len(req.OperationIDs))
	} else {
		log.Printf("✅ COMPLETE SUCCESS - All %d operations updated successfully", len(req.OperationIDs))
	}

	response := ApiResponse{
		Success: allSuccessful,
		Message: message,
		Data: map[string]interface{}{
			"device_id":              req.DeviceID,
			"total_operations":       len(req.OperationIDs),
			"updated_operations":     successfulUpdates, // Changed from successful_updates to match frontend expectation
			"failed_operations":      failedUpdates,     // Changed from failed_updates to match frontend expectation
			"success_count":          len(successfulUpdates),
			"failure_count":          len(failedUpdates),
			"success_rate":           float64(len(successfulUpdates)) / float64(len(req.OperationIDs)) * 100,
			"batch_processing_time":  batchProcessingDuration.Milliseconds(),
			"total_request_time":     totalRequestDuration.Milliseconds(),
		},
	}

	log.Printf("📤 ============= SENDING RESPONSE =============")
	log.Printf("   - Status Code: %d", statusCode)
	log.Printf("   - Response Success: %t", response.Success)
	log.Printf("   - Response Message: %s", response.Message)
	log.Printf("   - Response Time: %v", totalRequestDuration)
	log.Printf("📤 ==========================================")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("❌ Error encoding response: %v", err)
	}
}

// handleHealth maneja las solicitudes de health check
// Proporciona estado de salud del servicio para monitoreo
func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := ApiResponse{
		Success: true,
		Message: "Delta sync service is healthy",
		Data: map[string]interface{}{
			"service": "delta_sync",
			"status":  "healthy",
			"port":    "8103",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// main función principal que inicia el servidor HTTP
// Configura las rutas y middleware necesarios para el servicio
func main() {
	// Get port from environment or use default
	port := getEnvOrDefault("DELTA_SYNC_PORT", "8103")

	// Create router
	router := mux.NewRouter()

	// Define routes with CORS middleware
	router.HandleFunc("/delta-sync/fetch", corsMiddleware(handleFetchSyncOperations)).Methods("GET", "OPTIONS")
	router.HandleFunc("/delta-sync/add", corsMiddleware(handleAddSyncOperation)).Methods("POST", "OPTIONS")
	router.HandleFunc("/delta-sync/add-device", corsMiddleware(handleAddDeviceToOperation)).Methods("POST", "OPTIONS")
	router.HandleFunc("/delta-sync/batch-update-device", corsMiddleware(handleBatchUpdateDeviceOperations)).Methods("POST", "OPTIONS")
	router.HandleFunc("/health", corsMiddleware(handleHealth)).Methods("GET", "OPTIONS")

	log.Printf("🚀 Delta Sync service starting on port %s", port)
	log.Printf("📍 Available endpoints:")
	log.Printf("   GET  /delta-sync/fetch?user_id=<id>&timestamp=<unix_timestamp>")
	log.Printf("   POST /delta-sync/add")
	log.Printf("   POST /delta-sync/add-device")
	log.Printf("   POST /delta-sync/batch-update-device")
	log.Printf("   GET  /health")

	// Start server
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}