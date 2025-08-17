package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

func recalculateAllBalances(userID, transactionDate string, amount float64, paymentMethod, transactionType string, billID *int) error {
	log.Printf("Updating monthly balances for user %s after deleting %s transaction (amount: %.2f, method: %s, bill_id: %v)",
		userID, transactionType, amount, paymentMethod, billID)

	// Parse the transaction date
	date, err := time.Parse("2006-01-02", transactionDate[:10])
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}

	// Only update monthly balance for performance
	yearMonth := date.Format("2006-01")

	// Update the monthly balance
	err = updateMonthlyBalance(userID, yearMonth, amount, paymentMethod, transactionType)
	if err != nil {
		log.Printf("Error updating monthly balance: %v", err)
		return err
	}

	// Update subsequent months with cascade logic
	if strings.ToLower(transactionType) == "expense" && billID == nil {
		// For expenses without bill_id, apply cascade updates
		err = updateSubsequentMonthsForExpense(userID, date, amount, paymentMethod)
	} else if strings.ToLower(transactionType) == "income" {
		// For incomes, apply cascade updates
		err = updateSubsequentMonthsForIncome(userID, date, amount, paymentMethod)
	}
	if err != nil {
		log.Printf("Error updating subsequent months: %v", err)
	}

	return nil
}

func updateMonthlyBalance(userID, yearMonth string, amount float64, paymentMethod, transactionType string) error {
	// Check if the monthly record exists
	var exists bool
	checkQuery := `SELECT COUNT(*) > 0 FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err := db.QueryRow(checkQuery, userID, yearMonth).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking monthly balance existence: %v", err)
	}

	if !exists {
		log.Printf("Monthly balance record not found for %s, skipping balance update", yearMonth)
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

	// Build and execute the update query
	updateQuery := fmt.Sprintf("UPDATE monthly_cash_bank_balance SET %s WHERE user_id = ? AND year_month = ?",
		strings.Join(updates, ", "))

	// Add WHERE clause parameters
	params = append(params, userID, yearMonth)

	_, err = db.Exec(updateQuery, params...)
	if err != nil {
		return fmt.Errorf("error updating monthly balance: %v", err)
	}

	// Update total_balance separately
	totalBalanceQuery := `
		UPDATE monthly_cash_bank_balance 
		SET total_balance = COALESCE(total_previous_balance, 0) + (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND year_month = ?`

	_, err = db.Exec(totalBalanceQuery, userID, yearMonth)
	if err != nil {
		return fmt.Errorf("error updating total balance: %v", err)
	}

	log.Printf("Updated monthly balance for %s (amount change: %.2f %s, type: %s)",
		yearMonth, amount, paymentMethod, transactionType)

	return nil
}

// updateSubsequentMonthsForExpense updates all subsequent months for expense deletions with cascade logic
func updateSubsequentMonthsForExpense(userID string, transactionDate time.Time, amount float64, paymentMethod string) error {
	log.Printf("Updating subsequent months for expense with bill_id = null (cascade effect)")

	// Get all months after the transaction month
	var nextMonths []string
	for i := 1; i <= 24; i++ { // Look up to 24 months ahead
		nextDate := transactionDate.AddDate(0, i, 0)
		nextMonth := nextDate.Format("2006-01")
		nextMonths = append(nextMonths, nextMonth)
	}

	// Update each subsequent month that exists
	for _, nextMonth := range nextMonths {
		err := updateCascadeForMonth(userID, nextMonth, amount, paymentMethod, "expense")
		if err != nil {
			log.Printf("Error updating cascade for month %s: %v", nextMonth, err)
			// Continue with next month even if one fails
		}
	}

	return nil
}

// updateSubsequentMonthsForIncome updates all subsequent months for income deletions with cascade logic
func updateSubsequentMonthsForIncome(userID string, transactionDate time.Time, amount float64, paymentMethod string) error {
	log.Printf("Updating subsequent months for deleted income (cascade effect)")

	// Get all months after the transaction month
	var nextMonths []string
	for i := 1; i <= 24; i++ { // Look up to 24 months ahead
		nextDate := transactionDate.AddDate(0, i, 0)
		nextMonth := nextDate.Format("2006-01")
		nextMonths = append(nextMonths, nextMonth)
	}

	// Update each subsequent month that exists
	for _, nextMonth := range nextMonths {
		err := updateCascadeForMonth(userID, nextMonth, amount, paymentMethod, "income")
		if err != nil {
			log.Printf("Error updating cascade for income in month %s: %v", nextMonth, err)
			// Continue with next month even if one fails
		}
	}

	return nil
}

// updateCascadeForMonth updates a specific month with cascade logic
func updateCascadeForMonth(userID, month string, amount float64, paymentMethod, transactionType string) error {
	// Check if the month exists
	var exists bool
	checkQuery := `SELECT COUNT(*) > 0 FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err := db.QueryRow(checkQuery, userID, month).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking month existence: %v", err)
	}

	if !exists {
		// If month doesn't exist, do nothing (gap between months)
		return nil
	}

	// Prepare updates based on transaction type and payment method
	var updates []string
	var params []interface{}

	if transactionType == "expense" {
		if paymentMethod == "bank" {
			updates = append(updates, "bank_amount = bank_amount + ?")
			updates = append(updates, "balance_bank_amount = balance_bank_amount + ?")
			updates = append(updates, "total_previous_balance = total_previous_balance + ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "cash_amount = cash_amount + ?")
			updates = append(updates, "balance_cash_amount = balance_cash_amount + ?")
			updates = append(updates, "total_previous_balance = total_previous_balance + ?")
			params = append(params, amount, amount, amount)
		}
	} else if transactionType == "income" {
		if paymentMethod == "bank" {
			updates = append(updates, "bank_amount = bank_amount - ?")
			updates = append(updates, "balance_bank_amount = balance_bank_amount - ?")
			updates = append(updates, "total_previous_balance = total_previous_balance - ?")
			params = append(params, amount, amount, amount)
		} else { // cash
			updates = append(updates, "cash_amount = cash_amount - ?")
			updates = append(updates, "balance_cash_amount = balance_cash_amount - ?")
			updates = append(updates, "total_previous_balance = total_previous_balance - ?")
			params = append(params, amount, amount, amount)
		}
	}

	updates = append(updates, "updated_at = CURRENT_TIMESTAMP")

	// Execute main update
	updateQuery := fmt.Sprintf("UPDATE monthly_cash_bank_balance SET %s WHERE user_id = ? AND year_month = ?",
		strings.Join(updates, ", "))
	params = append(params, userID, month)

	_, err = db.Exec(updateQuery, params...)
	if err != nil {
		return fmt.Errorf("error updating cascade for month %s: %v", month, err)
	}

	// Update total_balance separately to use updated values
	totalBalanceQuery := `
		UPDATE monthly_cash_bank_balance 
		SET total_balance = total_previous_balance + (
			COALESCE(income_bank_amount, 0) + COALESCE(income_cash_amount, 0) - 
			COALESCE(expense_bank_amount, 0) - COALESCE(expense_cash_amount, 0) - 
			COALESCE(bill_bank_amount, 0) - COALESCE(bill_cash_amount, 0)
		)
		WHERE user_id = ? AND year_month = ?`

	_, err = db.Exec(totalBalanceQuery, userID, month)
	if err != nil {
		return fmt.Errorf("error updating total balance for month %s: %v", month, err)
	}

	log.Printf("Updated cascade for month %s: amount %.2f (%s, %s)", month, amount, paymentMethod, transactionType)
	return nil
}
