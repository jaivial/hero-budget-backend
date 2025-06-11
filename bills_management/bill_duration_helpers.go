package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// updateBillDurationLogic maneja toda la lógica de cambios de duración
func updateBillDurationLogic(db *sql.DB, updateData BillUpdateData) error {
	if updateData.OldDurationMonths == updateData.NewDurationMonths {
		log.Printf("Duration unchanged, skipping duration update logic")
		return nil
	}

	log.Printf("Duration changed from %d to %d months",
		updateData.OldDurationMonths, updateData.NewDurationMonths)

	// Calcular meses antiguos y nuevos
	oldMonths, err := calculateMonthsFromDuration(updateData.OldStartDate, updateData.OldDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating old months: %v", err)
	}

	newMonths, err := calculateMonthsFromDuration(updateData.NewStartDate, updateData.NewDurationMonths)
	if err != nil {
		return fmt.Errorf("error calculating new months: %v", err)
	}

	// Determinar meses que se eliminan y meses que se añaden
	removedMonths := findRemovedMonths(oldMonths, newMonths)
	addedMonths := findAddedMonths(oldMonths, newMonths)

	log.Printf("Removed months: %v", removedMonths)
	log.Printf("Added months: %v", addedMonths)

	// Procesar meses eliminados
	if len(removedMonths) > 0 {
		err = processRemovedMonths(db, updateData, removedMonths)
		if err != nil {
			return fmt.Errorf("error processing removed months: %v", err)
		}
	}

	// Procesar meses añadidos
	if len(addedMonths) > 0 {
		err = processAddedMonths(db, updateData, addedMonths)
		if err != nil {
			return fmt.Errorf("error processing added months: %v", err)
		}
	}

	return nil
}

// calculateMonthsFromDuration calcula todos los meses afectados por una duración
func calculateMonthsFromDuration(startDate string, durationMonths int) ([]string, error) {
	parsedDate, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date %s: %v", startDate, err)
	}

	var months []string
	for i := 0; i < durationMonths; i++ {
		monthDate := parsedDate.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		months = append(months, yearMonth)
	}

	return months, nil
}

// findRemovedMonths encuentra los meses que están en oldMonths pero no en newMonths
func findRemovedMonths(oldMonths, newMonths []string) []string {
	newMonthsMap := make(map[string]bool)
	for _, month := range newMonths {
		newMonthsMap[month] = true
	}

	var removedMonths []string
	for _, month := range oldMonths {
		if !newMonthsMap[month] {
			removedMonths = append(removedMonths, month)
		}
	}

	return removedMonths
}

// findAddedMonths encuentra los meses que están en newMonths pero no en oldMonths
func findAddedMonths(oldMonths, newMonths []string) []string {
	oldMonthsMap := make(map[string]bool)
	for _, month := range oldMonths {
		oldMonthsMap[month] = true
	}

	var addedMonths []string
	for _, month := range newMonths {
		if !oldMonthsMap[month] {
			addedMonths = append(addedMonths, month)
		}
	}

	return addedMonths
}

