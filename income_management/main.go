package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Definición de estructuras de datos
type Income struct {
	ID            int     `json:"id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"` // "cash" o "bank"
	Description   string  `json:"description,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

type AddIncomeRequest struct {
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	Category      string  `json:"category"`
	PaymentMethod string  `json:"payment_method"`
	Description   string  `json:"description,omitempty"`
}

type UpdateIncomeRequest struct {
	UserID        string  `json:"user_id"`
	IncomeID      int     `json:"income_id"`
	Amount        float64 `json:"amount,omitempty"`
	Date          string  `json:"date,omitempty"`
	Category      string  `json:"category,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	Description   string  `json:"description,omitempty"`
}

type DeleteIncomeRequest struct {
	UserID   string `json:"user_id"`
	IncomeID int    `json:"income_id"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var (
	db *sql.DB
)

func init() {
	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create tables if they don't exist
	createTablesIfNotExist()

	// Función para añadir columnas de forma segura a una tabla existente
	alterTableSafely := func(tableName, columnName, columnType string) {
		// Comprobar si la columna ya existe
		var exists bool
		query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
		rows, err := db.Query(query)
		if err != nil {
			log.Printf("Error checking table schema: %v", err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name string
			var dataType string
			var notnull int
			var dflt_value interface{}
			var pk int
			if err := rows.Scan(&cid, &name, &dataType, &notnull, &dflt_value, &pk); err != nil {
				log.Printf("Error scanning row: %v", err)
				return
			}
			if name == columnName {
				exists = true
				break
			}
		}

		// Si la columna no existe, añadirla
		if !exists {
			alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s DEFAULT 0", tableName, columnName, columnType)
			_, err := db.Exec(alterQuery)
			if err != nil {
				log.Printf("Error adding column %s to %s: %v", columnName, tableName, err)
				return
			}
			log.Printf("Added column %s to %s", columnName, tableName)
		}
	}

	// Verificar si la tabla existe
	tableExists := func(tableName string) bool {
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		var name string
		err := db.QueryRow(query, tableName).Scan(&name)
		return err == nil
	}

	// Añadir columnas necesarias a todas las tablas de balance
	ensureRequiredColumns := func() {
		tables := []string{
			"daily_cash_bank_balance",
			"weekly_cash_bank_balance",
			"monthly_cash_bank_balance",
			"quarterly_cash_bank_balance",
			"semiannual_cash_bank_balance",
			"annual_cash_bank_balance",
		}

		for _, table := range tables {
			if !tableExists(table) {
				continue
			}

			// Añadir columnas estándar a todas las tablas
			alterTableSafely(table, "cash_amount", "REAL")
			alterTableSafely(table, "bank_amount", "REAL")
			alterTableSafely(table, "previous_cash_amount", "REAL")
			alterTableSafely(table, "previous_bank_amount", "REAL")
			alterTableSafely(table, "balance_cash_amount", "REAL")
			alterTableSafely(table, "balance_bank_amount", "REAL")
			alterTableSafely(table, "total_previous_balance", "REAL")
			alterTableSafely(table, "total_balance", "REAL")

			// Columnas específicas para cada tabla
			if table == "weekly_cash_bank_balance" {
				alterTableSafely(table, "start_date", "TEXT")
				alterTableSafely(table, "end_date", "TEXT")
			}
		}

		// Asegurarse de que existe la tabla cash_bank
		if !tableExists("cash_bank") {
			_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS cash_bank (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id TEXT NOT NULL,
					month TEXT NOT NULL,
					cash_amount REAL NOT NULL DEFAULT 0,
					cash_percent REAL NOT NULL DEFAULT 0,
					bank_amount REAL NOT NULL DEFAULT 0,
					bank_percent REAL NOT NULL DEFAULT 0,
					monthly_total REAL NOT NULL DEFAULT 0,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(user_id, month)
				)
			`)
			if err != nil {
				log.Printf("Error creating cash_bank table: %v", err)
			} else {
				log.Println("Created cash_bank table")
			}
		}

		// Asegurarse de que existe la tabla cash_bank_transactions
		if !tableExists("cash_bank_transactions") {
			_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS cash_bank_transactions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id TEXT NOT NULL,
					transaction_type TEXT NOT NULL,
					amount REAL NOT NULL,
					date TEXT NOT NULL,
					created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				log.Printf("Error creating cash_bank_transactions table: %v", err)
			} else {
				log.Println("Created cash_bank_transactions table")
			}
		}
	}

	// Ejecutar las verificaciones y añadir columnas faltantes
	ensureRequiredColumns()

	log.Println("Database connection established successfully")
}

func createTablesIfNotExist() {
	// Create incomes table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS incomes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			category TEXT NOT NULL,
			payment_method TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create incomes table: %v", err)
	}

	// Crear tabla cash_bank para el balance global
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			month TEXT NOT NULL,
			cash_amount REAL NOT NULL DEFAULT 0,
			cash_percent REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			bank_percent REAL NOT NULL DEFAULT 0,
			monthly_total REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Tabla para las transacciones de cash_bank
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			transaction_type TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank_transactions table: %v", err)
	}

	// Crear nuevas tablas para cada periodo de tiempo con la estructura solicitada
	// Tabla daily_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			total_previous_balance REAL NOT NULL DEFAULT 0,
			total_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, date)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_cash_bank_balance table: %v", err)
	}

	// Create indices for daily_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_user ON daily_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_date ON daily_cash_bank_balance(date)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_cash_bank_balance: %v", err)
	}

	// Tabla weekly_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_week)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_cash_bank_balance table: %v", err)
	}

	// Create indices for weekly_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_balance_user ON weekly_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_balance_week ON weekly_cash_bank_balance(year_week)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_cash_bank_balance: %v", err)
	}

	// Tabla monthly_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_month TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create monthly_cash_bank_balance table: %v", err)
	}

	// Create indices for monthly_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_balance_user ON monthly_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_cash_bank_balance_month ON monthly_cash_bank_balance(year_month)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_cash_bank_balance: %v", err)
	}

	// Tabla quarterly_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_quarter)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_cash_bank_balance table: %v", err)
	}

	// Create indices for quarterly_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_cash_bank_balance_user ON quarterly_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_cash_bank_balance_quarter ON quarterly_cash_bank_balance(year_quarter)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_cash_bank_balance: %v", err)
	}

	// Tabla semiannual_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_half TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_half)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_cash_bank_balance table: %v", err)
	}

	// Create indices for semiannual_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_cash_bank_balance_user ON semiannual_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_cash_bank_balance_half ON semiannual_cash_bank_balance(year_half)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_cash_bank_balance: %v", err)
	}

	// Tabla annual_cash_bank_balance
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_cash_bank_balance table: %v", err)
	}

	// Create indices for annual_cash_bank_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_cash_bank_balance_user ON annual_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_cash_bank_balance_year ON annual_cash_bank_balance(year)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_cash_bank_balance: %v", err)
	}

	// Create daily_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_balance table: %v", err)
	}

	// Create indices for daily_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_user ON daily_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_date ON daily_balance(date)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}

	// Create weekly_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_balance table: %v", err)
	}

	// Create indices for weekly_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_user ON weekly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_week ON weekly_balance(year_week)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}

	// Create monthly_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_month TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create monthly_balance table: %v", err)
	}

	// Create indices for monthly_balance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_balance_user ON monthly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_monthly_balance_month ON monthly_balance(year_month)`)
	if err != nil {
		log.Fatalf("Failed to create index on monthly_balance: %v", err)
	}

	// Create quarterly_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_balance table: %v", err)
	}

	// Create semiannual_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_half TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_balance table: %v", err)
	}

	// Create annual_balance table (se mantiene por compatibilidad)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_balance table: %v", err)
	}
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/incomes", corsMiddleware(handleFetchIncomes))
	http.HandleFunc("/incomes/add", corsMiddleware(handleAddIncome))
	http.HandleFunc("/incomes/update", corsMiddleware(handleUpdateIncome))
	http.HandleFunc("/incomes/delete", corsMiddleware(handleDeleteIncome))

	port := 8093 // Nuevo puerto para el servicio de ingresos
	log.Printf("Income Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's OPTIONS, return with just the headers (preflight request)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler
		next(w, r)
	}
}

