package main

import (
	"database/sql"
	"fmt"
	"log"
)

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