// processRemovedMonths maneja la lógica cuando se eliminan meses de la duración
func processRemovedMonths(db *sql.DB, updateData BillUpdateData, removedMonths []string) error {
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

	for _, month := range removedMonths {
		log.Printf("Processing removed month: %s", month)

		// Si este mes TIENE expenses, actualizar tabla expenses y expense_* columns
		if expenseMonths[month] {
			// Restar de la tabla expenses
			err := subtractExpenseAmountForBill(db, updateData.BillID, updateData.UserID, month, updateData.OldAmount)
			if err != nil {
				log.Printf("Error subtracting expense amount for month %s: %v", month, err)
				continue
			}

			// Restar de expense_* columns en monthly_cash_bank_balance
			err = subtractExpenseAmountInMonthlyBalance(db, updateData.UserID, month,
				updateData.OldAmount, updateData.OldPaymentMethod)
			if err != nil {
				log.Printf("Error subtracting expense columns for month %s: %v", month, err)
			}

			// Restar de las columnas principales también para meses con expenses
			err = subtractFromMainBalanceColumns(db, updateData.UserID, month,
				updateData.OldAmount, updateData.OldPaymentMethod)
			if err != nil {
				log.Printf("Error subtracting from main balance columns for month %s: %v", month, err)
			}

			log.Printf("Subtracted from expense records for month %s (has expenses)", month)
		} else {
			// Solo procesar si este mes NO tiene expenses
			// Restar el importe de las columnas correspondientes
			err := subtractBillAmountFromMonth(db, updateData.UserID, month,
				updateData.OldAmount, updateData.OldPaymentMethod)
			if err != nil {
				log.Printf("Error subtracting bill amount for month %s: %v", month, err)
				continue
			}

			// Restar de las columnas principales
			err = subtractFromMainBalanceColumns(db, updateData.UserID, month,
				updateData.OldAmount, updateData.OldPaymentMethod)
			if err != nil {
				log.Printf("Error subtracting from main balance columns for month %s: %v", month, err)
			}

			log.Printf("Subtracted from bill records for month %s (no expenses)", month)
		}
	}

	// Actualizar previous_* en cascada desde el primer mes eliminado
	if len(removedMonths) > 0 {
		earliestRemovedMonth := findEarliestMonth(removedMonths)
		err := updatePreviousBalancesFromMonth(db, updateData.UserID, earliestRemovedMonth,
			-updateData.OldAmount, updateData.OldPaymentMethod)
		if err != nil {
			log.Printf("Error updating previous balances from month %s: %v", earliestRemovedMonth, err)
		}
	}

	return nil
}

// processAddedMonths maneja la lógica cuando se añaden meses a la duración
func processAddedMonths(db *sql.DB, updateData BillUpdateData, addedMonths []string) error {
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

	for _, month := range addedMonths {
		log.Printf("Processing added month: %s", month)

		// Crear registro si no existe
		_, err := db.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
			VALUES (?, ?)
		`, updateData.UserID, month)
		if err != nil {
			log.Printf("Error creating monthly record for %s: %v", month, err)
			continue
		}

		// Si este mes TIENE expenses, actualizar tabla expenses y expense_* columns
		if expenseMonths[month] {
			// Añadir a la tabla expenses
			err = addExpenseAmountForBill(db, updateData.BillID, updateData.UserID, month, updateData.NewAmount)
			if err != nil {
				log.Printf("Error adding expense amount for month %s: %v", month, err)
				continue
			}

			// Añadir a expense_* columns en monthly_cash_bank_balance
			err = addExpenseAmountInMonthlyBalance(db, updateData.UserID, month,
				updateData.NewAmount, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error adding expense columns for month %s: %v", month, err)
			}

			// Añadir a las columnas principales también para meses con expenses
			err = addToMainBalanceColumns(db, updateData.UserID, month,
				updateData.NewAmount, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error adding to main balance columns for month %s: %v", month, err)
			}

			log.Printf("Added to expense records for month %s (has expenses)", month)
		} else {
			// Solo procesar si este mes NO tiene expenses
			// Añadir el importe a las columnas correspondientes
			err = addBillAmountToMonth(db, updateData.UserID, month,
				updateData.NewAmount, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error adding bill amount for month %s: %v", month, err)
				continue
			}

			// Añadir a las columnas principales
			err = addToMainBalanceColumns(db, updateData.UserID, month,
				updateData.NewAmount, updateData.NewPaymentMethod)
			if err != nil {
				log.Printf("Error adding to main balance columns for month %s: %v", month, err)
			}

			log.Printf("Added to bill records for month %s (no expenses)", month)
		}
	}

	return nil
}

// subtractBillAmountFromMonth resta el importe de bill de un mes específico
func subtractBillAmountFromMonth(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

	var column string
	if paymentMethod == "cash" {
		column = "bill_cash_amount"
	} else {
		column = "bill_bank_amount"
	}

	_, err := db.Exec(fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s - ? 
		WHERE user_id = ? AND year_month = ?
	`, column, column), amount, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error subtracting from %s: %v", column, err)
	}

	log.Printf("Subtracted %.2f from %s for user %s in month %s",
		amount, column, userID, yearMonth)
	return nil
}

