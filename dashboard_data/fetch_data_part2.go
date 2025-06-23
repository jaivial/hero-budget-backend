package main

import (
	"database/sql"
	"time"
)

// fetchCashBankDistribution obtiene distribución de efectivo y banco
func fetchCashBankDistribution(userID string) (CashBank, error) {
	var cashBank CashBank

	// Query cash_bank data from database for user's current distribution
	err := db.QueryRow(`
		SELECT month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total
		FROM cash_bank
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&cashBank.Month,
		&cashBank.CashAmount,
		&cashBank.CashPercent,
		&cashBank.BankAmount,
		&cashBank.BankPercent,
		&cashBank.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// Return default values if no cash/bank data found
		cashBank.Month = time.Now().Format("January 2006")
		cashBank.CashAmount = 0
		cashBank.CashPercent = 0
		cashBank.BankAmount = 0
		cashBank.BankPercent = 0
		cashBank.MonthlyTotal = 0
		return cashBank, nil
	} else if err != nil {
		return cashBank, err
	}

	return cashBank, nil
}

// fetchFinanceMetrics obtiene métricas financieras para el período
func fetchFinanceMetrics(userID, period string) (FinanceMetrics, error) {
	var financeMetrics FinanceMetrics

	// Query finance_metrics data from database for specified period
	err := db.QueryRow(`
		SELECT income, expenses, bills
		FROM finance_metrics
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&financeMetrics.Income,
		&financeMetrics.Expenses,
		&financeMetrics.Bills,
	)

	if err == sql.ErrNoRows {
		// Return default values if no finance metrics found
		financeMetrics.Income = 0
		financeMetrics.Expenses = 0
		financeMetrics.Bills = 0
		return financeMetrics, nil
	} else if err != nil {
		return financeMetrics, err
	}

	return financeMetrics, nil
}

// fetchUpcomingBills obtiene facturas pendientes para el usuario
func fetchUpcomingBills(userID string) ([]Bill, error) {
	var bills []Bill

	// Get the current date for filtering upcoming bills
	currentDate := time.Now().Format("2006-01-02")

	// Query bills that are not paid and due in the future, or recurring bills
	rows, err := db.Query(`
		SELECT id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon
		FROM bills
		WHERE user_id = ? AND ((due_date >= ? AND paid = 0) OR recurring = 1) AND paid = 0
		ORDER BY due_date ASC
		LIMIT 10
	`, userID, currentDate)

	if err != nil {
		return bills, err
	}
	defer rows.Close()

	// Iterate through query results and build bills slice
	for rows.Next() {
		var bill Bill
		err := rows.Scan(
			&bill.ID,
			&bill.Name,
			&bill.Amount,
			&bill.DueDate,
			&bill.Paid,
			&bill.Overdue,
			&bill.OverdueDays,
			&bill.Recurring,
			&bill.Category,
			&bill.Icon,
		)
		if err != nil {
			return bills, err
		}
		bills = append(bills, bill)
	}

	return bills, nil
}