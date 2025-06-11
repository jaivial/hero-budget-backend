package main

import (
	"database/sql"
	"fmt"
	"log"
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
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// 1. Obtener datos de la factura y el locale del usuario
	var amount float64
	var paymentMethod, category, locale string
	err = tx.QueryRow(`
		SELECT b.amount, b.payment_method, b.category, COALESCE(u.locale, 'en') as locale
		FROM bills b
		JOIN users u ON b.user_id = CAST(u.id AS TEXT)
		WHERE b.id = ? AND b.user_id = ?
	`, billID, userID).Scan(&amount, &paymentMethod, &category, &locale)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	// Commit the current transaction to check for bill_payments
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing initial transaction: %v", err)
	}

	// 2. Verificar si existen registros de bill_payments, crear si no existen
	var paymentCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM bill_payments WHERE bill_id = ?
	`, billID).Scan(&paymentCount)
	if err != nil {
		return nil, fmt.Errorf("error checking payment records: %v", err)
	}

	if paymentCount == 0 {
		log.Printf("No payment records found for bill %d, creating retroactive records", billID)
		err = createBillPaymentRecordsRetroactive(db, billID)
		if err != nil {
			return nil, fmt.Errorf("error creating retroactive payment records: %v", err)
		}
	}

	// Start new transaction for the actual payment
	tx, err = db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting payment transaction: %v", err)
	}
	defer tx.Rollback()

	// 3. Verificar que el pago existe y no está pagado
	var alreadyPaid bool
	err = tx.QueryRow(`
		SELECT paid FROM bill_payments 
		WHERE bill_id = ? AND year_month = ?
	`, billID, yearMonth).Scan(&alreadyPaid)
	if err != nil {
		return nil, fmt.Errorf("payment record not found for bill %d in month %s: %v", billID, yearMonth, err)
	}
	if alreadyPaid {
		return nil, fmt.Errorf("bill for month %s is already paid", yearMonth)
	}

	// 3. Marcar pago como pagado en bill_payments
	_, err = tx.Exec(`
		UPDATE bill_payments
		SET paid = 1, payment_date = ?
		WHERE bill_id = ? AND year_month = ?
	`, paymentDate, billID, yearMonth)
	if err != nil {
		return nil, fmt.Errorf("error marking payment as paid: %v", err)
	}

	// 4. Restar el bill_amount para este mes específico en monthly_cash_bank_balance
	err = removeBillAmountFromMonth(tx, userID, yearMonth, amount, paymentMethod)
	if err != nil {
		return nil, fmt.Errorf("error removing bill amount: %v", err)
	}

	// 5. Crear registro en expenses para el pago de la factura
	err = createExpenseRecord(tx, userID, category, paymentDate, paymentMethod, locale, billID, amount)
	if err != nil {
		return nil, fmt.Errorf("error creating expense record: %v", err)
	}

	// 6. Verificar si todos los pagos están completados
	var totalPayments, paidPayments int
	err = tx.QueryRow(`
		SELECT COUNT(*) as total, SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END) as paid_count
		FROM bill_payments WHERE bill_id = ?
	`, billID).Scan(&totalPayments, &paidPayments)
	if err != nil {
		return nil, fmt.Errorf("error checking bill completion: %v", err)
	}

	// 7. Si todos los pagos están completados, marcar la factura como pagada
	billFullyPaid := false
	if totalPayments > 0 && paidPayments >= totalPayments {
		_, err = tx.Exec(`
			UPDATE bills SET paid = 1, updated_at = CURRENT_TIMESTAMP 
			WHERE id = ? AND user_id = ?
		`, billID, userID)
		if err != nil {
			return nil, fmt.Errorf("error updating bill status: %v", err)
		}
		billFullyPaid = true
	}

	// 8. Commit the transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}

	// 9. NO recalcular cascada para el mes del pago
	// Solo se modifican bill_*_amount y expense_*_amount
	// Las columnas principales de balance (bank_amount, cash_amount, etc.) no se tocan
	log.Printf("Payment processed for month %s - only bill_*_amount and expense_*_amount updated", yearMonth)

	// 10. Prepare response
	response := &PayBillResponse{
		BillID:            billID,
		UserID:            userID,
		YearMonth:         yearMonth,
		PaymentDate:       paymentDate,
		Amount:            amount,
		PaymentMethod:     paymentMethod,
		BillFullyPaid:     billFullyPaid,
		RemainingPayments: totalPayments - paidPayments,
	}

	return response, nil
}

// removeBillAmountFromMonth resta el importe del bill de las columnas bill_*
// y suma el importe a las columnas expense_* en monthly_cash_bank_balance
func removeBillAmountFromMonth(tx *sql.Tx, userID, yearMonth string, amount float64, paymentMethod string) error {
	if paymentMethod == "cash" {
		_, err := tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET bill_cash_amount = bill_cash_amount - ?,
			    expense_cash_amount = expense_cash_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error updating bill_cash_amount and expense_cash_amount: %v", err)
		}
	} else {
		_, err := tx.Exec(`
			UPDATE monthly_cash_bank_balance
			SET bill_bank_amount = bill_bank_amount - ?,
			    expense_bank_amount = expense_bank_amount + ?
			WHERE user_id = ? AND year_month = ?
		`, amount, amount, userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error updating bill_bank_amount and expense_bank_amount: %v", err)
		}
	}

	log.Printf("Moved bill amount %.2f from bill_%s to expense_%s for month %s (user %s)",
		amount, paymentMethod, paymentMethod, yearMonth, userID)
	return nil
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
		return fmt.Errorf("year month is required (format: YYYY-MM)")
	}

	// Validar formato de year_month
	_, err := time.Parse("2006-01", req.YearMonth)
	if err != nil {
		return fmt.Errorf("invalid year_month format, expected YYYY-MM: %v", err)
	}

	// Validar formato de payment_date si se proporciona
	if req.PaymentDate != "" {
		_, err := time.Parse("2006-01-02", req.PaymentDate)
		if err != nil {
			return fmt.Errorf("invalid payment_date format, expected YYYY-MM-DD: %v", err)
		}
	}

	return nil
}

// getBillPaymentStatus obtiene el estado de pagos de un bill específico
func getBillPaymentStatus(db *sql.DB, billID int, userID string) (map[string]interface{}, error) {
	// Obtener información básica del bill
	var billAmount float64
	var billName string
	var durationMonths int
	err := db.QueryRow(`
		SELECT name, amount, duration_months
		FROM bills WHERE id = ? AND user_id = ?
	`, billID, userID).Scan(&billName, &billAmount, &durationMonths)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	// Obtener estado de todos los pagos
	rows, err := db.Query(`
		SELECT year_month, paid, payment_date
		FROM bill_payments 
		WHERE bill_id = ? 
		ORDER BY year_month
	`, billID)
	if err != nil {
		return nil, fmt.Errorf("error fetching payment status: %v", err)
	}
	defer rows.Close()

	var payments []map[string]interface{}
	totalPayments := 0
	paidPayments := 0

	for rows.Next() {
		var yearMonth string
		var paid bool
		var paymentDate sql.NullString

		err := rows.Scan(&yearMonth, &paid, &paymentDate)
		if err != nil {
			return nil, fmt.Errorf("error scanning payment row: %v", err)
		}

		payment := map[string]interface{}{
			"year_month":   yearMonth,
			"paid":         paid,
			"payment_date": nil,
		}

		if paymentDate.Valid {
			payment["payment_date"] = paymentDate.String
		}

		payments = append(payments, payment)
		totalPayments++
		if paid {
			paidPayments++
		}
	}

	return map[string]interface{}{
		"bill_id":            billID,
		"bill_name":          billName,
		"bill_amount":        billAmount,
		"duration_months":    durationMonths,
		"total_payments":     totalPayments,
		"paid_payments":      paidPayments,
		"remaining_payments": totalPayments - paidPayments,
		"fully_paid":         paidPayments >= totalPayments && totalPayments > 0,
		"payments":           payments,
	}, nil
}

// createBillPaymentRecords crea registros en bill_payments para un bill nuevo
// Esta función se debe llamar cuando se crea un bill
func createBillPaymentRecords(db *sql.DB, billID int, userID string, startDate string, durationMonths int, paymentMethod string) error {
	// Parse start date
	currentDate, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("invalid start date format: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Crear un registro de bill_payments para cada mes de duración
	for i := 0; i < durationMonths; i++ {
		monthDate := currentDate.AddDate(0, i, 0)
		month := monthDate.Format("2006-01")

		_, err = tx.Exec(`
			INSERT INTO bill_payments (bill_id, year_month, paid, payment_date, payment_method)
			VALUES (?, ?, ?, ?, ?)
		`, billID, month, false, nil, paymentMethod)
		if err != nil {
			return fmt.Errorf("error creating bill payment record for month %s: %v", month, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing bill payment records: %v", err)
	}

	log.Printf("Created %d bill payment records for bill %d starting from %s",
		durationMonths, billID, startDate)
	return nil
}

// createBillPaymentRecordsRetroactive crea registros de bill_payments retroactivos
// para bills existentes que no los tienen
func createBillPaymentRecordsRetroactive(db *sql.DB, billID int) error {
	// Obtener información del bill
	var userID, startDate, paymentMethod string
	var durationMonths int
	err := db.QueryRow(`
		SELECT user_id, start_date, duration_months, payment_method
		FROM bills WHERE id = ?
	`, billID).Scan(&userID, &startDate, &durationMonths, &paymentMethod)
	if err != nil {
		return fmt.Errorf("bill not found: %v", err)
	}

	// Verificar si ya tiene registros de bill_payments
	var count int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM bill_payments WHERE bill_id = ?
	`, billID).Scan(&count)
	if err != nil {
		return fmt.Errorf("error checking existing payments: %v", err)
	}

	if count > 0 {
		log.Printf("Bill %d already has %d payment records", billID, count)
		return nil // Ya tiene registros
	}

	// Crear registros retroactivos
	err = createBillPaymentRecords(db, billID, userID, startDate, durationMonths, paymentMethod)
	if err != nil {
		return fmt.Errorf("error creating retroactive payment records: %v", err)
	}

	log.Printf("Created retroactive payment records for bill %d", billID)
	return nil
}

