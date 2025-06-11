package main

import (
	"fmt"
	"strings"
	"time"
)

func updatePreviousBalanceForPeriod(userID, tableName, period, periodType string) error {
	// Determine the correct column name for the period based on table name
	var periodColumn string
	switch {
	case strings.Contains(tableName, "daily"):
		periodColumn = "date"
	case strings.Contains(tableName, "weekly"):
		periodColumn = "year_week"
	case strings.Contains(tableName, "monthly"):
		periodColumn = "year_month"
	case strings.Contains(tableName, "quarterly"):
		periodColumn = "year_quarter"
	case strings.Contains(tableName, "semiannual"):
		periodColumn = "year_half"
	case strings.Contains(tableName, "annual"):
		periodColumn = "year"
	default:
		periodColumn = "period"
	}

	// Check if the period record exists
	var exists bool
	checkQuery := fmt.Sprintf(`SELECT COUNT(*) > 0 FROM %s WHERE user_id = ? AND %s = ?`, tableName, periodColumn)
	err := db.QueryRow(checkQuery, userID, period).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking period existence: %v", err)
	}

	if !exists {
		// If period doesn't exist, skip it
		return nil
	}

	// Calculate previous period identifier
	currentDate, err := parsePeriodIdentifier(period, periodType)
	if err != nil {
		return fmt.Errorf("error parsing period identifier: %v", err)
	}

	var previousDate time.Time
	switch periodType {
	case "monthly":
		previousDate = currentDate.AddDate(0, -1, 0)
	case "quarterly":
		previousDate = currentDate.AddDate(0, -3, 0)
	case "annual":
		previousDate = currentDate.AddDate(-1, 0, 0)
	default:
		return fmt.Errorf("unsupported period type: %s", periodType)
	}

	previousPeriod := calculatePeriodIdentifier(previousDate, periodType)

	// Get the total_balance from the previous period
	var previousTotalBalance float64
	var previousBankAmount, previousCashAmount float64

	previousQuery := fmt.Sprintf(`
		SELECT COALESCE(total_balance, 0), COALESCE(bank_amount, 0), COALESCE(cash_amount, 0)
		FROM %s 
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	err = db.QueryRow(previousQuery, userID, previousPeriod).Scan(&previousTotalBalance, &previousBankAmount, &previousCashAmount)
	if err != nil {
		// If previous period doesn't exist, use 0 as the previous balance
		previousTotalBalance = 0
		previousBankAmount = 0
		previousCashAmount = 0
	}

	// Update the current period's previous balance fields
	updateQuery := fmt.Sprintf(`
		UPDATE %s 
		SET total_previous_balance = ?, previous_bank_amount = ?, previous_cash_amount = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(updateQuery, previousTotalBalance, previousBankAmount, previousCashAmount, userID, period)
	if err != nil {
		return fmt.Errorf("error updating previous balance for period %s: %v", period, err)
	}

	// Recalculate total_balance for this period
	recalcQuery := fmt.Sprintf(`
		UPDATE %s 
		SET total_balance = total_previous_balance + (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(recalcQuery, userID, period)
	if err != nil {
		return fmt.Errorf("error recalculating total balance for period %s: %v", period, err)
	}

	return nil
}

func parsePeriodIdentifier(period, periodType string) (time.Time, error) {
	switch periodType {
	case "monthly":
		// Format: YYYY-MM
		return time.Parse("2006-01", period)
	case "quarterly":
		// Format: YYYY-QN (e.g., 2023-Q1)
		return time.Parse("2006-Q1", period)
	case "annual":
		// Format: YYYY
		return time.Parse("2006", period)
	default:
		return time.Time{}, fmt.Errorf("unsupported period type: %s", periodType)
	}
}

func calculatePeriodIdentifier(date time.Time, period string) string {
	switch period {
	case "daily":
		return date.Format("2006-01-02")
	case "weekly":
		year, week := date.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "monthly":
		return date.Format("2006-01")
	case "quarterly":
		quarter := (date.Month()-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", date.Year(), quarter)
	case "semiannual":
		half := (date.Month()-1)/6 + 1
		return fmt.Sprintf("%d-H%d", date.Year(), half)
	case "annual":
		return date.Format("2006")
	default:
		return date.Format("2006-01-02")
	}
}
