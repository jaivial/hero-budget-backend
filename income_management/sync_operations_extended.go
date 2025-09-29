package main

import (
	"database/sql"
	"fmt"
	"time"
)

// syncQuarterlyBalance sincroniza el balance trimestral para una fecha específica
func syncQuarterlyBalance(userID, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	year := date.Year()
	quarter := ((date.Month() - 1) / 3) + 1
	quarterStr := fmt.Sprintf("%d-Q%d", year, quarter)

	// Calculate quarter boundaries
	quarterStart := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
	quarterEnd := quarterStart.AddDate(0, 3, -1)
	quarterStartStr := quarterStart.Format("2006-01-02")
	quarterEndStr := quarterEnd.Format("2006-01-02")

	// Get all incomes for the quarter
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, quarterStartStr, quarterEndStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update quarterly balance
	checkQuery := `SELECT 1 FROM quarterly_balances WHERE user_id = ? AND quarter = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, quarterStr).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO quarterly_balances (user_id, year, quarter_number, quarter, quarter_start, quarter_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, quarter, quarterStr, quarterStartStr, quarterEndStr, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE quarterly_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND quarter = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, quarterStr)
	}

	return err
}

// syncSemiannualBalance sincroniza el balance semestral para una fecha específica
func syncSemiannualBalance(userID, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	year := date.Year()
	semester := 1
	if date.Month() > 6 {
		semester = 2
	}
	semesterStr := fmt.Sprintf("%d-S%d", year, semester)

	// Calculate semester boundaries
	var semesterStart, semesterEnd time.Time
	if semester == 1 {
		semesterStart = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		semesterEnd = time.Date(year, 6, 30, 0, 0, 0, 0, time.UTC)
	} else {
		semesterStart = time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC)
		semesterEnd = time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	semesterStartStr := semesterStart.Format("2006-01-02")
	semesterEndStr := semesterEnd.Format("2006-01-02")

	// Get all incomes for the semester
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, semesterStartStr, semesterEndStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update semiannual balance
	checkQuery := `SELECT 1 FROM semiannual_balances WHERE user_id = ? AND semester = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, semesterStr).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO semiannual_balances (user_id, year, semester_number, semester, semester_start, semester_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, semester, semesterStr, semesterStartStr, semesterEndStr, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE semiannual_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND semester = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, semesterStr)
	}

	return err
}

// syncAnnualBalance sincroniza el balance anual para una fecha específica
func syncAnnualBalance(userID, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	year := date.Year()
	yearStr := fmt.Sprintf("%d", year)

	// Get all incomes for the year
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND substr(date, 1, 4) = ?
	`

	var cashIncome, bankIncome, totalIncome float64
	err = db.QueryRow(query, userID, yearStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update annual balance
	checkQuery := `SELECT 1 FROM annual_balances WHERE user_id = ? AND year = ?`
	var exists bool
	err = db.QueryRow(checkQuery, userID, year).Scan(&exists)

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO annual_balances (user_id, year, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, 0, 0, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, totalIncome, totalIncome, cashIncome, bankIncome)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE annual_balances
			SET income_amount = ?, cash_amount = ?, bank_amount = ?,
				net_balance = income_amount - expense_amount - bills_amount,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`
		_, err = db.Exec(updateQuery, totalIncome, cashIncome, bankIncome, userID, year)
	}

	return err
}