// getPaymentDescription retorna la descripción del pago en el idioma especificado
func getPaymentDescription(locale, category, date string) string {
	// Mapa de traducciones para "Pago Factura:"
	translations := map[string]string{
		"en":  "Bill payment:",
		"es":  "Pago factura:",
		"fr":  "Paiement de facture:",
		"de":  "Rechnungszahlung:",
		"it":  "Pagamento bolletta:",
		"pt":  "Pagamento conta:",
		"ru":  "Оплата счета:",
		"ja":  "請求書支払い:",
		"zh":  "账单支付:",
		"hi":  "बिल भुगतान:",
		"el":  "Πληρωμή λογαριασμού:",
		"nl":  "Rekening betaling:",
		"da":  "Regning betaling:",
		"gsw": "Rächnig zahlig:",
	}

	// Obtener la traducción o usar inglés por defecto
	prefix, exists := translations[locale]
	if !exists {
		prefix = translations["en"]
	}

	return fmt.Sprintf("%s %s %s", prefix, category, date)
}

// createExpenseRecord crea un registro en la tabla expenses para el pago de la factura
func createExpenseRecord(tx *sql.Tx, userID, category, paymentDate, paymentMethod, locale string, billID int, amount float64) error {
	// Crear la descripción del pago
	description := getPaymentDescription(locale, category, paymentDate)

	// Insertar el registro en expenses
	_, err := tx.Exec(`
		INSERT INTO expenses (user_id, amount, date, category, payment_method, description, bill_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, userID, amount, paymentDate, category, paymentMethod, description, billID)

	if err != nil {
		return fmt.Errorf("error creating expense record: %v", err)
	}

	log.Printf("Created expense record for bill payment: %s (amount: %.2f, bill_id: %d)",
		description, amount, billID)
	return nil
}
