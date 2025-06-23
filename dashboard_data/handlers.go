package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// corsMiddleware maneja CORS para permitir requests desde el frontend
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set headers for cross-origin resource sharing
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// If it's OPTIONS request, return with just the headers (preflight request)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler in the chain
		next(w, r)
	}
}

// handleFetchDashboardData maneja requests para obtener datos del dashboard
func handleFetchDashboardData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get period from query parameter (default to 'monthly' if not provided)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "monthly"
	}

	// Try to get data from cache first for improved performance
	var dashboardData DashboardData
	if cacheManager != nil {
		err := cacheManager.GetDashboardData(userID, period, &dashboardData)
		if err == nil {
			log.Printf("✓ Cache HIT: dashboard data for user %s period %s", userID, period)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(dashboardData)
			return
		}
		log.Printf("🔍 Cache MISS: dashboard data for user %s period %s", userID, period)
	}

	// Get dashboard data from database if not in cache
	dashboardData, err := fetchDashboardData(userID, period)
	if err != nil {
		log.Printf("Error fetching dashboard data: %v", err)
		http.Error(w, "Error fetching dashboard data", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests to improve performance
	if cacheManager != nil {
		err = cacheManager.CacheDashboardData(userID, period, dashboardData)
		if err != nil {
			log.Printf("Warning: Failed to cache dashboard data: %v", err)
		}
	}

	// Return dashboard data as JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboardData)
}