func handleFetchIncomes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get incomes from database
	incomes, err := fetchIncomes(userID)
	if err != nil {
		log.Printf("Error fetching incomes: %v", err)
		sendErrorResponse(w, "Error fetching incomes", http.StatusInternalServerError)
		return
	}

	// Return incomes as JSON
	sendSuccessResponse(w, "Incomes fetched successfully", incomes)
}

func handleAddIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var addRequest AddIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&addRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if addRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if addRequest.Amount <= 0 {
		sendErrorResponse(w, "Amount must be greater than 0", http.StatusBadRequest)
		return
	}

	if addRequest.Date == "" {
		// Use current date if not provided
		addRequest.Date = time.Now().Format("2006-01-02")
	}

	if addRequest.Category == "" {
		sendErrorResponse(w, "Category is required", http.StatusBadRequest)
		return
	}

	if addRequest.PaymentMethod == "" || (addRequest.PaymentMethod != "cash" && addRequest.PaymentMethod != "bank") {
		sendErrorResponse(w, "Valid payment method (cash or bank) is required", http.StatusBadRequest)
		return
	}

	// Create an income object
	income := Income{
		UserID:        addRequest.UserID,
		Amount:        addRequest.Amount,
		Date:          addRequest.Date,
		Category:      addRequest.Category,
		PaymentMethod: addRequest.PaymentMethod,
		Description:   addRequest.Description,
	}

	// Add the income to the database
	incomeID, err := addIncome(income)
	if err != nil {
		log.Printf("Error adding income: %v", err)
		sendErrorResponse(w, "Error adding income", http.StatusInternalServerError)
		return
	}

	// Set the ID of the newly added income
	income.ID = incomeID

	// Update cash or bank balance based on payment method
	if err := updateBalance(income.UserID, income.Amount, income.PaymentMethod); err != nil {
		log.Printf("Error updating balance: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Actualizar los balances por periodos
	if err := updateTimeBalances(income.UserID, income.Amount, income.Date); err != nil {
		log.Printf("Error updating time balances: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Recalcular todos los balances en cascada
	if err := recalculateAllBalances(income.UserID, income.Date); err != nil {
		log.Printf("Error recalculating balances: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Return success response
	sendSuccessResponse(w, "Income added successfully", income)
}

func handleUpdateIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var updateRequest UpdateIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.IncomeID <= 0 {
		sendErrorResponse(w, "Valid income ID is required", http.StatusBadRequest)
		return
	}

	// Check if the income exists
	oldIncome, err := fetchIncomeByID(updateRequest.IncomeID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching income: %v", err)
		sendErrorResponse(w, "Error fetching income", http.StatusInternalServerError)
		return
	}

	if oldIncome == nil {
		sendErrorResponse(w, "Income not found", http.StatusNotFound)
		return
	}

	// Keep track of the old payment method and amount for balance adjustment
	oldAmount := oldIncome.Amount
	oldPaymentMethod := oldIncome.PaymentMethod
	oldDate := oldIncome.Date

	// Update the income with the provided values
	if updateRequest.Amount > 0 {
		oldIncome.Amount = updateRequest.Amount
	}

	if updateRequest.Date != "" {
		oldIncome.Date = updateRequest.Date
	}

	if updateRequest.Category != "" {
		oldIncome.Category = updateRequest.Category
	}

	if updateRequest.PaymentMethod != "" {
		if updateRequest.PaymentMethod != "cash" && updateRequest.PaymentMethod != "bank" {
			sendErrorResponse(w, "Valid payment method (cash or bank) is required", http.StatusBadRequest)
			return
		}
		oldIncome.PaymentMethod = updateRequest.PaymentMethod
	}

	if updateRequest.Description != "" {
		oldIncome.Description = updateRequest.Description
	}

	// Update the income in the database
	err = updateIncome(*oldIncome)
	if err != nil {
		log.Printf("Error updating income: %v", err)
		sendErrorResponse(w, "Error updating income", http.StatusInternalServerError)
		return
	}

	// Adjust balances if amount or payment method changed
	if oldAmount != oldIncome.Amount || oldPaymentMethod != oldIncome.PaymentMethod {
		// Remove the old amount from the old payment method
		if err := updateBalance(oldIncome.UserID, -oldAmount, oldPaymentMethod); err != nil {
			log.Printf("Error updating old balance: %v", err)
		}

		// Add the new amount to the new payment method
		if err := updateBalance(oldIncome.UserID, oldIncome.Amount, oldIncome.PaymentMethod); err != nil {
			log.Printf("Error updating new balance: %v", err)
		}
	}

	// Recalcular todos los balances para la fecha antigua y la nueva, si cambió
	if err := recalculateAllBalances(oldIncome.UserID, oldDate); err != nil {
		log.Printf("Error recalculating balances for old date: %v", err)
	}

	if oldDate != oldIncome.Date {
		if err := recalculateAllBalances(oldIncome.UserID, oldIncome.Date); err != nil {
			log.Printf("Error recalculating balances for new date: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Income updated successfully", oldIncome)
}

func handleDeleteIncome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var deleteRequest DeleteIncomeRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if deleteRequest.IncomeID <= 0 {
		sendErrorResponse(w, "Valid income ID is required", http.StatusBadRequest)
		return
	}

	// Check if the income exists and get its details for balance adjustment
	income, err := fetchIncomeByID(deleteRequest.IncomeID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching income: %v", err)
		sendErrorResponse(w, "Error fetching income", http.StatusInternalServerError)
		return
	}

	if income == nil {
		sendErrorResponse(w, "Income not found", http.StatusNotFound)
		return
	}

	// Guardar la fecha antes de eliminar
	incomeDate := income.Date

	// Delete the income from the database
	err = deleteIncome(deleteRequest.IncomeID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting income: %v", err)
		sendErrorResponse(w, "Error deleting income", http.StatusInternalServerError)
		return
	}

	// Adjust the balance (subtract the amount)
	if err := updateBalance(income.UserID, -income.Amount, income.PaymentMethod); err != nil {
		log.Printf("Error updating balance: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Recalcular todos los balances en cascada
	if err := recalculateAllBalances(income.UserID, incomeDate); err != nil {
		log.Printf("Error recalculating balances: %v", err)
		// Don't fail the entire request, just log the error
	}

	// Return success response
	sendSuccessResponse(w, "Income deleted successfully", nil)
}

func fetchIncomes(userID string) ([]Income, error) {
	// Query to get all incomes for the given user
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE user_id = ?
		ORDER BY date DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incomes []Income

	for rows.Next() {
		var income Income
		if err := rows.Scan(
			&income.ID,
			&income.UserID,
			&income.Amount,
			&income.Date,
			&income.Category,
			&income.PaymentMethod,
			&income.Description,
			&income.CreatedAt,
			&income.UpdatedAt,
		); err != nil {
			return nil, err
		}

		incomes = append(incomes, income)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return incomes, nil
}

func fetchIncomeByID(incomeID int, userID string) (*Income, error) {
	// Query to get a specific income
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE id = ? AND user_id = ?
	`

	var income Income
	err := db.QueryRow(query, incomeID, userID).Scan(
		&income.ID,
		&income.UserID,
		&income.Amount,
		&income.Date,
		&income.Category,
		&income.PaymentMethod,
		&income.Description,
		&income.CreatedAt,
		&income.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No income found
	} else if err != nil {
		return nil, err
	}

	return &income, nil
}

func addIncome(income Income) (int, error) {
	// Insert income into the database
	query := `
		INSERT INTO incomes (
			user_id, amount, date, category, payment_method, description
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		income.UserID,
		income.Amount,
		income.Date,
		income.Category,
		income.PaymentMethod,
		income.Description,
	)

	if err != nil {
		return 0, err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func updateIncome(income Income) error {
	// Update income in the database
	query := `
		UPDATE incomes
		SET amount = ?, date = ?, category = ?, payment_method = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(
		query,
		income.Amount,
		income.Date,
		income.Category,
		income.PaymentMethod,
		income.Description,
		income.ID,
		income.UserID,
	)

	return err
}

func deleteIncome(incomeID int, userID string) error {
	// Delete income from the database
	query := `
		DELETE FROM incomes
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(query, incomeID, userID)
	return err
}

func updateBalance(userID string, amount float64, paymentMethod string) error {
	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")

	// Fetch current cash-bank distribution
	var distribution struct {
		CashAmount   float64
		BankAmount   float64
		MonthlyTotal float64
		Exists       bool
	}

	// Check if a record exists for the current month
	checkQuery := `
		SELECT 1
		FROM cash_bank
		WHERE user_id = ? AND month = ?
	`
	var exists bool
	err := db.QueryRow(checkQuery, userID, currentMonth).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	distribution.Exists = err != sql.ErrNoRows

	if distribution.Exists {
		// Get current values
		getQuery := `
			SELECT cash_amount, bank_amount, monthly_total
			FROM cash_bank
			WHERE user_id = ? AND month = ?
		`
		err := db.QueryRow(getQuery, userID, currentMonth).Scan(
			&distribution.CashAmount,
			&distribution.BankAmount,
			&distribution.MonthlyTotal,
		)
		if err != nil {
			return err
		}

		// Update the appropriate amount based on payment method
		if paymentMethod == "cash" {
			distribution.CashAmount += amount
		} else if paymentMethod == "bank" {
			distribution.BankAmount += amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		// Update the record
		updateQuery := `
			UPDATE cash_bank
			SET cash_amount = ?, cash_percent = ?, bank_amount = ?, bank_percent = ?, monthly_total = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(
			updateQuery,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
			userID,
			currentMonth,
		)
		if err != nil {
			return err
		}
	} else {
		// Create a new record with initial values
		if paymentMethod == "cash" {
			distribution.CashAmount = amount
			distribution.BankAmount = 0
		} else if paymentMethod == "bank" {
			distribution.CashAmount = 0
			distribution.BankAmount = amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		// Insert the new record
		insertQuery := `
			INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(
			insertQuery,
			userID,
			currentMonth,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
		)
		if err != nil {
			return err
		}
	}

	// Add transaction record if it's an addition (positive amount)
	if amount > 0 {
		// Add a transaction record
		transactionQuery := `
			INSERT INTO cash_bank_transactions (user_id, transaction_type, amount, date)
			VALUES (?, ?, ?, ?)
		`
		transactionType := "income_" + paymentMethod
		_, err = db.Exec(
			transactionQuery,
			userID,
			transactionType,
			amount,
			time.Now().Format("2006-01-02"),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// Función para actualizar los balances por periodos al añadir un ingreso
func updateTimeBalances(userID string, amount float64, dateStr string) error {
	// Parse la fecha del ingreso
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Obtener la información del ingreso para determinar si fue cash o bank
	var paymentMethod string
	err = db.QueryRow(`
		SELECT payment_method FROM incomes
		WHERE user_id = ? AND date = ? AND amount = ?
		ORDER BY created_at DESC LIMIT 1
	`, userID, dateStr, amount).Scan(&paymentMethod)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error fetching payment method: %v", err)
	}

	if err == sql.ErrNoRows {
		// Si no se encuentra, asumimos bank por defecto
		paymentMethod = "bank"
	}

	// Calculamos los montos de cash y bank según el método de pago
	var cashAmount, bankAmount float64
	if paymentMethod == "cash" {
		cashAmount = amount
		bankAmount = 0
	} else {
		cashAmount = 0
		bankAmount = amount
	}

	// Actualizar balance diario con las nuevas tablas
	if err := updateDailyBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating daily balance: %v", err)
	}

	// Actualizar balance semanal con las nuevas tablas
	if err := updateWeeklyBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating weekly balance: %v", err)
	}

	// Actualizar balance mensual con las nuevas tablas
	if err := updateMonthlyBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating monthly balance: %v", err)
	}

	// Actualizar balance trimestral con las nuevas tablas
	if err := updateQuarterlyBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating quarterly balance: %v", err)
	}

	// Actualizar balance semestral con las nuevas tablas
	if err := updateSemiannualBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating semiannual balance: %v", err)
	}

	// Actualizar balance anual con las nuevas tablas
	if err := updateAnnualBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating annual balance: %v", err)
	}

	return nil
}

func updateDailyBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Formatear la fecha YYYY-MM-DD
	dateStr := date.Format("2006-01-02")

	// Calcular el día anterior
	prevDay := date.AddDate(0, 0, -1)
	prevDateStr := prevDay.Format("2006-01-02")

	var previousCashAmount, previousBankAmount float64
	var exists bool

	// Buscar registro del día anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount
		FROM daily_cash_bank_balance 
		WHERE user_id = ? AND date = ?
	`, userID, prevDateStr).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro del día anterior, buscar el último día anterior con registro
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM daily_cash_bank_balance 
			WHERE user_id = ? AND date < ?
			ORDER BY date DESC LIMIT 1
		`, userID, dateStr).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún día anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para esta fecha
	var existingIncomeBank, existingIncomeCash float64
	var existingExpenseBank, existingExpenseCash float64
	var existingBillBank, existingBillCash float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, expense_cash_amount, expense_bank_amount, bill_cash_amount, bill_bank_amount, cash_amount, bank_amount
		FROM daily_cash_bank_balance
		WHERE user_id = ? AND date = ?
	`, userID, dateStr).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO daily_cash_bank_balance (
				user_id, date, 
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, dateStr,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE daily_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				total_balance = (balance_cash_amount + balance_bank_amount),
				total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, dateStr)
	}

	if err != nil {
		return err
	}

	// Actualizar días posteriores en cascada
	return updateSubsequentDailyBalances(userID, date.AddDate(0, 0, 1))
}

// Nueva función para actualizar días posteriores en cascada
func updateSubsequentDailyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 365 días para evitar bucles infinitos
	endDate := startDate.AddDate(0, 0, 365)
	currentDate := startDate

	for currentDate.Before(endDate) {
		currentDateStr := currentDate.Format("2006-01-02")

		// Verificar si existe un registro para esta fecha
		var exists bool
		var incomeCashAmount, incomeBankAmount float64
		var expenseCashAmount, expenseBankAmount float64
		var billCashAmount, billBankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount 
			FROM daily_cash_bank_balance
			WHERE user_id = ? AND date = ?
		`, userID, currentDateStr).Scan(&exists, &incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener los valores del día anterior
		prevDate := currentDate.AddDate(0, 0, -1)
		prevDateStr := prevDate.Format("2006-01-02")

		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM daily_cash_bank_balance 
			WHERE user_id = ? AND date = ?
		`, userID, prevDateStr).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// Si no hay un día inmediatamente anterior, buscar el último día anterior disponible
			err = db.QueryRow(`
				SELECT cash_amount, bank_amount FROM daily_cash_bank_balance 
				WHERE user_id = ? AND date < ?
				ORDER BY date DESC LIMIT 1
			`, userID, currentDateStr).Scan(&prevCashAmount, &prevBankAmount)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// Si no se encuentra ningún día anterior, usar valores en cero
			if err == sql.ErrNoRows {
				prevCashAmount = 0
				prevBankAmount = 0
			}
		}

		// Calcular nuevos montos para cash y bank considerando los ingresos, gastos y facturas del día actual
		newCashAmount := prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
		newBankAmount := prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

		// Actualizar todos los campos
		_, err = db.Exec(`
			UPDATE daily_cash_bank_balance
			SET previous_cash_amount = ?,
				cash_amount = ?,
				previous_bank_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				total_balance = (balance_cash_amount + balance_bank_amount),
				total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, prevCashAmount, newCashAmount, prevBankAmount, newBankAmount, newCashAmount, newBankAmount, userID, currentDateStr)

		if err != nil {
			return err
		}

		// Pasar al siguiente día
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil
}

func updateWeeklyBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el número de semana y su rango de fechas
	year, week := date.ISOWeek()
	yearWeek := fmt.Sprintf("%d-%02d", year, week)

	// Calcular el inicio y fin de la semana
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Convertir domingo (0) a 7
	}
	startOfWeek := date.AddDate(0, 0, -(dayOfWeek - 1))
	endOfWeek := startOfWeek.AddDate(0, 0, 6)
	startDateStr := startOfWeek.Format("2006-01-02")
	endDateStr := endOfWeek.Format("2006-01-02")

	// Calcular la semana anterior
	prevWeek := startOfWeek.AddDate(0, 0, -7)
	prevYear, prevWeekNum := prevWeek.ISOWeek()
	prevYearWeek := fmt.Sprintf("%d-%02d", prevYear, prevWeekNum)

	var previousCashAmount, previousBankAmount float64

	// Buscar el balance de la semana anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM weekly_cash_bank_balance 
		WHERE user_id = ? AND year_week = ?
	`, userID, prevYearWeek).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro de la semana anterior, buscar la última semana anterior disponible
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM weekly_cash_bank_balance 
			WHERE user_id = ? AND year_week < ?
			ORDER BY year_week DESC LIMIT 1
		`, userID, yearWeek).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ninguna semana anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para esta semana
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount,
			cash_amount, bank_amount
		FROM weekly_cash_bank_balance
		WHERE user_id = ? AND year_week = ?
	`, userID, yearWeek).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO weekly_cash_bank_balance (
				user_id, year_week, start_date, end_date,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearWeek, startDateStr, endDateStr,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE weekly_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				total_balance = (balance_cash_amount + balance_bank_amount),
				total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_week = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, yearWeek)
	}

	if err != nil {
		return err
	}

	// Calcular la siguiente semana
	nextWeek := startOfWeek.AddDate(0, 0, 7)

	// Actualizar semanas posteriores en cascada
	return updateSubsequentWeeklyBalances(userID, nextWeek)
}

// Nueva función para actualizar semanas posteriores en cascada
func updateSubsequentWeeklyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a un año para evitar bucles infinitos
	endDate := startDate.AddDate(1, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		// Calcular el año y semana actual
		year, week := currentDate.ISOWeek()
		currentYearWeek := fmt.Sprintf("%d-%02d", year, week)

		// Verificar si existe un registro para esta semana
		var exists bool
		var incomeCashAmount, incomeBankAmount float64
		var expenseCashAmount, expenseBankAmount float64
		var billCashAmount, billBankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount 
			FROM weekly_cash_bank_balance
			WHERE user_id = ? AND year_week = ?
		`, userID, currentYearWeek).Scan(&exists, &incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener los valores de la semana anterior
		prevWeek := currentDate.AddDate(0, 0, -7)
		prevYear, prevWeekNum := prevWeek.ISOWeek()
		prevYearWeek := fmt.Sprintf("%d-%02d", prevYear, prevWeekNum)

		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM weekly_cash_bank_balance 
			WHERE user_id = ? AND year_week = ?
		`, userID, prevYearWeek).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// Si no hay una semana inmediatamente anterior, buscar la última semana anterior disponible
			err = db.QueryRow(`
				SELECT cash_amount, bank_amount FROM weekly_cash_bank_balance 
				WHERE user_id = ? AND year_week < ?
				ORDER BY year_week DESC LIMIT 1
			`, userID, currentYearWeek).Scan(&prevCashAmount, &prevBankAmount)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// Si no se encuentra ninguna semana anterior, usar valores en cero
			if err == sql.ErrNoRows {
				prevCashAmount = 0
				prevBankAmount = 0
			}
		}

		// Calcular nuevos montos para cash y bank considerando los ingresos, gastos y facturas de la semana actual
		newCashAmount := prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
		newBankAmount := prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

		// Actualizar todos los campos
		_, err = db.Exec(`
			UPDATE weekly_cash_bank_balance
			SET previous_cash_amount = ?,
				cash_amount = ?,
				previous_bank_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				total_balance = (balance_cash_amount + balance_bank_amount),
				total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_week = ?
		`, prevCashAmount, newCashAmount, prevBankAmount, newBankAmount, newCashAmount, newBankAmount, userID, currentYearWeek)

		if err != nil {
			return err
		}

		// Pasar a la siguiente semana
		currentDate = currentDate.AddDate(0, 0, 7)
	}

	return nil
}

func updateMonthlyBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el año y mes
	yearMonth := date.Format("2006-01")

	// Calcular el mes anterior
	prevMonth := date.AddDate(0, -1, 0)
	prevYearMonth := prevMonth.Format("2006-01")

	var previousCashAmount, previousBankAmount float64

	// Buscar los valores del mes anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month = ?
	`, userID, prevYearMonth).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro del mes anterior, buscar el último mes anterior disponible
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month < ?
			ORDER BY year_month DESC LIMIT 1
		`, userID, yearMonth).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún mes anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para este mes
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
		expense_cash_amount, expense_bank_amount, 
		bill_cash_amount, bill_bank_amount,
		cash_amount, bank_amount
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
	`, userID, yearMonth).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO monthly_cash_bank_balance (
				user_id, year_month,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearMonth,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, yearMonth)
	}

	if err != nil {
		return err
	}

	// Actualizar meses posteriores en cascada
	return updateSubsequentMonthlyBalances(userID, date.AddDate(0, 1, 0))
}

