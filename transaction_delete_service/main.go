package main

import (
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

// Transaction deletion request structure
// Includes sync operation parameters for incremental synchronization
type DeleteTransactionRequest struct {
	UserID          string `json:"user_id"`
	TransactionID   int    `json:"transaction_id"`
	TransactionType string `json:"transaction_type"`
	// Sync operation parameters for incremental synchronization
	OperationID string `json:"operation_id,omitempty"` // Unique ID for sync operation
	DeviceID    string `json:"device_id,omitempty"`    // Device identifier for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client-side timestamp
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var db *sql.DB

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

// addSyncOperation registra una operación de sincronización en la tabla sync_operations
// Implements timestamp adjustment and device_ids JSON array for multi-device sync
func addSyncOperation(userID, operationID, action, tableName, recordID string, data interface{}, deviceID string, clientTimestamp int64) error {
	log.Printf("Adding sync operation: user=%s, operation=%s, action=%s, table=%s, record=%s, device=%s", 
		userID, operationID, action, tableName, recordID, deviceID)
	
	// Serialize operation data to JSON for storage
	dataJSON, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling sync operation data: %v", err)
		return err
	}
	
	// Prepare device_ids JSON array
	var deviceIDs []string
	if deviceID != "" {
		deviceIDs = []string{deviceID}
	} else {
		deviceIDs = []string{} // Empty array if no device ID provided
	}
	
	// Marshal device IDs to JSON
	deviceIDsJSON, err := json.Marshal(deviceIDs)
	if err != nil {
		log.Printf("Error marshaling device_ids: %v", err)
		return err
	}
	
	// Timestamp adjustment: check if client timestamp is older than latest timestamp
	var latestTimestamp int64
	err = db.QueryRow("SELECT MAX(created_at) FROM sync_operations WHERE user_id = ?", userID).Scan(&latestTimestamp)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking latest timestamp: %v", err)
		return err
	}
	
	// Adjust timestamp if necessary to maintain chronological ordering
	adjustedTimestamp := clientTimestamp
	if clientTimestamp <= latestTimestamp {
		adjustedTimestamp = latestTimestamp + 1
		log.Printf("Adjusted client timestamp from %d to %d (latest was %d)", 
			clientTimestamp, adjustedTimestamp, latestTimestamp)
	}
	
	// Use current server timestamp
	serverTimestamp := time.Now().Unix()
	
	// Insert sync operation record with device_ids JSON array
	// Use adjusted timestamp for created_at to maintain proper synchronization ordering
	insertQuery := `
		INSERT INTO sync_operations (
			user_id, operation_id, action, table_name, record_id, data, 
			device_ids, client_timestamp, server_timestamp, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	result, err := db.Exec(
		insertQuery,
		userID,
		operationID,
		action,
		tableName,
		recordID,
		string(dataJSON),
		string(deviceIDsJSON), // Store device IDs as JSON array
		clientTimestamp,
		serverTimestamp,
		adjustedTimestamp, // Use adjusted timestamp for created_at
	)
	
	if err != nil {
		log.Printf("Error inserting sync operation: %v", err)
		return err
	}
	
	// Log successful operation insertion for debugging
	syncOpID, _ := result.LastInsertId()
	log.Printf("Successfully added sync operation with ID: %d, device_ids: %v, adjusted timestamp: %d", 
		syncOpID, deviceIDs, adjustedTimestamp)
	
	return nil
}

func main() {
	// CORS middleware function
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		}
	}

	// Health check endpoint
	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		response := ApiResponse{
			Success: true,
			Message: "Transaction Delete Service is running",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))

	// Delete transaction endpoint
	http.HandleFunc("/transactions/delete", corsMiddleware(handleDeleteTransaction))

	port := "8095" // Unique port for transaction delete service
	log.Printf("Transaction Delete Service starting on port %s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest DeleteTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		log.Printf("Error decoding request body: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Invalid request format",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if deleteRequest.UserID == "" || deleteRequest.TransactionID <= 0 || deleteRequest.TransactionType == "" {
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: user_id, transaction_id, or transaction_type",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Deleting transaction ID %d of type %s for user %s",
		deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)

	// Get transaction details before deletion for balance recalculation
	transaction, err := getTransactionDetails(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching transaction details: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Transaction not found or access denied",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle special case: expense with bill_id (corresponds to a bill payment)
	if deleteRequest.TransactionType == "expense" && transaction.BillID != nil {
		err = handleExpenseWithBillDeletion(*transaction)
		if err != nil {
			log.Printf("Error handling expense with bill deletion: %v", err)
			response := ApiResponse{
				Success: false,
				Message: "Failed to handle expense with bill deletion",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
	} else {
		// For regular transactions (income, bills, expenses without bill_id)
		// Delete the transaction first
		err = deleteTransaction(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
		if err != nil {
			log.Printf("Error deleting transaction: %v", err)
			response := ApiResponse{
				Success: false,
				Message: "Failed to delete transaction",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Recalculate balances for all time periods
		err = recalculateAllBalances(deleteRequest.UserID, transaction.Date, transaction.Amount, transaction.PaymentMethod, deleteRequest.TransactionType, transaction.BillID)
		if err != nil {
			log.Printf("Error recalculating balances: %v", err)
			// Don't fail the request if balance recalculation fails, just log it
		}
	}

	// Record sync operation if sync parameters are provided
	// Add operation to sync_operations table for multi-device synchronization
	if deleteRequest.OperationID != "" && deleteRequest.DeviceID != "" && deleteRequest.Timestamp > 0 {
		// Prepare sync operation data for recording
		syncOperationData := map[string]interface{}{
			"user_id":          deleteRequest.UserID,
			"transaction_id":   deleteRequest.TransactionID,
			"transaction_type": deleteRequest.TransactionType,
			"action":           "delete",
		}
		
		// Record the sync operation in sync_operations table
		err = addSyncOperation(
			deleteRequest.UserID,
			deleteRequest.OperationID,
			"delete",
			"transactions",
			fmt.Sprintf("%d", deleteRequest.TransactionID),
			syncOperationData,
			deleteRequest.DeviceID,
			deleteRequest.Timestamp,
		)
		
		if err != nil {
			log.Printf("Warning: Failed to record sync operation: %v", err)
			// Don't fail the response - sync operation recording is optional
		} else {
			log.Printf("✅ Sync operation recorded successfully for transaction deletion: %s", deleteRequest.OperationID)
		}
	}

	response := ApiResponse{
		Success: true,
		Message: "Transaction deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
