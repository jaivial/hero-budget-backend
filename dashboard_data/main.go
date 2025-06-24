package main

import (
	"github.com/joho/godotenv"	"flag"	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	

	"github.com/herobudget/backend/common"
	_ "github.com/mattn/go-sqlite3"
)


var (
	// Database connection for dashboard data persistence
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

	log.Println("Database connection established successfully")
}




func main() {
	// Set up CORS middleware
	http.HandleFunc("/dashboard/data", corsMiddleware(handleFetchDashboardData))

	port := 8087
	log.Printf("Dashboard Data service started on :%d", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}









// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
