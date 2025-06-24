package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

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