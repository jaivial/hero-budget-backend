package main

import (
	"log"
)

// revertBillEffectsFromCashBank revierte los efectos de una factura en monthly_cash_bank_balance
// NUEVA FUNCIÓN: Implementa la lógica inversa de addBillToCashBankBalance
func revertBillEffectsFromCashBank(billData BillData) error {
	log.Printf("🔄 Revirtiendo efectos de factura en monthly_cash_bank_balance: ID=%d, Amount=%.2f, Method=%s", 
		billData.ID, billData.Amount, billData.PaymentMethod)
	
	// Calcular meses afectados por la factura
	months, err := calculateMonthsFromDurationCashBank(billData.StartDate, billData.Duration)
	if err != nil {
		return err
	}
	
	// Revertir efectos para cada mes de la factura
	for _, month := range months {
		// REVERTIR: Restar bill_amount (inverso de sumar)
		if billData.PaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		}
		
		// REVERTIR: Sumar cash_amount o bank_amount (liberar dinero comprometido)
		if billData.PaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET cash_amount = cash_amount + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bank_amount = bank_amount + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		}
		
		// REVERTIR: Sumar balance_amount (restaurar balance disponible)
		if billData.PaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_cash_amount = balance_cash_amount + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_bank_amount = balance_bank_amount + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		}
	}
	
	// REVERTIR: Actualizar previous_amounts en cascada para meses posteriores (sumar lo que se restó)
	startTime, err := parseFlexibleDateCashBank(billData.StartDate)
	if err != nil {
		return err
	}
	startMonth := startTime.Format("2006-01")
	
	// Obtener meses posteriores al mes de inicio
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", billData.UserID, startMonth)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	var subsequentMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			subsequentMonths = append(subsequentMonths, month)
		}
	}
	
	// REVERTIR: Sumar en cascada en previous_amounts (inverso de restar)
	for _, month := range subsequentMonths {
		if billData.PaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.Amount, billData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.Amount, billData.UserID, month)
		}
	}
	
	// CORREGIDO: Recalcular total_balance para todos los meses afectados
	if recalculateErr := recalculateAllSubsequentMonthsBalance(db, billData.UserID, startMonth); recalculateErr != nil {
		log.Printf("Error recalculando balances tras eliminación: %v", recalculateErr)
		return recalculateErr
	}
	
	log.Printf("✅ Efectos de factura revertidos correctamente en monthly_cash_bank_balance")
	return nil
}

// Custom error types for better error handling
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string {
	return e.Message
}

func NewNotFoundError(message string) NotFoundError {
	return NotFoundError{Message: message}
}

// updateMainBalanceColumnsForDeletedBill updates main balance columns incrementally when a bill is deleted
// Eliminating a bill IMPROVES the month's balance, so we ADD the amounts to main balance columns
func updateMainBalanceColumnsForDeletedBill(billData BillData, targetMonth string) error {
	log.Printf("Updating main balance columns for deleted bill - Bill: %d, Amount: %.2f, Method: %s, Month: %s",
		billData.ID, billData.Amount, billData.PaymentMethod, targetMonth)

	// Prepare the update query based on payment method
	var updateQuery string
	switch billData.PaymentMethod {
	case "bank":
		updateQuery = `
			UPDATE monthly_balance 
			SET bank_amount = bank_amount + ?,
				total_balance = bank_amount + cash_amount
			WHERE user_id = ? AND year_month = ?`
	case "cash":
		updateQuery = `
			UPDATE monthly_balance 
			SET cash_amount = cash_amount + ?,
				total_balance = bank_amount + cash_amount
			WHERE user_id = ? AND year_month = ?`
	default:
		return NewValidationError("Invalid payment method: " + billData.PaymentMethod)
	}

	// Execute the update
	result, err := db.Exec(updateQuery, billData.Amount, billData.UserID, targetMonth)
	if err != nil {
		log.Printf("Error updating main balance columns for deleted bill: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("Warning: No rows affected when updating balance for month %s", targetMonth)
	} else {
		log.Printf("Successfully updated main balance columns for month %s", targetMonth)
	}

	return nil
}