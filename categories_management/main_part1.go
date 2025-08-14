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

	"github.com/joho/godotenv"
	"github.com/herobudget/backend/common"
	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras principales de datos para Categories Management
// Estas estructuras representan las categorías de ingresos y gastos del usuario
// Incluyen validación de tipos, codificación de emojis y gestión de metadatos

// Category representa una categoría de ingreso o gasto del usuario
// Incluye información completa para clasificación y visualización de transacciones
// Utilizada para organizar y filtrar operaciones financieras del usuario
type Category struct {
	ID        int    `json:"id"`                  // ID único de la categoría en la base de datos
	UserID    string `json:"user_id"`             // ID del usuario propietario de la categoría
	Name      string `json:"name"`                // Nombre descriptivo de la categoría
	Type      string `json:"type"`                // "income" para ingresos, "expense" para gastos
	Emoji     string `json:"emoji"`               // Emoji representativo para interfaz visual
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
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// UpdateCategoryRequest representa una solicitud para actualizar una categoría existente
// Permite actualización parcial de campos, manteniendo valores existentes para campos vacíos
// Incluye validación de permisos de usuario y existencia de la categoría
type UpdateCategoryRequest struct {
	UserID     string `json:"user_id"`             // ID del usuario propietario (requerido para validación)
	CategoryID int    `json:"category_id"`         // ID de la categoría a actualizar (requerido)
	Name       string `json:"name,omitempty"`      // Nuevo nombre (opcional)
	Type       string `json:"type,omitempty"`      // Nuevo tipo (opcional)
	Emoji      string `json:"emoji,omitempty"`     // Nuevo emoji (opcional)
	
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
}

// DeleteCategoryRequest representa una solicitud para eliminar una categoría
// Incluye validación de permisos y verificación de dependencias antes de eliminar
// Asegura que solo el propietario pueda eliminar sus categorías
type DeleteCategoryRequest struct {
	UserID     string `json:"user_id"`     // ID del usuario propietario (requerido)
	CategoryID int    `json:"category_id"` // ID de la categoría a eliminar (requerido)
	
	// Sync operation parameters for incremental synchronization tracking
	OperationID   string  `json:"operation_id,omitempty"`   // Unique operation identifier for sync
	DeviceID      string  `json:"device_id,omitempty"`      // Device identifier for sync
	Timestamp     int64   `json:"timestamp,omitempty"`      // Client-side timestamp for sync ordering
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

// main función principal que configura las rutas HTTP y arranca el servidor
// Define todos los endpoints disponibles y configura middleware CORS
// Establece el puerto de escucha y arranca el servidor HTTP
func main() {
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

// addSyncOperation registra una operación de sincronización en la tabla sync_operations
// Exactly like in other services for consistent synchronization tracking
func addSyncOperation(userID, operationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, operation=%s, action=%s, table=%s, record=%s", 
		userID, operationID, action, tableName, recordID)
	
	// Serialize operation data to JSON for storage
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling sync operation data: %v", err)
		return err
	}
	
	// Use current server timestamp
	serverTimestamp := time.Now().Unix()
	
	// Insert sync operation record with all required fields
	// Use client timestamp for created_at to maintain proper synchronization ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, action, table_name, record_id, data, 
			device_id, client_timestamp, server_timestamp, created_at
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
		deviceID,
		clientTimestamp,
		serverTimestamp,
		clientTimestamp, // Use client timestamp for created_at
	)
	
	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}
	
	// Log successful operation insertion for debugging
	insertedID, _ := result.LastInsertId()
	log.Printf("Successfully inserted sync operation with ID: %d", insertedID)
	
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