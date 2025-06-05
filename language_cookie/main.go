package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type LanguageRequest struct {
	Locale string `json:"locale"`
}

type LanguageResponse struct {
	Success bool   `json:"success"`
	Locale  string `json:"locale"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	// Set up CORS middleware
	http.HandleFunc("/language/set", corsMiddleware(handleSetLanguage))
	http.HandleFunc("/language/get", corsMiddleware(handleGetLanguage))
	http.HandleFunc("/health", corsMiddleware(handleHealth))

	log.Println("Language service started on :8083")
	log.Fatal(http.ListenAndServe(":8083", nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the actual handler
		next(w, r)
	}
}

func handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LanguageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Locale == "" {
		http.Error(w, "Locale is required", http.StatusBadRequest)
		return
	}

	// Set a cookie for the browser to store the language preference
	http.SetCookie(w, &http.Cookie{
		Name:     "preferred_language",
		Value:    req.Locale,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 1 year
		HttpOnly: false,              // Allow JavaScript access
		SameSite: http.SameSiteLaxMode,
	})

	// Return response (client will also store in localStorage)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LanguageResponse{
		Success: true,
		Locale:  req.Locale,
	})

	log.Printf("Language preference set to %s", req.Locale)
}

func handleGetLanguage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to get the language cookie
	cookie, err := r.Cookie("preferred_language")
	locale := ""
	if err == nil {
		locale = cookie.Value
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LanguageResponse{
		Success: locale != "",
		Locale:  locale,
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return success response
	sendSuccessResponse(w, "Language service is healthy", map[string]string{
		"status":    "healthy",
		"service":   "language_cookie",
		"port":      "8083",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
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
