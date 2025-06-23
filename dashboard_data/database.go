package main

import (
	"log"
)

// createTablesIfNotExist crea todas las tablas necesarias si no existen
func createTablesIfNotExist() {
	// Create budget table for storing budget information
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS budget (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			period TEXT NOT NULL,
			date TEXT NOT NULL,
			total_amount REAL NOT NULL,
			remaining_amount REAL NOT NULL,
			spent_amount REAL NOT NULL,
			upcoming_amount REAL NOT NULL,
			from_previous REAL NOT NULL,
			percent REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create budget table: %v", err)
	}

	// Create savings table for tracking savings goals
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS savings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			available REAL NOT NULL,
			goal REAL NOT NULL,
			period TEXT NOT NULL DEFAULT 'monthly',
			percent REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create savings table: %v", err)
	}

	// Add period column if it doesn't exist (for existing tables)
	_, err = db.Exec(`
		ALTER TABLE savings ADD COLUMN period TEXT NOT NULL DEFAULT 'monthly'
	`)
	if err != nil {
		// Column might already exist, which is fine
		log.Printf("Note: period column might already exist: %v", err)
	}

	// Create cash_bank table for cash/bank distribution
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			month TEXT NOT NULL,
			cash_amount REAL NOT NULL,
			cash_percent REAL NOT NULL,
			bank_amount REAL NOT NULL,
			bank_percent REAL NOT NULL,
			monthly_total REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Create finance_metrics table for financial metrics tracking
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS finance_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			period TEXT NOT NULL,
			income REAL NOT NULL,
			expenses REAL NOT NULL,
			bills REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create finance_metrics table: %v", err)
	}

	// Create bills table for managing upcoming bills
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bills (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			amount REAL NOT NULL,
			due_date TEXT NOT NULL,
			paid BOOLEAN NOT NULL,
			overdue BOOLEAN NOT NULL,
			overdue_days INTEGER NOT NULL,
			recurring BOOLEAN NOT NULL,
			category TEXT NOT NULL,
			icon TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create bills table: %v", err)
	}

	// Mock data insertion disabled to prevent data loss in production
	// insertMockDataIfEmpty() // DISABLED - Was causing data loss during service restarts
}

// Mock data insertion functions REMOVED to prevent data loss during service restarts
// These functions were causing production data to be overwritten with test data
// Original functions: insertMockDataIfEmpty() and insertMockData()
// REMOVED ON: 2025-06-23 to fix VPS data loss issue