package main

import (
	"database/sql"
	"fmt"
	"log"
)

// applyNewBillToCashBank aplica el nuevo bill con información actualizada a monthly_cash_bank_balance
// NUEVA FUNCIÓN: Aplica el bill con los nuevos parámetros tras el reseteo
func applyNewBillToCashBank(db *sql.DB, updateData BillCashBankUpdateData) error {
	log.Printf("🔄 Aplicando nuevo bill con información actualizada a monthly_cash_bank_balance\n")
	
	// Calcular meses del nuevo periodo
	newMonths, err := calculateMonthsFromDurationCashBank(updateData.NewStartDate, updateData.NewDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating new months: %v", err)
	}
	
	// Aplicar el nuevo bill a monthly_cash_bank_balance
	for _, month := range newMonths {
		// Asegurar que existe fila para el mes
		db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", updateData.UserID, month)
		
		// Sumar amount a bill_cash_amount o bill_bank_amount
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		}
		
		// Restar amount de bank_amount o cash_amount (comprometer dinero)
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET cash_amount = cash_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bank_amount = bank_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		}
		
		// Restar amount de balance_cash_amount o balance_bank_amount
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_cash_amount = balance_cash_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET balance_bank_amount = balance_bank_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		}
	}
	
	// Recalcular previous_amounts en cascada desde el mes POSTERIOR al nuevo inicio
	startDate, err := parseFlexibleDateCashBank(updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid new start date: %v", err)
	}
	startMonth := startDate.Format("2006-01")
	
	// Obtener meses posteriores al nuevo mes de inicio
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", updateData.UserID, startMonth)
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
	
	// Restar en cascada en previous_cash_amount o previous_bank_amount
	for _, month := range subsequentMonths {
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.NewAmount, updateData.UserID, month)
		}
	}
	
	// CORREGIDO: Recalcular total_balance para todos los meses afectados
	recalculateAllSubsequentMonthsBalance(db, updateData.UserID, startMonth)
	
	// Procesar pagos ya realizados
	processPaidBillPaymentsInCashBank(db, updateData)
	
	log.Printf("✅ Aplicación del nuevo bill completada en monthly_cash_bank_balance\n")
	return nil
}

// processPaidBillPaymentsInCashBank procesa pagos ya realizados en monthly_cash_bank_balance
// Maneja la lógica específica para pagos ya procesados
func processPaidBillPaymentsInCashBank(db *sql.DB, updateData BillCashBankUpdateData) {
	// Obtener meses con pagos realizados (paid = 1)
	rows, err := db.Query("SELECT year_month FROM bill_payments WHERE bill_id = ? AND paid = 1", updateData.BillID)
	if err != nil {
		return
	}
	defer rows.Close()
	
	var paidMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			paidMonths = append(paidMonths, month)
		}
	}
	
	// Para cada mes pagado, ajustar los balances
	// Los pagos ya realizados no necesitan transferencia bill->expense
	// Solo ajustar las cantidades según el nuevo amount
	for _, month := range paidMonths {
		// Diferencia entre nuevo y viejo amount
		amountDifference := updateData.NewAmount - updateData.OldAmount
		
		if amountDifference != 0 {
			// Ajustar bill_amount según la diferencia
			if updateData.NewPaymentMethod == "cash" {
				db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount + ? WHERE user_id = ? AND year_month = ?", amountDifference, updateData.UserID, month)
			} else {
				db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount + ? WHERE user_id = ? AND year_month = ?", amountDifference, updateData.UserID, month)
			}
		}
	}
}