package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

func recalculateAllBalances(userID, transactionDate string, amount float64, paymentMethod, transactionType string, billID *int) error {
	log.Printf("Updating balances for user %s after deleting %s transaction (amount: %.2f, method: %s, bill_id: %v)",
		userID, transactionType, amount, paymentMethod, billID)

	// Parse the transaction date
	date, err := time.Parse("2006-01-02", transactionDate[:10])
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	// Define all period types
	periods := []struct {
		name      string
		tableName string
	}{
		{"daily", "daily_cash_bank_balance"},
		{"weekly", "weekly_cash_bank_balance"},
		{"monthly", "monthly_cash_bank_balance"},
		{"quarterly", "quarterly_cash_bank_balance"},
		{"semiannual", "semiannual_cash_bank_balance"},
		{"annual", "annual_cash_bank_balance"},
	}

	for _, periodInfo := range periods {
		// Calculate the period identifier for the transaction date
		periodIdentifier := calculatePeriodIdentifier(date, periodInfo.name)

		// Update the specific period balance
		err = updatePeriodBalance(userID, periodInfo.tableName, periodIdentifier, amount, paymentMethod, transactionType)
		if err != nil {
			log.Printf("Error updating %s balance: %v", periodInfo.name, err)
			continue
		}

		// Update subsequent periods' previous balances
		// Usar lógica específica para expenses con bill_id = NULL
		if strings.ToLower(transactionType) == "expense" {
			err = updateSubsequentPeriodsForExpense(userID, periodInfo.tableName, periodInfo.name, date, amount, paymentMethod, billID)
		} else if strings.ToLower(transactionType) == "income" {
			err = updateSubsequentPeriodsForIncome(userID, periodInfo.tableName, periodInfo.name, date, amount, paymentMethod)
		} else {
			err = updateSubsequentPeriods(userID, periodInfo.tableName, periodInfo.name, date)
		}
		if err != nil {
			log.Printf("Error updating subsequent %s periods: %v", periodInfo.name, err)
		}
	}

	return nil
}

func updatePeriodBalance(userID, tableName, periodIdentifier string, amount float64, paymentMethod, transactionType string) error {
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
	err := db.QueryRow(checkQuery, userID, periodIdentifier).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking period existence: %v", err)
	}

	if !exists {
		// If period doesn't exist, log and skip (this shouldn't happen for deletion)
		log.Printf("Period %s not found in %s for user %s", periodIdentifier, tableName, userID)
		return nil
	}

	// Determine which columns to update and calculate the changes
	var updates []string
	var params []interface{}

	switch strings.ToLower(transactionType) {
	case "income":
		if paymentMethod == "bank" {
			updates = append(updates, "income_bank_amount = income_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount - ?")
			updates = append(updates, "balance_bank_amount = balance_bank_amount - ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "income_cash_amount = income_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount - ?")
			updates = append(updates, "balance_cash_amount = balance_cash_amount - ?")
			params = append(params, amount, amount, amount)
		}
	case "expense":
		if paymentMethod == "bank" {
			updates = append(updates, "expense_bank_amount = expense_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount + ?") // Adding back the expense
			updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "expense_cash_amount = expense_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount + ?") // Adding back the expense
			updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
			params = append(params, amount, amount, amount)
		}
	case "bill":
		if paymentMethod == "bank" {
			updates = append(updates, "bill_bank_amount = bill_bank_amount - ?")
			updates = append(updates, "bank_amount = bank_amount + ?") // Adding back the bill
			updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "bill_cash_amount = bill_cash_amount - ?")
			updates = append(updates, "cash_amount = cash_amount + ?") // Adding back the bill
			updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
			params = append(params, amount, amount, amount)
		}
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	// Build and execute the first update query (without total_balance)
	updateQuery := fmt.Sprintf("UPDATE %s SET %s WHERE user_id = ? AND %s = ?",
		tableName, strings.Join(updates, ", "), periodColumn)

	// Add WHERE clause parameters
	params = append(params, userID, periodIdentifier)

	_, err = db.Exec(updateQuery, params...)
	if err != nil {
		return fmt.Errorf("error updating period balance: %v", err)
	}

	// Now update total_balance separately to ensure it uses the updated values
	totalBalanceQuery := fmt.Sprintf(`
		UPDATE %s 
		SET total_balance = (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND %s = ?`, tableName, periodColumn)

	_, err = db.Exec(totalBalanceQuery, userID, periodIdentifier)
	if err != nil {
		return fmt.Errorf("error updating total balance: %v", err)
	}

	log.Printf("Updated %s balance for period %s (amount change: %.2f %s, type: %s)",
		tableName, periodIdentifier, amount, paymentMethod, transactionType)

	return nil
}

func updateSubsequentPeriods(userID, tableName, periodType string, transactionDate time.Time) error {
	// Get all periods after the transaction date to update their previous balances
	// For simplicity, we'll recalculate previous amounts for periods after the deleted transaction
	// This ensures cascade effect is properly maintained

	var nextPeriods []string

	switch periodType {
	case "monthly":
		// Get next months
		for i := 1; i <= 12; i++ { // Check next 12 months
			nextDate := transactionDate.AddDate(0, i, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "quarterly":
		// Get next quarters
		for i := 1; i <= 4; i++ { // Check next 4 quarters
			nextDate := transactionDate.AddDate(0, i*3, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	case "annual":
		// Get next years
		for i := 1; i <= 5; i++ { // Check next 5 years
			nextDate := transactionDate.AddDate(i, 0, 0)
			nextPeriod := calculatePeriodIdentifier(nextDate, periodType)
			nextPeriods = append(nextPeriods, nextPeriod)
		}
	}

	// Update each subsequent period
	for _, nextPeriod := range nextPeriods {
		err := updatePreviousBalanceForPeriod(userID, tableName, nextPeriod, periodType)
		if err != nil {
			log.Printf("Error updating previous balance for period %s: %v", nextPeriod, err)
		}
	}

	return nil
}

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
