package main

import (
	"database/sql"
	"fmt"
	"log"
)

// updateBalanceColumns actualiza columnas de balance de forma unificada
// Función centralizada para actualizar diferentes tipos de columnas en monthly_balance
// columnType: "bill", "expense", "main" - determina qué columna actualizar
// multiplier: 1 para sumar, -1 para restar
func updateBalanceColumns(db *sql.DB, userID, month string, amount float64, paymentMethod, columnType string, multiplier int) {
	var column string
	finalAmount := amount * float64(multiplier)

	// Determinar columna a actualizar según tipo y método de pago
	switch columnType {
	case "bill":
		column = "bills_amount" // Usar columna unificada bills_amount
	case "expense":
		column = "expense_amount" // Usar columna unificada expense_amount
	case "main":
		if paymentMethod == "cash" {
			column = "cash_amount"
		} else {
			column = "bank_amount"
		}
	default:
		return
	}

	// Ejecutar actualización con recalculo de total_balance
	query := fmt.Sprintf("UPDATE monthly_balance SET %s = %s + ?, total_balance = bank_amount + cash_amount WHERE user_id = ? AND year_month = ?", column, column)
	if _, err := db.Exec(query, finalAmount, userID, month); err != nil {
		log.Printf("Error updating %s: %v", column, err)
	}
}

// subtractExpenseAmountForBill resta amount de la tabla expenses
// Utilizada cuando se eliminan meses del periodo que tenían expenses
func subtractExpenseAmountForBill(db *sql.DB, billID int, userID, month string, amount float64) {
	db.Exec("UPDATE expenses SET amount = amount - ? WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", amount, billID, userID, month)
}

// updateExpenseAmountForBillDifference actualiza amount en expenses con diferencia
// Utilizada para ajustar importes en expenses cuando cambia el monto de la factura
func updateExpenseAmountForBillDifference(db *sql.DB, billID int, userID, month string, amountDiff float64) {
	db.Exec("UPDATE expenses SET amount = amount + ? WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", amountDiff, billID, userID, month)
}

// FUNCIÓN ELIMINADA: addNewBillToMonthlyBalance
// Esta función se utilizaba para actualizar la tabla monthly_balance que ha sido eliminada del sistema.
// Ahora se utiliza únicamente addBillToCashBankBalance para monthly_cash_bank_balance.

// updateMainBalanceColumnsForNewBill actualiza columnas principales para nuevo bill
// Función auxiliar para comprometer saldo principal cuando se crea una factura
func updateMainBalanceColumnsForNewBill(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	// Calcular meses afectados por la factura
	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		return err
	}

	// Comprometer saldo principal para cada mes del periodo
	for _, month := range months {
		updateBalanceColumns(db, userID, month, amount, paymentMethod, "main", -1)
	}
	return nil
}

// findEarliestMonth encuentra el mes más temprano en una lista
// Útil para determinar el punto de inicio de cálculos en cascada
func findEarliestMonth(months []string) string {
	if len(months) == 0 {
		return ""
	}

	// Comparación lexicográfica de strings en formato YYYY-MM
	earliest := months[0]
	for _, month := range months[1:] {
		if month < earliest {
			earliest = month
		}
	}
	return earliest
}

// updatePreviousBalancesFromMonth actualiza balances previos en cascada
// CORREGIDO: Actualiza previous_amounts desde el mes POSTERIOR al inicio
// Implementa el algoritmo de cascada descrito en las correcciones
func updatePreviousBalancesFromMonth(db *sql.DB, userID, startMonth string, amountDiff float64, paymentMethod string) error {
	// CORREGIDO: Obtener meses posteriores al mes de inicio (excluyendo el mes de inicio)
	// Si el startMonth es enero, empezamos desde febrero
	rows, err := db.Query("SELECT year_month FROM monthly_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", userID, startMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Recopilar meses posteriores al inicio
	var posteriorMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			posteriorMonths = append(posteriorMonths, month)
		}
	}

	// CORREGIDO: Actualizar previous_amounts y total_previous_balance para meses posteriores
	// La cascada afecta todos los meses desde el posterior al inicio hasta el más reciente
	for _, month := range posteriorMonths {
		if paymentMethod == "cash" {
			// Actualizar previous_cash_amount y total_previous_balance
			query := "UPDATE monthly_balance SET previous_cash_amount = previous_cash_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?"
			db.Exec(query, amountDiff, amountDiff, userID, month)
		} else {
			// Actualizar previous_bank_amount y total_previous_balance
			query := "UPDATE monthly_balance SET previous_bank_amount = previous_bank_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?"
			db.Exec(query, amountDiff, amountDiff, userID, month)
		}

		// CORREGIDO: Recalcular total_balance para el mes
		// Mantener consistencia en el balance total
		db.Exec("UPDATE monthly_balance SET total_balance = cash_amount + bank_amount WHERE user_id = ? AND year_month = ?", userID, month)
	}

	return nil
}
