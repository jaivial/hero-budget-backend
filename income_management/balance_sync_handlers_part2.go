package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleSyncQuarterlyBalance maneja requests para sincronizar balance trimestral
func handleSyncQuarterlyBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for quarterly balance sync
	var request struct {
		UserID string `json:"user_id"`
		Date   string `json:"date"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request parameters
	if request.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if request.Date == "" {
		sendErrorResponse(w, "Date is required", http.StatusBadRequest)
		return
	}

	// Synchronize quarterly balance
	err = syncQuarterlyBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing quarterly balance: %v", err)
		sendErrorResponse(w, "Error synchronizing quarterly balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "quarterly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Quarterly balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}

// handleSyncSemiannualBalance maneja requests para sincronizar balance semestral
func handleSyncSemiannualBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for semiannual balance sync
	var request struct {
		UserID string `json:"user_id"`
		Date   string `json:"date"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request parameters
	if request.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if request.Date == "" {
		sendErrorResponse(w, "Date is required", http.StatusBadRequest)
		return
	}

	// Synchronize semiannual balance
	err = syncSemiannualBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing semiannual balance: %v", err)
		sendErrorResponse(w, "Error synchronizing semiannual balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "semiannual")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Semiannual balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}

// handleSyncAnnualBalance maneja requests para sincronizar balance anual
func handleSyncAnnualBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for annual balance sync
	var request struct {
		UserID string `json:"user_id"`
		Date   string `json:"date"`
	}

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request parameters
	if request.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if request.Date == "" {
		sendErrorResponse(w, "Date is required", http.StatusBadRequest)
		return
	}

	// Synchronize annual balance
	err = syncAnnualBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing annual balance: %v", err)
		sendErrorResponse(w, "Error synchronizing annual balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "annual")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Annual balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}
