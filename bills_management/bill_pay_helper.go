package main

import (
	"database/sql"
	"fmt"
	"time"
)

// PayBillRequest represents the request structure for paying a bill
type PayBillRequest struct {
	UserID      string `json:"user_id"`
	BillID      int    `json:"bill_id"`
	YearMonth   string `json:"year_month"`   // Format: "2025-01"
	PaymentDate string `json:"payment_date"` // Format: "2025-01-15" (optional, defaults to current date)
}

// PayBillResponse represents the response structure for bill payment
type PayBillResponse struct {
	BillID            int     `json:"bill_id"`
	UserID            string  `json:"user_id"`
	YearMonth         string  `json:"year_month"`
	PaymentDate       string  `json:"payment_date"`
	Amount            float64 `json:"amount"`
	PaymentMethod     string  `json:"payment_method"`
	BillFullyPaid     bool    `json:"bill_fully_paid"`
	RemainingPayments int     `json:"remaining_payments"`
}

// markBillPaid marca una factura como pagada para un mes específico
// y actualiza la cascada de balances
func markBillPaid(db *sql.DB, billID int, userID, yearMonth, paymentDate string) (*PayBillResponse, error) {
	if paymentDate == "" {
		paymentDate = time.Now().Format("2006-01-02")
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	var amount float64
	var paymentMethod, category, locale string
	err = tx.QueryRow("SELECT b.amount, b.payment_method, b.category, COALESCE(u.locale, 'en') as locale FROM bills b JOIN users u ON b.user_id = CAST(u.id AS TEXT) WHERE b.id = ? AND b.user_id = ?", billID, userID).Scan(&amount, &paymentMethod, &category, &locale)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing initial transaction: %v", err)
	}

	var paymentCount int
	err = db.QueryRow("SELECT COUNT(*) FROM bill_payments WHERE bill_id = ?", billID).Scan(&paymentCount)
	if err != nil {
		return nil, fmt.Errorf("error checking payment records: %v", err)
	}

	if paymentCount == 0 {
		if err = createBillPaymentRecordsRetroactive(db, billID); err != nil {
			return nil, fmt.Errorf("error creating retroactive payment records: %v", err)
		}
	}

	tx, err = db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting payment transaction: %v", err)
	}
	defer tx.Rollback()

	var alreadyPaid bool
	err = tx.QueryRow("SELECT paid FROM bill_payments WHERE bill_id = ? AND year_month = ?", billID, yearMonth).Scan(&alreadyPaid)
	if err != nil {
		return nil, fmt.Errorf("payment record not found for bill %d in month %s: %v", billID, yearMonth, err)
	}
	if alreadyPaid {
		return nil, fmt.Errorf("bill for month %s is already paid", yearMonth)
	}

	_, err = tx.Exec("UPDATE bill_payments SET paid = 1, payment_date = ? WHERE bill_id = ? AND year_month = ?", paymentDate, billID, yearMonth)
	if err != nil {
		return nil, fmt.Errorf("error marking payment as paid: %v", err)
	}

	if err = removeBillAmountFromMonth(tx, userID, yearMonth, amount, paymentMethod); err != nil {
		return nil, fmt.Errorf("error removing bill amount: %v", err)
	}

	if err = createExpenseRecord(tx, userID, category, paymentDate, paymentMethod, locale, billID, amount); err != nil {
		return nil, fmt.Errorf("error creating expense record: %v", err)
	}

	var totalPayments, paidPayments int
	err = tx.QueryRow("SELECT COUNT(*) as total, SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END) as paid_count FROM bill_payments WHERE bill_id = ?", billID).Scan(&totalPayments, &paidPayments)
	if err != nil {
		return nil, fmt.Errorf("error checking bill completion: %v", err)
	}

	billFullyPaid := false
	if totalPayments > 0 && paidPayments >= totalPayments {
		_, err = tx.Exec("UPDATE bills SET paid = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?", billID, userID)
		if err != nil {
			return nil, fmt.Errorf("error updating bill status: %v", err)
		}
		billFullyPaid = true
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}

	return &PayBillResponse{
		BillID: billID, UserID: userID, YearMonth: yearMonth, PaymentDate: paymentDate,
		Amount: amount, PaymentMethod: paymentMethod, BillFullyPaid: billFullyPaid,
		RemainingPayments: totalPayments - paidPayments,
	}, nil
}

// removeBillAmountFromMonth resta el importe del bill de las columnas bill_*
// y suma el importe a las columnas expense_* en monthly_cash_bank_balance
func removeBillAmountFromMonth(tx *sql.Tx, userID, yearMonth string, amount float64, paymentMethod string) error {
	var query string
	if paymentMethod == "cash" {
		query = "UPDATE monthly_cash_bank_balance SET bill_cash_amount = bill_cash_amount - ?, expense_cash_amount = expense_cash_amount + ? WHERE user_id = ? AND year_month = ?"
	} else {
		query = "UPDATE monthly_cash_bank_balance SET bill_bank_amount = bill_bank_amount - ?, expense_bank_amount = expense_bank_amount + ? WHERE user_id = ? AND year_month = ?"
	}
	_, err := tx.Exec(query, amount, amount, userID, yearMonth)
	return err
}

