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

	"github.com/joho/godotenv"
	"github.com/herobudget/backend/common"
	_ "github.com/mattn/go-sqlite3"
)

// Variable global para la conexión a la base de datos
var db *sql.DB

// Context for database operations
var ctx = context.Background()

// Cache manager for Redis operations
var cacheManager *common.CacheManager

// Bill estructura que representa una factura en el sistema
// Contiene toda la información necesaria para gestionar facturas recurrentes
type Bill struct {
	ID             int     `json:"id"`
	UserID         string  `json:"user_id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	DueDate        string  `json:"due_date"`
	StartDate      string  `json:"start_date"`
	PaymentDay     int     `json:"payment_day"`
	DurationMonths int     `json:"duration_months"`
	Regularity     string  `json:"regularity"`
	Paid           bool    `json:"paid"`
	Overdue        bool    `json:"overdue"`
	OverdueDays    int     `json:"overdue_days"`
	Recurring      bool    `json:"recurring"`
	Category       string  `json:"category"`
	Icon           string  `json:"icon"`
	PaymentMethod  string  `json:"payment_method"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// AddBillRequest estructura para solicitudes de creación de facturas
// Incluye parámetros de sincronización para seguimiento de operaciones incrementales
type AddBillRequest struct {
	UserID         string  `json:"user_id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	DueDate        string  `json:"due_date"`
	Category       string  `json:"category"`
	Icon           string  `json:"icon"`
	StartDate      string  `json:"start_date"`
	PaymentDay     int     `json:"payment_day"`
	DurationMonths int     `json:"duration_months"`
	Regularity     string  `json:"regularity"`
	PaymentMethod  string  `json:"payment_method"`
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// UpdateBillRequest estructura para solicitudes de actualización de facturas
// Permite actualización parcial de campos usando omitempty
type UpdateBillRequest struct {
	UserID         string  `json:"user_id"`
	BillID         int     `json:"bill_id"`
	Name           string  `json:"name,omitempty"`
	Amount         float64 `json:"amount,omitempty"`
	StartDate      string  `json:"start_date,omitempty"`
	PaymentDay     int     `json:"payment_day,omitempty"`
	DurationMonths int     `json:"duration_months,omitempty"`
	Regularity     string  `json:"regularity,omitempty"`
	Category       string  `json:"category,omitempty"`
	Icon           string  `json:"icon,omitempty"`
	PaymentMethod  string  `json:"payment_method,omitempty"`
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// SyncOperation estructura para registrar operaciones de sincronización
type SyncOperation struct {
	ID           int    `json:"id"`
	UserID       string `json:"user_id"`
	OperationID  string `json:"operation_id"`
	Action       string `json:"action"`        // "create", "update", "delete"
	TableName    string `json:"table_name"`    // "bills", "expenses", etc.
	RecordID     string `json:"record_id"`     // ID del registro afectado
	Data         string `json:"data"`          // JSON con los datos de la operación
	DeviceID     string `json:"device_id"`
	ClientTimestamp int64 `json:"client_timestamp"`
	ServerTimestamp int64 `json:"server_timestamp"`
	CreatedAt    string `json:"created_at"`
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
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	fmt.Printf("Using database at: %s\n", dbPath)
	createTablesIfNotExist()
	
	// Initialize cache manager
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	}
	
	log.Println("Database connection established successfully")
}

// corsMiddleware maneja las cabeceras CORS para permitir solicitudes desde el frontend
// Necesario para la comunicación entre diferentes dominios
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// main función principal que configura las rutas y inicia el servidor
// Define todos los endpoints disponibles para la gestión de facturas
func main() {
	// Configurar rutas de la API
	http.HandleFunc("/bills", corsMiddleware(handleFetchBills))
	http.HandleFunc("/bills/add", corsMiddleware(handleAddBill))
	http.HandleFunc("/bills/add-cash-bank", corsMiddleware(handleAddBillCashBank))
	http.HandleFunc("/bills/pay", corsMiddleware(handlePayBill))
	http.HandleFunc("/bills/payment-status", corsMiddleware(handleGenericEndpoint))
	http.HandleFunc("/bills/update", corsMiddleware(handleUpdateBill))
	http.HandleFunc("/bills/update-cash-bank", corsMiddleware(handleUpdateBillCashBank))
	http.HandleFunc("/bills/delete", corsMiddleware(handleDeleteBill))
	http.HandleFunc("/bills/upcoming", corsMiddleware(handleGenericEndpoint))
	http.HandleFunc("/bills/debug-add", corsMiddleware(handleDebugAddBill))
	
	// Rutas de sincronización offline para facturas (siguiendo patrón de expense_management)
	http.HandleFunc("/bills/sync/health", corsMiddleware(handleSyncBillHealth))
	http.HandleFunc("/bills/sync/batch", corsMiddleware(handleSyncBillBatch))
	http.HandleFunc("/bills/sync/changes", corsMiddleware(handleSyncBillChanges))
	
	// Iniciar servidor
	fmt.Println("Bills Management service started on :8091")
	log.Fatal(http.ListenAndServe(":8091", nil))
}

// createTablesIfNotExist crea las tablas necesarias si no existen
// Garantiza que la estructura de la base de datos sea correcta
func createTablesIfNotExist() {
	// Crear tabla bills
	db.Exec(`CREATE TABLE IF NOT EXISTS bills (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		user_id TEXT NOT NULL, 
		name TEXT NOT NULL, 
		amount REAL NOT NULL, 
		due_date TEXT, 
		start_date TEXT NOT NULL, 
		payment_day INTEGER NOT NULL, 
		duration_months INTEGER NOT NULL, 
		regularity TEXT NOT NULL DEFAULT 'monthly', 
		paid BOOLEAN DEFAULT 0, 
		overdue BOOLEAN DEFAULT 0, 
		overdue_days INTEGER DEFAULT 0, 
		recurring BOOLEAN DEFAULT 1, 
		category TEXT DEFAULT 'general', 
		icon TEXT DEFAULT '💳', 
		payment_method TEXT, 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, 
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	
	// Crear tabla bill_payments
	db.Exec(`CREATE TABLE IF NOT EXISTS bill_payments (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		bill_id INTEGER NOT NULL, 
		year_month TEXT NOT NULL, 
		paid BOOLEAN DEFAULT 0, 
		payment_date TEXT, 
		payment_method TEXT, 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP, 
		FOREIGN KEY (bill_id) REFERENCES bills (id) ON DELETE CASCADE, 
		UNIQUE(bill_id, year_month)
	)`)
	
	// Create sync_operations table with new operation-id based schema
	// This matches the delta_sync format for consistency across services
	db.Exec(`CREATE TABLE IF NOT EXISTS sync_operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		operation_type TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		operation_data TEXT NOT NULL,
		device_ids TEXT DEFAULT '[]',
		client_timestamp INTEGER DEFAULT 0,
		server_timestamp INTEGER DEFAULT 0,
		created_at INTEGER DEFAULT 0,
		UNIQUE(operation_id)
	)`)
	
	// Create index on operation_id for fast lookups and ordering
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_operations_operation_id 
		ON sync_operations(operation_id)`)
	
	// Create index on user_id and operation_id for user-specific queries
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_operations_user_operation 
		ON sync_operations(user_id, operation_id)`)
	
	// Añadir columna bill_id a expenses si no existe
	db.Exec("ALTER TABLE expenses ADD COLUMN bill_id INTEGER;")
}

// sendErrorResponse envía una respuesta de error estandarizada
// Mantiene consistencia en el formato de errores
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{Success: false, Message: message})
}

