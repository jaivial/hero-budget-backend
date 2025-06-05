package common

import (
	"database/sql"
	"fmt"
	"time"
)

// UpdateCascadeBalances recalcula los saldos en cascada desde startMonth
func UpdateCascadeBalances(db *sql.DB, userID string, startMonth string) error {
	// Obtener todos los meses posteriores o iguales a startMonth
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months: %v", err)
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return fmt.Errorf("error scanning month: %v", err)
		}
		months = append(months, month)
	}

	for i, month := range months {
		// Obtener el mes anterior (si existe)
		var previousMonth string
		if i > 0 {
			previousMonth = months[i-1]
		} else if month != startMonth {
			row := db.QueryRow(`
				SELECT year_month FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month < ? ORDER BY year_month DESC LIMIT 1
			`, userID, month)
			if err := row.Scan(&previousMonth); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous month: %v", err)
			}
		}

		// Obtener saldos previos
		var previousCashAmount, previousBankAmount, totalPreviousBalance float64
		if previousMonth != "" {
			err := db.QueryRow(`
				SELECT cash_amount, bank_amount, total_balance
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, previousMonth).Scan(&previousCashAmount, &previousBankAmount, &totalPreviousBalance)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous balances: %v", err)
			}
		}

		// Obtener movimientos del mes actual
		var incomeCash, incomeBank, expenseCash, expenseBank, billCash, billBank float64
		err := db.QueryRow(`
			SELECT income_cash_amount, income_bank_amount,
			       expense_cash_amount, expense_bank_amount,
			       bill_cash_amount, bill_bank_amount
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`, userID, month).Scan(&incomeCash, &incomeBank, &expenseCash, &expenseBank, &billCash, &billBank)
		if err != nil {
			return fmt.Errorf("error fetching current month data: %v", err)
		}

		// Calcular saldos del mes actual
		cashAmount := previousCashAmount + incomeCash - expenseCash - billCash
		bankAmount := previousBankAmount + incomeBank - expenseBank - billBank
		balanceCashAmount := cashAmount
		balanceBankAmount := bankAmount
		totalBalance := balanceCashAmount + balanceBankAmount

		// Actualizar registro
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance
			SET cash_amount = ?,
			    bank_amount = ?,
			    balance_cash_amount = ?,
			    balance_bank_amount = ?,
			    total_balance = ?,
			    previous_cash_amount = ?,
			    previous_bank_amount = ?,
			    total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, cashAmount, bankAmount, balanceCashAmount, balanceBankAmount,
			totalBalance, previousCashAmount, previousBankAmount, totalPreviousBalance,
			userID, month)
		if err != nil {
			return fmt.Errorf("error updating balance for month %s: %v", month, err)
		}
	}

	return nil
}

// AddIncome registra un ingreso y actualiza los saldos
func AddIncome(db *sql.DB, userID string, amount float64, date, paymentMethod, category, description string) error {
	if amount <= 0 || (paymentMethod != "cash" && paymentMethod != "bank") {
		return fmt.Errorf("invalid income data")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}
	month := parsedDate.Format("2006-01")

	// Registrar ingreso
	_, err = tx.Exec(`
		INSERT INTO incomes (user_id, amount, date, payment_method, category, description)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, amount, date, paymentMethod, category, description)
	if err != nil {
		return fmt.Errorf("error inserting income: %v", err)
	}

	// Crear o actualizar registro mensual
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
		VALUES (?, ?)
	`, userID, month)
	if err != nil {
		return fmt.Errorf("error creating monthly record: %v", err)
	}

	// Actualizar ingresos
	if paymentMethod == "cash" {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET income_cash_amount = income_cash_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, month)
	} else {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET income_bank_amount = income_bank_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, month)
	}
	if err != nil {
		return fmt.Errorf("error updating income amount: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	// Recalcular saldos en cascada
	return UpdateCascadeBalances(db, userID, month)
}

// AddExpense registra un gasto y actualiza los saldos
func AddExpense(db *sql.DB, userID string, amount float64, date, paymentMethod, category, description string) error {
	if amount <= 0 || (paymentMethod != "cash" && paymentMethod != "bank") {
		return fmt.Errorf("invalid expense data")
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}
	month := parsedDate.Format("2006-01")

	// Registrar gasto
	_, err = tx.Exec(`
		INSERT INTO expenses (user_id, amount, date, payment_method, category, description)
		VALUES (?, ?, ?, ?, ?, ?)
	`, userID, amount, date, paymentMethod, category, description)
	if err != nil {
		return fmt.Errorf("error inserting expense: %v", err)
	}

	// Crear o actualizar registro mensual
	_, err = tx.Exec(`
		INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
		VALUES (?, ?)
	`, userID, month)
	if err != nil {
		return fmt.Errorf("error creating monthly record: %v", err)
	}

	// Actualizar gastos
	if paymentMethod == "cash" {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET expense_cash_amount = expense_cash_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, month)
	} else {
		_, err = tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET expense_bank_amount = expense_bank_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, userID, month)
	}
	if err != nil {
		return fmt.Errorf("error updating expense amount: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %v", err)
	}

	// Recalcular saldos en cascada
	return UpdateCascadeBalances(db, userID, month)
}
