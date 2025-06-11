package main

import (
	"database/sql"
	"fmt"
	"log"
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
	if updateData.OldAmount == updateData.NewAmount {
		log.Printf("Amount unchanged, skipping amount update logic")
		return nil
	}

	amountDifference := updateData.NewAmount - updateData.OldAmount
	log.Printf("Amount difference: %.2f (new: %.2f - old: %.2f)",
		amountDifference, updateData.NewAmount, updateData.OldAmount)

	// 1. Actualizar expenses con bill_id
	err := updateExpensesWithBillID(db, updateData, amountDifference)
	if err != nil {
		return fmt.Errorf("error updating expenses: %v", err)
	}

	// 2. Actualizar monthly_cash_bank_balance para todos los meses de la duración
	err = updateMonthlyBalancesForAmount(db, updateData, amountDifference)
	if err != nil {
		return fmt.Errorf("error updating monthly balances: %v", err)
	}

	// 3. Actualizar previous_* desde el primer mes de la duración hasta el más reciente
	startDate, err := time.Parse("2006-01-02", updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid start date for previous balances: %v", err)
	}
	firstMonth := startDate.Format("2006-01")

	err = updatePreviousBalancesForAmount(db, updateData.UserID, firstMonth,
		amountDifference, updateData.NewPaymentMethod)
	if err != nil {
		log.Printf("Error updating previous balances: %v", err)
	}

	return nil
}

// updateExpensesWithBillID actualiza la tabla expenses para el bill_id específico
func updateExpensesWithBillID(db *sql.DB, updateData BillUpdateData, amountDifference float64) error {
	// Buscar expenses con el bill_id
	rows, err := db.Query(`
		SELECT id, date FROM expenses 
		WHERE bill_id = ? AND user_id = ?
	`, updateData.BillID, updateData.UserID)
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
		if err := rows.Scan(&id, &date); err != nil {
			log.Printf("Error scanning expense row: %v", err)
			continue
		}
		expenseUpdates = append(expenseUpdates, struct {
			ID   int
			Date string
		}{id, date})
	}

	// Actualizar cada expense
	for _, expense := range expenseUpdates {
		_, err = db.Exec(`
			UPDATE expenses 
			SET amount = ? 
			WHERE id = ? AND user_id = ?
		`, updateData.NewAmount, expense.ID, updateData.UserID)
		if err != nil {
			log.Printf("Error updating expense %d: %v", expense.ID, err)
			continue
		}

		// Actualizar monthly_cash_bank_balance para este mes específico
		parsedDate, err := time.Parse("2006-01-02", expense.Date)
		if err != nil {
			log.Printf("Error parsing expense date %s: %v", expense.Date, err)
			continue
		}
		yearMonth := parsedDate.Format("2006-01")

		err = updateExpenseAmountInMonthlyBalance(db, updateData.UserID, yearMonth,
			amountDifference, updateData.NewPaymentMethod)
		if err != nil {
			log.Printf("Error updating monthly balance for expense: %v", err)
		}
	}

	return nil
}

// updateExpenseAmountForBill actualiza el amount en la tabla expenses para un bill específico
func updateExpenseAmountForBill(db *sql.DB, billID int, userID, yearMonth string, amountDifference float64) error {
	// Actualizar todos los expenses de este bill en el mes especificado
	result, err := db.Exec(`
		UPDATE expenses 
		SET amount = amount + ? 
		WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?
	`, amountDifference, billID, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error updating expenses amount: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected for expense update: %v", err)
	} else {
		log.Printf("Updated %d expense records with amount difference %.2f for bill %d in month %s",
			rowsAffected, amountDifference, billID, yearMonth)
	}

	return nil
}

