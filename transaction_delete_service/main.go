package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Transaction deletion request structure
type DeleteTransactionRequest struct {
	UserID          string `json:"user_id"`
	TransactionID   int    `json:"transaction_id"`
	TransactionType string `json:"transaction_type"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var db *sql.DB

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

	log.Println("Transaction Delete Service - Database connection established successfully")
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

	response := ApiResponse{
		Success: true,
		Message: "Transaction deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
