package common

import (
	"database/sql"
	"fmt"
	"time"
)

// AddBill registra una factura y sus pagos mensuales
func AddBill(db *sql.DB, userID, name string, amount float64, dueDate string, paymentDay, durationMonths int, paymentMethod, category, icon, regularity string) (int, error) {
	if amount <= 0 || durationMonths < 1 || paymentDay < 1 || paymentDay > 28 || (paymentMethod != "cash" && paymentMethod != "bank") {
		return 0, fmt.Errorf("invalid bill data")
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	startDate := dueDate // Asumimos que due_date es la fecha de inicio

	// Registrar factura
	result, err := tx.Exec(`
		INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, name, amount, dueDate, false, false, 0, true, category, icon, startDate, paymentDay, durationMonths, regularity, paymentMethod)
	if err != nil {
		return 0, fmt.Errorf("error inserting bill: %v", err)
	}

	billID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("error getting bill ID: %v", err)
	}

	// Calcular meses afectados
	currentDate, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, fmt.Errorf("invalid start date format: %v", err)
	}

	for i := 0; i < durationMonths; i++ {
		monthDate := currentDate.AddDate(0, i, 0)
		month := monthDate.Format("2006-01")

		// Crear registro en bill_payments
		_, err = tx.Exec(`
			INSERT INTO bill_payments (bill_id, user_id, year_month, paid, payment_date, payment_method)
			VALUES (?, ?, ?, ?, ?, ?)
		`, billID, userID, month, false, nil, paymentMethod)
		if err != nil {
			return 0, fmt.Errorf("error creating bill payment record: %v", err)
		}

		// Crear o actualizar registro mensual
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
			VALUES (?, ?)
		`, userID, month)
		if err != nil {
			return 0, fmt.Errorf("error creating monthly record: %v", err)
		}

		// Registrar el importe de la factura para este mes
		if paymentMethod == "cash" {
			_, err = tx.Exec(`
				UPDATE monthly_cash_bank_balance
				SET bill_cash_amount = bill_cash_amount + ?
				WHERE user_id = ? AND year_month = ?
			`, amount, userID, month)
		} else {
			_, err = tx.Exec(`
				UPDATE monthly_cash_bank_balance
				SET bill_bank_amount = bill_bank_amount + ?
				WHERE user_id = ? AND year_month = ?
			`, amount, userID, month)
		}
		if err != nil {
			return 0, fmt.Errorf("error updating bill amount for month %s: %v", month, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("error committing transaction: %v", err)
	}

	// Recalcular saldos en cascada desde el primer mes afectado
	firstMonth := currentDate.Format("2006-01")
	if err = UpdateCascadeBalances(db, userID, firstMonth); err != nil {
		return int(billID), fmt.Errorf("error updating cascade balances: %v", err)
	}

	return int(billID), nil
}

// MarkBillPaid marca una factura como pagada para un mes específico
func MarkBillPaid(db *sql.DB, billID int, userID, yearMonth string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Obtener datos de la factura
	var amount float64
	var paymentMethod string
	err = tx.QueryRow(`
		SELECT amount, payment_method
		FROM bills WHERE id = ? AND user_id = ?
	`, billID, userID).Scan(&amount, &paymentMethod)
	if err != nil {
		return fmt.Errorf("bill not found: %v", err)
	}

	// Verificar que el pago existe y no está pagado
	var alreadyPaid bool
	err = tx.QueryRow(`
		SELECT paid FROM bill_payments 
		WHERE bill_id = ? AND year_month = ?
	`, billID, yearMonth).Scan(&alreadyPaid)
	if err != nil {
		return fmt.Errorf("payment record not found: %v", err)
	}
	if alreadyPaid {
		return fmt.Errorf("bill for this month is already paid")
	}

	// Marcar pago como pagado
	paymentDate := time.Now().Format("2006-01-02")
	_, err = tx.Exec(`
		UPDATE bill_payments
		SET paid = 1, payment_date = ?
		WHERE bill_id = ? AND year_month = ?
	`, paymentDate, billID, yearMonth)
	if err != nil {
		return fmt.Errorf("error marking payment as paid: %v", err)
	}

	// IMPORTANTE: NO cambiar de bill_amount a expense_amount
	// Solo restar el bill_amount para este mes específico
	if paymentMethod == "cash" {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET bill_cash_amount = bill_cash_amount - ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, yearMonth)
	} else {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET bill_bank_amount = bill_bank_amount - ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, yearMonth)
	}
	if err != nil {
		return fmt.Errorf("error updating bill amount: %v", err)
	}

	// Verificar si todos los pagos están completados
	var totalPayments, paidPayments int
	err = tx.QueryRow(`
		SELECT COUNT(*) as total, SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END) as paid_count
		FROM bill_payments WHERE bill_id = ?
	`, billID).Scan(&totalPayments, &paidPayments)
	if err != nil {
		return fmt.Errorf("error checking bill completion: %v", err)
	}

	// Si todos los pagos están completados, marcar la factura como pagada
	if totalPayments > 0 && paidPayments >= totalPayments {
		_, err = tx.Exec(`
			UPDATE bills SET paid = 1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = ? AND user_id = ?
		`, billID, userID)
		if err != nil {
			return fmt.Errorf("error updating bill status: %v", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	// Recalcular saldos en cascada
	return UpdateCascadeBalances(db, userID, yearMonth)
}

// GetMonthlyCashBankBalance obtiene el balance mensual para un usuario y mes específico
func GetMonthlyCashBankBalance(db *sql.DB, userID, yearMonth string) (map[string]interface{}, error) {
	var balance map[string]interface{}

	row := db.QueryRow(`
		SELECT income_cash_amount, income_bank_amount,
		       expense_cash_amount, expense_bank_amount,
		       bill_cash_amount, bill_bank_amount,
		       cash_amount, bank_amount,
		       previous_cash_amount, previous_bank_amount,
		       balance_cash_amount, balance_bank_amount,
		       total_previous_balance, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
	`, userID, yearMonth)

	var incomeCash, incomeBank, expenseCash, expenseBank, billCash, billBank float64
	var cashAmount, bankAmount, prevCash, prevBank, balanceCash, balanceBank float64
	var totalPrev, totalBalance float64

	err := row.Scan(&incomeCash, &incomeBank, &expenseCash, &expenseBank,
		&billCash, &billBank, &cashAmount, &bankAmount, &prevCash, &prevBank,
		&balanceCash, &balanceBank, &totalPrev, &totalBalance)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no balance found for user %s in month %s", userID, yearMonth)
		}
		return nil, fmt.Errorf("error fetching balance: %v", err)
	}

	balance = map[string]interface{}{
		"user_id":                userID,
		"year_month":             yearMonth,
		"income_cash_amount":     incomeCash,
		"income_bank_amount":     incomeBank,
		"expense_cash_amount":    expenseCash,
		"expense_bank_amount":    expenseBank,
		"bill_cash_amount":       billCash,
		"bill_bank_amount":       billBank,
		"cash_amount":            cashAmount,
		"bank_amount":            bankAmount,
		"previous_cash_amount":   prevCash,
		"previous_bank_amount":   prevBank,
		"balance_cash_amount":    balanceCash,
		"balance_bank_amount":    balanceBank,
		"total_previous_balance": totalPrev,
		"total_balance":          totalBalance,
	}

	return balance, nil
}
