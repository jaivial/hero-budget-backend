// Microservicio initial-fetch-db: Fetch unificado de todos los datos del usuario
// Puerto: 8102
// Función: Recoger datos de todas las tablas de una vez desde /opt/hero_budget/database/hero_budget.db

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// Configuración del microservicio
type Config struct {
	Port         string
	DatabasePath string
}

// Estructura para respuesta unificada de datos iniciales
type InitialDataResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
	UserID  string                 `json:"user_id"`
	Tables  []string               `json:"tables"`
	Count   map[string]int         `json:"count"`
}

// Estructura para datos de error
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

// Variables globales
var (
	db     *sql.DB
	config Config
)

// Configuración por defecto
func init() {
	config = Config{
		Port:         getEnv("PORT", "8102"),
		DatabasePath: getEnv("DATABASE_PATH", "/opt/hero_budget/database/hero_budget.db"),
	}
}

// Función auxiliar para obtener variables de entorno
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Inicializar conexión a la base de datos
func initDatabase() error {
	var err error
	
	// Verificar que el archivo de base de datos existe
	if _, err := os.Stat(config.DatabasePath); os.IsNotExist(err) {
		return fmt.Errorf("database file does not exist: %s", config.DatabasePath)
	}
	
	// Abrir conexión a SQLite
	db, err = sql.Open("sqlite3", config.DatabasePath+"?_busy_timeout=30000&_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	
	// Configurar pool de conexiones
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	
	// Probar conexión
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}
	
	log.Printf("✅ Database connected successfully: %s", config.DatabasePath)
	return nil
}

// Endpoint principal: obtener todos los datos del usuario
func handleInitialFetch(w http.ResponseWriter, r *http.Request) {
	// Configurar headers CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Content-Type", "application/json")
	
	// Manejar preflight OPTIONS
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	// Solo permitir GET y POST
	if r.Method != "GET" && r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Obtener user_id de query parameters o body
	userID := getUserID(r)
	if userID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}
	
	log.Printf("🔍 Fetching initial data for user: %s", userID)
	
	// Obtener datos de todas las tablas
	allData, err := fetchAllUserData(userID)
	if err != nil {
		log.Printf("❌ Error fetching data for user %s: %v", userID, err)
		sendErrorResponse(w, fmt.Sprintf("Failed to fetch data: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Contar registros por tabla
	counts := make(map[string]int)
	tables := []string{}
	for tableName, data := range allData {
		tables = append(tables, tableName)
		if dataSlice, ok := data.([]map[string]interface{}); ok {
			counts[tableName] = len(dataSlice)
		} else {
			counts[tableName] = 1
		}
	}
	
	// Preparar respuesta
	response := InitialDataResponse{
		Success: true,
		Message: "Initial data fetched successfully",
		Data:    allData,
		UserID:  userID,
		Tables:  tables,
		Count:   counts,
	}
	
	// Log de éxito
	totalRecords := 0
	for _, count := range counts {
		totalRecords += count
	}
	log.Printf("✅ Successfully fetched %d records from %d tables for user %s", totalRecords, len(tables), userID)
	
	// Enviar respuesta
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("❌ Error encoding response: %v", err)
		sendErrorResponse(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Obtener user_id desde request
func getUserID(r *http.Request) string {
	// Intentar desde query parameters
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		return userID
	}
	
	// Intentar desde POST body
	if r.Method == "POST" {
		var requestData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&requestData); err == nil {
			if userID, ok := requestData["user_id"].(string); ok {
				return userID
			}
		}
	}
	
	return ""
}

// Función principal para obtener todos los datos del usuario
func fetchAllUserData(userID string) (map[string]interface{}, error) {
	allData := make(map[string]interface{})
	
	// Lista de tablas a consultar
	tables := []string{
		"bill_payments",
		"bills", 
		"categories",
		"expenses",
		"incomes",
		"monthly_cash_bank_balance",
		"savings",
		"users",
	}
	
	// Obtener datos de cada tabla
	for _, tableName := range tables {
		data, err := fetchTableData(tableName, userID)
		if err != nil {
			log.Printf("⚠️ Warning: Failed to fetch data from table %s: %v", tableName, err)
			// No fallar completamente, solo registrar el error y continuar
			allData[tableName] = []map[string]interface{}{}
			continue
		}
		allData[tableName] = data
	}
	
	return allData, nil
}

// Obtener datos de una tabla específica
func fetchTableData(tableName, userID string) (interface{}, error) {
	switch tableName {
	case "users":
		return fetchUsers(userID)
	case "savings":
		return fetchSavings(userID)
	default:
		return fetchGenericTable(tableName, userID)
	}
}

// Fetch genérico para tablas con user_id
func fetchGenericTable(tableName, userID string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE user_id = ?", tableName)
	
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("query failed for table %s: %v", tableName, err)
	}
	defer rows.Close()
	
	// Obtener nombres de columnas
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns for table %s: %v", tableName, err)
	}
	
	var results []map[string]interface{}
	
	for rows.Next() {
		// Crear slice de interfaces para escanear
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}
		
		// Escanear fila
		if err := rows.Scan(valuePointers...); err != nil {
			log.Printf("⚠️ Warning: Failed to scan row in table %s: %v", tableName, err)
			continue
		}
		
		// Convertir a mapa
		record := make(map[string]interface{})
		for i, column := range columns {
			val := values[i]
			
			// Convertir []byte a string para mejor JSON
			if b, ok := val.([]byte); ok {
				record[column] = string(b)
			} else {
				record[column] = val
			}
		}
		
		results = append(results, record)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error for table %s: %v", tableName, err)
	}
	
	return results, nil
}

