package main

import (
	"database/sql"
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
func updateMonthlyBalanceForDeletedBill(db *sql.DB, billData BillData) error {
	log.Printf("🗑️ Eliminando factura: user_id=%s, monto=%.2f, duración=%d meses, fecha=%s, método=%s",
		billData.UserID, billData.Amount, billData.Duration, billData.StartDate, billData.PaymentMethod)

	// Si la duración es mayor a 1 mes, usar reversión cascada
	if billData.Duration > 1 {
		log.Printf("🗑️ Factura multi-mes detectada, usando reversión cascada")
		return revertCascadeBillBalance(db, billData.UserID, billData.StartDate, billData.Duration, billData.Amount, billData.PaymentMethod)
	}

	// Para facturas de un solo mes, usar lógica simple
	log.Printf("🗑️ Factura de un solo mes, usando lógica simple")
	// Parse the start date to get the year_month
	startTime, err := time.Parse("2006-01-02", billData.StartDate)
	if err != nil {
		// Intentar con formato ISO si falla el formato simple
		startTime, err = time.Parse("2006-01-02T15:04:05Z", billData.StartDate)
		if err != nil {
			log.Printf("🗑️ Error parseando fecha: %v", err)
			return fmt.Errorf("error parsing start date: %v", err)
		}
	}

	yearMonth := startTime.Format("2006-01")
	log.Printf("🗑️ Mes calculado para eliminación: %s", yearMonth)

	// Revertir el balance para el mes único
	updateBalanceColumns(db, billData.UserID, yearMonth, billData.Amount, billData.PaymentMethod, "bill", -1)

	// Para facturas de un mes, sumar de vuelta el importe al balance correspondiente
	var updateQuery string
	if billData.PaymentMethod == "bank" {
		updateQuery = `
			UPDATE monthly_balance 
			SET bank_amount = bank_amount + ?, 
			    balance_bank_amount = balance_bank_amount + ?, 
			    total_balance = cash_amount + (bank_amount + ?)
			WHERE user_id = ? AND year_month = ?`
	} else {
		updateQuery = `
			UPDATE monthly_balance 
			SET cash_amount = cash_amount + ?, 
			    balance_cash_amount = balance_cash_amount + ?, 
			    total_balance = (cash_amount + ?) + bank_amount
			WHERE user_id = ? AND year_month = ?`
	}

	_, err = db.Exec(updateQuery, billData.Amount, billData.Amount, billData.Amount, billData.UserID, yearMonth)
	if err != nil {
		log.Printf("🗑️ Error actualizando balance tras eliminación: %v", err)
		return err
	}

	log.Printf("✅ Factura de un solo mes eliminada exitosamente")
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
		expenseAmountCol = "expense_amount"
		billAmountCol = "bills_amount"
	} else {
		expenseAmountCol = "expense_amount"
		billAmountCol = "bills_amount"
	}

	var query string
	if isExpenseMonth {
		// Month with expense: subtract from expense amount only
		query = fmt.Sprintf(`UPDATE monthly_balance 
			SET %s = %s - ?
			WHERE year_month = ? AND user_id = ?`,
			expenseAmountCol, expenseAmountCol)
	} else {
		// Month without expense: subtract from bill amount only
		query = fmt.Sprintf(`UPDATE monthly_balance 
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
