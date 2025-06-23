package main

import (
	"fmt"
	"log"
)

// ensureRequiredColumns verifica y añade columnas faltantes a tablas existentes
func ensureRequiredColumns() {
	tables := []string{
		"daily_cash_bank_balance",
		"weekly_cash_bank_balance",
		"monthly_cash_bank_balance",
		"quarterly_cash_bank_balance",
		"semiannual_cash_bank_balance",
		"annual_cash_bank_balance",
	}

	for _, table := range tables {
		if !tableExists(table) {
			continue
		}

		// Añadir columnas estándar a todas las tablas
		alterTableSafely(table, "cash_amount", "REAL")
		alterTableSafely(table, "bank_amount", "REAL")
		alterTableSafely(table, "previous_cash_amount", "REAL")
		alterTableSafely(table, "previous_bank_amount", "REAL")
		alterTableSafely(table, "balance_cash_amount", "REAL")
		alterTableSafely(table, "balance_bank_amount", "REAL")
		alterTableSafely(table, "total_previous_balance", "REAL")
		alterTableSafely(table, "total_balance", "REAL")

		// Columnas específicas para cada tabla
		if table == "weekly_cash_bank_balance" {
			alterTableSafely(table, "start_date", "TEXT")
			alterTableSafely(table, "end_date", "TEXT")
		}
	}

	// Asegurarse de que existe la tabla cash_bank
	ensureCashBankTable()

	// Asegurarse de que existe la tabla cash_bank_transactions
	ensureCashBankTransactionsTable()
}

// ensureCashBankTable verifica y crea la tabla cash_bank si no existe
func ensureCashBankTable() {
	if !tableExists("cash_bank") {
		_, err := db.Exec(`
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
			log.Printf("Error creating cash_bank table: %v", err)
		} else {
			log.Println("Created cash_bank table")
		}
	}
}

// ensureCashBankTransactionsTable verifica y crea la tabla de transacciones si no existe
func ensureCashBankTransactionsTable() {
	if !tableExists("cash_bank_transactions") {
		_, err := db.Exec(`
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
			log.Printf("Error creating cash_bank_transactions table: %v", err)
		} else {
			log.Println("Created cash_bank_transactions table")
		}
	}
}

// tableExists verifica si una tabla existe en la base de datos
func tableExists(tableName string) bool {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	var name string
	err := db.QueryRow(query, tableName).Scan(&name)
	return err == nil
}

// alterTableSafely añade una columna a una tabla de forma segura
func alterTableSafely(tableName, columnName, columnType string) {
	// Comprobar si la columna ya existe
	var exists bool
	query := fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error checking table schema: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notnull int
		var dflt_value interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notnull, &dflt_value, &pk); err != nil {
			log.Printf("Error scanning row: %v", err)
			return
		}
		if name == columnName {
			exists = true
			break
		}
	}

	// Si la columna no existe, añadirla
	if !exists {
		alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s DEFAULT 0", tableName, columnName, columnType)
		_, err := db.Exec(alterQuery)
		if err != nil {
			log.Printf("Error adding column %s to %s: %v", columnName, tableName, err)
			return
		}
		log.Printf("Added column %s to %s", columnName, tableName)
	}
}