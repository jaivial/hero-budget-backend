package main

import (
	"fmt"
	"log"
	"strings"
)

// Database table creation and schema management functions

func createTablesIfNotExist() {
	// Create expenses table for storing user expense records
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS expenses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			category TEXT NOT NULL,
			payment_method TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create expenses table: %v", err)
	}

	// Create balances table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS balances (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT UNIQUE NOT NULL,
			cash_balance REAL NOT NULL DEFAULT 0,
			bank_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create balances table: %v", err)
	}

	// Ensure cash_bank table exists for cash/bank distribution tracking
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			month TEXT NOT NULL,
			cash_amount REAL NOT NULL DEFAULT 0,
			cash_percent REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			bank_percent REAL NOT NULL DEFAULT 0,
			monthly_total REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Ensure cash_bank_transactions table exists for transaction history
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cash_bank_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			transaction_type TEXT NOT NULL,
			amount REAL NOT NULL,
			date TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank_transactions table: %v", err)
	}

	// Create daily_balance table for daily expense analytics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, date)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_balance table: %v", err)
	}

	// Create indices for daily_balance for improved query performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_user ON daily_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_balance_date ON daily_balance(date)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_balance: %v", err)
	}

	// Create weekly_balance table for weekly expense analytics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_week)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_balance table: %v", err)
	}

	// Create indices for weekly_balance for improved query performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_user ON weekly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_balance_week ON weekly_balance(year_week)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_balance: %v", err)
	}

	// Create quarterly_balance table for quarterly expense analytics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_quarter)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_balance table: %v", err)
	}

	// Create indices for quarterly_balance for improved query performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_balance_user ON quarterly_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_quarterly_balance_quarter ON quarterly_balance(year_quarter)`)
	if err != nil {
		log.Fatalf("Failed to create index on quarterly_balance: %v", err)
	}

	// Create semiannual_balance table for semiannual expense analytics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_half TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_half)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_balance table: %v", err)
	}

	// Create indices for semiannual_balance for improved query performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_balance_user ON semiannual_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_semiannual_balance_half ON semiannual_balance(year_half)`)
	if err != nil {
		log.Fatalf("Failed to create index on semiannual_balance: %v", err)
	}

	// Create annual_balance table for annual expense analytics
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_amount REAL NOT NULL DEFAULT 0,
			expense_amount REAL NOT NULL DEFAULT 0,
			bills_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			balance REAL NOT NULL DEFAULT 0,
			previous_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_balance table: %v", err)
	}

	// Create indices for annual_balance for improved query performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_balance_user ON annual_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_annual_balance_year ON annual_balance(year)`)
	if err != nil {
		log.Fatalf("Failed to create index on annual_balance: %v", err)
	}

	// Add cash_amount and bank_amount columns to existing tables if they don't exist
	// For daily_balance
	alterTableSafely("daily_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("daily_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For weekly_balance
	alterTableSafely("weekly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("weekly_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For quarterly_balance
	alterTableSafely("quarterly_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("quarterly_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For semiannual_balance
	alterTableSafely("semiannual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("semiannual_balance", "total_balance", "REAL NOT NULL DEFAULT 0")

	// For annual_balance
	alterTableSafely("annual_balance", "cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_cash_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "previous_bank_amount", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "total_previous_balance", "REAL NOT NULL DEFAULT 0")
	alterTableSafely("annual_balance", "total_balance", "REAL NOT NULL DEFAULT 0")
}

// Helper function to safely alter a table by adding a column if it doesn't exist
func alterTableSafely(tableName, columnName, columnType string) {
	// Check if the column exists by examining the table schema
	var existsQuery string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&existsQuery); err != nil {
		log.Printf("Error checking table %s: %v", tableName, err)
		return
	}

	// If the column doesn't exist in the schema, add it
	if !strings.Contains(existsQuery, columnName) {
		_, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, columnName, columnType))
		if err != nil {
			log.Printf("Error adding column %s to table %s: %v", columnName, tableName, err)
		} else {
			log.Printf("Added column %s to table %s successfully", columnName, tableName)
		}
	}
}

// Function to add cash_amount and bank_amount columns to all balance tables if needed
func addCashBankColumnsToAllTables() {
	// Tables that need cash/bank columns for expense tracking by payment method
	tables := []string{
		"daily_balance",
		"weekly_balance", 
		"quarterly_balance",
		"semiannual_balance",
		"annual_balance",
	}

	// Columns to add for tracking cash vs bank expenses
	columns := []struct {
		name string
		type_ string
	}{
		{"expense_cash_amount", "REAL NOT NULL DEFAULT 0"},
		{"expense_bank_amount", "REAL NOT NULL DEFAULT 0"},
		{"income_cash_amount", "REAL NOT NULL DEFAULT 0"},
		{"income_bank_amount", "REAL NOT NULL DEFAULT 0"},
		{"bill_cash_amount", "REAL NOT NULL DEFAULT 0"},
		{"bill_bank_amount", "REAL NOT NULL DEFAULT 0"},
		{"balance_cash_amount", "REAL NOT NULL DEFAULT 0"},
		{"balance_bank_amount", "REAL NOT NULL DEFAULT 0"},
		{"total_previous_balance", "REAL NOT NULL DEFAULT 0"},
		{"total_balance", "REAL NOT NULL DEFAULT 0"},
	}

	// Add columns to each table
	for _, table := range tables {
		for _, column := range columns {
			alterTableSafely(table, column.name, column.type_)
		}
	}
}