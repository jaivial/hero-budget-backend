package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// updateBillDurationLogic maneja cambios de duración y fecha de inicio
func updateBillDurationLogic(db *sql.DB, updateData BillUpdateData) error {
	durationChanged := updateData.OldDurationMonths != updateData.NewDurationMonths
	startDateChanged := updateData.OldStartDate != updateData.NewStartDate

	if !durationChanged && !startDateChanged {
		return nil
	}

	oldMonths, err := calculateMonthsFromDuration(updateData.OldStartDate, updateData.OldDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating old months: %v", err)
	}

	newMonths, err := calculateMonthsFromDuration(updateData.NewStartDate, updateData.NewDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating new months: %v", err)
	}

	removedMonths := findDifferenceMonths(oldMonths, newMonths)
	addedMonths := findDifferenceMonths(newMonths, oldMonths)
	remainingMonths := findCommonMonths(oldMonths, newMonths)

	if len(removedMonths) > 0 {
		if err := processRemovedMonths(db, updateData, removedMonths); err != nil {
			return fmt.Errorf("error processing removed months: %v", err)
		}
	}

	if len(addedMonths) > 0 {
		if err := processAddedMonths(db, updateData, addedMonths); err != nil {
			return fmt.Errorf("error processing added months: %v", err)
		}
	}

	if len(remainingMonths) > 0 && updateData.OldAmount != updateData.NewAmount {
		amountDiff := updateData.NewAmount - updateData.OldAmount
		if err := processRemainingMonthsAmountChange(db, updateData, remainingMonths, amountDiff); err != nil {
			return fmt.Errorf("error processing amount changes: %v", err)
		}
	}

	return nil
}

// calculateMonthsFromDuration calcula meses afectados por duración
func calculateMonthsFromDuration(startDate string, durationMonths int) ([]string, error) {
	var parsedDate time.Time
	var err error

	// Intentar parsear diferentes formatos de fecha
	if strings.Contains(startDate, "T") {
		// Formato ISO con tiempo: "2025-01-15T00:00:00Z"
		parsedDate, err = time.Parse("2006-01-02T15:04:05Z", startDate)
	} else {
		// Formato solo fecha: "2025-01-15"
		parsedDate, err = time.Parse("2006-01-02", startDate)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid start date %s: %v", startDate, err)
	}

	var months []string
	for i := 0; i < durationMonths; i++ {
		monthDate := parsedDate.AddDate(0, i, 0)
		months = append(months, monthDate.Format("2006-01"))
	}
	return months, nil
}

// findDifferenceMonths encuentra meses que están en list1 pero no en list2
func findDifferenceMonths(list1, list2 []string) []string {
	list2Map := make(map[string]bool)
	for _, month := range list2 {
		list2Map[month] = true
	}

	var result []string
	for _, month := range list1 {
		if !list2Map[month] {
			result = append(result, month)
		}
	}
	return result
}

// findCommonMonths encuentra meses comunes entre dos listas
func findCommonMonths(list1, list2 []string) []string {
	list1Map := make(map[string]bool)
	for _, month := range list1 {
		list1Map[month] = true
	}

	var result []string
	for _, month := range list2 {
		if list1Map[month] {
			result = append(result, month)
		}
	}
	return result
}

// processRemovedMonths maneja meses eliminados
func processRemovedMonths(db *sql.DB, updateData BillUpdateData, removedMonths []string) error {
	expenseMonths := getExpenseMonths(db, updateData.BillID, updateData.UserID)

	for _, month := range removedMonths {
		if expenseMonths[month] {
			subtractExpenseAmountForBill(db, updateData.BillID, updateData.UserID, month, updateData.OldAmount)
			updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "expense", -1)
		} else {
			updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "bill", -1)
		}
		updateBalanceColumns(db, updateData.UserID, month, updateData.OldAmount, updateData.OldPaymentMethod, "main", 1)
	}
	return nil
}

// processAddedMonths maneja meses añadidos
func processAddedMonths(db *sql.DB, updateData BillUpdateData, addedMonths []string) error {
	for _, month := range addedMonths {
		db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", updateData.UserID, month)
		updateBalanceColumns(db, updateData.UserID, month, updateData.NewAmount, updateData.NewPaymentMethod, "bill", 1)
		updateBalanceColumns(db, updateData.UserID, month, updateData.NewAmount, updateData.NewPaymentMethod, "main", -1)
	}
	return nil
}

// processRemainingMonthsAmountChange maneja cambios de importe en meses existentes
func processRemainingMonthsAmountChange(db *sql.DB, updateData BillUpdateData, remainingMonths []string, amountDiff float64) error {
	expenseMonths := getExpenseMonths(db, updateData.BillID, updateData.UserID)

	for _, month := range remainingMonths {
		if expenseMonths[month] {
			updateExpenseAmountForBillDifference(db, updateData.BillID, updateData.UserID, month, amountDiff)
			updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "expense", 1)
		} else {
			updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "bill", 1)
		}
		updateBalanceColumns(db, updateData.UserID, month, amountDiff, updateData.NewPaymentMethod, "main", -1)
	}
	return nil
}

// getExpenseMonths obtiene meses con expenses para un bill
func getExpenseMonths(db *sql.DB, billID int, userID string) map[string]bool {
	expenseMonths := make(map[string]bool)
	rows, err := db.Query("SELECT DISTINCT strftime('%Y-%m', date) FROM expenses WHERE bill_id = ? AND user_id = ?", billID, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				expenseMonths[month] = true
			}
		}
	}
	return expenseMonths
}