// addBillAmountToMonth añade el importe de bill a un mes específico
func addBillAmountToMonth(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

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
	`, column, column), amount, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error adding to %s: %v", column, err)
	}

	log.Printf("Added %.2f to %s for user %s in month %s",
		amount, column, userID, yearMonth)
	return nil
}

// subtractFromMainBalanceColumns resta de las columnas principales de balance
func subtractFromMainBalanceColumns(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

	if paymentMethod == "cash" {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET cash_amount = cash_amount - ?,
			    balance_cash_amount = balance_cash_amount - ?,
			    total_balance = total_balance - ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, amount, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error subtracting from cash balance columns: %v", err)
		}
	} else {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET bank_amount = bank_amount - ?,
			    balance_bank_amount = balance_bank_amount - ?,
			    total_balance = total_balance - ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, amount, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error subtracting from bank balance columns: %v", err)
		}
	}

	log.Printf("Subtracted %.2f from main balance columns for user %s in month %s",
		amount, userID, yearMonth)
	return nil
}

// addToMainBalanceColumns añade a las columnas principales de balance
func addToMainBalanceColumns(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

	if paymentMethod == "cash" {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET cash_amount = cash_amount + ?,
			    balance_cash_amount = balance_cash_amount + ?,
			    total_balance = total_balance + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, amount, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error adding to cash balance columns: %v", err)
		}
	} else {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET bank_amount = bank_amount + ?,
			    balance_bank_amount = balance_bank_amount + ?,
			    total_balance = total_balance + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, amount, userID, yearMonth)

		if err != nil {
			return fmt.Errorf("error adding to bank balance columns: %v", err)
		}
	}

	log.Printf("Added %.2f to main balance columns for user %s in month %s",
		amount, userID, yearMonth)
	return nil
}

// findEarliestMonth encuentra el mes más temprano en una lista
func findEarliestMonth(months []string) string {
	if len(months) == 0 {
		return ""
	}

	earliest := months[0]
	for _, month := range months[1:] {
		if month < earliest {
			earliest = month
		}
	}

	return earliest
}

// updatePreviousBalancesFromMonth actualiza las columnas previous_* desde un mes específico
func updatePreviousBalancesFromMonth(db *sql.DB, userID, startMonth string,
	amountDifference float64, paymentMethod string) error {

	// Obtener todos los meses posteriores al startMonth
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

	// Actualizar previous_* para cada mes posterior
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

	log.Printf("Updated previous balances from month %s with difference %.2f",
		startMonth, amountDifference)
	return nil
}

// subtractExpenseAmountForBill resta el amount en la tabla expenses para un bill específico
func subtractExpenseAmountForBill(db *sql.DB, billID int, userID, yearMonth string, amount float64) error {
	// Restar de todos los expenses de este bill en el mes especificado
	result, err := db.Exec(`
		UPDATE expenses 
		SET amount = amount - ? 
		WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?
	`, amount, billID, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error subtracting expenses amount: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected for expense subtraction: %v", err)
	} else {
		log.Printf("Subtracted %.2f from %d expense records for bill %d in month %s",
			amount, rowsAffected, billID, yearMonth)
	}

	return nil
}

// subtractExpenseAmountInMonthlyBalance resta las columnas expense_* en monthly_cash_bank_balance
func subtractExpenseAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

	var column string
	if paymentMethod == "cash" {
		column = "expense_cash_amount"
	} else {
		column = "expense_bank_amount"
	}

	_, err := db.Exec(fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s - ? 
		WHERE user_id = ? AND year_month = ?
	`, column, column), amount, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error subtracting from %s: %v", column, err)
	}

	log.Printf("Subtracted %.2f from %s for user %s in month %s",
		amount, column, userID, yearMonth)
	return nil
}

// addExpenseAmountForBill añade el amount en la tabla expenses para un bill específico
func addExpenseAmountForBill(db *sql.DB, billID int, userID, yearMonth string, amount float64) error {
	// Añadir a todos los expenses de este bill en el mes especificado
	result, err := db.Exec(`
		UPDATE expenses 
		SET amount = amount + ? 
		WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?
	`, amount, billID, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error adding expenses amount: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Could not get rows affected for expense addition: %v", err)
	} else {
		log.Printf("Added %.2f to %d expense records for bill %d in month %s",
			amount, rowsAffected, billID, yearMonth)
	}

	return nil
}

