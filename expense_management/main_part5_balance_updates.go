package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

// Balance update functions for maintaining financial data consistency across time periods

func updateBalance(userID string, amount float64, paymentMethod string) error {
	log.Printf("updateBalance called with userID: %s, amount: %.2f, paymentMethod: %s", userID, amount, paymentMethod)

	// SQL query to check if user exists in the balances table
	checkQuery := `
		SELECT COUNT(*)
		FROM balances
		WHERE user_id = ?
	`

	var count int
	err := db.QueryRow(checkQuery, userID).Scan(&count)
	if err != nil {
		log.Printf("Error checking balances table: %v", err)
		return err
	}

	log.Printf("Found %d records in balances table for user %s", count, userID)
	var query string

	// If user doesn't exist in balances table, insert a new record
	if count == 0 {
		query = `
			INSERT INTO balances (user_id, cash_balance, bank_balance)
			VALUES (?, ?, ?)
		`
		cashAmount := 0.0
		bankAmount := 0.0

		if paymentMethod == "cash" {
			cashAmount = amount
		} else {
			bankAmount = amount
		}

		log.Printf("Inserting new balance record with cash: %.2f, bank: %.2f", cashAmount, bankAmount)
		_, err = db.Exec(query, userID, cashAmount, bankAmount)
	} else {
		// Update existing balance
		if paymentMethod == "cash" {
			query = `
				UPDATE balances
				SET cash_balance = cash_balance + ?
				WHERE user_id = ?
			`
		} else {
			query = `
				UPDATE balances
				SET bank_balance = bank_balance + ?
				WHERE user_id = ?
			`
		}

		log.Printf("Updating existing balance with amount: %.2f for method: %s", amount, paymentMethod)
		_, err = db.Exec(query, amount, userID)
	}

	if err != nil {
		log.Printf("Error updating balances table: %v", err)
		return err
	} else {
		log.Printf("Successfully updated balances table")
	}

	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")
	log.Printf("Processing cash_bank for month: %s", currentMonth)

	// Fetch current cash-bank distribution
	var distribution struct {
		CashAmount   float64
		BankAmount   float64
		MonthlyTotal float64
		Exists       bool
	}

	// Check if a record exists for the current month
	cashBankCheckQuery := `
		SELECT 1
		FROM cash_bank
		WHERE user_id = ? AND month = ?
	`
	var exists bool
	err = db.QueryRow(cashBankCheckQuery, userID, currentMonth).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking cash_bank: %v", err)
		return err
	}

	distribution.Exists = err != sql.ErrNoRows
	log.Printf("Record exists in cash_bank for user %s and month %s: %v", userID, currentMonth, distribution.Exists)

	if distribution.Exists {
		// Get current values
		getQuery := `
			SELECT cash_amount, bank_amount, monthly_total
			FROM cash_bank
			WHERE user_id = ? AND month = ?
		`
		err := db.QueryRow(getQuery, userID, currentMonth).Scan(
			&distribution.CashAmount,
			&distribution.BankAmount,
			&distribution.MonthlyTotal,
		)
		if err != nil {
			log.Printf("Error fetching cash_bank data: %v", err)
			return err
		}

		log.Printf("Current cash_bank values - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Update the appropriate amount based on payment method
		if paymentMethod == "cash" {
			distribution.CashAmount += amount
		} else if paymentMethod == "bank" {
			distribution.BankAmount += amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		log.Printf("Updated cash_bank values - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		log.Printf("Calculated cash_bank percentages - cash: %.2f%%, bank: %.2f%%",
			cashPercent, bankPercent)

		// Update the record
		updateQuery := `
			UPDATE cash_bank
			SET cash_amount = ?, cash_percent = ?, bank_amount = ?, bank_percent = ?, monthly_total = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(
			updateQuery,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
			userID,
			currentMonth,
		)
		if err != nil {
			log.Printf("Error updating cash_bank: %v", err)
			return err
		}
		log.Printf("Successfully updated cash_bank record")
	} else {
		// Create a new record with initial values
		if paymentMethod == "cash" {
			distribution.CashAmount = amount
			distribution.BankAmount = 0
		} else if paymentMethod == "bank" {
			distribution.CashAmount = 0
			distribution.BankAmount = amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		log.Printf("Creating new cash_bank record - cash: %.2f, bank: %.2f, total: %.2f",
			distribution.CashAmount, distribution.BankAmount, distribution.MonthlyTotal)

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		log.Printf("Calculated cash_bank percentages for new record - cash: %.2f%%, bank: %.2f%%",
			cashPercent, bankPercent)

		// Insert the new record
		insertQuery := `
			INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(
			insertQuery,
			userID,
			currentMonth,
			distribution.CashAmount,
			cashPercent,
			distribution.BankAmount,
			bankPercent,
			distribution.MonthlyTotal,
		)
		if err != nil {
			log.Printf("Error inserting new cash_bank record: %v", err)
			return err
		}
		log.Printf("Successfully inserted new cash_bank record")
	}

	// Add transaction record for the expense (negative amount)
	// Note: For expenses, we record a negative transaction
	transactionQuery := `
		INSERT INTO cash_bank_transactions (user_id, transaction_type, amount, date)
		VALUES (?, ?, ?, ?)
	`
	transactionType := "expense_" + paymentMethod
	transactionAmount := -math.Abs(amount) // Ensure amount is negative for expenses

	log.Printf("Recording transaction - type: %s, amount: %.2f", transactionType, transactionAmount)

	_, err = db.Exec(
		transactionQuery,
		userID,
		transactionType,
		transactionAmount,
		time.Now().Format("2006-01-02"),
	)
	if err != nil {
		log.Printf("Error recording cash_bank_transaction: %v", err)
		return err
	}
	log.Printf("Successfully recorded cash_bank_transaction")

	log.Printf("updateBalance completed successfully")
	return nil
}

// cascadeBalanceUpdatesExpense actualiza balances acumulativos para todos los meses posteriores al mes especificado (para gastos)
func cascadeBalanceUpdatesExpense(userID string, fromYearMonth string, expenseCashDelta, expenseBankDelta float64) error {
	// Get all months after the specified month for this user, ordered by year_month
	query := `
		SELECT year_month, previous_cash_amount, previous_bank_amount, 
			   cash_amount, bank_amount, total_previous_balance, total_balance
		FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month > ?
		ORDER BY year_month ASC
	`

	rows, err := db.Query(query, userID, fromYearMonth)
	if err != nil {
		return fmt.Errorf("error querying future months: %v", err)
	}
	defer rows.Close()

	// Collect all existing months to identify gaps
	var existingMonths []struct {
		month            string
		prevCash         float64
		prevBank         float64
		currentCash      float64
		currentBank      float64
		totalPrevBalance float64
		totalBalance     float64
	}

	for rows.Next() {
		var monthData struct {
			month            string
			prevCash         float64
			prevBank         float64
			currentCash      float64
			currentBank      float64
			totalPrevBalance float64
			totalBalance     float64
		}

		err := rows.Scan(&monthData.month, &monthData.prevCash, &monthData.prevBank,
			&monthData.currentCash, &monthData.currentBank, &monthData.totalPrevBalance, &monthData.totalBalance)
		if err != nil {
			return fmt.Errorf("error scanning row: %v", err)
		}
		existingMonths = append(existingMonths, monthData)
	}

	// Fill gaps between fromYearMonth and first existing month, and between existing months
	err = fillGapMonthsExpense(userID, fromYearMonth, existingMonths, expenseCashDelta, expenseBankDelta)
	if err != nil {
		return fmt.Errorf("error filling gap months: %v", err)
	}

	// Process each existing month to update cumulative balances (expenses reduce balances)
	for _, monthData := range existingMonths {
		// Update cumulative amounts for this month (subtract expense amounts)
		newPrevCash := monthData.prevCash - expenseCashDelta
		newPrevBank := monthData.prevBank - expenseBankDelta
		newCurrentCash := monthData.currentCash - expenseCashDelta
		newCurrentBank := monthData.currentBank - expenseBankDelta
		newTotalPrevBalance := monthData.totalPrevBalance - expenseCashDelta - expenseBankDelta
		newTotalBalance := monthData.totalBalance - expenseCashDelta - expenseBankDelta

		// Update the record
		updateQuery := `
			UPDATE monthly_cash_bank_balance 
			SET previous_cash_amount = ?,
				previous_bank_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
				total_previous_balance = ?,
				total_balance = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`

		_, err = db.Exec(updateQuery,
			newPrevCash, newPrevBank,
			newCurrentCash, newCurrentBank,
			newCurrentCash, newCurrentBank, // balance amounts equal current amounts
			newTotalPrevBalance, newTotalBalance,
			userID, monthData.month)
		if err != nil {
			return fmt.Errorf("error updating cascaded balance for month %s: %v", monthData.month, err)
		}

		log.Printf("Cascaded expense balance update for user %s, month %s - cash delta: -%v, bank delta: -%v",
			userID, monthData.month, expenseCashDelta, expenseBankDelta)
	}

	return nil
}

// fillGapMonthsExpense crea registros para meses faltantes entre el mes de origen y los meses existentes (para gastos)
func fillGapMonthsExpense(userID string, fromYearMonth string, existingMonths []struct {
	month            string
	prevCash         float64
	prevBank         float64
	currentCash      float64
	currentBank      float64
	totalPrevBalance float64
	totalBalance     float64
}, expenseCashDelta, expenseBankDelta float64) error {

	// Parse fromYearMonth to start iteration
	fromDate, err := time.Parse("2006-01", fromYearMonth)
	if err != nil {
		return fmt.Errorf("error parsing fromYearMonth: %v", err)
	}

	// Get the balance from fromYearMonth to propagate
	var baseCash, baseBank, baseTotalBalance float64
	baseQuery := `SELECT cash_amount, bank_amount, total_balance FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err = db.QueryRow(baseQuery, userID, fromYearMonth).Scan(&baseCash, &baseBank, &baseTotalBalance)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting base month balance: %v", err)
	}

	// Subtract the expense delta from the base amounts (expenses reduce balances)
	baseCash -= expenseCashDelta
	baseBank -= expenseBankDelta
	baseTotalBalance -= expenseCashDelta + expenseBankDelta

	currentMonth := fromDate.AddDate(0, 1, 0) // Start from next month after fromYearMonth

	for _, existingMonth := range existingMonths {
		existingDate, err := time.Parse("2006-01", existingMonth.month)
		if err != nil {
			continue
		}

		// Fill gaps between currentMonth and existingMonth
		for currentMonth.Before(existingDate) {
			currentMonthStr := currentMonth.Format("2006-01")

			// Check if this month already exists
			var exists bool
			checkQuery := `SELECT 1 FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
			err := db.QueryRow(checkQuery, userID, currentMonthStr).Scan(&exists)

			if err == sql.ErrNoRows {
				// Create gap month record
				insertQuery := `
					INSERT INTO monthly_cash_bank_balance (
						user_id, year_month, 
						income_bank_amount, income_cash_amount,
						expense_bank_amount, expense_cash_amount,
						bill_bank_amount, bill_cash_amount,
						bank_amount, cash_amount,
						previous_bank_amount, previous_cash_amount,
						balance_cash_amount, balance_bank_amount,
						total_previous_balance, total_balance,
						created_at, updated_at
					) VALUES (?, ?, 0, 0, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
				`

				// For gap months, previous amounts are from the previous month
				prevMonth := currentMonth.AddDate(0, -1, 0)
				var prevCash, prevBank, prevTotalBalance float64

				if prevMonth.Format("2006-01") == fromYearMonth {
					// Previous month is the fromYearMonth
					prevCash = baseCash + expenseCashDelta // Add delta back to get original amount
					prevBank = baseBank + expenseBankDelta
					prevTotalBalance = baseTotalBalance + expenseCashDelta + expenseBankDelta
				} else {
					// Get from database
					prevQuery := `SELECT cash_amount, bank_amount, total_balance FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
					err = db.QueryRow(prevQuery, userID, prevMonth.Format("2006-01")).Scan(&prevCash, &prevBank, &prevTotalBalance)
					if err != nil && err != sql.ErrNoRows {
						return fmt.Errorf("error getting previous month balance: %v", err)
					}
				}

				_, err = db.Exec(insertQuery,
					userID, currentMonthStr,
					baseCash, baseCash, // current amounts (propagated from base with expense impact)
					prevCash, prevBank, // previous month amounts
					baseCash, baseBank, // balance amounts
					prevTotalBalance, baseTotalBalance) // total balances

				if err != nil {
					return fmt.Errorf("error inserting gap month %s: %v", currentMonthStr, err)
				}

				log.Printf("Created gap month record for expense user %s, month %s - cash: %v, bank: %v",
					userID, currentMonthStr, baseCash, baseBank)
			}

			currentMonth = currentMonth.AddDate(0, 1, 0)
		}

		// Move currentMonth to after this existing month
		currentMonth = existingDate.AddDate(0, 1, 0)
	}

	return nil
}

// updateMonthlyCashBankBalanceExpense actualiza la tabla monthly_cash_bank_balance con efecto cascada para gastos
func updateMonthlyCashBankBalanceExpense(userID string, expenseAmount, cashAmount, bankAmount float64, date time.Time) error {
	yearMonth := date.Format("2006-01")

	// Get previous month's balance to calculate cumulative values
	prevMonth := date.AddDate(0, -1, 0).Format("2006-01")
	var prevCashTotal, prevBankTotal, prevTotalBalance float64

	prevQuery := `SELECT cash_amount, bank_amount, total_balance FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err := db.QueryRow(prevQuery, userID, prevMonth).Scan(&prevCashTotal, &prevBankTotal, &prevTotalBalance)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error getting previous month balance: %v", err)
	}
	// If no previous month, prevTotals remain 0

	// Check if record exists for this user and month
	var exists bool
	checkQuery := `SELECT 1 FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err = db.QueryRow(checkQuery, userID, yearMonth).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error checking existing record: %v", err)
	}

	if err == sql.ErrNoRows {
		// Create new record with cumulative values (expenses reduce balances)
		newCashTotal := prevCashTotal - cashAmount
		newBankTotal := prevBankTotal - bankAmount
		newTotalBalance := prevTotalBalance - cashAmount - bankAmount

		insertQuery := `
			INSERT INTO monthly_cash_bank_balance (
				user_id, year_month, 
				income_bank_amount, income_cash_amount,
				expense_bank_amount, expense_cash_amount,
				bill_bank_amount, bill_cash_amount,
				bank_amount, cash_amount,
				previous_bank_amount, previous_cash_amount,
				balance_cash_amount, balance_bank_amount,
				total_previous_balance, total_balance,
				created_at, updated_at
			) VALUES (?, ?, 0, 0, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		_, err = db.Exec(insertQuery,
			userID, yearMonth,
			bankAmount, cashAmount, // expense amounts for this month
			newBankTotal, newCashTotal, // cumulative totals (reduced by expenses)
			prevBankTotal, prevCashTotal, // previous month totals
			newCashTotal, newBankTotal, // balance amounts
			prevTotalBalance, newTotalBalance) // total balances
		if err != nil {
			return fmt.Errorf("error inserting monthly balance: %v", err)
		}
	} else {
		// Update existing record (add to expense amounts and update cumulative totals)
		updateQuery := `
			UPDATE monthly_cash_bank_balance 
			SET expense_bank_amount = expense_bank_amount + ?,
				expense_cash_amount = expense_cash_amount + ?,
				bank_amount = bank_amount - ?,
				cash_amount = cash_amount - ?,
				balance_bank_amount = balance_bank_amount - ?,
				balance_cash_amount = balance_cash_amount - ?,
				total_balance = total_balance - ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`
		totalExpenseAmount := cashAmount + bankAmount
		_, err = db.Exec(updateQuery,
			bankAmount, cashAmount, // add to expense amounts
			bankAmount, cashAmount, // subtract from current totals
			bankAmount, cashAmount, // subtract from balance amounts
			totalExpenseAmount, // subtract from total balance
			userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error updating monthly balance: %v", err)
		}
	}

	// Trigger cascade update for all future months
	err = cascadeBalanceUpdatesExpense(userID, yearMonth, cashAmount, bankAmount)
	if err != nil {
		log.Printf("Error cascading expense balance updates: %v", err)
		return err
	}

	log.Printf("Updated monthly_cash_bank_balance for expense - user %s, month %s - cash: %v, bank: %v (with cascade)",
		userID, yearMonth, cashAmount, bankAmount)

	return nil
}

// Function to update balances across all time periods when adding an expense
func updateTimeBalances(userID string, amount float64, dateStr string) error {
	// Parse the expense date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Get payment method information for the expense to determine cash vs bank
	var paymentMethod string
	err = db.QueryRow(`
		SELECT payment_method FROM expenses
		WHERE user_id = ? AND date = ? AND amount = ?
		ORDER BY created_at DESC LIMIT 1
	`, userID, dateStr, amount).Scan(&paymentMethod)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error fetching payment method: %v", err)
	}

	if err == sql.ErrNoRows {
		// If not found, assume bank as default
		paymentMethod = "bank"
	}

	// Calculate cash and bank amounts based on payment method
	var cashAmount, bankAmount float64
	if paymentMethod == "cash" {
		cashAmount = amount
		bankAmount = 0
	} else {
		cashAmount = 0
		bankAmount = amount
	}

	// Update monthly cash bank balance table for expenses
	if err := updateMonthlyCashBankBalanceExpense(userID, amount, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating monthly cash bank balance for expense: %v", err)
		return err
	}

	// Update daily balance
	if err := updateDailyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating daily balance: %v", err)
	}

	return nil
}

