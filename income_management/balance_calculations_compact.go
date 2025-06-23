package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// updateBalance actualiza el balance cash/bank para el mes actual
func updateBalance(userID string, amount float64, paymentMethod string) error {
	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")

	// Fetch current cash-bank distribution
	var distribution struct {
		CashAmount   float64
		BankAmount   float64
		MonthlyTotal float64
		Exists       bool
	}

	// Check if a record exists for the current month
	checkQuery := `SELECT 1 FROM cash_bank WHERE user_id = ? AND month = ?`
	var exists bool
	err := db.QueryRow(checkQuery, userID, currentMonth).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	distribution.Exists = err != sql.ErrNoRows

	if distribution.Exists {
		// Get current values
		getQuery := `SELECT cash_amount, bank_amount, monthly_total FROM cash_bank WHERE user_id = ? AND month = ?`
		err := db.QueryRow(getQuery, userID, currentMonth).Scan(&distribution.CashAmount, &distribution.BankAmount, &distribution.MonthlyTotal)
		if err != nil {
			return err
		}

		// Update the appropriate amount based on payment method
		if paymentMethod == "cash" {
			distribution.CashAmount += amount
		} else if paymentMethod == "bank" {
			distribution.BankAmount += amount
		}

		distribution.MonthlyTotal = distribution.CashAmount + distribution.BankAmount

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		// Update the record
		updateQuery := `
			UPDATE cash_bank
			SET cash_amount = ?, cash_percent = ?, bank_amount = ?, bank_percent = ?, monthly_total = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND month = ?
		`
		_, err = db.Exec(updateQuery, distribution.CashAmount, cashPercent, distribution.BankAmount, bankPercent, distribution.MonthlyTotal, userID, currentMonth)
		if err != nil {
			return err
		}
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

		// Calculate percentages
		var cashPercent, bankPercent float64
		if distribution.MonthlyTotal > 0 {
			cashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			bankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}

		// Insert the new record
		insertQuery := `
			INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertQuery, userID, currentMonth, distribution.CashAmount, cashPercent, distribution.BankAmount, bankPercent, distribution.MonthlyTotal)
		if err != nil {
			return err
		}
	}

	// Add transaction record if it's an addition (positive amount)
	if amount > 0 {
		// Add a transaction record
		transactionQuery := `
			INSERT INTO cash_bank_transactions (user_id, transaction_type, amount, date)
			VALUES (?, ?, ?, ?)
		`
		transactionType := "income_" + paymentMethod
		_, err = db.Exec(transactionQuery, userID, transactionType, amount, time.Now().Format("2006-01-02"))
		if err != nil {
			return err
		}
	}

	return nil
}

// cascadeBalanceUpdates actualiza balances acumulativos para todos los meses posteriores al mes especificado
func cascadeBalanceUpdates(userID string, fromYearMonth string, cashDelta, bankDelta float64) error {
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

	// Process each future month to update cumulative balances
	for rows.Next() {
		var month string
		var prevCash, prevBank, currentCash, currentBank, totalPrevBalance, totalBalance float64
		
		err := rows.Scan(&month, &prevCash, &prevBank, &currentCash, &currentBank, &totalPrevBalance, &totalBalance)
		if err != nil {
			return fmt.Errorf("error scanning row: %v", err)
		}

		// Update cumulative amounts for this month
		newPrevCash := prevCash + cashDelta
		newPrevBank := prevBank + bankDelta
		newCurrentCash := currentCash + cashDelta
		newCurrentBank := currentBank + bankDelta
		newTotalPrevBalance := totalPrevBalance + cashDelta + bankDelta
		newTotalBalance := totalBalance + cashDelta + bankDelta

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
			newCurrentCash, newCurrentBank,  // balance amounts equal current amounts
			newTotalPrevBalance, newTotalBalance,
			userID, month)
		if err != nil {
			return fmt.Errorf("error updating cascaded balance for month %s: %v", month, err)
		}

		log.Printf("Cascaded balance update for user %s, month %s - cash delta: %v, bank delta: %v", 
			userID, month, cashDelta, bankDelta)
	}

	return nil
}

// updateMonthlyCashBankBalance actualiza la tabla monthly_cash_bank_balance con efecto cascada
func updateMonthlyCashBankBalance(userID string, incomeAmount, cashAmount, bankAmount float64, date time.Time) error {
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
		// Create new record with cumulative values
		newCashTotal := prevCashTotal + cashAmount
		newBankTotal := prevBankTotal + bankAmount
		newTotalBalance := prevTotalBalance + cashAmount + bankAmount
		
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
			) VALUES (?, ?, ?, ?, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`
		_, err = db.Exec(insertQuery, 
			userID, yearMonth,
			bankAmount, cashAmount,   // income amounts for this month
			newBankTotal, newCashTotal,  // cumulative totals
			prevBankTotal, prevCashTotal,  // previous month totals
			newCashTotal, newBankTotal,   // balance amounts  
			prevTotalBalance, newTotalBalance)  // total balances
		if err != nil {
			return fmt.Errorf("error inserting monthly balance: %v", err)
		}
	} else {
		// Update existing record (add to income amounts and update cumulative totals)
		updateQuery := `
			UPDATE monthly_cash_bank_balance 
			SET income_bank_amount = income_bank_amount + ?,
				income_cash_amount = income_cash_amount + ?,
				bank_amount = bank_amount + ?,
				cash_amount = cash_amount + ?,
				balance_bank_amount = balance_bank_amount + ?,
				balance_cash_amount = balance_cash_amount + ?,
				total_balance = total_balance + ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`
		totalIncomeAmount := cashAmount + bankAmount
		_, err = db.Exec(updateQuery,
			bankAmount, cashAmount,  // add to income amounts
			bankAmount, cashAmount,  // add to current totals
			bankAmount, cashAmount,  // add to balance amounts
			totalIncomeAmount,       // add to total balance
			userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error updating monthly balance: %v", err)
		}
	}

	// Trigger cascade update for all future months
	err = cascadeBalanceUpdates(userID, yearMonth, cashAmount, bankAmount)
	if err != nil {
		log.Printf("Error cascading balance updates: %v", err)
		return err
	}

	log.Printf("Updated monthly_cash_bank_balance for user %s, month %s - cash: %v, bank: %v (with cascade)", 
		userID, yearMonth, cashAmount, bankAmount)
	
	return nil
}

// updateTimeBalances actualiza los balances por períodos al añadir un ingreso
func updateTimeBalances(userID string, amount float64, dateStr string) error {
	// Parse la fecha del ingreso
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Obtener la información del ingreso para determinar si fue cash o bank
	var paymentMethod string
	err = db.QueryRow(`
		SELECT payment_method FROM incomes
		WHERE user_id = ? AND date = ? AND amount = ?
		ORDER BY created_at DESC LIMIT 1
	`, userID, dateStr, amount).Scan(&paymentMethod)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error fetching payment method: %v", err)
	}

	if err == sql.ErrNoRows {
		// Si no se encuentra, asumimos bank por defecto
		paymentMethod = "bank"
	}

	// Calculamos los montos de cash y bank según el método de pago
	var cashAmount, bankAmount float64
	if paymentMethod == "cash" {
		cashAmount = amount
		bankAmount = 0
	} else {
		cashAmount = 0
		bankAmount = amount
	}

	// Update monthly cash bank balance table
	if err := updateMonthlyCashBankBalance(userID, amount, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating monthly cash bank balance: %v", err)
		return err
	}

	return nil
}