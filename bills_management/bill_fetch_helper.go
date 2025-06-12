package main

import (
	"fmt"
	"log"
	"time"
)

// BillWithPeriodStatus representa una factura con estado de pago específico para un período
type BillWithPeriodStatus struct {
	Bill
	PaidForPeriod bool   `json:"paid"`         // Estado de pago para el período específico
	SpecificDate  string `json:"due_date"`     // Fecha calculada para el período específico
	PeriodMonth   string `json:"period_month"` // Mes del período (YYYY-MM)
}

// fetchBillsForPeriod obtiene facturas con estado de pago específico para un período dado
func fetchBillsForPeriod(userID, period, date string) ([]BillWithPeriodStatus, error) {
	log.Printf("🔍 fetchBillsForPeriod: userID=%s, period=%s, date=%s", userID, period, date)

	// Extraer año-mes del parámetro date
	targetYearMonth, err := extractYearMonth(date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %v", err)
	}

	log.Printf("📅 Target year-month: %s", targetYearMonth)

	// Query que une bills con bill_payments para obtener el estado de pago específico del período
	query := `
		SELECT 
			b.id, b.user_id, b.name, b.amount, b.start_date, b.payment_day, 
			b.duration_months, b.regularity, b.recurring, b.category, b.icon, 
			COALESCE(b.payment_method, 'cash') as payment_method,
			COALESCE(b.created_at, '') as created_at, 
			COALESCE(b.updated_at, '') as updated_at,
			COALESCE(bp.paid, 0) as period_paid,
			CASE 
				WHEN b.recurring = 1 THEN 
					printf('%04d-%02d-%02d', 
						CAST(substr(?, 1, 4) AS INTEGER),
						CAST(substr(?, 6, 2) AS INTEGER),
						CASE 
							WHEN b.payment_day <= CAST(strftime('%d', date(? || '-01', 'start of month', '+1 month', '-1 day')) AS INTEGER)
							THEN b.payment_day 
							ELSE CAST(strftime('%d', date(? || '-01', 'start of month', '+1 month', '-1 day')) AS INTEGER)
						END
					)
				ELSE b.due_date 
			END as calculated_due_date
		FROM bills b 
		LEFT JOIN bill_payments bp ON b.id = bp.bill_id AND bp.year_month = ?
		WHERE b.user_id = ? 
		AND (
			(b.recurring = 1 AND 
			 strftime('%Y-%m', b.start_date) <= ? AND 
			 strftime('%Y-%m', date(b.start_date, '+' || (b.duration_months - 1) || ' months')) >= ?) OR
			(b.recurring = 0 AND strftime('%Y-%m', b.due_date) = ?)
		)
		ORDER BY calculated_due_date ASC, b.id ASC
	`

	log.Printf("🔍 Executing query with targetYearMonth: %s", targetYearMonth)

	rows, err := db.Query(query,
		targetYearMonth, targetYearMonth, targetYearMonth, targetYearMonth, // Para calculated_due_date
		targetYearMonth,                                   // Para LEFT JOIN bill_payments
		userID,                                            // Para WHERE user_id
		targetYearMonth, targetYearMonth, targetYearMonth, // Para condiciones de fecha
	)
	if err != nil {
		log.Printf("❌ Error executing query: %v", err)
		return nil, fmt.Errorf("error querying bills: %v", err)
	}
	defer rows.Close()

	var bills []BillWithPeriodStatus
	for rows.Next() {
		var billData BillWithPeriodStatus
		var periodPaid int // SQLite devuelve BOOLEAN como INTEGER

		err := rows.Scan(
			&billData.ID, &billData.UserID, &billData.Name, &billData.Amount,
			&billData.StartDate, &billData.PaymentDay, &billData.DurationMonths,
			&billData.Regularity, &billData.Recurring, &billData.Category,
			&billData.Icon, &billData.PaymentMethod, &billData.CreatedAt,
			&billData.UpdatedAt, &periodPaid, &billData.SpecificDate,
		)
		if err != nil {
			log.Printf("❌ Error scanning bill row: %v", err)
			continue
		}

		// Convertir el estado de pago
		billData.PaidForPeriod = periodPaid == 1
		billData.PeriodMonth = targetYearMonth

		// Asignar la fecha calculada como DueDate para compatibilidad
		billData.DueDate = billData.SpecificDate

		// Los campos Paid, Overdue y OverdueDays se calculan basándose en el período específico
		billData.Paid = billData.PaidForPeriod
		billData.Overdue = calculateOverdue(billData.SpecificDate, billData.PaidForPeriod)
		billData.OverdueDays = calculateOverdueDays(billData.SpecificDate, billData.PaidForPeriod)

		log.Printf("📋 Bill ID %d (%s): period=%s, paid=%t, dueDate=%s",
			billData.ID, billData.Name, targetYearMonth, billData.PaidForPeriod, billData.SpecificDate)

		bills = append(bills, billData)
	}

	log.Printf("✅ fetchBillsForPeriod: Found %d bills for period %s", len(bills), targetYearMonth)
	return bills, nil
}

// extractYearMonth extrae el año-mes del parámetro date en formato YYYY-MM
func extractYearMonth(date string) (string, error) {
	if len(date) < 7 {
		return "", fmt.Errorf("date too short: %s", date)
	}

	// Manejar diferentes formatos de fecha
	if len(date) >= 10 { // YYYY-MM-DD
		return date[:7], nil
	} else if len(date) == 7 { // YYYY-MM
		return date, nil
	}

	return "", fmt.Errorf("unsupported date format: %s", date)
}

// calculateOverdue determina si una factura está vencida basándose en su fecha específica
func calculateOverdue(dueDate string, paid bool) bool {
	if paid {
		return false
	}

	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		log.Printf("❌ Error parsing due date %s: %v", dueDate, err)
		return false
	}

	today := time.Now()
	return due.Before(today)
}

// calculateOverdueDays calcula los días de vencimiento
func calculateOverdueDays(dueDate string, paid bool) int {
	if paid || !calculateOverdue(dueDate, paid) {
		return 0
	}

	due, err := time.Parse("2006-01-02", dueDate)
	if err != nil {
		log.Printf("❌ Error parsing due date %s: %v", dueDate, err)
		return 0
	}

	today := time.Now()
	days := int(today.Sub(due).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// convertBillWithPeriodStatusToBill convierte BillWithPeriodStatus a Bill para compatibilidad
func convertBillWithPeriodStatusToBill(billWithStatus BillWithPeriodStatus) Bill {
	return Bill{
		ID:             billWithStatus.ID,
		UserID:         billWithStatus.UserID,
		Name:           billWithStatus.Name,
		Amount:         billWithStatus.Amount,
		DueDate:        billWithStatus.SpecificDate, // Usar la fecha específica calculada
		StartDate:      billWithStatus.StartDate,
		PaymentDay:     billWithStatus.PaymentDay,
		DurationMonths: billWithStatus.DurationMonths,
		Regularity:     billWithStatus.Regularity,
		Paid:           billWithStatus.PaidForPeriod, // Usar estado del período específico
		Overdue:        billWithStatus.Overdue,
		OverdueDays:    billWithStatus.OverdueDays,
		Recurring:      billWithStatus.Recurring,
		Category:       billWithStatus.Category,
		Icon:           billWithStatus.Icon,
		PaymentMethod:  billWithStatus.PaymentMethod,
		CreatedAt:      billWithStatus.CreatedAt,
		UpdatedAt:      billWithStatus.UpdatedAt,
	}
}
