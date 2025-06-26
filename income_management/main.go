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

var (
	db *sql.DB
	// Context for database operations
	ctx = context.Background()
	// Cache manager for Redis operations
	cacheManager *common.CacheManager
)

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

	// CENTRALIZED SCHEMA: DDL operations moved to database_schema.sql
	// Tables are now created by centralized database initialization
	log.Println("✅ Using centralized database schema - no local DDL operations")

	// Initialize cache manager
	var err2 error
	cacheManager, err2 = common.NewCacheManager()
	if err2 != nil {
		log.Printf("Warning: Failed to initialize cache manager: %v", err2)
		cacheManager = nil
	}

	log.Println("Database connection established successfully")
}

func main() {
	// Set up CORS middleware and routes for income management
	http.HandleFunc("/incomes/add", corsMiddleware(handleAddIncome))
	http.HandleFunc("/incomes/list", corsMiddleware(handleListIncomes))
	http.HandleFunc("/incomes", corsMiddleware(handleListIncomes)) // Compatible with Flutter frontend expectation
	http.HandleFunc("/incomes/update", corsMiddleware(handleUpdateIncome))
	http.HandleFunc("/incomes/delete", corsMiddleware(handleDeleteIncome))

	// Cash/Bank balance routes
	http.HandleFunc("/cash-bank/balance/update", corsMiddleware(handleUpdateCashBankBalance))
	http.HandleFunc("/cash-bank/balance/get", corsMiddleware(handleGetCashBankBalance))

	// Balance synchronization routes for different periods
	http.HandleFunc("/balance/sync/daily", corsMiddleware(handleSyncDailyBalance))
	http.HandleFunc("/balance/sync/weekly", corsMiddleware(handleSyncWeeklyBalance))
	http.HandleFunc("/balance/sync/monthly", corsMiddleware(handleSyncMonthlyBalance))
	http.HandleFunc("/balance/sync/quarterly", corsMiddleware(handleSyncQuarterlyBalance))
	http.HandleFunc("/balance/sync/semiannual", corsMiddleware(handleSyncSemiannualBalance))
	http.HandleFunc("/balance/sync/annual", corsMiddleware(handleSyncAnnualBalance))

	// Generic balance query routes
	http.HandleFunc("/balance/get", corsMiddleware(handleGetBalance))

	port := 8093 // Puerto para el servicio de income management
	log.Printf("Income Management service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}