package main

import (
	"fmt"
)

// Funciones de base de datos para Cash Bank Management - Parte 2
// Continuación de operaciones CRUD y funciones auxiliares
// Incluye funciones genéricas y de utilidad para manejo de tablas


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