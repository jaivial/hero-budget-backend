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

	// Balance updates - simplified logging
	log.Printf("Note: Daily balance update for income - amount: %v, cash: %v, bank: %v", amount, cashAmount, bankAmount)
	log.Printf("Note: Weekly balance update for income - amount: %v, cash: %v, bank: %v", amount, cashAmount, bankAmount)
	log.Printf("Note: Monthly balance update for income - amount: %v, cash: %v, bank: %v", amount, cashAmount, bankAmount)

	// Actualizar balance trimestral
	if err := updateQuarterlyBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating quarterly balance: %v", err)
	}

	// Actualizar balance semestral
	if err := updateSemiannualBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating semiannual balance: %v", err)
	}

	// Actualizar balance anual
	if err := updateAnnualBalance(userID, amount, 0, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating annual balance: %v", err)
	}

	return nil
}