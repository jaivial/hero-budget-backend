package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DatabaseInitializer maneja la inicialización centralizada de la base de datos
type DatabaseInitializer struct {
	dbPath     string
	schemaPath string
	db         *sql.DB
}

// NewDatabaseInitializer crea una nueva instancia del inicializador
func NewDatabaseInitializer() (*DatabaseInitializer, error) {
	// Obtener directorio actual
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %v", err)
	}

	// Configurar rutas
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	schemaPath := filepath.Join(cwd, "..", "database_schema.sql")

	// Verificar que el archivo de esquema existe
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("schema file not found at: %s", schemaPath)
	}

	// Abrir conexión a la base de datos
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &DatabaseInitializer{
		dbPath:     dbPath,
		schemaPath: schemaPath,
		db:         db,
	}, nil
}

// InitializeSchema aplica el esquema centralizado a la base de datos
func (di *DatabaseInitializer) InitializeSchema() error {
	log.Printf("🔧 Aplicando esquema centralizado desde: %s", di.schemaPath)

	// Leer el archivo de esquema
	schemaContent, err := ioutil.ReadFile(di.schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	// Ejecutar el esquema SQL
	_, err = di.db.Exec(string(schemaContent))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %v", err)
	}

	log.Println("✅ Esquema centralizado aplicado exitosamente")

	// Verificar versión del esquema
	version, err := di.GetSchemaVersion()
	if err != nil {
		log.Printf("⚠️ Warning: Could not verify schema version: %v", err)
	} else {
		log.Printf("📋 Versión del esquema: %s", version)
	}

	return nil
}

// GetSchemaVersion obtiene la versión actual del esquema
func (di *DatabaseInitializer) GetSchemaVersion() (string, error) {
	var version string
	err := di.db.QueryRow("SELECT version FROM schema_version WHERE id = 1").Scan(&version)
	if err != nil {
		return "", err
	}
	return version, nil
}

// VerifyTables verifica que todas las tablas críticas existen
func (di *DatabaseInitializer) VerifyTables() error {
	criticalTables := []string{
		"users",
		"categories",
		"incomes",
		"expenses", 
		"bills",
		"bill_payments",
		"cash_bank",
		"cash_bank_transactions",
		"daily_cash_bank_balance",
		"weekly_cash_bank_balance",
		"monthly_cash_bank_balance",
		"quarterly_cash_bank_balance",
		"semiannual_cash_bank_balance",
		"annual_cash_bank_balance",
		"budget",
		"savings",
		"finance_metrics",
		"balances",
		"daily_balance",
		"weekly_balance",
		"quarterly_balance",
		"semiannual_balance",
		"annual_balance",
		"schema_version",
	}

	log.Println("🔍 Verificando existencia de tablas críticas...")
	
	for _, table := range criticalTables {
		var name string
		err := di.db.QueryRow(`
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name=?
		`, table).Scan(&name)
		
		if err == sql.ErrNoRows {
			return fmt.Errorf("tabla crítica faltante: %s", table)
		} else if err != nil {
			return fmt.Errorf("error verificando tabla %s: %v", table, err)
		}
	}

	log.Printf("✅ Todas las %d tablas críticas verificadas exitosamente", len(criticalTables))
	return nil
}

// GetDatabaseStats obtiene estadísticas de la base de datos
func (di *DatabaseInitializer) GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Conteo de usuarios
	var userCount int
	err := di.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		return nil, fmt.Errorf("error counting users: %v", err)
	}
	stats["total_users"] = userCount

	// Conteo de tablas
	var tableCount int
	err = di.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
	`).Scan(&tableCount)
	if err != nil {
		return nil, fmt.Errorf("error counting tables: %v", err)
	}
	stats["total_tables"] = tableCount

	// Tamaño de la base de datos
	fileInfo, err := os.Stat(di.dbPath)
	if err != nil {
		return nil, fmt.Errorf("error getting database file size: %v", err)
	}
	stats["database_size_bytes"] = fileInfo.Size()
	stats["database_size_mb"] = float64(fileInfo.Size()) / (1024 * 1024)

	// Última modificación
	stats["last_modified"] = fileInfo.ModTime().Format(time.RFC3339)

	return stats, nil
}

// Close cierra la conexión a la base de datos
func (di *DatabaseInitializer) Close() error {
	if di.db != nil {
		return di.db.Close()
	}
	return nil
}

// API Response structure
type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// handleInitializeSchema inicializa el esquema de la base de datos
func handleInitializeSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Crear inicializador
	initializer, err := NewDatabaseInitializer()
	if err != nil {
		log.Printf("Error creating database initializer: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	defer initializer.Close()

	// Inicializar esquema
	err = initializer.InitializeSchema()
	if err != nil {
		log.Printf("Error initializing schema: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	// Verificar tablas
	err = initializer.VerifyTables()
	if err != nil {
		log.Printf("Error verifying tables: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	// Obtener estadísticas
	stats, err := initializer.GetDatabaseStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		stats = map[string]interface{}{"error": "Could not retrieve stats"}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success": true, "message": "Database schema initialized successfully", "data": %v}`, stats)
}

// handleDatabaseStatus obtiene el estado de la base de datos
func handleDatabaseStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Crear inicializador
	initializer, err := NewDatabaseInitializer()
	if err != nil {
		log.Printf("Error creating database initializer: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	defer initializer.Close()

	// Verificar tablas
	err = initializer.VerifyTables()
	if err != nil {
		log.Printf("Error verifying tables: %v", err)
		http.Error(w, fmt.Sprintf("Tables verification failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Obtener estadísticas
	stats, err := initializer.GetDatabaseStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	// Obtener versión del esquema
	version, err := initializer.GetSchemaVersion()
	if err != nil {
		log.Printf("Error getting schema version: %v", err)
		version = "unknown"
	}
	stats["schema_version"] = version

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success": true, "message": "Database status retrieved successfully", "data": %v}`, stats)
}

func main() {
	// Configurar rutas HTTP
	http.HandleFunc("/database/init", corsMiddleware(handleInitializeSchema))
	http.HandleFunc("/database/status", corsMiddleware(handleDatabaseStatus))

	// Health check endpoint
	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "message": "Database initialization service is healthy", "service": "database_init", "timestamp": %d}`, time.Now().Unix())
	}))

	port := 8200 // Puerto dedicado para el servicio de inicialización de base de datos
	log.Printf("🚀 Database Initialization Service started on :%d", port)
	log.Printf("📋 Endpoints available:")
	log.Printf("  POST /database/init   - Initialize database schema")
	log.Printf("  GET  /database/status - Get database status")
	log.Printf("  GET  /health          - Health check")
	
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}