package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

// Variable global para la conexión a la base de datos
var db *sql.DB

// Redis client for caching frequently accessed bill lists and bill data
var rdb *redis.Client

// Context for Redis operations with timeout handling
var ctx = context.Background()

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
	var err error
	dbPath := "../google_auth/users.db"
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Using database at: %s\n", dbPath)
	createTablesIfNotExist()
	log.Println("Database connection established successfully")

	// Initialize Redis connection for caching bill data
	initRedis()
}

// initRedis initializes Redis connection for caching bill lists and data
// Provides performance optimization for frequently accessed bill information
func initRedis() {
	// Redis connection configuration with environment variable support
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address (localhost on VPS)
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "Jva-Mvc-5171" // Default Redis AUTH password
	}
	redisDB := 0 // Default Redis database index

	// Create Redis client with connection pooling and automatic failover
	rdb = redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		DB:           redisDB,
		DialTimeout:  5 * time.Second, // Connection timeout for Redis dial
		ReadTimeout:  3 * time.Second, // Read timeout for Redis operations
		WriteTimeout: 3 * time.Second, // Write timeout for Redis operations
		PoolSize:     10,              // Connection pool size for concurrent operations
		MinIdleConns: 2,               // Minimum idle connections in pool
	})

	// Test Redis connection with ping command
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Printf("⚠️ Redis connection failed: %v (continuing without cache)", err)
		return
	}

	log.Printf("✅ Redis connected successfully: %s", pong)
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

// getStringValueOrDefault retorna el string proporcionado o el valor por defecto si está vacío
// Útil para campos de texto opcionales en actualizaciones
func getStringValueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
