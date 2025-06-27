package main

import (
	"fmt"
)

// Funciones de base de datos para Cash Bank Management - Parte 2
// Continuación de operaciones CRUD y funciones auxiliares
// Incluye funciones genéricas y de utilidad para manejo de tablas

// updatePeriodTable función genérica para actualizar cualquier tabla periódica
// Permite reutilizar lógica para todas las tablas de diferentes períodos
// Maneja tanto inserción como actualización según existencia de registro
func updatePeriodTable(tableName, periodColumn, periodValue string, distribution CashBankDistribution) error {
	// Check if entry exists for this user and period
	// Verificar si existe entrada para este usuario y período
	var count int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)
	err := db.QueryRow(query, distribution.UserID, periodValue).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing entry with new values
		// Actualizar entrada existente con nuevos valores
		updateQuery := fmt.Sprintf(`
			UPDATE %s
			SET cash_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
				total_balance = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND %s = ?
		`, tableName, periodColumn)

		_, err = db.Exec(updateQuery,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
			distribution.UserID,
			periodValue,
		)
	} else {
		// Insert new entry for this period
		// Insertar nueva entrada para este período
		insertQuery := fmt.Sprintf(`
			INSERT INTO %s (
				user_id, %s, cash_amount, bank_amount, balance_cash_amount, balance_bank_amount, total_balance
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, tableName, periodColumn)

		_, err = db.Exec(insertQuery,
			distribution.UserID,
			periodValue,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
		)
	}

	return err
}

// addTransaction registra nueva transacción en el historial
// Mantiene trazabilidad completa de todas las operaciones
// Esencial para auditoría y análisis de patrones de uso
func addTransaction(userID, transactionType string, amount float64, date string) error {
	// Insert transaction record with all relevant details
	// Insertar registro de transacción con todos los detalles relevantes
	_, err := db.Exec(`
		INSERT INTO cash_bank_transactions (
			user_id, transaction_type, amount, date
		) VALUES (?, ?, ?, ?)
	`,
		userID,         // ID del usuario que realiza la transacción
		transactionType, // Tipo: cash_update, bank_update, cash_to_bank, bank_to_cash
		amount,         // Cantidad involucrada en la transacción
		date,           // Fecha de la transacción (formato ISO)
	)

	return err
}