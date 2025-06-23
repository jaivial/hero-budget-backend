package main

import (
	"database/sql"
	"fmt"
	"time"
)

// getQuarterlyBalance obtiene el balance trimestral para una fecha específica
func getQuarterlyBalance(userID, date string) (interface{}, error) {
	// Parse date to determine quarter
	dateObj, err := parseDate(date)
	if err != nil {
		return nil, err
	}

	year := dateObj.Year()
	quarter := ((dateObj.Month() - 1) / 3) + 1
	quarterStr := formatQuarter(year, int(quarter))

	query := `
		SELECT quarter, quarter_start, quarter_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM quarterly_balances
		WHERE user_id = ? AND quarter = ?
	`

	var balance struct {
		Quarter       string  `json:"quarter"`
		QuarterStart  string  `json:"quarter_start"`
		QuarterEnd    string  `json:"quarter_end"`
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err = db.QueryRow(query, userID, quarterStr).Scan(
		&balance.Quarter,
		&balance.QuarterStart,
		&balance.QuarterEnd,
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		balance.Quarter = quarterStr
		return balance, nil
	}

	return balance, err
}

// getSemiannualBalance obtiene el balance semestral para una fecha específica
func getSemiannualBalance(userID, date string) (interface{}, error) {
	// Parse date to determine semester
	dateObj, err := parseDate(date)
	if err != nil {
		return nil, err
	}

	year := dateObj.Year()
	semester := 1
	if dateObj.Month() > 6 {
		semester = 2
	}
	semesterStr := formatSemester(year, semester)

	query := `
		SELECT semester, semester_start, semester_end, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM semiannual_balances
		WHERE user_id = ? AND semester = ?
	`

	var balance struct {
		Semester      string  `json:"semester"`
		SemesterStart string  `json:"semester_start"`
		SemesterEnd   string  `json:"semester_end"`
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err = db.QueryRow(query, userID, semesterStr).Scan(
		&balance.Semester,
		&balance.SemesterStart,
		&balance.SemesterEnd,
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		balance.Semester = semesterStr
		return balance, nil
	}

	return balance, err
}

// getAnnualBalance obtiene el balance anual para una fecha específica
func getAnnualBalance(userID, date string) (interface{}, error) {
	// Parse date to determine year
	dateObj, err := parseDate(date)
	if err != nil {
		return nil, err
	}

	year := dateObj.Year()

	query := `
		SELECT year, income_amount, expense_amount, bills_amount, net_balance, cash_amount, bank_amount
		FROM annual_balances
		WHERE user_id = ? AND year = ?
	`

	var balance struct {
		Year          int     `json:"year"`
		IncomeAmount  float64 `json:"income_amount"`
		ExpenseAmount float64 `json:"expense_amount"`
		BillsAmount   float64 `json:"bills_amount"`
		NetBalance    float64 `json:"net_balance"`
		CashAmount    float64 `json:"cash_amount"`
		BankAmount    float64 `json:"bank_amount"`
	}

	err = db.QueryRow(query, userID, year).Scan(
		&balance.Year,
		&balance.IncomeAmount,
		&balance.ExpenseAmount,
		&balance.BillsAmount,
		&balance.NetBalance,
		&balance.CashAmount,
		&balance.BankAmount,
	)

	if err == sql.ErrNoRows {
		// Return zero balance if no record found
		balance.Year = year
		return balance, nil
	}

	return balance, err
}

// Helper functions for date parsing and formatting

// parseDate parsea una fecha en formato YYYY-MM-DD
func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

// formatQuarter formatea el trimestre en formato YYYY-QX
func formatQuarter(year, quarter int) string {
	return fmt.Sprintf("%d-Q%d", year, quarter)
}

// formatSemester formatea el semestre en formato YYYY-SX
func formatSemester(year, semester int) string {
	return fmt.Sprintf("%d-S%d", year, semester)
}