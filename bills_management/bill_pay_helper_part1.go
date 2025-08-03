package main

import (
	"database/sql"
	"fmt"
	"time"
)

// PayBillRequest representa la estructura de solicitud para pagar una factura
// Contiene toda la información necesaria para procesar un pago
type PayBillRequest struct {
	UserID      string `json:"user_id"`
	BillID      int    `json:"bill_id"`
	YearMonth   string `json:"year_month"`   // Formato: "2025-01"
	PaymentDate string `json:"payment_date"` // Formato: "2025-01-15" (opcional, por defecto fecha actual)
}

// PayBillResponse representa la estructura de respuesta para el pago de facturas
// Proporciona información detallada sobre el pago procesado
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
// CORREGIDO: Implementa lógica precisa de transferencia de bill_amount a expense_amount
func markBillPaid(db *sql.DB, billID int, userID, yearMonth, paymentDate string) (*PayBillResponse, error) {
	// Si no se proporciona fecha de pago, usar la fecha actual
	if paymentDate == "" {
		paymentDate = time.Now().Format("2006-01-02")
	}

	// Iniciar transacción para garantizar consistencia
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Obtener información de la factura
	var amount float64
	var paymentMethod, category string
	err = tx.QueryRow("SELECT amount, payment_method, category FROM bills WHERE id = ? AND user_id = ?", billID, userID).Scan(&amount, &paymentMethod, &category)
	if err != nil {
		return nil, fmt.Errorf("bill not found: %v", err)
	}

	// Confirmar transacción inicial
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing initial transaction: %v", err)
	}

	// Verificar si existen registros de pago, crearlos si es necesario
	var paymentCount int
	err = db.QueryRow("SELECT COUNT(*) FROM bill_payments WHERE bill_id = ?", billID).Scan(&paymentCount)
	if err != nil {
		return nil, fmt.Errorf("error checking payment records: %v", err)
	}

	// Crear registros de pago retroactivos si no existen
	if paymentCount == 0 {
		if err = createBillPaymentRecordsRetroactive(db, billID); err != nil {
			return nil, fmt.Errorf("error creating retroactive payment records: %v", err)
		}
	}

	// Iniciar nueva transacción para el procesamiento del pago
	tx, err = db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error starting payment transaction: %v", err)
	}
	defer tx.Rollback()

	// Verificar que el pago no haya sido ya procesado, crear record si no existe
	var alreadyPaid bool
	err = tx.QueryRow("SELECT paid FROM bill_payments WHERE bill_id = ? AND year_month = ?", billID, yearMonth).Scan(&alreadyPaid)
	if err != nil {
		if err == sql.ErrNoRows {
			// Record no existe, crearlo con paid=0
			_, insertErr := tx.Exec(`
				INSERT INTO bill_payments (bill_id, year_month, paid, payment_method, created_at, user_id) 
				VALUES (?, ?, 0, ?, CURRENT_TIMESTAMP, ?)
			`, billID, yearMonth, paymentMethod, userID)
			if insertErr != nil {
				return nil, fmt.Errorf("error creating missing payment record for bill %d in month %s: %v", billID, yearMonth, insertErr)
			}
			alreadyPaid = false // Recién creado, no está pagado
		} else {
			return nil, fmt.Errorf("error checking payment record for bill %d in month %s: %v", billID, yearMonth, err)
		}
	}
	if alreadyPaid {
		return nil, fmt.Errorf("bill for month %s is already paid", yearMonth)
	}

	// Marcar el pago como realizado
	_, err = tx.Exec("UPDATE bill_payments SET paid = 1, payment_date = ? WHERE bill_id = ? AND year_month = ?", paymentDate, billID, yearMonth)
	if err != nil {
		return nil, fmt.Errorf("error marking payment as paid: %v", err)
	}

	// Transferir importe de bill_amount a expense_amount
	if err = removeBillAmountFromMonth(tx, userID, yearMonth, amount, paymentMethod); err != nil {
		return nil, fmt.Errorf("error removing bill amount: %v", err)
	}

	// Crear registro de gasto correspondiente
	if err = createExpenseRecord(tx, userID, category, paymentDate, paymentMethod, billID, amount); err != nil {
		return nil, fmt.Errorf("error creating expense record: %v", err)
	}

	// Verificar si la factura está completamente pagada
	var totalPayments, paidPayments int
	err = tx.QueryRow("SELECT COUNT(*) as total, SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END) as paid_count FROM bill_payments WHERE bill_id = ?", billID).Scan(&totalPayments, &paidPayments)
	if err != nil {
		return nil, fmt.Errorf("error checking bill completion: %v", err)
	}

	// Actualizar estado de la factura si está completamente pagada
	billFullyPaid := false
	if totalPayments > 0 && paidPayments >= totalPayments {
		_, err = tx.Exec("UPDATE bills SET paid = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?", billID, userID)
		if err != nil {
			return nil, fmt.Errorf("error updating bill status: %v", err)
		}
		billFullyPaid = true
	}

	// Confirmar todas las operaciones
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %v", err)
	}

	// Retornar información del pago procesado
	return &PayBillResponse{
		BillID: billID, UserID: userID, YearMonth: yearMonth, PaymentDate: paymentDate,
		Amount: amount, PaymentMethod: paymentMethod, BillFullyPaid: billFullyPaid,
		RemainingPayments: totalPayments - paidPayments,
	}, nil
}

// removeBillAmountFromMonth resta el importe del bill de las columnas bill_*
// y suma el importe a las columnas expense_* en monthly_cash_bank_balance
// CORREGIDO: Usa monthly_cash_bank_balance en lugar de monthly_balance (deprecated)
func removeBillAmountFromMonth(tx *sql.Tx, userID, yearMonth string, amount float64, paymentMethod string) error {
	// Determinar columnas según método de pago
	var billColumn, expenseColumn string
	if paymentMethod == "cash" {
		billColumn = "bill_cash_amount"
		expenseColumn = "expense_cash_amount"
	} else {
		billColumn = "bill_bank_amount"
		expenseColumn = "expense_bank_amount"
	}

	// Transferir de bill_amount a expense_amount en monthly_cash_bank_balance
	query := fmt.Sprintf(`
		UPDATE monthly_cash_bank_balance 
		SET %s = %s - ?, 
		    %s = %s + ?
		WHERE user_id = ? AND year_month = ?
	`, billColumn, billColumn, expenseColumn, expenseColumn)

	_, err := tx.Exec(query, amount, amount, userID, yearMonth)
	if err != nil {
		return fmt.Errorf("error transferring bill amount to expense in monthly_cash_bank_balance: %v", err)
	}

	return nil
}

// createExpenseRecord crea un registro de gasto cuando se paga una factura
// Mantiene consistencia entre bill_payments y expenses
func createExpenseRecord(tx *sql.Tx, userID, category, paymentDate, paymentMethod string, billID int, amount float64) error {
	// Crear registro en la tabla expenses
	_, err := tx.Exec(`
		INSERT INTO expenses (
			user_id, category, description, amount, date, 
			payment_method, bill_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, userID, category, fmt.Sprintf("Bill Payment - %s", category), amount, paymentDate, paymentMethod, billID)

	if err != nil {
		return fmt.Errorf("error creating expense record: %v", err)
	}

	return nil
}
