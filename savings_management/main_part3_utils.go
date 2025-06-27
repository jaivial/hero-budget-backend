package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Funciones de utilidad para savings_management
// Contiene handlers de health check y funciones auxiliares

// handleHealth maneja las solicitudes de health check del servicio - Endpoint: GET /health
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	sendSuccessResponse(w, "Savings Management service is healthy", map[string]string{
		"status": "healthy", "service": "savings_management", "timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

// handleSavingsHealth maneja health check específico para savings - Endpoint: GET /savings/health
func handleSavingsHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := db.Ping(); err != nil {
		log.Printf("Health check failed - database connection error: %v", err)
		sendErrorResponse(w, "Database connection failed", http.StatusInternalServerError)
		return
	}
	sendSuccessResponse(w, "Savings Management service is healthy", map[string]string{
		"status": "healthy", "service": "savings_management", "timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	})
}

// sendSuccessResponse envía una respuesta JSON de éxito estandarizada
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ApiResponse{Success: true, Message: message, Data: data})
}

// sendErrorResponse envía una respuesta JSON de error estandarizada
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{Success: false, Message: message})
}