// Nueva función para actualizar meses posteriores en cascada
func updateSubsequentMonthlyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	currentDate := startDate

	// Mantener un registro de los meses procesados
	processedMonths := make(map[string]struct{})

	// Encontrar el mes anterior al mes de inicio para saber el valor de partida
	var lastPrevCashAmount, lastPrevBankAmount float64
	prevToStartDate := startDate.AddDate(0, -1, 0)
	prevToStartYearMonth := prevToStartDate.Format("2006-01")

	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month = ?
	`, userID, prevToStartYearMonth).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// Si no hay un mes inmediatamente anterior, buscar el último mes anterior disponible
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month < ?
			ORDER BY year_month DESC LIMIT 1
		`, userID, startDate.Format("2006-01")).Scan(&lastPrevCashAmount, &lastPrevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún mes anterior, usar valores en cero
		if err == sql.ErrNoRows {
			lastPrevCashAmount = 0
			lastPrevBankAmount = 0
		}
	}

	// Obtener todos los meses existentes desde la fecha de inicio hasta 5 años después
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month ASC
	`, userID, startDate.Format("2006-01"))

	if err != nil {
		return err
	}
	defer rows.Close()

	var existingMonths []string
	for rows.Next() {
		var yearMonth string
		if err := rows.Scan(&yearMonth); err != nil {
			return err
		}
		existingMonths = append(existingMonths, yearMonth)
	}

	// Si no hay meses existentes, no hay nada que procesar
	if len(existingMonths) == 0 {
		return nil
	}

	// Determinar el último mes que necesitamos procesar
	lastMonthStr := existingMonths[len(existingMonths)-1]
	lastMonthParts := strings.Split(lastMonthStr, "-")
	if len(lastMonthParts) != 2 {
		return fmt.Errorf("invalid month format: %s", lastMonthStr)
	}
	lastYear, _ := strconv.Atoi(lastMonthParts[0])
	lastMonth, _ := strconv.Atoi(lastMonthParts[1])
	lastDate := time.Date(lastYear, time.Month(lastMonth), 1, 0, 0, 0, 0, time.UTC)

	// Crear un mapa para acceso rápido a los meses existentes
	existingMonthsMap := make(map[string]bool)
	for _, m := range existingMonths {
		existingMonthsMap[m] = true
	}

	// Variables para mantener los valores acumulados
	currentCashAmount := lastPrevCashAmount
	currentBankAmount := lastPrevBankAmount

	// Iterar mes por mes desde la fecha de inicio hasta el último mes existente
	for currentDate.Before(lastDate.AddDate(0, 1, 0)) {
		currentYearMonth := currentDate.Format("2006-01")

		if existingMonthsMap[currentYearMonth] {
			// Si el mes existe, actualizarlo con los valores correctos
			var incomeCashAmount, incomeBankAmount float64
			var expenseCashAmount, expenseBankAmount float64
			var billCashAmount, billBankAmount float64

			err := db.QueryRow(`
				SELECT income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount 
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, currentYearMonth).Scan(&incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

			if err != nil {
				return err
			}

			// Calcular nuevos montos considerando los ingresos, gastos y facturas del mes actual
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			currentCashAmount = prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
			currentBankAmount = prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

			// Actualizar el registro
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance
				SET previous_cash_amount = ?,
					cash_amount = ?,
					previous_bank_amount = ?,
					bank_amount = ?,
					balance_cash_amount = ?,
					balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
					updated_at = CURRENT_TIMESTAMP
				WHERE user_id = ? AND year_month = ?
			`, prevCashAmount, currentCashAmount, prevBankAmount, currentBankAmount, currentCashAmount, currentBankAmount, userID, currentYearMonth)

			if err != nil {
				return err
			}
		} else {
			// Si el mes no existe, crear un registro para él con valores en 0 para ingresos/gastos/facturas
			// pero con los valores correctos de previous_amount basados en el mes anterior
			prevCashAmount := currentCashAmount
			prevBankAmount := currentBankAmount

			// Los montos acumulados no cambian ya que no hay transacciones
			// Sin embargo, creamos el registro para mantener la continuidad
			_, err = db.Exec(`
				INSERT INTO monthly_cash_bank_balance (
					user_id, year_month,
					income_cash_amount, income_bank_amount, 
					expense_cash_amount, expense_bank_amount, 
					bill_cash_amount, bill_bank_amount, 
					cash_amount, previous_cash_amount,
					bank_amount, previous_bank_amount,
					balance_cash_amount, balance_bank_amount
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, userID, currentYearMonth,
				0, 0,
				0, 0,
				0, 0,
				prevCashAmount, prevCashAmount,
				prevBankAmount, prevBankAmount,
				prevCashAmount, prevBankAmount)

			if err != nil {
				// Si el error es por duplicado, ignorarlo
				if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
					return err
				}
			}
		}

		// Marcar este mes como procesado
		processedMonths[currentYearMonth] = struct{}{}

		// Avanzar al mes siguiente
		currentDate = currentDate.AddDate(0, 1, 0)
	}

	return nil
}

func updateQuarterlyBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el trimestre
	quarter := (int(date.Month()) - 1) / 3
	year := date.Year()
	yearQuarter := fmt.Sprintf("%d-Q%d", year, quarter+1)

	// Calcular el trimestre anterior
	var prevYear, prevQuarter int
	if quarter == 0 {
		prevYear = year - 1
		prevQuarter = 3 // Q4 del año anterior
	} else {
		prevYear = year
		prevQuarter = quarter - 1
	}
	prevYearQuarter := fmt.Sprintf("%d-Q%d", prevYear, prevQuarter+1)

	var previousCashAmount, previousBankAmount float64

	// Buscar los valores del trimestre anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
		WHERE user_id = ? AND year_quarter = ?
	`, userID, prevYearQuarter).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro del trimestre anterior, buscar el último trimestre anterior disponible
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
			WHERE user_id = ? AND year_quarter < ?
			ORDER BY year_quarter DESC LIMIT 1
		`, userID, yearQuarter).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún trimestre anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para este trimestre
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
		expense_cash_amount, expense_bank_amount, 
		bill_cash_amount, bill_bank_amount,
		cash_amount, bank_amount
		FROM quarterly_cash_bank_balance
		WHERE user_id = ? AND year_quarter = ?
	`, userID, yearQuarter).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO quarterly_cash_bank_balance (
				user_id, year_quarter,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearQuarter,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE quarterly_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_quarter = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, yearQuarter)
	}

	if err != nil {
		return err
	}

	// Calcular el trimestre siguiente
	var nextYear, nextQuarter int
	if quarter == 3 {
		nextYear = year + 1
		nextQuarter = 0
	} else {
		nextYear = year
		nextQuarter = quarter + 1
	}
	nextDate := time.Date(nextYear, time.Month(nextQuarter*3+1), 1, 0, 0, 0, 0, time.UTC)

	// Actualizar trimestres posteriores en cascada
	return updateSubsequentQuarterlyBalances(userID, nextDate)
}

// Nueva función para actualizar años posteriores en cascada
func updateSubsequentAnnualBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 20 años para evitar bucles infinitos
	endDate := startDate.AddDate(20, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		currentYear := strconv.Itoa(currentDate.Year())

		// Verificar si existe un registro para este año
		var exists bool
		var incomeCashAmount, incomeBankAmount float64
		var expenseCashAmount, expenseBankAmount float64
		var billCashAmount, billBankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount 
			FROM annual_cash_bank_balance
			WHERE user_id = ? AND year = ?
		`, userID, currentYear).Scan(&exists, &incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance del año anterior
		prevYear := strconv.Itoa(currentDate.Year() - 1)

		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM annual_cash_bank_balance 
			WHERE user_id = ? AND year = ?
		`, userID, prevYear).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// Si no hay un año inmediatamente anterior, buscar el último año anterior disponible
			err = db.QueryRow(`
				SELECT cash_amount, bank_amount FROM annual_cash_bank_balance 
				WHERE user_id = ? AND year < ?
				ORDER BY year DESC LIMIT 1
			`, userID, currentYear).Scan(&prevCashAmount, &prevBankAmount)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// Si no se encuentra ningún año anterior, usar valores en cero
			if err == sql.ErrNoRows {
				prevCashAmount = 0
				prevBankAmount = 0
			}
		}

		// Calcular nuevos montos para cash y bank considerando los ingresos, gastos y facturas del año actual
		newCashAmount := prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
		newBankAmount := prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

		// Actualizar todos los campos
		_, err = db.Exec(`
			UPDATE annual_cash_bank_balance
			SET previous_cash_amount = ?,
				cash_amount = ?,
				previous_bank_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`, prevCashAmount, newCashAmount, prevBankAmount, newBankAmount, newCashAmount, newBankAmount, userID, currentYear)

		if err != nil {
			return err
		}

		// Pasar al siguiente año
		currentDate = currentDate.AddDate(1, 0, 0)
	}

	return nil
}

// Función para recalcular todos los balances en cascada
func recalculateAllBalances(userID string, dateStr string) error {
	// Parse la fecha de la transacción
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Recalcular balances diarios en cascada desde la fecha de la transacción
	if err := updateSubsequentDailyBalances(userID, date); err != nil {
		return fmt.Errorf("error updating daily balances: %v", err)
	}

	// Calcular el inicio de la semana que contiene la fecha
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Convertir domingo (0) a 7
	}
	startOfWeek := date.AddDate(0, 0, -(dayOfWeek - 1))

	// Recalcular balances semanales en cascada desde la semana que contiene la transacción
	if err := updateSubsequentWeeklyBalances(userID, startOfWeek); err != nil {
		return fmt.Errorf("error updating weekly balances: %v", err)
	}

	// Calcular el inicio del mes que contiene la fecha
	startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances mensuales en cascada desde el mes que contiene la transacción
	if err := updateSubsequentMonthlyBalances(userID, startOfMonth); err != nil {
		return fmt.Errorf("error updating monthly balances: %v", err)
	}

	// Calcular el inicio del trimestre que contiene la fecha
	quarter := (int(date.Month()) - 1) / 3
	startOfQuarter := time.Date(date.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances trimestrales en cascada desde el trimestre que contiene la transacción
	if err := updateSubsequentQuarterlyBalances(userID, startOfQuarter); err != nil {
		return fmt.Errorf("error updating quarterly balances: %v", err)
	}

	// Calcular el inicio del semestre que contiene la fecha
	halfYear := (int(date.Month()) - 1) / 6
	startOfHalfYear := time.Date(date.Year(), time.Month(halfYear*6+1), 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances semestrales en cascada desde el semestre que contiene la transacción
	if err := updateSubsequentSemiannualBalances(userID, startOfHalfYear); err != nil {
		return fmt.Errorf("error updating semiannual balances: %v", err)
	}

	// Calcular el inicio del año que contiene la fecha
	startOfYear := time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	// Recalcular balances anuales en cascada desde el año que contiene la transacción
	if err := updateSubsequentAnnualBalances(userID, startOfYear); err != nil {
		return fmt.Errorf("error updating annual balances: %v", err)
	}

	return nil
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	response := ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := ApiResponse{
		Success: false,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Nueva función para actualizar trimestres posteriores en cascada
func updateSubsequentQuarterlyBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	endDate := startDate.AddDate(5, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		// Calcular el trimestre actual
		quarter := (int(currentDate.Month()) - 1) / 3
		year := currentDate.Year()
		currentYearQuarter := fmt.Sprintf("%d-Q%d", year, quarter+1)

		// Verificar si existe un registro para este trimestre
		var exists bool
		var incomeCashAmount, incomeBankAmount float64
		var expenseCashAmount, expenseBankAmount float64
		var billCashAmount, billBankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount 
			FROM quarterly_cash_bank_balance
			WHERE user_id = ? AND year_quarter = ?
		`, userID, currentYearQuarter).Scan(&exists, &incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Calcular el trimestre anterior
		var prevYear, prevQuarter int
		if quarter == 0 {
			prevYear = year - 1
			prevQuarter = 3 // Q4 del año anterior
		} else {
			prevYear = year
			prevQuarter = quarter - 1
		}
		prevYearQuarter := fmt.Sprintf("%d-Q%d", prevYear, prevQuarter+1)

		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
			WHERE user_id = ? AND year_quarter = ?
		`, userID, prevYearQuarter).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// Si no hay un trimestre inmediatamente anterior, buscar el último trimestre anterior disponible
			err = db.QueryRow(`
				SELECT cash_amount, bank_amount FROM quarterly_cash_bank_balance 
				WHERE user_id = ? AND year_quarter < ?
				ORDER BY year_quarter DESC LIMIT 1
			`, userID, currentYearQuarter).Scan(&prevCashAmount, &prevBankAmount)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// Si no se encuentra ningún trimestre anterior, usar valores en cero
			if err == sql.ErrNoRows {
				prevCashAmount = 0
				prevBankAmount = 0
			}
		}

		// Calcular nuevos montos para cash y bank considerando los ingresos, gastos y facturas del trimestre actual
		newCashAmount := prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
		newBankAmount := prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

		// Actualizar todos los campos
		_, err = db.Exec(`
			UPDATE quarterly_cash_bank_balance
			SET previous_cash_amount = ?,
				cash_amount = ?,
				previous_bank_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_quarter = ?
		`, prevCashAmount, newCashAmount, prevBankAmount, newBankAmount, newCashAmount, newBankAmount, userID, currentYearQuarter)

		if err != nil {
			return err
		}

		// Avanzar al siguiente trimestre
		if quarter == 3 {
			// Si estamos en Q4, pasar al Q1 del siguiente año
			currentDate = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)
		} else {
			// Avanzar al siguiente trimestre del mismo año
			currentDate = time.Date(year, time.Month((quarter+1)*3+1), 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return nil
}

func updateSemiannualBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el semestre
	halfYear := (int(date.Month()) - 1) / 6
	year := date.Year()
	yearHalf := fmt.Sprintf("%d-H%d", year, halfYear+1)

	// Calcular el semestre anterior
	var prevYear, prevHalf int
	if halfYear == 0 {
		prevYear = year - 1
		prevHalf = 1 // H2 del año anterior
	} else {
		prevYear = year
		prevHalf = 0 // H1 del mismo año
	}
	prevYearHalf := fmt.Sprintf("%d-H%d", prevYear, prevHalf+1)

	var previousCashAmount, previousBankAmount float64

	// Buscar los valores del semestre anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM semiannual_cash_bank_balance 
		WHERE user_id = ? AND year_half = ?
	`, userID, prevYearHalf).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro del semestre anterior, buscar el último semestre anterior disponible
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM semiannual_cash_bank_balance 
			WHERE user_id = ? AND year_half < ?
			ORDER BY year_half DESC LIMIT 1
		`, userID, yearHalf).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún semestre anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para este semestre
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
		expense_cash_amount, expense_bank_amount, 
		bill_cash_amount, bill_bank_amount,
		cash_amount, bank_amount
		FROM semiannual_cash_bank_balance
		WHERE user_id = ? AND year_half = ?
	`, userID, yearHalf).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO semiannual_cash_bank_balance (
				user_id, year_half,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, yearHalf,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE semiannual_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_half = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, yearHalf)
	}

	if err != nil {
		return err
	}

	// Calcular el semestre siguiente
	var nextYear, nextHalf int
	if halfYear == 1 {
		nextYear = year + 1
		nextHalf = 0
	} else {
		nextYear = year
		nextHalf = 1
	}
	nextDate := time.Date(nextYear, time.Month(nextHalf*6+1), 1, 0, 0, 0, 0, time.UTC)

	// Actualizar semestres posteriores en cascada
	return updateSubsequentSemiannualBalances(userID, nextDate)
}

