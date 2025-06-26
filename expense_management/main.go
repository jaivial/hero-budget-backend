package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	log.Printf("🚀 Starting Expense Management Service with NEW ENDPOINTS...")
	
	// Set up CORS middleware and routes for expense management endpoints
	http.HandleFunc("/expenses/add", corsMiddleware(handleAddExpense))
	http.HandleFunc("/expenses/update", corsMiddleware(handleUpdateExpense))
	http.HandleFunc("/expenses/delete", corsMiddleware(handleDeleteExpense))
	http.HandleFunc("/expenses/fetch", corsMiddleware(handleFetchExpenses))
	
	// CRITICAL: Add the /expenses endpoint that Flutter expects
	http.HandleFunc("/expenses", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("🔥 NEW ENDPOINT /expenses called with method: %s, query: %s", r.Method, r.URL.RawQuery)
		handleFetchExpenses(w, r)
	}))
	
	http.HandleFunc("/expenses/analytics/daily", corsMiddleware(handleDailyAnalytics))
	http.HandleFunc("/expenses/analytics/weekly", corsMiddleware(handleWeeklyAnalytics))
	http.HandleFunc("/expenses/analytics/monthly", corsMiddleware(handleMonthlyAnalytics))
	http.HandleFunc("/expenses/analytics/quarterly", corsMiddleware(handleQuarterlyAnalytics))
	http.HandleFunc("/expenses/analytics/semiannual", corsMiddleware(handleSemiannualAnalytics))
	http.HandleFunc("/expenses/analytics/annual", corsMiddleware(handleAnnualAnalytics))
	http.HandleFunc("/balance/fetch", corsMiddleware(handleFetchBalance))
	http.HandleFunc("/balance/update-cash", corsMiddleware(handleUpdateCashBalance))
	http.HandleFunc("/balance/update-bank", corsMiddleware(handleUpdateBankBalance))

	// Start the HTTP server on port 8094
	port := 8094
	log.Printf("Expense Management service started on :%d", port)
	log.Printf("Available endpoints:")
	log.Printf("  POST /expenses/add - Add new expense")
	log.Printf("  PUT  /expenses/update - Update existing expense")
	log.Printf("  DELETE /expenses/delete - Delete expense")
	log.Printf("  GET  /expenses/fetch - Fetch expenses for user")
	log.Printf("  GET  /expenses - Fetch expenses for user (NEW COMPATIBLE ENDPOINT)")
	log.Printf("  GET  /expenses/analytics/* - Various analytics endpoints")
	log.Printf("  GET  /balance/fetch - Get user balance")
	log.Printf("  POST /balance/update-* - Update balances")
	log.Printf("🎯 Service ready to handle /expenses endpoint!")

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}