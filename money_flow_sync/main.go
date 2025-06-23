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
	// Set up CORS middleware and routes
	http.HandleFunc("/money-flow/sync", corsMiddleware(handleSyncMoneyFlow))
	http.HandleFunc("/money-flow/data", corsMiddleware(handleGetMoneyFlowData))

	port := 8097 // Puerto para el servicio de sincronización de money flow
	log.Printf("Money Flow Sync service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}