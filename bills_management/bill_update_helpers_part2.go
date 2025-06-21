package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// parseFlexibleDate parsea fechas manejando múltiples formatos
// Función auxiliar para compatibilidad con formatos ISO y estándar
func parseFlexibleDatePart2(dateStr string) (time.Time, error) {
	if strings.Contains(dateStr, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		return time.Parse("2006-01-02T15:04:05Z", dateStr)
	}
	// Formato solo fecha: "2025-01-15"
	return time.Parse("2006-01-02", dateStr)
}

// applyNewBillToMonthlyBalance aplica el nuevo bill con información actualizada
// NUEVA FUNCIÓN: Aplica el bill con los nuevos parámetros tras el reseteo
// Implementa la aplicación correcta del bill actualizado según el algoritmo
func applyNewBillToMonthlyBalance(db *sql.DB, updateData BillUpdateData) error {
	fmt.Printf("🔄 Aplicando nuevo bill con información actualizada\n")
	
	// Calcular meses del nuevo periodo
	newMonths, err := calculateMonthsFromDuration(updateData.NewStartDate, updateData.NewDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating new months: %v", err)
	}
	
	// Aplicar el nuevo bill a monthly_cash_bank_balance
	for _, month := range newMonths {
		// Asegurar que existe fila para el mes
		db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", updateData.UserID, month)
		
		// Sumar amount a bill_bank_amount o bill_cash_amount
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
	// CORREGIDO: Usar parseFlexibleDate para manejo de formatos ISO
	startDate, err := parseFlexibleDatePart2(updateData.NewStartDate)
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
	
	// Procesar pagos realizados
	processPaidBillPayments(db, updateData)
	
	fmt.Printf("✅ Aplicación del nuevo bill completada\n")
	return nil
}

// processPaidBillPayments procesa pagos ya realizados según algoritmo corregido
// Transfiere importes de bill_amount a expense_amount para meses pagados
func processPaidBillPayments(db *sql.DB, updateData BillUpdateData) {
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
	
	// Para cada mes pagado, transferir de bill_amount a expense_amount
	for _, month := range paidMonths {
		// Restar del bill_amount
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		}
		
		// Sumar al expense_amount
		if updateData.NewPaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET expense_cash_amount = expense_cash_amount + ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET expense_bank_amount = expense_bank_amount + ? WHERE user_id = ? AND year_month = ?", updateData.NewAmount, updateData.UserID, month)
		}
	}
}