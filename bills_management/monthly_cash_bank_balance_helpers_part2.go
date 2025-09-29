package main

import (
	"database/sql"
	"fmt"
	"log"
)

// revertOldBillEffectsInCashBank revierte los efectos del bill anterior en monthly_cash_bank_balance
// CORREGIDO: Lógica específica para monthly_cash_bank_balance
func revertOldBillEffectsInCashBank(db *sql.DB, updateData BillCashBankUpdateData) error {
	log.Printf("🔄 Revirtiendo efectos del bill anterior (ID: %d) en monthly_cash_bank_balance\n", updateData.BillID)

	// Calcular meses del periodo anterior
	oldMonths, err := calculateMonthsFromDurationCashBank(updateData.OldStartDate, updateData.OldDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating old months: %v", err)
	}

	// Identificar meses con pagos realizados (paid = 1)
	oldBillMonthsWithPayment := make(map[string]bool)
	oldBillMonthsWithoutPayment := make(map[string]bool)

	// Inicializar todos los meses como sin pago
	for _, month := range oldMonths {
		oldBillMonthsWithoutPayment[month] = true
	}

	// Identificar meses con pagos realizados
	rows, err := db.Query("SELECT DISTINCT year_month FROM bill_payments WHERE bill_id = ? AND paid = 1", updateData.BillID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				// Mover de without_payment a with_payment
				delete(oldBillMonthsWithoutPayment, month)
				oldBillMonthsWithPayment[month] = true
			}
		}
	}

	// Revertir efectos para meses CON pago realizado
	// En estos meses, la factura ya fue "gastada", solo revertir el bill_amount
	for month := range oldBillMonthsWithPayment {
		// Los pagos realizados solo necesitan revertir el bill_amount
		// No afectan expense_amount porque ya fue procesado
		if updateData.OldPaymentMethod == "cash" {
			// Restar de bill_cash_amount (revertir la suma original)
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.UserID, month)
		} else {
			// Restar de bill_bank_amount (revertir la suma original)
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.UserID, month)
		}
	}

	// Revertir efectos para meses SIN pago realizado
	// En estos meses, la factura está pendiente, revertir bill_amount y liberar dinero comprometido
	for month := range oldBillMonthsWithoutPayment {
		if updateData.OldPaymentMethod == "cash" {
			// Restar de bill_cash_amount y sumar a cash_amount (liberar dinero comprometido)
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ?, cash_amount = cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
			// Sumar a balance_cash_amount (restaurar balance disponible)
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_cash_amount = balance_cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.UserID, month)
		} else {
			// Restar de bill_bank_amount y sumar a bank_amount (liberar dinero comprometido)
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ?, bank_amount = bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
			// Sumar a balance_bank_amount (restaurar balance disponible)
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_bank_amount = balance_bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.UserID, month)
		}
	}

	// Recalcular previous_amounts en cascada desde el mes POSTERIOR al inicio anterior
	startDate, err := parseFlexibleDateCashBank(updateData.OldStartDate)
	if err != nil {
		return fmt.Errorf("invalid old start date %s: %v", updateData.OldStartDate, err)
	}
	startMonth := startDate.Format("2006-01")

	// Obtener meses posteriores al mes de inicio anterior
	rows, err = db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", updateData.UserID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching subsequent months: %v", err)
	}
	defer rows.Close()

	var subsequentMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			subsequentMonths = append(subsequentMonths, month)
		}
	}

	// Sumar en cascada en previous_cash_amount o previous_bank_amount (revertir la resta anterior)
	for _, month := range subsequentMonths {
		if updateData.OldPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		}
	}

	// CORREGIDO: Recalcular total_balance para todos los meses afectados
	recalculateAllSubsequentMonthsBalance(db, updateData.UserID, startMonth)

	log.Printf("✅ Reversión de efectos del bill anterior completada en monthly_cash_bank_balance\n")
	return nil
}