// Fetch específico para tabla users (por ID en lugar de user_id)
func fetchUsers(userID string) (map[string]interface{}, error) {
	query := "SELECT * FROM users WHERE id = ?"
	
	row := db.QueryRow(query, userID)
	
	// Obtener información de columnas ejecutando una query auxiliar
	columnsQuery := "PRAGMA table_info(users)"
	rows, err := db.Query(columnsQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get table info for users: %v", err)
	}
	defer rows.Close()
	
	var columns []string
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString
		
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			continue
		}
		columns = append(columns, name)
	}
	
	// Crear slice para escanear valores
	values := make([]interface{}, len(columns))
	valuePointers := make([]interface{}, len(columns))
	for i := range values {
		valuePointers[i] = &values[i]
	}
	
	// Escanear fila
	if err := row.Scan(valuePointers...); err != nil {
		if err == sql.ErrNoRows {
			return map[string]interface{}{}, nil // Usuario no encontrado, devolver objeto vacío
		}
		return nil, fmt.Errorf("failed to scan user data: %v", err)
	}
	
	// Convertir a mapa
	result := make(map[string]interface{})
	for i, column := range columns {
		val := values[i]
		
		// Convertir []byte a string para mejor JSON
		if b, ok := val.([]byte); ok {
			result[column] = string(b)
		} else {
			result[column] = val
		}
	}
	
	return result, nil
}

// Fetch específico para tabla savings - usar método genérico para evitar errores de schema
func fetchSavings(userID string) ([]map[string]interface{}, error) {
	// Usar el método genérico que maneja dinámicamente las columnas
	return fetchGenericTable("savings", userID)
}

// Función auxiliar para obtener valores de sql.Null*
func getValue(v interface{}) interface{} {
	switch val := v.(type) {
	case sql.NullString:
		if val.Valid {
			return val.String
		}
		return ""
	case sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return 0
	case sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return 0.0
	case sql.NullBool:
		if val.Valid {
			return val.Bool
		}
		return false
	default:
		return v
	}
}

// Endpoint de health check
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Verificar conexión a la base de datos
	if err := db.Ping(); err != nil {
		sendErrorResponse(w, fmt.Sprintf("Database connection failed: %v", err), http.StatusServiceUnavailable)
		return
	}
	
	response := map[string]interface{}{
		"status":        "healthy",
		"service":       "initial-fetch-db",
		"version":       "1.0.0",
		"database":      "connected",
		"database_path": config.DatabasePath,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Función para enviar respuesta de error
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := ErrorResponse{
		Success: false,
		Error:   message,
		Code:    statusCode,
	}
	json.NewEncoder(w).Encode(response)
}

// Configurar rutas
func setupRoutes() *mux.Router {
	r := mux.NewRouter()
	
	// Endpoint principal
	r.HandleFunc("/initial-fetch", handleInitialFetch).Methods("GET", "POST", "OPTIONS")
	
	// Health check
	r.HandleFunc("/health", handleHealth).Methods("GET")
	r.HandleFunc("/initial-fetch/health", handleHealth).Methods("GET")
	
	return r
}

// Función principal
func main() {
	log.Printf("🚀 Starting initial-fetch-db microservice on port %s", config.Port)
	log.Printf("📁 Database path: %s", config.DatabasePath)
	
	// Inicializar base de datos
	if err := initDatabase(); err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Configurar rutas
	router := setupRoutes()
	
	// Configurar servidor
	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	log.Printf("✅ initial-fetch-db microservice started successfully")
	log.Printf("🔗 Health check: http://localhost:%s/health", config.Port)
	log.Printf("🔗 Initial fetch: http://localhost:%s/initial-fetch?user_id=USER_ID", config.Port)
	
	// Iniciar servidor
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}