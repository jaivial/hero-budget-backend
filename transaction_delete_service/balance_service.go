package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// updateSubsequentPeriodsForExpense maneja la actualización en cascada específica para expenses con bill_id = NULL
func updateSubsequentPeriodsForExpense(userID, tableName, periodType string, transactionDate time.Time, amount float64, paymentMethod string, billID *int) error {
	// Solo aplicar cascada si bill_id = NULL
	if billID != nil {
		// Si tiene bill_id, usar la lógica original
		return updateSubsequentPeriods(userID, tableName, periodType, transactionDate)
	}

	log.Printf("Updating subsequent periods for expense with bill_id = NULL (cascade effect)")

	// Obtener todos los períodos posteriores al mes de la transacción eliminada
	var nextPeriods []string

	switch periodType {
	case "monthly":
		// Buscar períodos hasta 24 meses hacia adelante para cubrir gaps
		for i := 1; i <= 24; i++ {
			nextDate := transactionDate.AddDate(0, i, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "quarterly":
		// Buscar períodos hasta 8 trimestres hacia adelante
		for i := 1; i <= 8; i++ {
			nextDate := transactionDate.AddDate(0, i*3, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "annual":
		// Buscar períodos hasta 10 años hacia adelante
		for i := 1; i <= 10; i++ {
			nextDate := transactionDate.AddDate(i, 0, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	default:
		// Para otros tipos, usar lógica original
		return updateSubsequentPeriods(userID, tableName, periodType, transactionDate)
	}

	// Determinar columna de período
	var periodColumn string
	switch {
	case strings.Contains(tableName, "monthly"):
		periodColumn = "year_month"
	case strings.Contains(tableName, "quarterly"):
		periodColumn = "year_quarter"
	case strings.Contains(tableName, "annual"):
		periodColumn = "year"
	default:
		periodColumn = "period"
	}

	// Actualizar cada período posterior que exista
	for _, nextPeriod := range nextPeriods {
		err := updateCascadeForPeriod(userID, tableName, periodColumn, nextPeriod, amount, paymentMethod)
		if err != nil {
			log.Printf("Error updating cascade for period %s: %v", nextPeriod, err)
			// Continuar con el siguiente período aunque falle uno
		}
	}

	return nil
}

// updateCascadeForPeriod actualiza un período específico con la lógica de cascada
func updateCascadeForPeriod(userID, tableName, periodColumn, period string, amount float64, paymentMethod string) error {
	// Verificar si el período existe
	var exists bool
	checkQuery := fmt.Sprintf(`SELECT COUNT(*) > 0 FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)
	err := db.QueryRow(checkQuery, userID, period).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking period existence: %v", err)
	}

	if !exists {
		// Si el período no existe, no hacer nada (gap entre períodos)
		return nil
	}

	// Preparar las actualizaciones según el tipo de pago
	var updates []string
	var params []interface{}

	if paymentMethod == "bank" {
		// Sumar el amount de vuelta a bank_amount (balance actual incrementa por expense eliminado)
		updates = append(updates, "bank_amount = bank_amount + ?")
		params = append(params, amount)

		// Sumar el amount de vuelta a previous_bank_amount (ya que el expense anterior se eliminó)
		updates = append(updates, "previous_bank_amount = previous_bank_amount + ?")
		params = append(params, amount)

		// Sumar el amount de vuelta a balance_bank_amount
		updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
		params = append(params, amount)
	} else { // cash
		// Sumar el amount de vuelta a cash_amount (balance actual incrementa por expense eliminado)
		updates = append(updates, "cash_amount = cash_amount + ?")
		params = append(params, amount)

		// Sumar el amount de vuelta a previous_cash_amount (ya que el expense anterior se eliminó)
		updates = append(updates, "previous_cash_amount = previous_cash_amount + ?")
		params = append(params, amount)

		// Sumar el amount de vuelta a balance_cash_amount
		updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
		params = append(params, amount)
	}

	// Sumar el amount a total_previous_balance (porque el expense que contribuía negativamente se eliminó)
	updates = append(updates, "total_previous_balance = total_previous_balance + ?")
	params = append(params, amount)

	// Agregar timestamp
	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	// Ejecutar la actualización principal
	updateQuery := fmt.Sprintf("UPDATE %s SET %s WHERE user_id = ? AND %s = ?",
		tableName, strings.Join(updates, ", "), periodColumn)

	// Agregar parámetros de WHERE
	params = append(params, userID, period)

	_, err = db.Exec(updateQuery, params...)
	if err != nil {
		return fmt.Errorf("error updating cascade for period %s: %v", period, err)
	}

	// Actualizar total_balance por separado para usar valores actualizados
	totalBalanceQuery := fmt.Sprintf(`
		UPDATE %s 
		SET total_balance = total_previous_balance + (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(totalBalanceQuery, userID, period)
	if err != nil {
		return fmt.Errorf("error updating total balance for period %s: %v", period, err)
	}

	log.Printf("Updated cascade for period %s: amount %.2f (%s)", period, amount, paymentMethod)
	return nil
}
