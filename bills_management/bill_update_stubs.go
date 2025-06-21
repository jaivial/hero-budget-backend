package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// parseFlexibleDate parsea fechas manejando múltiples formatos
// Soporta formato ISO con tiempo y formato solo fecha
func parseFlexibleDate(dateStr string) (time.Time, error) {
	if strings.Contains(dateStr, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		return time.Parse("2006-01-02T15:04:05Z", dateStr)
	}
	// Formato solo fecha: "2025-01-15"
	return time.Parse("2006-01-02", dateStr)
}

// revertOldBillEffects revierte los efectos del bill anterior en el sistema
// Simula la eliminación completa del bill anterior para revertir todos sus efectos
func revertOldBillEffects(db *sql.DB, updateData BillUpdateData) error {
	fmt.Printf("🔄 Revirtiendo efectos del bill anterior (ID: %d)\n", updateData.BillID)
	
	// Calcular meses del periodo anterior
	oldMonths, err := calculateMonthsFromDuration(updateData.OldStartDate, updateData.OldDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating old months: %v", err)
	}
	
	// Identificar meses con expenses (pagos realizados)
	oldBillMonthsWithExpense := make(map[string]bool)
	oldBillMonthsWithoutExpense := make(map[string]bool)
	
	// Inicializar todos los meses como sin expense
	for _, month := range oldMonths {
		oldBillMonthsWithoutExpense[month] = true
	}
	
	// Identificar meses con expenses
	rows, err := db.Query("SELECT DISTINCT strftime('%Y-%m', date) as year_month FROM expenses WHERE bill_id = ? AND user_id = ?", updateData.BillID, updateData.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				// Mover de without_expense a with_expense
				delete(oldBillMonthsWithoutExpense, month)
				oldBillMonthsWithExpense[month] = true
			}
		}
	}
	
	// Revertir efectos para meses CON expense
	for month := range oldBillMonthsWithExpense {
		if updateData.OldPaymentMethod == "cash" {
			// Restar de expense_cash_amount, sumar a cash_amount y balance_cash_amount
			db.Exec("UPDATE monthly_cash_bank_balance SET expense_cash_amount = expense_cash_amount - ?, cash_amount = cash_amount + ?, balance_cash_amount = balance_cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		} else {
			// Restar de expense_bank_amount, sumar a bank_amount y balance_bank_amount
			db.Exec("UPDATE monthly_cash_bank_balance SET expense_bank_amount = expense_bank_amount - ?, bank_amount = bank_amount + ?, balance_bank_amount = balance_bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		}
		// Sumar a total_balance
		db.Exec("UPDATE monthly_cash_bank_balance SET total_balance = total_balance + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.UserID, month)
	}
	
	// Revertir efectos para meses SIN expense
	for month := range oldBillMonthsWithoutExpense {
		if updateData.OldPaymentMethod == "cash" {
			// Restar de bill_cash_amount, sumar a cash_amount y balance_cash_amount
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ?, cash_amount = cash_amount + ?, balance_cash_amount = balance_cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		} else {
			// Restar de bill_bank_amount, sumar a bank_amount y balance_bank_amount
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ?, bank_amount = bank_amount + ?, balance_bank_amount = balance_bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.OldAmount, updateData.OldAmount, updateData.OldAmount, updateData.UserID, month)
		}
	}
	
	// Recalcular previous_amounts en cascada desde el mes POSTERIOR al inicio anterior
	// CORREGIDO: Usar función parseFlexibleDate centralizada
	startDate, err := parseFlexibleDate(updateData.OldStartDate)
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
	
	// Recalcular total_balance desde el mes de inicio anterior hasta el más reciente
	rows, err = db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month >= ? ORDER BY year_month", updateData.UserID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months for balance recalculation: %v", err)
	}
	defer rows.Close()
	
	var allAffectedMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			allAffectedMonths = append(allAffectedMonths, month)
		}
	}
	
	// Recalcular total_balance para todos los meses afectados
	for _, month := range allAffectedMonths {
		db.Exec("UPDATE monthly_cash_bank_balance SET total_balance = cash_amount + bank_amount WHERE user_id = ? AND year_month = ?", updateData.UserID, month)
	}
	
	fmt.Printf("✅ Reversión de efectos del bill anterior completada\n")
	return nil
}

// cleanupExpensesForNewPeriod limpia expenses que tienen fechas anteriores al nuevo start_date
// Elimina expenses cuya fecha sea anterior al nuevo periodo del bill
func cleanupExpensesForNewPeriod(db *sql.DB, updateData BillUpdateData) error {
	// Eliminar expenses con fechas anteriores al nuevo start_date
	query := `DELETE FROM expenses WHERE bill_id = ? AND user_id = ? AND date < ?`
	_, err := db.Exec(query, updateData.BillID, updateData.UserID, updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("error deleting old expenses: %v", err)
	}
	fmt.Printf("Cleaned up expenses for bill %d with dates before %s\n", updateData.BillID, updateData.NewStartDate)
	return nil
}

// updateBillPaymentsForNewPeriod actualiza los bill_payments según el nuevo periodo
// Elimina registros fuera del nuevo periodo y crea los faltantes
func updateBillPaymentsForNewPeriod(db *sql.DB, updateData BillUpdateData) error {
	// CORREGIDO: Usar parseFlexibleDate para cálculos de fechas
	startDate, err := parseFlexibleDate(updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}

	// Calcular los meses del nuevo periodo
	newPeriodMonths := make(map[string]bool)
	for i := 0; i < updateData.NewDurationMonths; i++ {
		monthDate := startDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		newPeriodMonths[yearMonth] = true
	}

	// Eliminar registros que están fuera del nuevo periodo
	rows, err := db.Query("SELECT id, year_month FROM bill_payments WHERE bill_id = ?", updateData.BillID)
	if err != nil {
		return fmt.Errorf("error fetching bill payments: %v", err)
	}
	defer rows.Close()

	var toDelete []int
	var existingMonths []string
	for rows.Next() {
		var id int
		var yearMonth string
		if err := rows.Scan(&id, &yearMonth); err == nil {
			if !newPeriodMonths[yearMonth] {
				toDelete = append(toDelete, id)
			} else {
				existingMonths = append(existingMonths, yearMonth)
			}
		}
	}

	// Eliminar registros fuera del periodo
	for _, id := range toDelete {
		_, err := db.Exec("DELETE FROM bill_payments WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("error deleting bill payment %d: %v", id, err)
		}
	}

	// Crear registros faltantes
	existingSet := make(map[string]bool)
	for _, month := range existingMonths {
		existingSet[month] = true
	}

	for yearMonth := range newPeriodMonths {
		if !existingSet[yearMonth] {
			_, err := db.Exec(`INSERT INTO bill_payments (bill_id, year_month, paid, payment_method, created_at) VALUES (?, ?, 0, ?, datetime('now'))`, updateData.BillID, yearMonth, updateData.NewPaymentMethod)
			if err != nil {
				return fmt.Errorf("error creating bill payment for %s: %v", yearMonth, err)
			}
		}
	}

	fmt.Printf("Updated bill payments for bill %d - new period: %d months starting %s\n", updateData.BillID, updateData.NewDurationMonths, updateData.NewStartDate)
	return nil
}