// addExpenseAmountInMonthlyBalance añade a las columnas expense_* en monthly_cash_bank_balance
func addExpenseAmountInMonthlyBalance(db *sql.DB, userID, yearMonth string,
	amount float64, paymentMethod string) error {

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
	`, column, column), amount, userID, yearMonth)

	if err != nil {
		return fmt.Errorf("error updating %s: %v", column, err)
	}

	log.Printf("Added %.2f to %s for user %s in month %s",
		amount, column, userID, yearMonth)
	return nil
}

// addNewBillToMonthlyBalance agrega un nuevo bill a monthly_cash_bank_balance
// y actualiza la cascada de balances desde el mes de inicio hacia adelante
func addNewBillToMonthlyBalance(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	// Calcular meses afectados
	affectedMonths, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		return fmt.Errorf("error calculating affected months: %v", err)
	}

	// Para cada mes afectado, agregar el importe del bill
	for _, month := range affectedMonths {
		// Crear registro en monthly_cash_bank_balance si no existe
		_, err := db.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
			VALUES (?, ?)
		`, userID, month)
		if err != nil {
			return fmt.Errorf("error creating monthly balance record for month %s: %v", month, err)
		}

		err = addBillAmountToMonth(db, userID, month, amount, paymentMethod)
		if err != nil {
			return fmt.Errorf("error adding to bill amount for month %s: %v", month, err)
		}
	}

	// Ejecutar cascada desde el primer mes afectado
	if len(affectedMonths) > 0 {
		err = updateCascadeBalancesFromMonth(db, userID, affectedMonths[0])
		if err != nil {
			return fmt.Errorf("error updating cascade balances: %v", err)
		}
	}

	return nil
}

// updateCascadeBalancesFromMonth actualiza los saldos en cascada desde startMonth hacia adelante
// siguiendo la misma lógica que updateCascadeBalances de main.go
func updateCascadeBalancesFromMonth(db *sql.DB, userID string, startMonth string) error {
	// Obtener todos los meses posteriores o iguales a startMonth
	rows, err := db.Query(`
		SELECT year_month FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months: %v", err)
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return fmt.Errorf("error scanning month: %v", err)
		}
		months = append(months, month)
	}

	for i, month := range months {
		// Obtener el mes anterior (si existe)
		var previousMonth string
		if i > 0 {
			previousMonth = months[i-1]
		} else if month != startMonth {
			row := db.QueryRow(`
				SELECT year_month FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month < ? ORDER BY year_month DESC LIMIT 1
			`, userID, month)
			if err := row.Scan(&previousMonth); err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous month: %v", err)
			}
		}

		// Obtener saldos previos
		var previousCashAmount, previousBankAmount, totalPreviousBalance float64
		if previousMonth != "" {
			err := db.QueryRow(`
				SELECT cash_amount, bank_amount, total_balance
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, previousMonth).Scan(&previousCashAmount, &previousBankAmount, &totalPreviousBalance)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error fetching previous balances: %v", err)
			}
		}

		// Obtener movimientos del mes actual
		var incomeCash, incomeBank, expenseCash, expenseBank, billCash, billBank float64
		err := db.QueryRow(`
			SELECT income_cash_amount, income_bank_amount,
			       expense_cash_amount, expense_bank_amount,
			       bill_cash_amount, bill_bank_amount
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`, userID, month).Scan(&incomeCash, &incomeBank, &expenseCash, &expenseBank, &billCash, &billBank)
		if err != nil {
			return fmt.Errorf("error fetching current month data: %v", err)
		}

		// Calcular saldos del mes actual (cascada acumulativa)
		cashAmount := previousCashAmount + incomeCash - expenseCash - billCash
		bankAmount := previousBankAmount + incomeBank - expenseBank - billBank
		balanceCashAmount := cashAmount
		balanceBankAmount := bankAmount
		totalBalance := balanceCashAmount + balanceBankAmount

		// Actualizar registro con cascada completa
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance
			SET cash_amount = ?,
			    bank_amount = ?,
			    balance_cash_amount = ?,
			    balance_bank_amount = ?,
			    total_balance = ?,
			    previous_cash_amount = ?,
			    previous_bank_amount = ?,
			    total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, cashAmount, bankAmount, balanceCashAmount, balanceBankAmount,
			totalBalance, previousCashAmount, previousBankAmount, totalPreviousBalance,
			userID, month)
		if err != nil {
			return fmt.Errorf("error updating balance for month %s: %v", month, err)
		}
	}

	return nil
}
