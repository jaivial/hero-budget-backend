package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// UpdateBillInMonthlyCashBankBalance estructura para actualizar facturas en monthly_cash_bank_balance
// CORREGIDO: Enfoque específico para monthly_cash_bank_balance con lógica coherente
type BillCashBankUpdateData struct {
	BillID            int
	UserID            string
	OldAmount         float64
	NewAmount         float64
	OldDurationMonths int
	NewDurationMonths int
	OldStartDate      string
	NewStartDate      string
	OldPaymentMethod  string
	NewPaymentMethod  string
	OldPaymentDay     int
	NewPaymentDay     int
}

// parseFlexibleDateCashBank parsea fechas manejando múltiples formatos
// Función auxiliar para compatibilidad con formatos ISO y estándar
func parseFlexibleDateCashBank(dateStr string) (time.Time, error) {
	if strings.Contains(dateStr, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		if parsed, err := time.Parse("2006-01-02T15:04:05Z", dateStr); err == nil {
			return parsed, nil
		}
		if parsed, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
			return parsed, nil
		}
	}
	// Formato solo fecha: "2025-01-15"
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		return parsed, nil
	}
	// Formato año-mes: "2025-01" (agregar día 01)
	if parsed, err := time.Parse("2006-01", dateStr); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// calculateMonthsFromDurationCashBank calcula los meses afectados por una factura
// Genera lista de meses en formato YYYY-MM basado en start_date y duration_months
func calculateMonthsFromDurationCashBank(startDateStr string, durationMonths int) ([]string, error) {
	startDate, err := parseFlexibleDateCashBank(startDateStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing start date: %v", err)
	}

	var months []string
	for i := 0; i < durationMonths; i++ {
		monthDate := startDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		months = append(months, yearMonth)
	}
	return months, nil
}

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
	
	log.Printf("✅ Reversión de efectos del bill anterior completada en monthly_cash_bank_balance\n")
	return nil
}

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
	
	// Recalcular total_balance desde el mes de inicio hasta el más reciente
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

// updateBillPaymentsForNewPeriodCashBank actualiza los bill_payments según el nuevo periodo para cash bank
// Versión específica para monthly_cash_bank_balance
func updateBillPaymentsForNewPeriodCashBank(db *sql.DB, updateData BillCashBankUpdateData) error {
	// Usar la función parseFlexibleDateCashBank para cálculos de fechas
	startDate, err := parseFlexibleDateCashBank(updateData.NewStartDate)
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

	log.Printf("Updated bill payments for bill %d - new period: %d months starting %s\n", updateData.BillID, updateData.NewDurationMonths, updateData.NewStartDate)
	return nil
}