package main

import (
	"fmt"
	"log"
	"time"
)

// getUpcomingBillsAmount calcula el monto de facturas pendientes para el período
func getUpcomingBillsAmount(userID, startDate, endDate string) (float64, error) {
	// Para calcular las facturas pendientes, necesitamos consultar la tabla bill_payments
	// y obtener las facturas que NO han sido pagadas en el período actual

	// Convertir las fechas del período al formato year_month para consultar bill_payments
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0, fmt.Errorf("error parsing start date: %v", err)
	}

	// Para períodos mensuales, usar el año-mes del startDate
	yearMonth := startTime.Format("2006-01")

	// Consultar bill_payments para obtener facturas NO pagadas del mes actual
	query := `
		SELECT COALESCE(SUM(b.amount), 0)
		FROM bills b
		INNER JOIN bill_payments bp ON b.id = bp.bill_id
		WHERE b.user_id = ? 
		AND bp.year_month = ? 
		AND bp.paid = 0
	`

	var upcomingAmount float64
	err = db.QueryRow(query, userID, yearMonth).Scan(&upcomingAmount)
	if err != nil {
		log.Printf("Error getting upcoming bills amount from bill_payments: %v", err)
		// Fallback a la lógica original si falla la nueva consulta
		return getUpcomingBillsAmountFallback(userID, startDate, endDate)
	}

	log.Printf("📋 Found upcoming bills for %s: amount=%.2f", yearMonth, upcomingAmount)
	return upcomingAmount, nil
}

// getUpcomingBillsAmountFallback función de fallback para mantener compatibilidad
func getUpcomingBillsAmountFallback(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT amount, due_date, paid, recurring
		FROM bills
		WHERE user_id = ? AND due_date BETWEEN ? AND ? AND paid = 0
	`

	rows, err := db.Query(query, userID, startDate, endDate)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var upcomingAmount float64
	for rows.Next() {
		var bill Bill
		err := rows.Scan(&bill.Amount, &bill.DueDate, &bill.Paid, &bill.Recurring)
		if err != nil {
			return 0, err
		}

		// Only count bills that haven't been paid yet
		// (This check is now redundant since we filter in SQL, but keeping for safety)
		if !bill.Paid {
			upcomingAmount += bill.Amount
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	log.Printf("📋 Fallback upcoming bills: amount=%.2f", upcomingAmount)
	return upcomingAmount, nil
}