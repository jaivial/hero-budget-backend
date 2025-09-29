package main

import (
	"database/sql"
)

// getCashBankBalance obtiene el balance cash/bank para un mes específico
func getCashBankBalance(userID, month string) (interface{}, error) {
	query := `
		SELECT cash_amount, cash_percent, bank_amount, bank_percent, monthly_total
		FROM cash_bank
		WHERE user_id = ? AND month = ?
	`

	var balance struct {
		CashAmount   float64 `json:"cash_amount"`
		CashPercent  float64 `json:"cash_percent"`
		BankAmount   float64 `json:"bank_amount"`
		BankPercent  float64 `json:"bank_percent"`
		MonthlyTotal float64 `json:"monthly_total"`
	}

	err := db.QueryRow(query, userID, month).Scan(
		&balance.CashAmount,
		&balance.CashPercent,
		&balance.BankAmount,
		&balance.BankPercent,
		&balance.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		return balance, nil
	}

	return balance, err
}

// getDailyBalance obtiene el balance diario para una fecha específica
func getDailyBalance(userID, date string) (interface{}, error) {
	query := `
		SELECT income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM daily_balances
		WHERE user_id = ? AND date = ?
	`

	var balance struct {
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err := db.QueryRow(query, userID, date).Scan(
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		return balance, nil
	}

	return balance, err
}

// getWeeklyBalance obtiene el balance semanal para una fecha específica
func getWeeklyBalance(userID, date string) (interface{}, error) {
	query := `
		SELECT week_start, week_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM weekly_balances
		WHERE user_id = ? AND ? BETWEEN week_start AND week_end
	`

	var balance struct {
		WeekStart     string  `json:"week_start"`
		WeekEnd       string  `json:"week_end"`
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err := db.QueryRow(query, userID, date).Scan(
		&balance.WeekStart,
		&balance.WeekEnd,
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		return balance, nil
	}

	return balance, err
}

// getMonthlyBalance obtiene el balance mensual para una fecha específica
func getMonthlyBalance(userID, date string) (interface{}, error) {
	query := `
		SELECT month, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM monthly_balances
		WHERE user_id = ? AND month = substr(?, 1, 7)
	`

	var balance struct {
		Month         string  `json:"month"`
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err := db.QueryRow(query, userID, date).Scan(
		&balance.Month,
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		return balance, nil
	}

	return balance, err
}

// updateCashBankBalance actualiza el balance cash/bank para un mes específico
func updateCashBankBalance(userID, month string, cashAmount, bankAmount float64) error {
	// Calculate monthly total and percentages
	monthlyTotal := cashAmount + bankAmount
	var cashPercent, bankPercent float64

	if monthlyTotal > 0 {
		cashPercent = (cashAmount / monthlyTotal) * 100
		bankPercent = (bankAmount / monthlyTotal) * 100
	}

	// Check if record exists
	var exists bool
	checkQuery := `SELECT 1 FROM cash_bank WHERE user_id = ? AND month = ?`
	err := db.QueryRow(checkQuery, userID, month).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, month, cashAmount, cashPercent, bankAmount, bankPercent, monthlyTotal)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE cash_bank
			SET cash_amount = ?, cash_percent = ?, bank_amount = ?, bank_percent = ?, monthly_total = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(updateQuery, cashAmount, cashPercent, bankAmount, bankPercent, monthlyTotal, userID, month)
	}

	return err
}
