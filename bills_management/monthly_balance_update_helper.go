package main

import (
	"fmt"
	"log"
	"time"
)

// getBillDataBeforeDelete retrieves bill data before deletion for balance updates
func getBillDataBeforeDelete(billID int, userID string) (*BillData, error) {
	var billData BillData
	query := `SELECT id, user_id, amount, payment_method, start_date, duration_months 
			  FROM bills WHERE id = ? AND user_id = ?`

	err := db.QueryRow(query, billID, userID).Scan(
		&billData.ID,
		&billData.UserID,
		&billData.Amount,
		&billData.PaymentMethod,
		&billData.StartDate,
		&billData.Duration,
	)

	if err != nil {
		log.Printf("Error getting bill data: %v", err)
		return nil, err
	}

	return &billData, nil
}

// getExpenseMonthsForBill finds months where the bill has associated expenses
func getExpenseMonthsForBill(billID int) ([]ExpenseMonth, error) {
	var expenseMonths []ExpenseMonth
	query := `SELECT DISTINCT strftime('%Y-%m', date) as year_month, date 
			  FROM expenses WHERE bill_id = ? ORDER BY date`

	rows, err := db.Query(query, billID)
	if err != nil {
		log.Printf("Error querying expense months: %v", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var expenseMonth ExpenseMonth
		err := rows.Scan(&expenseMonth.YearMonth, &expenseMonth.Date)
		if err != nil {
			log.Printf("Error scanning expense month: %v", err)
			return nil, err
		}
		expenseMonths = append(expenseMonths, expenseMonth)
	}

	return expenseMonths, nil
}

// updateMonthlyBalanceForDeletedBill updates monthly balance when deleting a bill
func updateMonthlyBalanceForDeletedBill(billData *BillData) error {
	// Get expense months for this bill
	expenseMonths, err := getExpenseMonthsForBill(billData.ID)
	if err != nil {
		return err
	}

	// Create map of expense months for quick lookup
	expenseMonthsMap := make(map[string]bool)
	for _, em := range expenseMonths {
		expenseMonthsMap[em.YearMonth] = true
	}

	// Parse start date string to time.Time for calculations
	startDate, err := time.Parse("2006-01-02", billData.StartDate)
	if err != nil {
		log.Printf("Error parsing start date: %v", err)
		return err
	}

	// Generate all months for the bill duration
	allBillMonths := generateBillMonths(startDate, billData.Duration)

	// Update balances for each month
	for _, yearMonth := range allBillMonths {
		isExpenseMonth := expenseMonthsMap[yearMonth]

		if err := updateMonthBalance(billData, yearMonth, isExpenseMonth); err != nil {
			return err
		}
	}

	// Update cascade balances from start date using existing function
	startYearMonth := startDate.Format("2006-01")
	if err := updateCascadeBalances(db, billData.UserID, startYearMonth); err != nil {
		return err
	}

	return nil
}

// generateBillMonths generates year-month strings for bill duration
func generateBillMonths(startDate time.Time, duration int) []string {
	var months []string
	currentDate := startDate

	for i := 0; i < duration; i++ {
		yearMonth := currentDate.Format("2006-01")
		months = append(months, yearMonth)
		currentDate = currentDate.AddDate(0, 1, 0)
	}

	return months
}

// updateMonthBalance updates balance for a specific month
func updateMonthBalance(billData *BillData, yearMonth string, isExpenseMonth bool) error {
	// Determine column names based on payment method
	var expenseAmountCol, billAmountCol string

	if billData.PaymentMethod == "bank" {
		expenseAmountCol = "expense_bank_amount"
		billAmountCol = "bill_bank_amount"
	} else {
		expenseAmountCol = "expense_cash_amount"
		billAmountCol = "bill_cash_amount"
	}

	var query string
	if isExpenseMonth {
		// Month with expense: subtract from expense amount only
		query = fmt.Sprintf(`UPDATE monthly_cash_bank_balance 
			SET %s = %s - ?
			WHERE year_month = ? AND user_id = ?`,
			expenseAmountCol, expenseAmountCol)
	} else {
		// Month without expense: subtract from bill amount only
		query = fmt.Sprintf(`UPDATE monthly_cash_bank_balance 
			SET %s = %s - ?
			WHERE year_month = ? AND user_id = ?`,
			billAmountCol, billAmountCol)
	}

	_, err := db.Exec(query, billData.Amount, yearMonth, billData.UserID)

	if err != nil {
		log.Printf("Error updating month balance for %s: %v", yearMonth, err)
		return err
	}

	return nil
}
