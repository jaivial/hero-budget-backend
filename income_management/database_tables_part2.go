package main

import "log"

// createMonthlyAndLongerPeriodTables crea tablas para períodos mensuales y mayores
func createMonthlyAndLongerPeriodTables() {
	// Tabla monthly_cash_bank_balance para balance mensual
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS monthly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_month TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			total_previous_balance REAL NOT NULL DEFAULT 0,
			total_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create monthly_cash_bank_balance table: %v", err)
	}

	// Create quarterly, semiannual, and annual balance tables with similar structure
	createLongerPeriodTables()
}

// createLongerPeriodTables crea tablas para períodos trimestrales, semestrales y anuales
func createLongerPeriodTables() {
	// Quarterly balance table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS quarterly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_quarter TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			total_previous_balance REAL NOT NULL DEFAULT 0,
			total_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_quarter)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create quarterly_cash_bank_balance table: %v", err)
	}

	// Semiannual and Annual tables follow similar pattern
	createSemiannualAndAnnualTables()
}

// createSemiannualAndAnnualTables crea tablas semestrales y anuales
func createSemiannualAndAnnualTables() {
	// Semiannual balance table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS semiannual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_semester TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			total_previous_balance REAL NOT NULL DEFAULT 0,
			total_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year_semester)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create semiannual_cash_bank_balance table: %v", err)
	}

	// Annual balance table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS annual_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year TEXT NOT NULL,
			income_bank_amount REAL NOT NULL DEFAULT 0,
			income_cash_amount REAL NOT NULL DEFAULT 0,
			expense_bank_amount REAL NOT NULL DEFAULT 0,
			expense_cash_amount REAL NOT NULL DEFAULT 0,
			bill_bank_amount REAL NOT NULL DEFAULT 0,
			bill_cash_amount REAL NOT NULL DEFAULT 0,
			bank_amount REAL NOT NULL DEFAULT 0,
			previous_bank_amount REAL NOT NULL DEFAULT 0,
			cash_amount REAL NOT NULL DEFAULT 0,
			previous_cash_amount REAL NOT NULL DEFAULT 0,
			balance_cash_amount REAL NOT NULL DEFAULT 0,
			balance_bank_amount REAL NOT NULL DEFAULT 0,
			total_previous_balance REAL NOT NULL DEFAULT 0,
			total_balance REAL NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, year)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create annual_cash_bank_balance table: %v", err)
	}
}