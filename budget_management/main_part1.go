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
	_ "github.com/mattn/go-sqlite3"
)

// Variables globales para conexión a la base de datos y contexto
// Permiten acceso consistente a recursos compartidos en todo el servicio
var (
	// Database connection for budget data persistence
	db *sql.DB
	// Context for database operations
	ctx = context.Background()
)

// Estructuras de datos principales para gestión de presupuestos
// Definición de tipos utilizados en toda la aplicación de presupuestos

// BudgetData representa los datos de presupuesto de un usuario
// Contiene información completa sobre ingresos, gastos y balance disponible
type BudgetData struct {
	UserID          string  `json:"user_id"`          // ID único del usuario propietario
	Period          string  `json:"period"`           // Período del presupuesto (mensual, semanal, etc.)
	Date            string  `json:"date"`             // Fecha de creación/actualización
	TotalAmount     float64 `json:"total_amount"`     // Monto total disponible
	RemainingAmount float64 `json:"remaining_amount"` // Monto restante disponible
	SpentAmount     float64 `json:"spent_amount"`     // Monto ya gastado
	UpcomingAmount  float64 `json:"upcoming_amount"`  // Monto comprometido en gastos futuros
	FromPrevious    float64 `json:"from_previous"`    // Monto heredado del período anterior
	Percent         float64 `json:"percent"`          // Porcentaje de presupuesto utilizado
	TotalIncome     float64 `json:"total_income"`     // Total de ingresos del período
}

// BudgetUpdateRequest estructura para solicitudes de actualización de presupuesto
// Permite actualizar valores específicos sin afectar el resto del presupuesto
type BudgetUpdateRequest struct {
	UserID         string  `json:"user_id"`         // ID del usuario que realiza la actualización
	Period         string  `json:"period"`          // Período del presupuesto a actualizar
	TotalAmount    float64 `json:"total_amount"`    // Nuevo monto total (si aplica)
	SpentAmount    float64 `json:"spent_amount"`    // Monto gastado actualizado
	UpcomingAmount float64 `json:"upcoming_amount"` // Monto de gastos futuros
	FromPrevious   float64 `json:"from_previous"`   // Monto heredado actualizado
	TotalIncome    float64 `json:"total_income"`    // Total de ingresos actualizado
}

// ApiResponse estructura estándar para respuestas de la API de presupuestos
// Proporciona format consistente para todas las respuestas del servicio
type ApiResponse struct {
	Success bool        `json:"success"`          // Indica si la operación fue exitosa
	Message string      `json:"message,omitempty"` // Mensaje descriptivo del resultado
	Data    interface{} `json:"data,omitempty"`   // Datos de respuesta (opcional)
}

// init inicializa la conexión a la base de datos y configura el entorno
// Se ejecuta automáticamente al iniciar el servicio
func init() {
	// Parse command line flags para configuración de entorno
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
	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	log.Println("Database connection established successfully")
}

// main función principal que inicia el servidor y configura las rutas
// Establece el servidor HTTP con middleware CORS y handlers de presupuesto
func main() {
	// Set up CORS middleware and budget management routes
	http.HandleFunc("/budget/fetch", corsMiddleware(handleFetchBudget))
	http.HandleFunc("/budget/update", corsMiddleware(handleUpdateBudget))
	
	// Rutas de sincronización offline para presupuestos
	http.HandleFunc("/budget/sync/health", corsMiddleware(handleSyncBudgetHealth))
	http.HandleFunc("/budget/sync/batch", corsMiddleware(handleSyncBudgetBatch))
	http.HandleFunc("/budget/sync/changes", corsMiddleware(handleSyncBudgetChanges))

	port := 8088
	log.Printf("Budget Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// corsMiddleware aplica headers CORS a todas las respuestas
// Permite acceso desde diferentes dominios y métodos HTTP
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers para permitir acceso cross-origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

// handleFetchBudget maneja las solicitudes de obtención de datos de presupuesto
// Endpoint: GET /budget/fetch
// Retorna los datos del presupuesto para un usuario y período específicos
func handleFetchBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract user ID from query parameters
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Extract period with default value
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	// Fetch budget data from database
	budget, err := fetchBudgetData(userID, period)
	if err != nil {
		log.Printf("Error fetching budget data: %v", err)
		sendErrorResponse(w, "Error fetching budget data", http.StatusInternalServerError)
		return
	}

	// Return successful response with budget data
	sendSuccessResponse(w, "Budget data fetched successfully", budget)
}

// handleUpdateBudget maneja las solicitudes de actualización de presupuesto
// Endpoint: POST /budget/update
// Actualiza los datos del presupuesto para un usuario específico
func handleUpdateBudget(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest BudgetUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Set default period if not provided
	if updateRequest.Period == "" {
		updateRequest.Period = "monthly"
	}

	// Calculate derived values
	remainingAmount := updateRequest.FromPrevious + updateRequest.TotalIncome - updateRequest.SpentAmount - updateRequest.UpcomingAmount
	
	var percent float64
	totalAvailable := updateRequest.FromPrevious + updateRequest.TotalIncome
	if totalAvailable > 0 {
		percent = ((updateRequest.SpentAmount + updateRequest.UpcomingAmount) / totalAvailable) * 100
	}

	// Create budget data structure
	budget := BudgetData{
		UserID:          updateRequest.UserID,
		Period:          updateRequest.Period,
		Date:            time.Now().Format("2006-01-02"),
		TotalAmount:     totalAvailable,
		RemainingAmount: remainingAmount,
		SpentAmount:     updateRequest.SpentAmount,
		UpcomingAmount:  updateRequest.UpcomingAmount,
		FromPrevious:    updateRequest.FromPrevious,
		Percent:         percent,
		TotalIncome:     updateRequest.TotalIncome,
	}

	// Update budget in database
	err = updateBudgetData(budget)
	if err != nil {
		log.Printf("Error updating budget data: %v", err)
		sendErrorResponse(w, "Error updating budget data", http.StatusInternalServerError)
		return
	}

	// Return successful response with updated budget data
	sendSuccessResponse(w, "Budget updated successfully", budget)
}

// getEnvOrDefault obtiene el valor de una variable de entorno o retorna un valor por defecto
// Utilizado para configuración flexible del servicio
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}