func updateDailyBalance(userID string, incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64, date time.Time) error {
	dateStr := date.Format("2006-01-02")

	// Get the balance from the previous day to calculate previous balance
	prevDate := date.AddDate(0, 0, -1)
	prevDateStr := prevDate.Format("2006-01-02")

	var previousBalance float64
	var prevCashAmount, prevBankAmount float64

	// Search for the previous day's balance
	err := db.QueryRow(`
		SELECT balance, cash_amount, bank_amount FROM daily_balance 
		WHERE user_id = ? AND date = ?
	`, userID, prevDateStr).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

	if err != nil && err != sql.ErrNoRows {
		return err
	}
	// If there's no record for the previous day, the previous balance is 0
	if err == sql.ErrNoRows {
		previousBalance = 0
		prevCashAmount = 0
		prevBankAmount = 0
	}

	// Calculate balance as: previous balance + income - expenses - bills
	balance := previousBalance + incomeAmount - expenseAmount - billsAmount

	// Check if a record already exists for this date
	var exists bool
	var existingCash, existingBank float64
	var existingIncome, existingExpense, existingBills float64
	err = db.QueryRow(`
		SELECT 1, cash_amount, bank_amount, income_amount, expense_amount, bills_amount FROM daily_balance
		WHERE user_id = ? AND date = ?
	`, userID, dateStr).Scan(&exists, &existingCash, &existingBank, &existingIncome, &existingExpense, &existingBills)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err == sql.ErrNoRows {
		// No record exists, insert a new one
		// Cash and bank amounts should accumulate from the previous period
		totalCashAmount := prevCashAmount + cashAmount
		totalBankAmount := prevBankAmount + bankAmount

		_, err = db.Exec(`
			INSERT INTO daily_balance (user_id, date, income_amount, expense_amount, bills_amount, cash_amount, bank_amount, balance, previous_balance, previous_cash_amount, previous_bank_amount)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, userID, dateStr, incomeAmount, expenseAmount, billsAmount, totalCashAmount, totalBankAmount, balance, previousBalance, prevCashAmount, prevBankAmount)
	} else {
		// Update existing record
		// Calculate new totals by adding existing values
		newIncome := existingIncome + incomeAmount
		newExpense := existingExpense + expenseAmount
		newBills := existingBills + billsAmount

		// Update cash and bank amounts by adding new values to existing ones
		newCashAmount := existingCash + cashAmount
		newBankAmount := existingBank + bankAmount

		// Recalculate balance
		balance := previousBalance + newIncome - newExpense - newBills

		_, err = db.Exec(`
			UPDATE daily_balance
			SET income_amount = ?,
				expense_amount = ?,
				bills_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_balance = ?,
				balance = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, newIncome, newExpense, newBills, newCashAmount, newBankAmount, previousBalance, balance, prevCashAmount, prevBankAmount, userID, dateStr)
	}

	if err != nil {
		return err
	}

	// Update all subsequent days in cascade
	return updateSubsequentDailyBalances(userID, date.AddDate(0, 0, 1))
}