// updateExpenseAmountInMonthlyBalance actualiza las columnas expense_* en monthly_cash_bank_balance
func updateExpenseAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string,
	amountDifference float64, paymentMethod string) error {

	var column string
	if paymentMethod == "cash" {
		column = "expense_cash_amount"
	} else {
		column = "expense_bank_amount"
	}

	_, err := db.Exec(fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s + ? 
		WHERE user_id = ? AND year_month = ?
	`, column, column), amountDifference, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error updating %s: %v", column, err)
	}

	log.Printf("Updated %s by %.2f for user %s in month %s",
		column, amountDifference, userID, yearMonth)
	return nil
}

// updateMonthlyBalancesForAmount actualiza todos los meses de la duración del bill
func updateMonthlyBalancesForAmount(db *sql.DB, updateData BillUpdateData, amountDifference float64) error {
	// Calcular todos los meses afectados por la duración actual
	startDate, err := time.Parse("2006-01-02", updateData.NewStartDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}

	// Obtener meses que tienen expenses para este bill
	expenseMonths := make(map[string]bool)
	rows, err := db.Query(`
		SELECT DISTINCT strftime('%Y-%m', date) as year_month 
		FROM expenses 
		WHERE bill_id = ? AND user_id = ?
	`, updateData.BillID, updateData.UserID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var month string
			if rows.Scan(&month) == nil {
				expenseMonths[month] = true
			}
		}
	}

	// Actualizar todos los meses de la duración
	for i := 0; i < updateData.NewDurationMonths; i++ {
		monthDate := startDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")

		// Crear registro si no existe
		_, err = db.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
			VALUES (?, ?)
		`, updateData.UserID, yearMonth)
		if err != nil {
			log.Printf("Error creating monthly record for %s: %v", yearMonth, err)
			continue
		}

		// Si este mes TIENE expenses, actualizar tabla expenses y expense_* columns
		if expenseMonths[yearMonth] {
			// Actualizar la tabla expenses
			err = updateExpenseAmountForBill(db, updateData.BillID, updateData.UserID, yearMonth, amountDifference)
			if err != nil {
				log.Printf("Error updating expense amount for month %s: %v", yearMonth, err)
			}

			// Actualizar expense_* columns en monthly_cash_bank_balance
			err = updateExpenseAmountInMonthlyBalance(db, updateData.UserID, yearMonth,
				amountDifference, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error updating expense columns for month %s: %v", yearMonth, err)
			}

			// Actualizar columnas principales también para meses con expenses
			err = updateMainBalanceColumns(db, updateData.UserID, yearMonth,
				amountDifference, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error updating main balance columns for month %s: %v", yearMonth, err)
			}

			log.Printf("Updated expense records for month %s (has expenses)", yearMonth)
		} else {
			// Si este mes NO tiene expenses, actualizar bill_amount y columnas principales
			err = updateBillAmountInMonthlyBalance(db, updateData.UserID, yearMonth,
				amountDifference, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error updating bill amount for %s: %v", yearMonth, err)
			}

			// Actualizar columnas principales para meses sin expenses
			err = updateMainBalanceColumns(db, updateData.UserID, yearMonth,
				amountDifference, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error updating main balance columns for %s: %v", yearMonth, err)
			}

			log.Printf("Updated bill records for month %s (no expenses)", yearMonth)
		}
	}

	return nil
}

// updateBillAmountInMonthlyBalance actualiza las columnas bill_* en monthly_cash_bank_balance
func updateBillAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string,
	amountDifference float64, paymentMethod string) error {

	var column string
	if paymentMethod == "cash" {
		column = "bill_cash_amount"
	} else {
		column = "bill_bank_amount"
	}

	_, err := db.Exec(fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s + ? 
		WHERE user_id = ? AND year_month = ?
	`, column, column), amountDifference, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error updating %s: %v", column, err)
	}

	log.Printf("Updated %s by %.2f for user %s in month %s",
		column, amountDifference, userID, yearMonth)
	return nil
}

// updateMainBalanceColumns actualiza las columnas principales de balance
func updateMainBalanceColumns(db *sql.DB, userID, yearMonth string,
	amountDifference float64, paymentMethod string) error {

	// Actualizar las columnas apropiadas
	if paymentMethod == "cash" {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET cash_amount = cash_amount + ?,
			    balance_cash_amount = balance_cash_amount + ?,
			    total_balance = total_balance + ?
			WHERE user_id = ? AND year_month = ?
		`, amountDifference, amountDifference, amountDifference, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error updating cash balance columns: %v", err)
		}
	} else {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET bank_amount = bank_amount + ?,
			    balance_bank_amount = balance_bank_amount + ?,
			    total_balance = total_balance + ?
			WHERE user_id = ? AND year_month = ?
		`, amountDifference, amountDifference, amountDifference, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error updating bank balance columns: %v", err)
		}
	}

	log.Printf("Updated main balance columns by %.2f for user %s in month %s",
		amountDifference, userID, yearMonth)
	return nil
}

// updatePreviousBalancesForAmount actualiza las columnas previous_* para cambios de importe
func updatePreviousBalancesForAmount(db *sql.DB, userID string, startMonth string,
	amountDifference float64, paymentMethod string) error {

	// Obtener todos los meses posteriores al startMonth (excluyendo el startMonth)
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month > ?
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching subsequent months: %v", err)
	}
	defer rows.Close()

	var subsequentMonths []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			log.Printf("Error scanning month: %v", err)
			continue
		}
		subsequentMonths = append(subsequentMonths, month)
	}

	// Actualizar previous_* para cada mes posterior (excluyendo mes de inicio)
	for _, month := range subsequentMonths {
		if paymentMethod == "cash" {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_cash_amount = previous_cash_amount + ?,
				    total_previous_balance = total_previous_balance + ?
				WHERE user_id = ? AND year_month = ?
			`, amountDifference, amountDifference, userID, month)
		} else {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_bank_amount = previous_bank_amount + ?,
				    total_previous_balance = total_previous_balance + ?
				WHERE user_id = ? AND year_month = ?
			`, amountDifference, amountDifference, userID, month)
		}

		if err != nil {
			log.Printf("Error updating previous balances for month %s: %v", month, err)
		}
	}

	log.Printf("Updated previous balances from month %s (excluding start month) with difference %.2f",
		startMonth, amountDifference)
	return nil
}
