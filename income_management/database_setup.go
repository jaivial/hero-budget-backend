package main

import (
	"log"
)

// createTablesIfNotExist crea todas las tablas necesarias para income management
func createTablesIfNotExist() {
	// Create incomes table for storing user income records
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS incomes (
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
		log.Fatalf("Failed to create incomes table: %v", err)
	}

	// Crear tabla cash_bank para el balance global de efectivo/banco
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
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, month)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create cash_bank table: %v", err)
	}

	// Tabla para las transacciones de cash_bank tracking
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

	// Create balance tables for different time periods
	createBalanceTables()
}