// sendSuccessResponse envía una respuesta de éxito estandarizada
// Mantiene consistencia en el formato de respuestas exitosas
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(ApiResponse{Success: true, Message: message, Data: data})
}

// handleGenericEndpoint maneja endpoints genéricos que solo devuelven estado disponible
// Útil para endpoints en desarrollo o de verificación de estado
func handleGenericEndpoint(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Endpoint available", map[string]string{"status": "available"})
}

// getValueOrDefault retorna el valor proporcionado o el valor por defecto si es 0
// Útil para campos opcionales en actualizaciones
func getValueOrDefault(value, defaultValue float64) float64 {
	if value > 0 {
		return value
	}
	return defaultValue
}

// getIntValueOrDefault retorna el valor entero proporcionado o el valor por defecto si es 0
// Útil para campos enteros opcionales en actualizaciones
func getIntValueOrDefault(value, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getStringValueOrDefault retorna el string proporcionado o el valor por defecto si está vacío
// Útil para campos de texto opcionales en actualizaciones
func getStringValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// Operation ID utility functions for timestamp-based format

// isValidOperationId validates if operation ID follows timestamp-based format
// Expected format: {timestamp_ms}_{sequence_number}
// Parameters:
//   - operationId: Operation ID to validate
// Returns: boolean - true if valid format, false otherwise
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
// Parameters:
//   - operationId: Operation ID in timestamp_sequence format
// Returns: int64 - timestamp in milliseconds or 0 if invalid
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
// Parameters:
//   - userID: User identifier
// Returns: string - last operation ID or empty string if none exists
func getLastOperationIdForUser(userID string) (string, error) {
	var lastOperationId string
	err := db.QueryRow("SELECT operation_id FROM sync_operations WHERE user_id = ? ORDER BY operation_id DESC LIMIT 1", userID).Scan(&lastOperationId)
	
	if err != nil {
		if err == sql.ErrNoRows {
			// No operations found for this user
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
// Parameters:
//   - userID: User identifier
// Returns: string - new operation ID in format {timestamp_ms}_{sequence_number}
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
	// Using 001 format for sequence to maintain 3-digit consistency
	operationId := fmt.Sprintf("%d_%03d", nextTimestamp, sequenceNumber)
	
	log.Printf("Generated operation ID: %s", operationId)
	return operationId, nil
}

// addSyncOperation registra una operación de sincronización en la tabla sync_operations
// Now uses the new operation_id system with timestamp-based format and automatic generation
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
		// Store null for empty device_ids
		deviceIDsJSON = []byte("null")
		log.Printf("Device ID empty, storing null in device_ids column")
	}
	
	// For operation-based sync, use the timestamp from the operation ID
	// Extract timestamp from operation ID for created_at field
	operationTimestamp := extractTimestampFromOperationId(operationID)
	if operationTimestamp == 0 {
		// Fallback to current timestamp if extraction fails
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
		tableName,         // entity_type (bills, bill_payments)
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
