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

// Definición de estructuras principales de datos para Categories Management
// Estas estructuras representan las categorías de ingresos y gastos del usuario
// Incluyen validación de tipos, codificación de emojis y gestión de metadatos

// Category representa una categoría de ingreso o gasto del usuario
// Incluye información completa para clasificación y visualización de transacciones
// Utilizada para organizar y filtrar operaciones financieras del usuario
type Category struct {
	ID        int    `json:"id"`                   // ID único de la categoría en la base de datos
	UserID    string `json:"user_id"`              // ID del usuario propietario de la categoría
	Name      string `json:"name"`                 // Nombre descriptivo de la categoría
	Type      string `json:"type"`                 // "income" para ingresos, "expense" para gastos
	Emoji     string `json:"emoji"`                // Emoji representativo para interfaz visual
	CreatedAt string `json:"created_at,omitempty"` // Timestamp de creación de la categoría
	UpdatedAt string `json:"updated_at,omitempty"` // Timestamp de última actualización
}

// AddCategoryRequest representa una solicitud para crear una nueva categoría
// Utilizada en el endpoint de adición de categorías para validar y procesar datos
// Incluye todos los campos requeridos para la creación exitosa
type AddCategoryRequest struct {
	UserID string `json:"user_id"` // ID del usuario que crea la categoría (requerido)
	Name   string `json:"name"`    // Nombre de la categoría (requerido, único por tipo)
	Type   string `json:"type"`    // Tipo de categoría: "income" o "expense" (requerido)
	Emoji  string `json:"emoji"`   // Emoji representativo (opcional, se asigna predeterminado)

	// Sync operation parameters for incremental synchronization tracking
	OperationID string `json:"operation_id,omitempty"` // Unique operation identifier for sync
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp for sync ordering
}

// UpdateCategoryRequest representa una solicitud para actualizar una categoría existente
// Permite actualización parcial de campos, manteniendo valores existentes para campos vacíos
// Incluye validación de permisos de usuario y existencia de la categoría
type UpdateCategoryRequest struct {
	UserID     string `json:"user_id"`         // ID del usuario propietario (requerido para validación)
	CategoryID int    `json:"category_id"`     // ID de la categoría a actualizar (requerido)
	Name       string `json:"name,omitempty"`  // Nuevo nombre (opcional)
	Type       string `json:"type,omitempty"`  // Nuevo tipo (opcional)
	Emoji      string `json:"emoji,omitempty"` // Nuevo emoji (opcional)

	// Sync operation parameters for incremental synchronization tracking
	OperationID string `json:"operation_id,omitempty"` // Unique operation identifier for sync
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp for sync ordering
}

// DeleteCategoryRequest representa una solicitud para eliminar una categoría
// Incluye validación de permisos y verificación de dependencias antes de eliminar
// Asegura que solo el propietario pueda eliminar sus categorías
type DeleteCategoryRequest struct {
	UserID     string `json:"user_id"`     // ID del usuario propietario (requerido)
	CategoryID int    `json:"category_id"` // ID de la categoría a eliminar (requerido)

	// Sync operation parameters for incremental synchronization tracking
	OperationID string `json:"operation_id,omitempty"` // Unique operation identifier for sync
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp for sync ordering
}

// ApiResponse estructura estándar para respuestas de la API REST
// Proporciona formato consistente para todas las respuestas del servicio
// Incluye indicador de éxito, mensaje descriptivo y datos opcionales
type ApiResponse struct {
	Success bool        `json:"success"`           // Indica si la operación fue exitosa
	Message string      `json:"message,omitempty"` // Mensaje descriptivo del resultado
	Data    interface{} `json:"data,omitempty"`    // Datos de respuesta (categorías, confirmaciones, etc.)
}

// Variables globales del sistema para gestión de base de datos y cache
// Estas variables son inicializadas una vez y reutilizadas en toda la aplicación
var (
	// Database connection for category data persistence
	// Conexión principal a la base de datos SQLite para persistencia de categorías
	db *sql.DB

	// Context for database operations
	// Contexto compartido para todas las operaciones de base de datos
	ctx = context.Background()

	// Cache manager for Redis operations to improve performance
	// Gestor de cache Redis para optimizar consultas frecuentes de categorías
	cacheManager *common.CacheManager
)

