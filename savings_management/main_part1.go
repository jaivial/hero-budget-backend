package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/herobudget/backend/common"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// Variables globales para conexión a la base de datos y gestión de cache
// Permiten acceso consistente a recursos compartidos en todo el servicio de ahorros
var (
	// Database connection for savings data persistence
	db *sql.DB
	// Cache manager for Redis operations to improve performance
	cacheManager *common.CacheManager
)

// Estructuras de datos principales para gestión de ahorros
// Definición de tipos utilizados en toda la aplicación de savings

// SavingsData representa los datos de ahorro de un usuario
// Contiene información completa sobre metas de ahorro, disponible y progreso
type SavingsData struct {
	UserID      string  `json:"user_id"`      // ID único del usuario propietario
	Available   float64 `json:"available"`    // Cantidad disponible actualmente ahorrada
	Goal        float64 `json:"goal"`         // Meta de ahorro establecida
	Period      string  `json:"period"`       // Período para alcanzar la meta (monthly, weekly, etc.)
	Percent     float64 `json:"percent"`      // Porcentaje de progreso hacia la meta
	NeedToSave  float64 `json:"need_to_save"` // Cantidad que falta para alcanzar la meta
	DailyTarget float64 `json:"daily_target"` // Meta diaria para alcanzar el objetivo
}

// SavingsUpdateRequest estructura para solicitudes de actualización de ahorros
// Permite actualizar valores específicos sin afectar el resto de los datos de savings
// Incluye parámetros de sincronización para seguimiento de operaciones incrementales
type SavingsUpdateRequest struct {
	UserID    string  `json:"user_id"`             // ID del usuario que realiza la actualización
	Available float64 `json:"available,omitempty"` // Nueva cantidad disponible (opcional)
	Goal      float64 `json:"goal,omitempty"`      // Nueva meta de ahorro (opcional)
	Period    string  `json:"period,omitempty"`    // Nuevo período para la meta (opcional)
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

// SavingsDeleteRequest estructura para solicitudes de eliminación de metas de ahorro
// Permite eliminar completamente los datos de ahorro de un usuario
// Incluye parámetros de sincronización para seguimiento de operaciones incrementales
type SavingsDeleteRequest struct {
	UserID string `json:"user_id"` // ID del usuario cuyos datos se van a eliminar
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

// ApiResponse estructura estándar para respuestas de la API de ahorros
// Proporciona formato consistente para todas las respuestas del servicio
type ApiResponse struct {
	Success bool        `json:"success"`           // Indica si la operación fue exitosa
	Message string      `json:"message,omitempty"` // Mensaje descriptivo del resultado
	Data    interface{} `json:"data,omitempty"`    // Datos de respuesta (opcional)
}

// init inicializa la conexión a la base de datos y configura el entorno
// Se ejecuta automáticamente al iniciar el servicio de savings management
func init() {
	// Parse command line flags para configuración de entorno
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	// Esto permite configuración flexible entre entornos de desarrollo y producción
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Determine database path based on environment flag
	// La ruta de la base de datos cambia según el entorno de ejecución
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
	// Open the database connection with error handling
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection to ensure database is accessible
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	log.Println("Database connection established successfully")

	// Initialize cache manager for improved performance
	// Cache manager ayuda a reducir consultas repetitivas a la base de datos
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	} else {
		log.Println("✅ Cache manager initialized successfully")
	}

	// Update sync_operations schema to support savings-specific operation types
	if err := updateSyncOperationsSchema(); err != nil {
		log.Printf("Warning: Failed to update sync_operations schema: %v", err)
	}

	log.Println("Savings Management service initialized successfully")
}

// CENTRALIZED SCHEMA MIGRATION:
// This function has been removed - all DDL operations are now centralized
// in backend/database_schema.sql and managed by the centralized database
// initialization service to maintain consistency across all services.

// addSyncOperation function is now in sync_operations_core.go
// This provides enhanced operation ID generation and schema validation

// main función principal que inicia el servidor y configura las rutas
// Establece el servidor HTTP con middleware CORS y handlers de savings
func main() {
	// Set up CORS middleware and savings management routes
	// Rutas principales para operaciones CRUD de ahorros
	http.HandleFunc("/savings/fetch", corsMiddleware(handleFetchSavings))
	http.HandleFunc("/savings/create", corsMiddleware(handleCreateSavings))
	http.HandleFunc("/savings/update", corsMiddleware(handleUpdateSavings))
	http.HandleFunc("/savings/delete", corsMiddleware(handleDeleteSavings))
	http.HandleFunc("/health", corsMiddleware(handleHealth))
	http.HandleFunc("/savings/health", corsMiddleware(handleSavingsHealth))

	// Rutas de sincronización offline para ahorros
	// Siguiendo el patrón exitoso de budget_management, bills_management y expense_management
	http.HandleFunc("/savings/sync/health", corsMiddleware(handleSyncSavingsHealth))
	http.HandleFunc("/savings/sync/batch", corsMiddleware(handleSyncSavingsBatch))
	http.HandleFunc("/savings/sync/changes", corsMiddleware(handleSyncSavingsChanges))
	http.HandleFunc("/savings/sync/resolve-conflict", corsMiddleware(handleSyncSavingsConflictResolution))

	port := 8089
	log.Printf("Savings Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// corsMiddleware aplica headers CORS a todas las respuestas
// Permite acceso desde diferentes dominios y métodos HTTP para savings management
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers para permitir acceso cross-origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight OPTIONS requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler in the chain
		next(w, r)
	}
}

// handleFetchSavings maneja las solicitudes de obtención de datos de ahorros
// Endpoint: GET /savings/fetch
// Retorna los datos de ahorro para un usuario específico con cache optimization
func handleFetchSavings(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter with validation
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Try cache first for improved performance
	// Cache reduce la latencia y carga en la base de datos
	if cacheManager != nil {
		var cachedSavings SavingsData
		err := cacheManager.GetSavingsData(userID, &cachedSavings)
		if err == nil {
			log.Printf("✅ Cache HIT: savings data for user %s", userID)
			sendSuccessResponse(w, "Savings data fetched successfully from cache", cachedSavings)
			return
		}
		log.Printf("🔍 Cache MISS: savings data for user %s", userID)
	}

	// Get savings data from database as fallback
	savings, err := fetchSavingsData(userID)
	if err != nil {
		log.Printf("Error fetching savings data: %v", err)
		sendErrorResponse(w, "Error fetching savings data", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests to improve performance
	if cacheManager != nil {
		err = cacheManager.CacheSavingsData(userID, savings)
		if err != nil {
			log.Printf("Warning: Failed to cache savings data for user %s: %v", userID, err)
		}
	}

	// Return savings data as JSON with success response
	sendSuccessResponse(w, "Savings data fetched successfully", savings)
}

// getEnvOrDefault obtiene el valor de una variable de entorno o retorna un valor por defecto
// Utilizado para configuración flexible del servicio entre diferentes entornos
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