// updateBalanceColumns actualiza columnas de balance de forma unificada
func updateBalanceColumns(db *sql.DB, userID, month string, amount float64, paymentMethod, columnType string, multiplier int) {
	var column string
	finalAmount := amount * float64(multiplier)

	switch columnType {
	case "bill":
		if paymentMethod == "cash" {
			column = "bill_cash_amount"
		} else {
			column = "bill_bank_amount"
		}
	case "expense":
		if paymentMethod == "cash" {
			column = "expense_cash_amount"
		} else {
			column = "expense_bank_amount"
		}
	case "main":
		if paymentMethod == "cash" {
			column = "cash_amount"
		} else {
			column = "bank_amount"
		}
	default:
		return
	}

	query := fmt.Sprintf("UPDATE monthly_cash_bank_balance SET %s = %s + ?, total_balance = bank_amount + cash_amount WHERE user_id = ? AND year_month = ?", column, column)
	if _, err := db.Exec(query, finalAmount, userID, month); err != nil {
		log.Printf("Error updating %s: %v", column, err)
	}
}

// subtractExpenseAmountForBill resta amount de la tabla expenses
func subtractExpenseAmountForBill(db *sql.DB, billID int, userID, month string, amount float64) {
	db.Exec("UPDATE expenses SET amount = amount - ? WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", amount, billID, userID, month)
}

// updateExpenseAmountForBillDifference actualiza amount en expenses con diferencia
func updateExpenseAmountForBillDifference(db *sql.DB, billID int, userID, month string, amountDiff float64) {
	db.Exec("UPDATE expenses SET amount = amount + ? WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", amountDiff, billID, userID, month)
}

// addNewBillToMonthlyBalance agrega nuevo bill a monthly_cash_bank_balance
func addNewBillToMonthlyBalance(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	log.Printf("🔥 DEBUG: addNewBillToMonthlyBalance llamada con userID=%s, amount=%.2f, durationMonths=%d, paymentMethod=%s", userID, amount, durationMonths, paymentMethod)

	// NUEVA LÓGICA: Usar acumulación en cascada para facturas con duración
	if durationMonths > 1 {
		log.Printf("🔄 Aplicando lógica de acumulación en cascada para factura de %.2f desde %s durante %d meses", amount, startDate, durationMonths)
		result := updateCascadeBillBalance(db, userID, startDate, durationMonths, amount, paymentMethod)
		log.Printf("🔥 DEBUG: updateCascadeBillBalance terminó con resultado: %v", result)
		return result
	}

	log.Printf("🔥 DEBUG: Aplicando lógica simple para factura de un mes")

	// LÓGICA SIMPLE: Para facturas de un solo mes (duración = 1)
	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		return err
	}

	for _, month := range months {
		db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", userID, month)

		// Registrar el importe de la factura
		updateBalanceColumns(db, userID, month, amount, paymentMethod, "bill", 1)

		// Restar del saldo principal (dinero comprometido)
		updateBalanceColumns(db, userID, month, amount, paymentMethod, "main", -1)

		// CORREGIDO: Actualizar balance_cash_amount/balance_bank_amount manualmente
		if paymentMethod == "bank" {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET balance_bank_amount = bank_amount, 
				    total_balance = cash_amount + bank_amount
				WHERE user_id = ? AND year_month = ?
			`, userID, month)
		} else {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET balance_cash_amount = cash_amount, 
				    total_balance = cash_amount + bank_amount
				WHERE user_id = ? AND year_month = ?
			`, userID, month)
		}

		if err != nil {
			log.Printf("Error actualizando balance para mes %s: %v", month, err)
		}

		// Actualizar previous_amounts y total_previous_balance de meses posteriores
		updatePreviousBalancesFromMonth(db, userID, month, -amount, paymentMethod)
	}

	return nil
}

// updateMainBalanceColumnsForNewBill actualiza columnas principales para nuevo bill
func updateMainBalanceColumnsForNewBill(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		return err
	}

	for _, month := range months {
		updateBalanceColumns(db, userID, month, amount, paymentMethod, "main", -1)
	}
	return nil
}

// findEarliestMonth encuentra el mes más temprano
func findEarliestMonth(months []string) string {
	if len(months) == 0 {
		return ""
	}

	earliest := months[0]
	for _, month := range months[1:] {
		if month < earliest {
			earliest = month
		}
	}
	return earliest
}

// updatePreviousBalancesFromMonth actualiza balances previos en cascada
func updatePreviousBalancesFromMonth(db *sql.DB, userID, startMonth string, amountDiff float64, paymentMethod string) error {
	// Obtener meses posteriores al mes de inicio (excluyendo el mes de inicio)
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", userID, startMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	var posteriorMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			posteriorMonths = append(posteriorMonths, month)
		}
	}

	// Actualizar previous_amounts y total_previous_balance para meses posteriores
	for _, month := range posteriorMonths {
		if paymentMethod == "cash" {
			query := "UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?"
			db.Exec(query, amountDiff, amountDiff, userID, month)
		} else {
			query := "UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount + ?, total_previous_balance = total_previous_balance + ? WHERE user_id = ? AND year_month = ?"
			db.Exec(query, amountDiff, amountDiff, userID, month)
		}

		// Recalcular total_balance para el mes
		db.Exec("UPDATE monthly_cash_bank_balance SET total_balance = cash_amount + bank_amount WHERE user_id = ? AND year_month = ?", userID, month)
	}

	return nil
}
