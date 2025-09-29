package main

import (
	"database/sql"
	"fmt"
	"time"
)

// updateQuarterlyBalance actualiza el balance trimestral
func updateQuarterlyBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	year := date.Year()
	quarter := ((date.Month() - 1) / 3) + 1
	quarterStr := fmt.Sprintf("%d-Q%d", year, quarter)

	// Calculate quarter start and end dates
	quarterStart := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
	quarterEnd := quarterStart.AddDate(0, 3, -1) // Last day of the quarter

	quarterStartStr := quarterStart.Format("2006-01-02")
	quarterEndStr := quarterEnd.Format("2006-01-02")

	// Check if record exists
	var exists bool
	checkQuery := `SELECT 1 FROM quarterly_balances WHERE user_id = ? AND quarter = ?`
	err := db.QueryRow(checkQuery, userID, quarterStr).Scan(&exists)

	// Calculate net balance
	netBalance := incomeAmount - expenseAmount - billsAmount

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO quarterly_balances (user_id, year, quarter_number, quarter, quarter_start, quarter_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, quarter, quarterStr, quarterStartStr, quarterEndStr, incomeAmount, expenseAmount, billsAmount, netBalance, cashAmount, bankAmount)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE quarterly_balances
			SET income_amount = income_amount + ?, expense_amount = expense_amount + ?, bills_amount = bills_amount + ?,
				net_balance = income_amount - expense_amount - bills_amount,
				cash_amount = cash_amount + ?, bank_amount = bank_amount + ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND quarter = ?
		`
		_, err = db.Exec(updateQuery, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount, userID, quarterStr)
	}

	return err
}

// updateSemiannualBalance actualiza el balance semestral
func updateSemiannualBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	year := date.Year()
	semester := 1
	if date.Month() > 6 {
		semester = 2
	}
	semesterStr := fmt.Sprintf("%d-S%d", year, semester)

	// Calculate semester start and end dates
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

	// Check if record exists
	var exists bool
	checkQuery := `SELECT 1 FROM semiannual_balances WHERE user_id = ? AND semester = ?`
	err := db.QueryRow(checkQuery, userID, semesterStr).Scan(&exists)

	// Calculate net balance
	netBalance := incomeAmount - expenseAmount - billsAmount

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO semiannual_balances (user_id, year, semester_number, semester, semester_start, semester_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, semester, semesterStr, semesterStartStr, semesterEndStr, incomeAmount, expenseAmount, billsAmount, netBalance, cashAmount, bankAmount)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE semiannual_balances
			SET income_amount = income_amount + ?, expense_amount = expense_amount + ?, bills_amount = bills_amount + ?,
				net_balance = income_amount - expense_amount - bills_amount,
				cash_amount = cash_amount + ?, bank_amount = bank_amount + ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND semester = ?
		`
		_, err = db.Exec(updateQuery, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount, userID, semesterStr)
	}

	return err
}

// updateAnnualBalance actualiza el balance anual
func updateAnnualBalance(userID string, incomeAmount, expenseAmount, billsAmount float64, cashAmount, bankAmount float64, date time.Time) error {
	year := date.Year()

	// Check if record exists
	var exists bool
	checkQuery := `SELECT 1 FROM annual_balances WHERE user_id = ? AND year = ?`
	err := db.QueryRow(checkQuery, userID, year).Scan(&exists)

	// Calculate net balance
	netBalance := incomeAmount - expenseAmount - billsAmount

	if err == sql.ErrNoRows {
		// Insert new record
		insertQuery := `
			INSERT INTO annual_balances (user_id, year, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, year, incomeAmount, expenseAmount, billsAmount, netBalance, cashAmount, bankAmount)
	} else if err == nil {
		// Update existing record
		updateQuery := `
			UPDATE annual_balances
			SET income_amount = income_amount + ?, expense_amount = expense_amount + ?, bills_amount = bills_amount + ?,
				net_balance = income_amount - expense_amount - bills_amount,
				cash_amount = cash_amount + ?, bank_amount = bank_amount + ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year = ?
		`
		_, err = db.Exec(updateQuery, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount, userID, year)
	}

	return err
}
