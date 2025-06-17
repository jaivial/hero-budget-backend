package main

import (
	"database/sql"
	"fmt"
	"time"
)

// BillUpdateData contiene los datos necesarios para la actualización
type BillUpdateData struct {
	BillID            int
	UserID            string
	OldAmount         float64
	NewAmount         float64
	OldDurationMonths int
	NewDurationMonths int
	OldStartDate      string
	NewStartDate      string
	OldPaymentMethod  string
	NewPaymentMethod  string
}

// updateBillAmountLogic maneja toda la lógica de actualización de importes
func updateBillAmountLogic(db *sql.DB, updateData BillUpdateData) error {
	if updateData.OldStartDate != updateData.NewStartDate || updateData.OldDurationMonths != updateData.NewDurationMonths {
		return nil
	}
	if updateData.OldAmount == updateData.NewAmount {
		return nil
	}

	amountDifference := updateData.NewAmount - updateData.OldAmount
	if err := updateExpensesWithBillID(db, updateData, amountDifference); err != nil {
		return fmt.Errorf("error updating expenses: %v", err)
	}
	if err := updateMonthlyBalancesForAmount(db, updateData, amountDifference); err != nil {
		return fmt.Errorf("error updating monthly balances: %v", err)
	}

	startDate, err := time.Parse("2006-01-02", updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid start date for previous balances: %v", err)
	}
	firstMonth := startDate.Format("2006-01")
	updatePreviousBalancesForAmount(db, updateData.UserID, firstMonth, amountDifference, updateData.NewPaymentMethod)
	return nil
}

// updateExpensesWithBillID actualiza la tabla expenses para el bill_id específico
func updateExpensesWithBillID(db *sql.DB, updateData BillUpdateData, amountDifference float64) error {
	rows, err := db.Query("SELECT id, date FROM expenses WHERE bill_id = ? AND user_id = ?", updateData.BillID, updateData.UserID)
	if err != nil {
		return fmt.Errorf("error fetching expenses: %v", err)
	}
	defer rows.Close()

	var expenseUpdates []struct {
		ID   int
		Date string
	}

	for rows.Next() {
		var id int
		var date string
		if err := rows.Scan(&id, &date); err == nil {
			expenseUpdates = append(expenseUpdates, struct {
				ID   int
				Date string
			}{id, date})
		}
	}

	for _, expense := range expenseUpdates {
		db.Exec("UPDATE expenses SET amount = ? WHERE id = ? AND user_id = ?", updateData.NewAmount, expense.ID, updateData.UserID)
		if parsedDate, err := time.Parse("2006-01-02", expense.Date); err == nil {
			yearMonth := parsedDate.Format("2006-01")
			updateExpenseAmountInMonthlyBalance(db, updateData.UserID, yearMonth, amountDifference, updateData.NewPaymentMethod)
		}
	}
	return nil
}

// updateExpenseAmountForBill actualiza el amount en la tabla expenses para un bill específico
func updateExpenseAmountForBill(db *sql.DB, billID int, userID, yearMonth string, amountDifference float64) error {
	_, err := db.Exec("UPDATE expenses SET amount = amount + ? WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", amountDifference, billID, userID, yearMonth)
	return err
}

// updateExpenseAmountInMonthlyBalance actualiza las columnas expense_* en monthly_cash_bank_balance
func updateExpenseAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string, amountDifference float64, paymentMethod string) error {
	var column string
	if paymentMethod == "cash" {
		column = "expense_cash_amount"
	} else {
		column = "expense_bank_amount"
	}
	query := fmt.Sprintf("UPDATE monthly_cash_bank_balance SET %s = %s + ? WHERE user_id = ? AND year_month = ?", column, column)
	_, err := db.Exec(query, amountDifference, userID, yearMonth)
	return err
}

// updateMonthlyBalancesForAmount actualiza todos los meses de la duración del bill
func updateMonthlyBalancesForAmount(db *sql.DB, updateData BillUpdateData, amountDifference float64) error {
	startDate, err := time.Parse("2006-01-02", updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}

	expenseMonths := make(map[string]bool)
	rows, err := db.Query("SELECT DISTINCT strftime('%Y-%m', date) as year_month FROM expenses WHERE bill_id = ? AND user_id = ?", updateData.BillID, updateData.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				expenseMonths[month] = true
			}
		}
	}

	for i := 0; i < updateData.NewDurationMonths; i++ {
		monthDate := startDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")

		if expenseMonths[yearMonth] {
			updateExpenseAmountInMonthlyBalance(db, updateData.UserID, yearMonth, amountDifference, updateData.NewPaymentMethod)
		} else {
			updateBillAmountInMonthlyBalance(db, updateData.UserID, yearMonth, amountDifference, updateData.NewPaymentMethod)
		}
		updateMainBalanceColumns(db, updateData.UserID, yearMonth, amountDifference, updateData.NewPaymentMethod)
	}
	return nil
}

// updateBillAmountInMonthlyBalance actualiza las columnas bill_* en monthly_cash_bank_balance
func updateBillAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string, amountDifference float64, paymentMethod string) error {
	var column string
	if paymentMethod == "cash" {
		column = "bill_cash_amount"
	} else {
		column = "bill_bank_amount"
	}
	query := fmt.Sprintf("UPDATE monthly_cash_bank_balance SET %s = %s + ? WHERE user_id = ? AND year_month = ?", column, column)
	_, err := db.Exec(query, amountDifference, userID, yearMonth)
	return err
}

// updateMainBalanceColumns actualiza las columnas principales de balance
func updateMainBalanceColumns(db *sql.DB, userID, yearMonth string, amountDifference float64, paymentMethod string) error {
	var query string
	if paymentMethod == "cash" {
		query = "UPDATE monthly_cash_bank_balance SET cash_amount = cash_amount - ?, balance_cash_amount = balance_cash_amount - ?, total_balance = total_balance - ? WHERE user_id = ? AND year_month = ?"
	} else {
		query = "UPDATE monthly_cash_bank_balance SET bank_amount = bank_amount - ?, balance_bank_amount = balance_bank_amount - ?, total_balance = total_balance - ? WHERE user_id = ? AND year_month = ?"
	}
	_, err := db.Exec(query, amountDifference, amountDifference, amountDifference, userID, yearMonth)
	return err
}

// updatePreviousBalancesForAmount actualiza las columnas previous_* para cambios de importe
func updatePreviousBalancesForAmount(db *sql.DB, userID string, startMonth string, amountDifference float64, paymentMethod string) error {
	rows, err := db.Query("SELECT year_month FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month > ? ORDER BY year_month", userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching subsequent months: %v", err)
	}
	defer rows.Close()

	var subsequentMonths []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err == nil {
			subsequentMonths = append(subsequentMonths, month)
		}
	}

	for _, month := range subsequentMonths {
		var query string
		if paymentMethod == "cash" {
			query = "UPDATE monthly_cash_bank_balance SET previous_cash_amount = previous_cash_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?"
		} else {
			query = "UPDATE monthly_cash_bank_balance SET previous_bank_amount = previous_bank_amount - ?, total_previous_balance = total_previous_balance - ? WHERE user_id = ? AND year_month = ?"
		}
		db.Exec(query, amountDifference, amountDifference, userID, month)
	}
	return nil
}
