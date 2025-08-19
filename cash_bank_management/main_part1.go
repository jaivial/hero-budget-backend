package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/herobudget/backend/common"
	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras principales de datos para Cash Bank Management
// Estas estructuras representan la distribución de efectivo y banco, así como
// las operaciones de transferencia entre ambos medios de almacenamiento

// CashBankDistribution representa la distribución actual de efectivo y banco del usuario
// Incluye tanto las cantidades absolutas como los porcentajes de distribución
// Utilizada para mostrar el balance actual y realizar cálculos de disponibilidad
type CashBankDistribution struct {
	UserID       string  `json:"user_id"`       // ID único del usuario propietario
	Month        string  `json:"month"`         // Mes de referencia para la distribución
	CashAmount   float64 `json:"cash_amount"`   // Cantidad disponible en efectivo
	CashPercent  float64 `json:"cash_percent"`  // Porcentaje del total que representa el efectivo
	BankAmount   float64 `json:"bank_amount"`   // Cantidad disponible en banco
	BankPercent  float64 `json:"bank_percent"`  // Porcentaje del total que representa el banco
	MonthlyTotal float64 `json:"monthly_total"` // Total combinado de efectivo y banco
}

// TransferRequest representa una solicitud de transferencia entre efectivo y banco
// Utilizada para operaciones de movimiento de dinero entre los dos medios
// Incluye validación de fecha para mantener histórico correcto y parámetros de sync
type TransferRequest struct {
	UserID string  `json:"user_id"` // ID del usuario que realiza la transferencia
	Amount float64 `json:"amount"`  // Cantidad a transferir (debe ser positiva)
	Date   string  `json:"date"`    // Fecha de la transferencia en formato ISO
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

// UpdateAmountRequest representa una solicitud de actualización directa de cantidades
// Permite modificar el saldo de efectivo o banco directamente
// Útil para correcciones o ajustes manuales del usuario
type UpdateAmountRequest struct {
	UserID string  `json:"user_id"` // ID del usuario que actualiza la cantidad
	Amount float64 `json:"amount"`  // Nueva cantidad (puede ser cero o positiva)
	Date   string  `json:"date"`    // Fecha de la actualización para histórico
}

// ApiResponse estructura estándar para respuestas de la API REST
// Proporciona formato consistente para todas las respuestas del servicio
// Incluye indicador de éxito, mensaje descriptivo y datos opcionales
type ApiResponse struct {
	Success bool        `json:"success"`           // Indica si la operación fue exitosa
	Message string      `json:"message,omitempty"` // Mensaje descriptivo del resultado
	Data    interface{} `json:"data,omitempty"`    // Datos de respuesta (opcional)
}

// Variables globales del sistema para gestión de base de datos y cache
// Estas variables son inicializadas una vez y reutilizadas en toda la aplicación
var (
	// Database connection for financial data persistence
	// Conexión principal a la base de datos SQLite para persistencia de datos financieros
	db *sql.DB
	
	// Context for database operations
	// Contexto compartido para todas las operaciones de base de datos
	ctx = context.Background()
	
	// Cache manager for Redis operations
	// Gestor de cache Redis para optimizar consultas frecuentes
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

	// CENTRALIZED SCHEMA: DDL operations moved to database_schema.sql
	// Schema centralizado: todas las operaciones DDL se manejan centralmente
	// Las tablas son creadas por el servicio de inicialización centralizada
	log.Println("✅ Using centralized database schema - no local DDL operations")
	
	// Update sync_operations schema for cash_bank operation types
	// Actualiza el esquema de sync_operations para soportar operaciones cash_bank
	if err := updateSyncOperationsSchema(); err != nil {
		log.Fatalf("❌ Failed to update sync_operations schema: %v", err)
	}
	
	// Initialize cache manager for performance optimization
	// Inicializa el gestor de cache Redis para optimizar consultas frecuentes
	// Si falla la inicialización, continúa sin cache (degradación elegante)
	cacheManager, err = common.NewCacheManager()
	if err != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err)
		cacheManager = nil
	}

	log.Println("Database connection established successfully")
	log.Println("Cash Bank Management service initialized successfully")
}