// Function to update subsequent days in cascade
func updateSubsequentDailyBalances(userID string, startDate time.Time) error {
	// Limit the process to one year to avoid infinite loops
	endDate := startDate.AddDate(1, 0, 0)
	currentDate := startDate

	for currentDate.Before(endDate) {
		currentDateStr := currentDate.Format("2006-01-02")

		// Check if a record exists for this date
		var exists bool
		var incomeAmount, expenseAmount, billsAmount, cashAmount, bankAmount float64
		err := db.QueryRow(`
			SELECT 1, income_amount, expense_amount, bills_amount, cash_amount, bank_amount FROM daily_balance
			WHERE user_id = ? AND date = ?
		`, userID, currentDateStr).Scan(&exists, &incomeAmount, &expenseAmount, &billsAmount, &cashAmount, &bankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			// No more records to update
			break
		}

		// Get the balance from the previous day
		prevDate := currentDate.AddDate(0, 0, -1)
		prevDateStr := prevDate.Format("2006-01-02")

		var previousBalance float64
		var prevCashAmount, prevBankAmount float64
		err = db.QueryRow(`
			SELECT balance, cash_amount, bank_amount FROM daily_balance 
			WHERE user_id = ? AND date = ?
		`, userID, prevDateStr).Scan(&previousBalance, &prevCashAmount, &prevBankAmount)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows {
			previousBalance = 0
			prevCashAmount = 0
			prevBankAmount = 0
		}

		// Update the balance with the new previous balance
		balance := previousBalance + incomeAmount - expenseAmount - billsAmount

		// LOGIC CHANGE: Always accumulate values from the previous day
		// regardless of whether there are transactions on this day or not
		hasTransactions := incomeAmount != 0 || expenseAmount != 0 || billsAmount != 0

		// Initialize with values from the previous day
		newCashAmount := prevCashAmount
		newBankAmount := prevBankAmount

		// If there are own transactions on this day, add them to what was inherited
		if hasTransactions {
			// Add this day's own transactions
			newCashAmount += cashAmount
			newBankAmount += bankAmount
		}

		_, err = db.Exec(`
			UPDATE daily_balance
			SET previous_balance = ?,
				balance = ?,
				cash_amount = ?,
				bank_amount = ?,
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND date = ?
		`, previousBalance, balance, newCashAmount, newBankAmount, prevCashAmount, prevBankAmount, userID, currentDateStr)

		if err != nil {
			return err
		}

		// Move to the next day
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return nil
}

// Function to recalculate all balances in cascade
func recalculateAllBalances(userID string, dateStr string) error {
	// Parse the transaction date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Recalculate daily balances in cascade from the transaction date
	if err := updateSubsequentDailyBalances(userID, date); err != nil {
		return fmt.Errorf("error updating daily balances: %v", err)
	}

	// Calculate the start of the week containing the date
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Convert Sunday (0) to 7
	}
	startOfWeek := date.AddDate(0, 0, -(dayOfWeek - 1))

	// Subsequent balance recalculations - simplified logging
	log.Printf("Subsequent balance recalculations for userID: %s from date: %v", userID, date)
	log.Printf("Weekly balances recalculation from: %v", startOfWeek)

	startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
	log.Printf("Monthly balances recalculation from: %v", startOfMonth)

	quarter := (int(date.Month()) - 1) / 3
	startOfQuarter := time.Date(date.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, time.UTC)
	log.Printf("Quarterly balances recalculation from: %v", startOfQuarter)

	// Semiannual and annual balance recalculations - simplified logging
	halfYear := (int(date.Month()) - 1) / 6
	startOfHalfYear := time.Date(date.Year(), time.Month(halfYear*6+1), 1, 0, 0, 0, 0, time.UTC)
	log.Printf("Semiannual balances recalculation from: %v", startOfHalfYear)

	startOfYear := time.Date(date.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	log.Printf("Annual balances recalculation from: %v", startOfYear)

	return nil
}
