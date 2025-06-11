package main

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type TransactionDetails struct {
	ID            int     `json:"id"`
	UserID        string  `json:"user_id"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date"`
	PaymentMethod string  `json:"payment_method"`
	BillID        *int    `json:"bill_id,omitempty"`
}

func getTransactionDetails(transactionID int, transactionType, userID string) (*TransactionDetails, error) {
	var transaction TransactionDetails
	var query string

	switch strings.ToLower(transactionType) {
	case "expense":
		query = `SELECT id, user_id, amount, date, payment_method, bill_id FROM expenses WHERE id = ? AND user_id = ?`
		row := db.QueryRow(query, transactionID, userID)
		err := row.Scan(&transaction.ID, &transaction.UserID, &transaction.Amount,
			&transaction.Date, &transaction.PaymentMethod, &transaction.BillID)
		if err != nil {
			return nil, err
		}
	case "income":
		query = `SELECT id, user_id, amount, date, payment_method FROM incomes WHERE id = ? AND user_id = ?`
		row := db.QueryRow(query, transactionID, userID)
		err := row.Scan(&transaction.ID, &transaction.UserID, &transaction.Amount,
			&transaction.Date, &transaction.PaymentMethod)
		if err != nil {
			return nil, err
		}
		// Para income, bill_id siempre es NULL
		transaction.BillID = nil
	case "bill":
		query = `SELECT id, user_id, amount, due_date as date, 'bank' as payment_method FROM bills WHERE id = ? AND user_id = ?`
		row := db.QueryRow(query, transactionID, userID)
		err := row.Scan(&transaction.ID, &transaction.UserID, &transaction.Amount,
			&transaction.Date, &transaction.PaymentMethod)
		if err != nil {
			return nil, err
		}
		// Para bill, bill_id es el mismo ID
		billID := transactionID
		transaction.BillID = &billID
	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", transactionType)
	}

	return &transaction, nil
}

func deleteTransaction(transactionID int, transactionType, userID string) error {
	var query string

	switch strings.ToLower(transactionType) {
	case "expense":
		query = `DELETE FROM expenses WHERE id = ? AND user_id = ?`
	case "income":
		query = `DELETE FROM incomes WHERE id = ? AND user_id = ?`
	case "bill":
		query = `DELETE FROM bills WHERE id = ? AND user_id = ?`
	default:
		return fmt.Errorf("unsupported transaction type: %s", transactionType)
	}

	result, err := db.Exec(query, transactionID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no transaction found with ID %d for user %s", transactionID, userID)
	}

	return nil
}

// handleExpenseWithBillDeletion handles the special case when deleting an expense that corresponds to a bill payment
func handleExpenseWithBillDeletion(transaction TransactionDetails) error {
	log.Printf("Handling expense with bill_id %d deletion for user %s", *transaction.BillID, transaction.UserID)

	// Step 1: Extract month from date (format YYYY-MM-DD to YYYY-MM)
	transactionDate, err := time.Parse("2006-01-02", transaction.Date)
	if err != nil {
		return fmt.Errorf("invalid date format: %v", err)
	}
	yearMonth := transactionDate.Format("2006-01")

	log.Printf("Processing expense deletion - Bill ID: %d, Month: %s, Amount: %.2f, Payment Method: %s",
		*transaction.BillID, yearMonth, transaction.Amount, transaction.PaymentMethod)

	// Step 2: Update bill_payments table to mark as unpaid
	billPaymentQuery := `UPDATE bill_payments SET paid = 0 WHERE bill_id = ? AND year_month = ?`
	result, err := db.Exec(billPaymentQuery, *transaction.BillID, yearMonth)
	if err != nil {
		return fmt.Errorf("error updating bill_payments: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking bill_payments update: %v", err)
	}

	if rowsAffected == 0 {
		log.Printf("Warning: No bill_payments record found for bill_id %d and year_month %s", *transaction.BillID, yearMonth)
	} else {
		log.Printf("Updated bill_payments: marked bill_id %d as unpaid for %s", *transaction.BillID, yearMonth)
	}

	// Step 3: Update monthly_cash_bank_balance table
	// Move amount from expense_amount to bill_amount
	var expenseColumn, billColumn string
	if transaction.PaymentMethod == "bank" {
		expenseColumn = "expense_bank_amount"
		billColumn = "bill_bank_amount"
	} else {
		expenseColumn = "expense_cash_amount"
		billColumn = "bill_cash_amount"
	}

	balanceQuery := fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s - ?, 
		    %s = %s + ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ? AND year_month = ?`,
		expenseColumn, expenseColumn, billColumn, billColumn)

	result, err = db.Exec(balanceQuery, transaction.Amount, transaction.Amount, transaction.UserID, yearMonth)
	if err != nil {
		return fmt.Errorf("error updating monthly_cash_bank_balance: %v", err)
	}

	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking monthly_cash_bank_balance update: %v", err)
	}

	if rowsAffected == 0 {
		log.Printf("Warning: No monthly_cash_bank_balance record found for user %s and year_month %s", transaction.UserID, yearMonth)
	} else {
		log.Printf("Updated monthly_cash_bank_balance: moved %.2f from %s to %s for %s",
			transaction.Amount, expenseColumn, billColumn, yearMonth)
	}

	// Step 4: Delete the expense transaction
	err = deleteTransaction(transaction.ID, "expense", transaction.UserID)
	if err != nil {
		return fmt.Errorf("error deleting expense transaction: %v", err)
	}

	log.Printf("Successfully handled expense with bill deletion - ID: %d, Bill ID: %d", transaction.ID, *transaction.BillID)
	return nil
}
