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
