package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// handleSyncDailyBalance maneja requests para sincronizar balance diario
func handleSyncDailyBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for daily balance sync
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

	// Synchronize daily balance
	err = syncDailyBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing daily balance: %v", err)
		sendErrorResponse(w, "Error synchronizing daily balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "daily")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Daily balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}

// handleSyncWeeklyBalance maneja requests para sincronizar balance semanal
func handleSyncWeeklyBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for weekly balance sync
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

	// Synchronize weekly balance
	err = syncWeeklyBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing weekly balance: %v", err)
		sendErrorResponse(w, "Error synchronizing weekly balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "weekly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Weekly balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}

// handleSyncMonthlyBalance maneja requests para sincronizar balance mensual
func handleSyncMonthlyBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body for monthly balance sync
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

	// Synchronize monthly balance
	err = syncMonthlyBalance(request.UserID, request.Date)
	if err != nil {
		log.Printf("Error synchronizing monthly balance: %v", err)
		sendErrorResponse(w, "Error synchronizing monthly balance", http.StatusInternalServerError)
		return
	}

	// Invalidate related cache entries
	if cacheManager != nil {
		err = cacheManager.InvalidateDashboardCache(request.UserID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache: %v", err)
		}
	}

	// Return success response
	sendSuccessResponse(w, "Monthly balance synchronized successfully", map[string]interface{}{
		"user_id": request.UserID,
		"date":    request.Date,
	})
}