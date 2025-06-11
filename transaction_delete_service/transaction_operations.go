package main

import (
	"fmt"
	"strings"
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
