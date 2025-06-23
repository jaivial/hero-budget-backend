package main

import "log"

// createBalanceTables crea tablas de balance para diferentes períodos
func createBalanceTables() {
	// Tabla daily_cash_bank_balance para balance diario
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS daily_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			date TEXT NOT NULL,
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
			UNIQUE(user_id, date)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create daily_cash_bank_balance table: %v", err)
	}

	// Create indices for daily_cash_bank_balance for better performance
	createDailyBalanceIndices()

	// Tabla weekly_cash_bank_balance para balance semanal
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS weekly_cash_bank_balance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			year_week TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
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
			UNIQUE(user_id, year_week)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create weekly_cash_bank_balance table: %v", err)
	}

	// Create indices for weekly_cash_bank_balance for better performance
	createWeeklyBalanceIndices()

	// Continue creating monthly, quarterly, semiannual, and annual tables
	createMonthlyAndLongerPeriodTables()
}

// createDailyBalanceIndices crea índices para la tabla de balance diario
func createDailyBalanceIndices() {
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_user ON daily_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_cash_bank_balance_date ON daily_cash_bank_balance(date)`)
	if err != nil {
		log.Fatalf("Failed to create index on daily_cash_bank_balance: %v", err)
	}
}

// createWeeklyBalanceIndices crea índices para la tabla de balance semanal
func createWeeklyBalanceIndices() {
	_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_balance_user ON weekly_cash_bank_balance(user_id)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_cash_bank_balance: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_weekly_cash_bank_balance_year_week ON weekly_cash_bank_balance(year_week)`)
	if err != nil {
		log.Fatalf("Failed to create index on weekly_cash_bank_balance: %v", err)
	}
}