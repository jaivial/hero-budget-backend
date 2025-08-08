package main

import (
	"database/sql"
	"fmt"
)

// validatePayBillRequest valida los datos de la request de pago
// Verifica que todos los campos requeridos estén presentes y sean válidos
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
// Proporciona información detallada sobre todos los pagos de una factura
func getBillPaymentStatus(db *sql.DB, billID int, userID string) (map[string]interface{}, error) {
	// Obtener información básica de la factura
	var billName string
	var totalAmount float64
	var startDate, paymentMethod string
	var durationMonths int

	err := db.QueryRow("SELECT name, amount, start_date, duration_months, payment_method FROM bills WHERE id = ? AND user_id = ?", billID, userID).Scan(&billName, &totalAmount, &startDate, &durationMonths, &paymentMethod)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	// Obtener registros de pagos
	rows, err := db.Query("SELECT year_month, paid, payment_date FROM bill_payments WHERE bill_id = ? ORDER BY year_month", billID)
	if err != nil {
		return nil, fmt.Errorf("error fetching payment records: %v", err)
	}
	defer rows.Close()

	// Procesar registros de pagos
	var payments []map[string]interface{}
	var totalPaid, remainingPayments int

	for rows.Next() {
		var yearMonth, paymentDate string
		var paid bool
		if err := rows.Scan(&yearMonth, &paid, &paymentDate); err != nil {
			continue
		}

		// Crear objeto de pago
		payment := map[string]interface{}{
			"year_month":   yearMonth,
			"paid":         paid,
			"amount":       totalAmount,
			"payment_date": paymentDate,
		}
		payments = append(payments, payment)

		// Contar pagos realizados y pendientes
		if paid {
			totalPaid++
		} else {
			remainingPayments++
		}
	}

	// Retornar estado completo de los pagos
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
// CORREGIDO: Usa meses consecutivos correctos
func createBillPaymentRecords(db *sql.DB, billID int, userID string, startDate string, durationMonths int, paymentMethod string) error {
	// CORREGIDO: Usar calculateMonthsFromDuration para meses consecutivos
	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		return fmt.Errorf("error calculating months: %v", err)
	}

	// Crear registro para cada mes del periodo
	for _, yearMonth := range months {
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
	// Obtener datos de la factura
	var userID string
	var startDate string
	var durationMonths int
	var paymentMethod string

	err := db.QueryRow("SELECT user_id, start_date, duration_months, payment_method FROM bills WHERE id = ?", billID).Scan(&userID, &startDate, &durationMonths, &paymentMethod)
	if err != nil {
		return fmt.Errorf("error fetching bill data: %v", err)
	}

	// Crear registros de pago using la función corregida
	return createBillPaymentRecords(db, billID, userID, startDate, durationMonths, paymentMethod)
}

// getPaymentDescription retorna la descripción del pago en el idioma especificado
// Proporciona descripciones localizadas para diferentes tipos de facturas
func getPaymentDescription(locale, category, date string) string {
	// Diccionario de descripciones por idioma y categoría
	descriptions := map[string]map[string]string{
		"en": {
			"general":   "Bill payment",
			"utilities": "Utility payment",
			"rent":      "Rent payment",
			"insurance": "Insurance payment",
		},
		"es": {
			"general":   "Pago de factura",
			"utilities": "Pago de servicios",
			"rent":      "Pago de alquiler",
			"insurance": "Pago de seguro",
		},
		"fr": {
			"general":   "Paiement de facture",
			"utilities": "Paiement de services",
			"rent":      "Paiement de loyer",
			"insurance": "Paiement d'assurance",
		},
	}

	// Buscar descripción por idioma y categoría
	if localeDesc, exists := descriptions[locale]; exists {
		if desc, exists := localeDesc[category]; exists {
			return desc
		}
		// Si no existe la categoría, usar descripción general
		return localeDesc["general"]
	}
	// Si no existe el idioma, usar inglés por defecto
	return descriptions["en"]["general"]
}

// undoBillPayment deshace el pago de una factura
// Útil para correcciones o cancelaciones de pagos
func undoBillPayment(db *sql.DB, billID int, userID, yearMonth string) error {
	// Iniciar transacción
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Obtener información de la factura
	var amount float64
	var paymentMethod string
	err = tx.QueryRow("SELECT amount, payment_method FROM bills WHERE id = ? AND user_id = ?", billID, userID).Scan(&amount, &paymentMethod)
	if err != nil {
		return fmt.Errorf("bill not found: %v", err)
	}

	// Verificar que el pago existe y está marcado como pagado
	var paid bool
	err = tx.QueryRow("SELECT paid FROM bill_payments WHERE bill_id = ? AND year_month = ?", billID, yearMonth).Scan(&paid)
	if err != nil {
		return fmt.Errorf("payment record not found: %v", err)
	}
	if !paid {
		return fmt.Errorf("payment for month %s is not marked as paid", yearMonth)
	}

	// Deshacer el pago en bill_payments
	_, err = tx.Exec("UPDATE bill_payments SET paid = 0, payment_date = NULL WHERE bill_id = ? AND year_month = ?", billID, yearMonth)
	if err != nil {
		return fmt.Errorf("error undoing payment status: %v", err)
	}

	// Revertir transferencia en monthly_balance
	if err = revertBillAmountToMonth(tx, userID, yearMonth, amount, paymentMethod); err != nil {
		return fmt.Errorf("error reverting bill amount: %v", err)
	}

	// Eliminar registro de expense asociado
	_, err = tx.Exec("DELETE FROM expenses WHERE bill_id = ? AND user_id = ? AND strftime('%Y-%m', date) = ?", billID, userID, yearMonth)
	if err != nil {
		return fmt.Errorf("error removing expense record: %v", err)
	}

	// CORREGIDO: Actualizar estado de la factura basándose en duration_months
	// Primero obtener duration_months para calcular si la factura sigue estando completamente pagada
	var durationMonths int
	err = tx.QueryRow("SELECT duration_months FROM bills WHERE id = ? AND user_id = ?", billID, userID).Scan(&durationMonths)
	if err != nil {
		return fmt.Errorf("error fetching bill duration: %v", err)
	}

	// Contar cuántos pagos quedan pagados después de deshacer este pago
	var paidPayments int
	err = tx.QueryRow("SELECT SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END) as paid_count FROM bill_payments WHERE bill_id = ?", billID).Scan(&paidPayments)
	if err != nil {
		return fmt.Errorf("error counting paid payments: %v", err)
	}

	// Actualizar estado de la factura: solo marcar como no pagada si no tiene todos los pagos esperados
	billPaidStatus := 0
	if durationMonths > 0 && paidPayments >= durationMonths {
		billPaidStatus = 1
	}

	_, err = tx.Exec("UPDATE bills SET paid = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?", billPaidStatus, billID, userID)
	if err != nil {
		return fmt.Errorf("error updating bill status: %v", err)
	}

	// Confirmar transacción
	return tx.Commit()
}

// revertBillAmountToMonth revierte la transferencia de expense_amount a bill_amount
// Inverso de removeBillAmountFromMonth
func revertBillAmountToMonth(tx *sql.Tx, userID, yearMonth string, amount float64, paymentMethod string) error {
	// Determinar columnas según método de pago
	var billColumn, expenseColumn string
	if paymentMethod == "cash" {
		billColumn = "bills_amount"
		expenseColumn = "expense_amount"
	} else {
		billColumn = "bills_amount"
		expenseColumn = "expense_amount"
	}

	// Revertir transferencia: de expense_amount a bill_amount
	query := fmt.Sprintf(`
		UPDATE monthly_balance 
		SET %s = %s + ?, 
		    %s = %s - ?
		WHERE user_id = ? AND year_month = ?
	`, billColumn, billColumn, expenseColumn, expenseColumn)

	_, err := tx.Exec(query, amount, amount, userID, yearMonth)
	if err != nil {
		return fmt.Errorf("error reverting expense amount to bill: %v", err)
	}

	return nil
}