// init función de inicialización que se ejecuta automáticamente al arrancar el servicio
// Configura la conexión a base de datos, variables de entorno y cache manager
// Valida que todos los recursos necesarios estén disponibles antes de iniciar el servicio
func init() {
	// Parse command line flags para determinar modo de ejecución
	// Permite configurar el entorno (desarrollo/producción) desde línea de comandos
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	// Carga variables de configuración desde archivo .env del directorio padre
	// Continúa con variables del sistema si no encuentra el archivo
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Determine database path based on environment flag
	// Selecciona la ruta de base de datos según el modo de ejecución
	// Producción utiliza ruta centralizada, desarrollo usa archivo local
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
	// Open the database connection with SQLite driver
	// Establece conexión con la base de datos usando el driver SQLite3
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection to ensure database is accessible
	// Verifica que la conexión esté operativa antes de continuar
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	log.Println("Database connection established successfully")

	// Initialize cache manager for performance optimization
	// Inicializa el gestor de cache Redis para optimizar consultas frecuentes
	// Si falla la inicialización, continúa sin cache (degradación elegante)
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	} else {
		log.Println("✅ Cache manager initialized successfully")
	}

	log.Println("Categories Management service initialized successfully")
}

// Operation ID utility functions for timestamp-based format following implementation guide
// These functions ensure consistent operation ID generation across all handlers

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

