package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	// Set up CORS middleware and routes
	http.HandleFunc("/", corsMiddleware(handleRoot))
	http.HandleFunc("/health", corsMiddleware(handleHealth))
	http.HandleFunc("/language/get", corsMiddleware(handleLanguageGet))
	http.HandleFunc("/language/set", corsMiddleware(handleLanguageSet))

	port := 8083
	fmt.Printf("Main Hero Budget service started on port %d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Println(err)
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

func handleRoot(w http.ResponseWriter, r *http.Request) {
	sendSuccessResponse(w, "Hello from Hero Budget Backend!", map[string]string{
		"service": "main",
		"version": "1.0.0",
		"status":  "active",
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return general system health
	sendSuccessResponse(w, "HeroBudget system is healthy", map[string]interface{}{
		"status":    "healthy",
		"service":   "main",
		"port":      "8083",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
		"system":    "HeroBudget Backend",
		"version":   "1.0.0",
		"services": map[string]string{
			"language":        "8083",
			"savings":         "8089",
			"budget_overview": "8098",
			"google_auth":     "8081",
			"dashboard":       "8085",
			"profile":         "8092",
			"bills":           "8091",
			"income":          "8093",
			"expense":         "8094",
			"cash_bank":       "8090",
			"categories":      "8096",
		},
	})
}

func handleLanguageGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Return default language
	sendSuccessResponse(w, "Language retrieved successfully", map[string]string{
		"user_id":  userID,
		"language": "en",
		"locale":   "en-US",
	})
}

func handleLanguageSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID, ok := request["user_id"].(string)
	if !ok || userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	locale, ok := request["locale"].(string)
	if !ok || locale == "" {
		sendErrorResponse(w, "Locale is required", http.StatusBadRequest)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Language updated successfully", map[string]string{
		"user_id": userID,
		"locale":  locale,
	})
}

func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}
