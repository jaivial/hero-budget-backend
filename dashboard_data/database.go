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

	// Insert mock data for testing purposes
	insertMockDataIfEmpty()
}

// insertMockDataIfEmpty inserta datos de prueba si las tablas están vacías
func insertMockDataIfEmpty() {
	// Check if there is data in the budget table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM budget").Scan(&count)
	if err != nil {
		log.Printf("Error checking budget table: %v", err)
		return
	}

	// If there's no data, insert mock data for testing
	if count == 0 {
		insertMockData()
	}
}

// insertMockData inserta datos de ejemplo para pruebas
func insertMockData() {
	// Insert mock budget data for development testing
	_, err := db.Exec(`
		INSERT INTO budget (user_id, period, date, total_amount, remaining_amount, spent_amount, upcoming_amount, from_previous, percent)
		VALUES ('1', 'monthly', '2025-05-01', 975.00, 875.00, 0.00, 100.00, 975.00, 10.0)
	`)
	if err != nil {
		log.Printf("Error inserting mock budget data: %v", err)
	}

	// Insert mock savings data for development testing
	_, err = db.Exec(`
		INSERT INTO savings (user_id, available, goal, percent)
		VALUES ('1', 875.00, 1000.00, 88.0)
	`)
	if err != nil {
		log.Printf("Error inserting mock savings data: %v", err)
	}

	// Insert mock cash_bank data for development testing
	_, err = db.Exec(`
		INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
		VALUES ('1', 'mayo de 2025', 200.00, 100.0, 0.00, 0.0, 200.00)
	`)
	if err != nil {
		log.Printf("Error inserting mock cash_bank data: %v", err)
	}

	// Insert mock finance_metrics data for development testing
	_, err = db.Exec(`
		INSERT INTO finance_metrics (user_id, period, income, expenses, bills)
		VALUES ('1', 'monthly', 0.00, 0.00, 100.00)
	`)
	if err != nil {
		log.Printf("Error inserting mock finance_metrics data: %v", err)
	}

	// Insert mock bills data for development testing
	_, err = db.Exec(`
		INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon)
		VALUES ('1', 'Cash', 100.00, '2025-05-28', false, true, 8751, true, 'Rent', '🏠')
	`)
	if err != nil {
		log.Printf("Error inserting mock bills data: %v", err)
	}
}