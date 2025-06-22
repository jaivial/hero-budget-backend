package main

import (
	"database/sql"
	"fmt"
	"log"
)

// addBillToCashBankBalance añade una nueva factura a monthly_cash_bank_balance
// NUEVO: Función específica para trabajar con monthly_cash_bank_balance
func addBillToCashBankBalance(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	log.Printf("🔄 Añadiendo bill a monthly_cash_bank_balance: user=%s, amount=%.2f, start=%s, duration=%d, method=%s", 
		userID, amount, startDate, durationMonths, paymentMethod)
	
	// Parsear fecha de inicio
	startTime, err := parseFlexibleDateCashBank(startDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}
	
	// Aplicar la factura a cada mes del periodo
	for i := 0; i < durationMonths; i++ {
		monthDate := startTime.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		
		// Asegurar que existe fila para el mes
		_, err := db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error creating monthly record for %s: %v", yearMonth, err)
		}
		
		// Sumar amount a bill_cash_amount o bill_bank_amount
		if paymentMethod == "cash" {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount + ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		} else {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount + ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		}
		if err != nil {
			return fmt.Errorf("error updating bill amount for %s: %v", yearMonth, err)
		}
		
		// Restar amount de cash_amount o bank_amount (comprometer dinero)
		if paymentMethod == "cash" {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET cash_amount = cash_amount - ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		} else {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET bank_amount = bank_amount - ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		}
		if err != nil {
			return fmt.Errorf("error updating available amount for %s: %v", yearMonth, err)
		}
		
		// Restar amount de balance_cash_amount o balance_bank_amount
		if paymentMethod == "cash" {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET balance_cash_amount = balance_cash_amount - ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		} else {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET balance_bank_amount = balance_bank_amount - ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		}
		if err != nil {
			return fmt.Errorf("error updating balance amount for %s: %v", yearMonth, err)
		}
	}
	
	// Actualizar previous_amounts en cascada para meses posteriores
	startMonth := startTime.Format("2006-01")
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", userID, startMonth)
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
		if paymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?", amount, amount, userID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?", amount, amount, userID, month)
		}
	}
	
	// Recalcular total_balance para todos los meses afectados
	allAffectedMonths := append([]string{}, subsequentMonths...)
	for i := 0; i < durationMonths; i++ {
		monthDate := startTime.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		allAffectedMonths = append(allAffectedMonths, yearMonth)
	}
	
	for _, month := range allAffectedMonths {
		db.Exec("UPDATE monthly_cash_bank_balance SET total_balance = cash_amount + bank_amount WHERE user_id = ? AND year_month = ?", userID, month)
	}
	
	log.Printf("✅ Bill añadido a monthly_cash_bank_balance correctamente")
	return nil
}

// getBillDataForCashBank obtiene datos de una factura para operaciones con cash bank
// Función auxiliar para obtener información necesaria para updates
func getBillDataForCashBank(db *sql.DB, billID int, userID string) (*Bill, error) {
	var bill Bill
	err := db.QueryRow(`
		SELECT id, user_id, name, amount, due_date, start_date, payment_day, duration_months, 
		       regularity, paid, overdue, overdue_days, recurring, category, icon, payment_method, 
		       created_at, updated_at
		FROM bills WHERE id = ? AND user_id = ?
	`, billID, userID).Scan(
		&bill.ID, &bill.UserID, &bill.Name, &bill.Amount, &bill.DueDate, &bill.StartDate,
		&bill.PaymentDay, &bill.DurationMonths, &bill.Regularity, &bill.Paid, &bill.Overdue,
		&bill.OverdueDays, &bill.Recurring, &bill.Category, &bill.Icon, &bill.PaymentMethod,
		&bill.CreatedAt, &bill.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}
	return &bill, nil
}

// updateBillInDatabaseCashBank actualiza los datos básicos de una factura
// Versión específica para operaciones con cash bank
func updateBillInDatabaseCashBank(db *sql.DB, updateRequest UpdateBillRequest) error {
	query := "UPDATE bills SET updated_at = CURRENT_TIMESTAMP"
	args := []interface{}{}
	
	if updateRequest.Name != "" {
		query += ", name = ?"
		args = append(args, updateRequest.Name)
	}
	if updateRequest.Amount > 0 {
		query += ", amount = ?"
		args = append(args, updateRequest.Amount)
	}
	if updateRequest.StartDate != "" {
		query += ", start_date = ?"
		args = append(args, updateRequest.StartDate)
	}
	if updateRequest.PaymentDay > 0 {
		query += ", payment_day = ?"
		args = append(args, updateRequest.PaymentDay)
	}
	if updateRequest.DurationMonths > 0 {
		query += ", duration_months = ?"
		args = append(args, updateRequest.DurationMonths)
	}
	if updateRequest.Regularity != "" {
		query += ", regularity = ?"
		args = append(args, updateRequest.Regularity)
	}
	if updateRequest.Category != "" {
		query += ", category = ?"
		args = append(args, updateRequest.Category)
	}
	if updateRequest.Icon != "" {
		query += ", icon = ?"
		args = append(args, updateRequest.Icon)
	}
	if updateRequest.PaymentMethod != "" {
		query += ", payment_method = ?"
		args = append(args, updateRequest.PaymentMethod)
	}
	
	query += " WHERE id = ? AND user_id = ?"
	args = append(args, updateRequest.BillID, updateRequest.UserID)
	
	_, err := db.Exec(query, args...)
	return err
}

// getCashBankBalanceForMonth obtiene el balance de monthly_cash_bank_balance para un mes
// Función auxiliar para verificar el estado de los balances
func getCashBankBalanceForMonth(db *sql.DB, userID, yearMonth string) (map[string]interface{}, error) {
	var result map[string]interface{}
	
	row := db.QueryRow(`
		SELECT income_bank_amount, income_cash_amount, expense_bank_amount, expense_cash_amount,
		       bill_bank_amount, bill_cash_amount, bank_amount, cash_amount,
		       previous_bank_amount, previous_cash_amount, balance_bank_amount, balance_cash_amount,
		       total_previous_balance, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
	`, userID, yearMonth)
	
	var incomeBank, incomeCash, expenseBank, expenseCash, billBank, billCash float64
	var bankAmount, cashAmount, prevBank, prevCash, balanceBank, balanceCash float64
	var totalPrev, totalBalance float64
	
	err := row.Scan(&incomeBank, &incomeCash, &expenseBank, &expenseCash,
		&billBank, &billCash, &bankAmount, &cashAmount,
		&prevBank, &prevCash, &balanceBank, &balanceCash,
		&totalPrev, &totalBalance)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no balance found for %s in %s", userID, yearMonth)
		}
		return nil, fmt.Errorf("error fetching balance: %v", err)
	}
	
	result = map[string]interface{}{
		"user_id":                userID,
		"year_month":             yearMonth,
		"income_bank_amount":     incomeBank,
		"income_cash_amount":     incomeCash,
		"expense_bank_amount":    expenseBank,
		"expense_cash_amount":    expenseCash,
		"bill_bank_amount":       billBank,
		"bill_cash_amount":       billCash,
		"bank_amount":            bankAmount,
		"cash_amount":            cashAmount,
		"previous_bank_amount":   prevBank,
		"previous_cash_amount":   prevCash,
		"balance_bank_amount":    balanceBank,
		"balance_cash_amount":    balanceCash,
		"total_previous_balance": totalPrev,
		"total_balance":          totalBalance,
	}
	
	return result, nil
}