// validatePayBillRequest valida los datos de la request de pago
func validatePayBillRequest(req PayBillRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if req.BillID <= 0 {
		return fmt.Errorf("valid bill ID is required")
	}
	if req.YearMonth == "" {
		return fmt.Errorf("year_month is required")
	}
	return nil
}

// getBillPaymentStatus obtiene el estado de pagos de un bill específico
func getBillPaymentStatus(db *sql.DB, billID int, userID string) (map[string]interface{}, error) {
	var billName string
	var totalAmount float64
	var startDate, paymentMethod string
	var durationMonths int

	err := db.QueryRow("SELECT name, amount, start_date, duration_months, payment_method FROM bills WHERE id = ? AND user_id = ?", billID, userID).Scan(&billName, &totalAmount, &startDate, &durationMonths, &paymentMethod)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	rows, err := db.Query("SELECT year_month, paid, payment_date FROM bill_payments WHERE bill_id = ? ORDER BY year_month", billID)
	if err != nil {
		return nil, fmt.Errorf("error fetching payment records: %v", err)
	}
	defer rows.Close()

	var payments []map[string]interface{}
	var totalPaid, remainingPayments int

	for rows.Next() {
		var yearMonth, paymentDate string
		var paid bool
		if err := rows.Scan(&yearMonth, &paid, &paymentDate); err != nil {
			continue
		}

		payment := map[string]interface{}{
			"year_month":   yearMonth,
			"paid":         paid,
			"amount":       totalAmount,
			"payment_date": paymentDate,
		}
		payments = append(payments, payment)

		if paid {
			totalPaid++
		} else {
			remainingPayments++
		}
	}

	return map[string]interface{}{
		"bill_id":            billID,
		"bill_name":          billName,
		"amount":             totalAmount,
		"payment_method":     paymentMethod,
		"total_payments":     len(payments),
		"paid_payments":      totalPaid,
		"remaining_payments": remainingPayments,
		"payments":           payments,
	}, nil
}

// createBillPaymentRecords crea registros en bill_payments para un bill nuevo
// Esta función se debe llamar cuando se crea un bill
func createBillPaymentRecords(db *sql.DB, billID int, userID string, startDate string, durationMonths int, paymentMethod string) error {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}

	for i := 0; i < durationMonths; i++ {
		monthDate := startTime.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")
		_, err = db.Exec("INSERT OR IGNORE INTO bill_payments (bill_id, year_month, paid, payment_method) VALUES (?, ?, 0, ?)", billID, yearMonth, paymentMethod)
		if err != nil {
			return fmt.Errorf("error creating payment record for month %s: %v", yearMonth, err)
		}
	}
	return nil
}

// createBillPaymentRecordsRetroactive crea registros de bill_payments retroactivos
// para bills existentes que no los tienen
func createBillPaymentRecordsRetroactive(db *sql.DB, billID int) error {
	var userID string
	var startDate string
	var durationMonths int
	var paymentMethod string

	err := db.QueryRow("SELECT user_id, start_date, duration_months, payment_method FROM bills WHERE id = ?", billID).Scan(&userID, &startDate, &durationMonths, &paymentMethod)
	if err != nil {
		return fmt.Errorf("error fetching bill data: %v", err)
	}

	return createBillPaymentRecords(db, billID, userID, startDate, durationMonths, paymentMethod)
}

// getPaymentDescription retorna la descripción del pago en el idioma especificado
func getPaymentDescription(locale, category, date string) string {
	descriptions := map[string]map[string]string{
		"en": {"general": "Bill payment", "utilities": "Utility payment", "rent": "Rent payment", "insurance": "Insurance payment"},
		"es": {"general": "Pago de factura", "utilities": "Pago de servicios", "rent": "Pago de alquiler", "insurance": "Pago de seguro"},
		"fr": {"general": "Paiement de facture", "utilities": "Paiement de services", "rent": "Paiement de loyer", "insurance": "Paiement d'assurance"},
	}

	if localeDesc, exists := descriptions[locale]; exists {
		if desc, exists := localeDesc[category]; exists {
			return desc
		}
		return localeDesc["general"]
	}
	return descriptions["en"]["general"]
}

// createExpenseRecord crea un registro en la tabla expenses para el pago de la factura
func createExpenseRecord(tx *sql.Tx, userID, category, paymentDate, paymentMethod, locale string, billID int, amount float64) error {
	description := getPaymentDescription(locale, category, paymentDate)
	_, err := tx.Exec("INSERT INTO expenses (user_id, category, amount, date, description, payment_method, bill_id) VALUES (?, ?, ?, ?, ?, ?, ?)", userID, category, amount, paymentDate, description, paymentMethod, billID)
	return err
}