// main función principal que configura las rutas HTTP y arranca el servidor
// Define todos los endpoints disponibles y configura middleware CORS
// Establece el puerto de escucha y arranca el servidor HTTP
func main() {
	// Set up CORS middleware and routes para endpoints principales
	// Configura middleware CORS y define todas las rutas HTTP disponibles
	// Cada ruta incluye validación de métodos HTTP y manejo de errores
	
	// Endpoint para obtener distribución actual de efectivo/banco
	http.HandleFunc("/cash-bank/distribution", corsMiddleware(handleFetchDistribution))
	
	// Endpoints para actualizar cantidades directamente - CON SOPORTE DE SYNC CONSISTENTE
	http.HandleFunc("/cash-bank/cash/update", corsMiddleware(handleUpdateCashWithSync))
	http.HandleFunc("/cash-bank/bank/update", corsMiddleware(handleUpdateBankWithSync))
	
	// Endpoints para transferencias entre efectivo y banco - CON SOPORTE DE SYNC CONSISTENTE
	http.HandleFunc("/transfer/cash-to-bank", corsMiddleware(handleCashToBankTransferWithSync))
	http.HandleFunc("/transfer/bank-to-cash", corsMiddleware(handleBankToCashTransferWithSync))

	// Rutas de sincronización offline - Integración del sistema de sync
	// Endpoints para sincronización bidireccional con clientes offline
	// Permite operaciones por lotes y resolución de conflictos
	
	// Sincronización por lotes de operaciones offline
	http.HandleFunc("/sync/cashbank/batch", corsMiddleware(handleSyncCashBankBatch))
	
	// Obtener cambios del servidor desde último sync
	http.HandleFunc("/sync/cashbank/changes", corsMiddleware(handleSyncCashBankChanges))
	
	// Obtener estadísticas de sincronización del usuario
	http.HandleFunc("/sync/cashbank/stats", corsMiddleware(handleSyncCashBankStats))

	// Rutas de sincronización de eliminación de transacciones - Transaction Delete Service
	// Endpoints especializados para sincronización de eliminaciones offline
	// Incluyen validación de integridad y detección de conflictos específicos de eliminaciones
	
	// Sincronización por lotes de eliminaciones de transacciones offline
	http.HandleFunc("/sync/transaction-delete/batch", corsMiddleware(handleSyncTransactionDeleteBatch))
	
	// Obtener cambios de eliminaciones del servidor desde último sync
	http.HandleFunc("/sync/transaction-delete/changes", corsMiddleware(handleSyncTransactionDeleteChanges))
	
	// Obtener estadísticas de sincronización de eliminaciones
	http.HandleFunc("/sync/transaction-delete/stats", corsMiddleware(handleSyncTransactionDeleteStats))
	
	// Resolver conflictos específicos de eliminación de transacciones
	http.HandleFunc("/sync/transaction-delete/resolve-conflict", corsMiddleware(handleSyncTransactionDeleteConflictResolution))

	// Configure and start HTTP server on designated port
	// Configura y arranca el servidor HTTP en el puerto designado
	// Puerto 8090 dedicado para el servicio de gestión de efectivo/banco
	port := 8090
	log.Printf("Cash Bank Management service started on :%d", port)
	log.Printf("Available endpoints:")
	log.Printf("  - GET  /cash-bank/distribution")
	log.Printf("  - POST /cash-bank/cash/update")
	log.Printf("  - POST /cash-bank/bank/update") 
	log.Printf("  - POST /transfer/cash-to-bank")
	log.Printf("  - POST /transfer/bank-to-cash")
	log.Printf("  - POST /sync/cashbank/batch")
	log.Printf("  - GET  /sync/cashbank/changes")
	log.Printf("  - POST /sync/cashbank/resolve-conflict")
	log.Printf("  - GET  /sync/cashbank/stats")
	log.Printf("  - POST /sync/transaction-delete/batch")
	log.Printf("  - GET  /sync/transaction-delete/changes")
	log.Printf("  - GET  /sync/transaction-delete/stats")
	log.Printf("  - POST /sync/transaction-delete/resolve-conflict")
	
	// Start HTTP server with fatal error handling
	// Arranca el servidor con manejo de errores fatales
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
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