// main función principal que configura las rutas HTTP y arranca el servidor
// Define todos los endpoints disponibles y configura middleware CORS
// Establece el puerto de escucha y arranca el servidor HTTP
func main() {
	// Run database migration on service startup
	// Ejecuta migración de base de datos al iniciar el servicio
	log.Printf("🔄 Starting database migration for Categories Management service...")
	if err := runDatabaseMigration(); err != nil {
		log.Fatalf("❌ Database migration failed: %v", err)
	}
	log.Printf("✅ Database migration completed successfully")

	// Set up CORS middleware and routes para endpoints principales de categorías
	// Configura middleware CORS y define todas las rutas HTTP disponibles
	// Cada ruta incluye validación de métodos HTTP y manejo de errores

	// Endpoints principales para gestión de categorías
	http.HandleFunc("/categories", corsMiddleware(handleFetchCategories))
	http.HandleFunc("/categories/add", corsMiddleware(handleAddCategory))
	http.HandleFunc("/categories/update", corsMiddleware(handleUpdateCategory))
	http.HandleFunc("/categories/delete", corsMiddleware(handleDeleteCategory))
	http.HandleFunc("/categories/fix-emojis", corsMiddleware(handleFixEmojis))

	// Rutas de sincronización offline - Integración del sistema de sync
	// Endpoints para sincronización bidireccional con clientes offline
	// Permite operaciones por lotes y resolución de conflictos específicos para categorías

	// Sincronización por lotes de operaciones offline de categorías
	http.HandleFunc("/sync/categories/batch", corsMiddleware(handleSyncCategoriesBatch))

	// Obtener cambios del servidor desde último sync de categorías
	http.HandleFunc("/sync/categories/changes", corsMiddleware(handleSyncCategoriesChanges))

	// Obtener estadísticas de sincronización del usuario para categorías
	http.HandleFunc("/sync/categories/stats", corsMiddleware(handleSyncCategoriesStats))

	// Resolver conflictos específicos de categorías de forma manual
	http.HandleFunc("/sync/categories/resolve-conflict", corsMiddleware(handleSyncCategoriesConflictResolution))

	// Configure and start HTTP server on designated port
	// Configura y arranca el servidor HTTP en el puerto designado
	// Puerto 8096 dedicado para el servicio de gestión de categorías
	port := 8096
	log.Printf("Categories Management service started on :%d", port)
	log.Printf("Available endpoints:")
	log.Printf("  - GET  /categories")
	log.Printf("  - POST /categories/add")
	log.Printf("  - POST /categories/update")
	log.Printf("  - POST /categories/delete")
	log.Printf("  - GET  /categories/fix-emojis")
	log.Printf("  - POST /sync/categories/batch")
	log.Printf("  - GET  /sync/categories/changes")
	log.Printf("  - GET  /sync/categories/stats")
	log.Printf("  - POST /sync/categories/resolve-conflict")

	// Start HTTP server with fatal error handling
	// Arranca el servidor con manejo de errores fatales
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// runDatabaseMigration executes database migrations on service startup
// Applies schema changes from database_migration.sql to ensure database compatibility
// Returns error if migration fails, allowing service to fail fast on startup
func runDatabaseMigration() error {
	log.Printf("🔄 Categories Management - Running database migration...")

	// Get database path - use absolute production database path
	dbPath := "/opt/hero_budget/database/hero_budget.db"

	log.Printf("📂 Using database path: %s", dbPath)

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	log.Printf("✅ Database connection established")

	// Execute migration SQL statements
	migrationStatements := []string{
		// Add category_id column to incomes table
		`ALTER TABLE incomes ADD COLUMN category_id INTEGER;`,

		// Add category_id column to expenses table
		`ALTER TABLE expenses ADD COLUMN category_id INTEGER;`,

		// Create indexes for new columns
		`CREATE INDEX IF NOT EXISTS idx_incomes_category_id ON incomes(category_id);`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_category_id ON expenses(category_id);`,

		// Populate category_id for existing records - incomes
		`UPDATE incomes
		 SET category_id = (
			 SELECT c.id
			 FROM categories c
			 WHERE c.name = incomes.category
			 AND c.user_id = incomes.user_id
			 AND c.type = 'income'
			 LIMIT 1
		 )
		 WHERE category_id IS NULL AND category IS NOT NULL;`,

		// Populate category_id for existing records - expenses
		`UPDATE expenses
		 SET category_id = (
			 SELECT c.id
			 FROM categories c
			 WHERE c.name = expenses.category
			 AND c.user_id = expenses.user_id
			 AND c.type = 'expense'
			 LIMIT 1
		 )
		 WHERE category_id IS NULL AND category IS NOT NULL;`,
	}

	// Execute each migration statement
	for i, statement := range migrationStatements {
		log.Printf("🔄 Executing migration statement %d/%d...", i+1, len(migrationStatements))

		_, err := db.Exec(statement)
		if err != nil {
			// Check if error is about column already existing (not a real error)
			if strings.Contains(err.Error(), "duplicate column name") ||
				strings.Contains(err.Error(), "already exists") {
				log.Printf("💡 Migration statement %d already applied, skipping: %s", i+1, err.Error())
				continue
			}

			log.Printf("❌ Migration statement %d failed: %s", i+1, statement)
			return fmt.Errorf("migration statement %d failed: %v", i+1, err)
		}

		log.Printf("✅ Migration statement %d completed successfully", i+1)
	}

	// Verify migration success with statistics
	var incomesTotal, incomesWithCategoryId int
	var expensesTotal, expensesWithCategoryId int

	db.QueryRow("SELECT COUNT(*) FROM incomes").Scan(&incomesTotal)
	db.QueryRow("SELECT COUNT(*) FROM incomes WHERE category_id IS NOT NULL").Scan(&incomesWithCategoryId)
	db.QueryRow("SELECT COUNT(*) FROM expenses").Scan(&expensesTotal)
	db.QueryRow("SELECT COUNT(*) FROM expenses WHERE category_id IS NOT NULL").Scan(&expensesWithCategoryId)

	log.Printf("📊 Migration Statistics:")
	log.Printf("  - Incomes: %d total, %d with category_id (%.1f%%)",
		incomesTotal, incomesWithCategoryId,
		float64(incomesWithCategoryId)/float64(incomesTotal)*100)
	log.Printf("  - Expenses: %d total, %d with category_id (%.1f%%)",
		expensesTotal, expensesWithCategoryId,
		float64(expensesWithCategoryId)/float64(expensesTotal)*100)

	log.Printf("🎉 Categories Management - Database migration completed successfully!")
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
		action,                // operation_type (create, update, delete)
		tableName,             // entity_type (categories)
		recordID,              // entity_id
		string(dataJSON),      // operation_data
		string(deviceIDsJSON), // device_ids as JSON array or null
		clientTimestampValue,  // client_timestamp (original from client or null)
		serverTimestamp,       // server_timestamp (when processed)
		operationTimestamp,    // created_at (extracted from operation_id for ordering)
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

// corsMiddleware aplica headers CORS a todas las respuestas HTTP
// Permite peticiones desde cualquier origen para compatibilidad con frontend
// Maneja peticiones OPTIONS para pre-flight requests del navegador
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers para permitir peticiones cross-origin
		// Configura headers necesarios para comunicación entre dominios
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight OPTIONS requests
		// Maneja peticiones preflight OPTIONS del navegador
		// Responde con headers CORS sin procesar la petición real
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler in the chain
		// Ejecuta el siguiente handler en la cadena de middleware
		next(w, r)
	}
}

// getEnvOrDefault función utilitaria para obtener variables de entorno
// Retorna el valor de la variable de entorno o un valor por defecto
// Utilizada para configuración flexible del servicio
func getEnvOrDefault(key, defaultValue string) string {
	// Retrieve environment variable value
	// Obtiene el valor de la variable de entorno especificada
	if value := os.Getenv(key); value != "" {
		return value
	}
	// Return default value if environment variable is not set
	// Retorna valor por defecto si la variable no está configurada
	return defaultValue
}
