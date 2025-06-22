package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// fetchBills obtiene todas las facturas de un usuario específico
// Retorna una lista completa de facturas ordenadas por ID
func fetchBills(userID string) ([]Bill, error) {

	// Consulta SQL con COALESCE para manejar valores nulos
	query := `SELECT 
		id, user_id, name, amount, 
		COALESCE(due_date, start_date), start_date, 
		payment_day, duration_months, regularity, 
		paid, overdue, overdue_days, recurring, 
		category, icon, 
		COALESCE(payment_method, 'cash'), 
		COALESCE(created_at, ''), 
		COALESCE(updated_at, '') 
	FROM bills 
	WHERE user_id = ? 
	ORDER BY id ASC`
	
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	// Procesar resultados
	var bills []Bill
	for rows.Next() {
		var bill Bill
		if err := rows.Scan(
			&bill.ID, &bill.UserID, &bill.Name, &bill.Amount,
			&bill.DueDate, &bill.StartDate, &bill.PaymentDay, &bill.DurationMonths,
			&bill.Regularity, &bill.Paid, &bill.Overdue, &bill.OverdueDays,
			&bill.Recurring, &bill.Category, &bill.Icon, &bill.PaymentMethod,
			&bill.CreatedAt, &bill.UpdatedAt,
		); err == nil {
			bills = append(bills, bill)
		}
	}

	return bills, nil
}

// getBillOldData obtiene los datos actuales de una factura antes de actualizarla
// Necesario para el algoritmo de actualización que compara estado anterior con nuevo
func getBillOldData(db *sql.DB, billID int, userID string) (*Bill, error) {
	// Consulta SQL idéntica a fetchBills pero para una factura específica
	query := `SELECT 
		id, user_id, name, amount, 
		COALESCE(due_date, start_date), start_date, 
		payment_day, duration_months, regularity, 
		paid, overdue, overdue_days, recurring, 
		category, icon, 
		COALESCE(payment_method, 'cash'), 
		COALESCE(created_at, ''), 
		COALESCE(updated_at, '') 
	FROM bills 
	WHERE id = ? AND user_id = ?`
	
	var bill Bill
	err := db.QueryRow(query, billID, userID).Scan(
		&bill.ID, &bill.UserID, &bill.Name, &bill.Amount,
		&bill.DueDate, &bill.StartDate, &bill.PaymentDay, &bill.DurationMonths,
		&bill.Regularity, &bill.Paid, &bill.Overdue, &bill.OverdueDays,
		&bill.Recurring, &bill.Category, &bill.Icon, &bill.PaymentMethod,
		&bill.CreatedAt, &bill.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &bill, nil
}

// updateBillInDatabase actualiza los campos básicos de una factura en la base de datos
// Construye dinámicamente la consulta SQL según los campos proporcionados
func updateBillInDatabase(db *sql.DB, updateRequest UpdateBillRequest) error {
	// Construir consulta dinámicamente
	setParts := []string{}
	args := []interface{}{}
	
	// Añadir campos no vacíos a la actualización
	if updateRequest.Name != "" {
		setParts = append(setParts, "name = ?")
		args = append(args, updateRequest.Name)
	}
	if updateRequest.Amount > 0 {
		setParts = append(setParts, "amount = ?")
		args = append(args, updateRequest.Amount)
	}
	if updateRequest.StartDate != "" {
		setParts = append(setParts, "start_date = ?")
		args = append(args, updateRequest.StartDate)
	}
	if updateRequest.PaymentDay > 0 {
		setParts = append(setParts, "payment_day = ?")
		args = append(args, updateRequest.PaymentDay)
	}
	if updateRequest.DurationMonths > 0 {
		setParts = append(setParts, "duration_months = ?")
		args = append(args, updateRequest.DurationMonths)
	}
	if updateRequest.Regularity != "" {
		setParts = append(setParts, "regularity = ?")
		args = append(args, updateRequest.Regularity)
	}
	if updateRequest.Category != "" {
		setParts = append(setParts, "category = ?")
		args = append(args, updateRequest.Category)
	}
	if updateRequest.Icon != "" {
		setParts = append(setParts, "icon = ?")
		args = append(args, updateRequest.Icon)
	}
	if updateRequest.PaymentMethod != "" {
		setParts = append(setParts, "payment_method = ?")
		args = append(args, updateRequest.PaymentMethod)
	}
	
	// Verificar que hay campos para actualizar
	if len(setParts) == 0 {
		return fmt.Errorf("no fields to update")
	}
	
	// Añadir timestamp de actualización
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")
	setClause := strings.Join(setParts, ", ")
	
	// Construir y ejecutar consulta
	query := fmt.Sprintf("UPDATE bills SET %s WHERE id = ? AND user_id = ?", setClause)
	args = append(args, updateRequest.BillID, updateRequest.UserID)
	
	_, err := db.Exec(query, args...)
	return err
}


// deleteBillAndRevertEffects elimina una factura y revierte todos sus efectos
// Utiliza el helper delete_bill_helper.go para el proceso completo
func deleteBillAndRevertEffects(db *sql.DB, billData *Bill) error {
	// El algoritmo de eliminación está implementado en delete_bill_helper.go
	// Esta función actuaría como wrapper si fuera necesario
	return fmt.Errorf("delete bill functionality pending implementation")
}

