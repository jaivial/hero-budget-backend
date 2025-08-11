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
type SyncOperation struct {
	OperationID   string `json:"operation_id"`
	UserID        string `json:"user_id"`
	CreatedAt     int64  `json:"created_at"`
	OperationType string `json:"operation_type"` // create, update, delete
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	OperationData string `json:"operation_data"`
	DeviceID      string `json:"device_id,omitempty"`
}

// AddSyncOperationRequest estructura para solicitudes de adición de operaciones
// Contiene todos los campos necesarios para registrar una nueva operación de sincronización
type AddSyncOperationRequest struct {
	OperationID   string `json:"operation_id"`
	UserID        string `json:"user_id"`
	OperationType string `json:"operation_type"`
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	OperationData string `json:"operation_data"`
	DeviceID      string `json:"device_id,omitempty"`
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
	// Date: 2025-01-12
	// Purpose: Enable incremental sync functionality for efficient data synchronization

	// Create sync_operations table for tracking all data operations
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS sync_operations (
		operation_id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		operation_type TEXT NOT NULL CHECK (operation_type IN ('create', 'update', 'delete')),
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		operation_data TEXT NOT NULL,
		device_id TEXT
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("error creating sync_operations table: %v", err)
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

	// Query database for sync operations
	query := `
	SELECT operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data, COALESCE(device_id, '')
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
		err := rows.Scan(&op.OperationID, &op.UserID, &op.CreatedAt, &op.OperationType, 
			&op.EntityType, &op.EntityID, &op.OperationData, &op.DeviceID)
		if err != nil {
			log.Printf("❌ Row scan error: %v", err)
			continue
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

	// Insert into database
	currentTime := time.Now().Unix()
	insertSQL := `
	INSERT INTO sync_operations (operation_id, user_id, created_at, operation_type, entity_type, entity_id, operation_data, device_id)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(insertSQL, req.OperationID, req.UserID, currentTime, req.OperationType, 
		req.EntityType, req.EntityID, req.OperationData, req.DeviceID)
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

	log.Printf("✅ Added sync operation: %s for user %s (type: %s, entity: %s/%s)", 
		req.OperationID, req.UserID, req.OperationType, req.EntityType, req.EntityID)

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
	router.HandleFunc("/health", corsMiddleware(handleHealth)).Methods("GET", "OPTIONS")

	log.Printf("🚀 Delta Sync service starting on port %s", port)
	log.Printf("📍 Available endpoints:")
	log.Printf("   GET  /delta-sync/fetch?user_id=<id>&timestamp=<unix_timestamp>")
	log.Printf("   POST /delta-sync/add")
	log.Printf("   GET  /health")

	// Start server
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}