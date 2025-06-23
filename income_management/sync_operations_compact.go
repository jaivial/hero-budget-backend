package main

import (
	"database/sql"
	"fmt"
	"time"
)

// syncDailyBalance sincroniza el balance diario para una fecha específica
func syncDailyBalance(userID, dateStr string) error {
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Get all incomes for the specific date
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND date = ?
	`
	
	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, dateStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update daily balance with calculated values
	checkQuery := `SELECT 1 FROM daily_balances WHERE user_id = ? AND date = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, dateStr).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO daily_balances (user_id, date, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, dateStr, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE daily_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, dateStr)
	}

	return err
}

// syncWeeklyBalance sincroniza el balance semanal para una fecha específica
func syncWeeklyBalance(userID, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Calculate week boundaries
	year, week := date.ISOWeek()
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7
	}
	startOfWeek := date.AddDate(0, 0, -(dayOfWeek-1))
	endOfWeek := startOfWeek.AddDate(0, 0, 6)

	weekStart := startOfWeek.Format("2006-01-02")
	weekEnd := endOfWeek.Format("2006-01-02")

	// Get all incomes for the week
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`
	
	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, weekStart, weekEnd).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update weekly balance
	checkQuery := `SELECT 1 FROM weekly_balances WHERE user_id = ? AND week_start = ? AND week_end = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, weekStart, weekEnd).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO weekly_balances (user_id, year, week_number, week_start, week_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, week, weekStart, weekEnd, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE weekly_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND week_start = ? AND week_end = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, weekStart, weekEnd)
	}

	return err
}

// syncMonthlyBalance sincroniza el balance mensual para una fecha específica
func syncMonthlyBalance(userID, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	month := date.Format("2006-01")

	// Get all incomes for the month
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND substr(date, 1, 7) = ?
	`
	
	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, month).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update monthly balance
	checkQuery := `SELECT 1 FROM monthly_balances WHERE user_id = ? AND month = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, month).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO monthly_balances (user_id, month, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, month, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE monthly_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, month)
	}

	return err
}