// Nueva función para actualizar semestres posteriores en cascada
func updateSubsequentSemiannualBalances(userID string, startDate time.Time) error {
	// Limitar el proceso a 5 años para evitar bucles infinitos
	endDate := startDate.AddDate(5, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		// Determinar el semestre actual
		half := (int(currentDate.Month())-1)/6 + 1
		currentYearHalf := fmt.Sprintf("%d-H%d", currentDate.Year(), half)

		// Verificar si existe un registro para este semestre
		var exists bool
		var incomeCashAmount, incomeBankAmount float64
		var expenseCashAmount, expenseBankAmount float64
		var billCashAmount, billBankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_cash_amount, income_bank_amount, 
			expense_cash_amount, expense_bank_amount, 
			bill_cash_amount, bill_bank_amount 
			FROM semiannual_cash_bank_balance
			WHERE user_id = ? AND year_half = ?
		`, userID, currentYearHalf).Scan(&exists, &incomeCashAmount, &incomeBankAmount, &expenseCashAmount, &expenseBankAmount, &billCashAmount, &billBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No hay más registros para actualizar
			break
		}

		// Obtener el balance del semestre anterior
		prevHalfDate := currentDate.AddDate(0, -6, 0)
		prevHalf := (int(prevHalfDate.Month())-1)/6 + 1
		prevYearHalf := fmt.Sprintf("%d-H%d", prevHalfDate.Year(), prevHalf)

		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM semiannual_cash_bank_balance 
			WHERE user_id = ? AND year_half = ?
		`, userID, prevYearHalf).Scan(&prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// Si no hay un semestre inmediatamente anterior, buscar el último semestre anterior disponible
			err = db.QueryRow(`
				SELECT cash_amount, bank_amount FROM semiannual_cash_bank_balance 
				WHERE user_id = ? AND year_half < ?
				ORDER BY year_half DESC LIMIT 1
			`, userID, currentYearHalf).Scan(&prevCashAmount, &prevBankAmount)

			if err != nil && err != sql.ErrNoRows {
				return err
			}

			// Si no se encuentra ningún semestre anterior, usar valores en cero
			if err == sql.ErrNoRows {
				prevCashAmount = 0
				prevBankAmount = 0
			}
		}

		// Calcular nuevos montos para cash y bank considerando los ingresos, gastos y facturas del semestre actual
		newCashAmount := prevCashAmount + incomeCashAmount - expenseCashAmount - billCashAmount
		newBankAmount := prevBankAmount + incomeBankAmount - expenseBankAmount - billBankAmount

		// Actualizar todos los campos
		_, err = db.Exec(`
			UPDATE semiannual_cash_bank_balance
			SET previous_cash_amount = ?,
				cash_amount = ?,
				previous_bank_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_half = ?
		`, prevCashAmount, newCashAmount, prevBankAmount, newBankAmount, newCashAmount, newBankAmount, userID, currentYearHalf)

		if err != nil {
			return err
		}

		// Pasar al siguiente semestre
		currentDate = currentDate.AddDate(0, 6, 0)
	}

	return nil
}

func updateAnnualBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	// Calcular el año
	year := strconv.Itoa(date.Year())

	// Calcular el año anterior
	prevYear := strconv.Itoa(date.Year() - 1)

	var previousCashAmount, previousBankAmount float64

	// Buscar los valores del año anterior
	err := db.QueryRow(`
		SELECT cash_amount, bank_amount FROM annual_cash_bank_balance 
		WHERE user_id = ? AND year = ?
	`, userID, prevYear).Scan(&previousCashAmount, &previousBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Si no existe registro del año anterior, buscar el último año anterior disponible
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			SELECT cash_amount, bank_amount FROM annual_cash_bank_balance 
			WHERE user_id = ? AND year < ?
			ORDER BY year DESC LIMIT 1
		`, userID, year).Scan(&previousCashAmount, &previousBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		// Si no se encuentra ningún año anterior, ambos valores son 0
		if err == sql.ErrNoRows {
			previousCashAmount = 0
			previousBankAmount = 0
		}
	}

	// Verificar si ya existe un registro para este año
	var exists bool
	var existingIncomeCash, existingIncomeBank float64
	var existingExpenseCash, existingExpenseBank float64
	var existingBillCash, existingBillBank float64
	var existingCashAmount, existingBankAmount float64
	err = db.QueryRow(`
		SELECT 1, income_cash_amount, income_bank_amount, 
		expense_cash_amount, expense_bank_amount, 
		bill_cash_amount, bill_bank_amount,
		cash_amount, bank_amount
		FROM annual_cash_bank_balance
		WHERE user_id = ? AND year = ?
	`, userID, year).Scan(&exists, &existingIncomeCash, &existingIncomeBank, &existingExpenseCash, &existingExpenseBank, &existingBillCash, &existingBillBank, &existingCashAmount, &existingBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No existe registro, crear uno nuevo
		// Calcular los nuevos montos en efectivo y banco
		newCashAmount := previousCashAmount + cashAmount
		newBankAmount := previousBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO annual_cash_bank_balance (
				user_id, year,
				income_cash_amount, income_bank_amount, 
				expense_cash_amount, expense_bank_amount, 
				bill_cash_amount, bill_bank_amount, 
				cash_amount, previous_cash_amount,
				bank_amount, previous_bank_amount,
				balance_cash_amount, balance_bank_amount
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, year,
			cashAmount, bankAmount,
			0, 0,
			0, 0,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount)
	} else {
		// Actualizar registro existente
		// IMPORTANTE: Mantener los valores actuales y sumar el nuevo ingreso
		newCashAmount := existingCashAmount + cashAmount
		newBankAmount := existingBankAmount + bankAmount

		_, err = db.Exec(`
			UPDATE annual_cash_bank_balance SET
				income_cash_amount = income_cash_amount + ?,
				income_bank_amount = income_bank_amount + ?,
				cash_amount = ?,
				previous_cash_amount = ?,
				bank_amount = ?,
				previous_bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
			total_balance = (balance_cash_amount + balance_bank_amount),
			total_previous_balance = (previous_cash_amount + previous_bank_amount),
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`, cashAmount, bankAmount,
			newCashAmount, previousCashAmount,
			newBankAmount, previousBankAmount,
			newCashAmount, newBankAmount,
			userID, year)
	}

	if err != nil {
		return err
	}

	// Actualizar años posteriores en cascada
	nextYear := date.Year() + 1
	nextYearDate := time.Date(nextYear, 1, 1, 0, 0, 0, 0, date.Location())
	return updateSubsequentAnnualBalances(userID, nextYearDate)
}
