package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleUpdateBill maneja las solicitudes POST para actualizar facturas existentes
// CORREGIDO: Implementa el nuevo algoritmo de actualización con reseteo y aplicación
func handleUpdateBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Decodificar solicitud de actualización
	var updateRequest UpdateBillRequest
	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil || updateRequest.UserID == "" || updateRequest.BillID <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Obtener datos actuales de la factura
	oldBillData, err := getBillOldData(db, updateRequest.BillID, updateRequest.UserID)
	if err != nil {
		sendErrorResponse(w, "Bill not found", http.StatusNotFound)
		return
	}
	
	// Actualizar datos básicos en la base de datos
	if err = updateBillInDatabase(db, updateRequest); err != nil {
		sendErrorResponse(w, "Error updating bill", http.StatusInternalServerError)
		return
	}
	
	// Preparar datos para el algoritmo de actualización
	updateData := BillUpdateData{
		BillID: updateRequest.BillID, 
		UserID: updateRequest.UserID,
		OldAmount: oldBillData.Amount, 
		NewAmount: getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths, 
		NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate: oldBillData.StartDate, 
		NewStartDate: getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod: oldBillData.PaymentMethod, 
		NewPaymentMethod: getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
	}
	
	// NUEVO ALGORITMO CORREGIDO: Ejecutar secuencia de actualización
	
	// 1. Simular eliminación del bill antiguo (revertir efectos)
	if err = revertOldBillEffects(db, updateData); err != nil {
		sendErrorResponse(w, "Error reverting old bill effects", http.StatusInternalServerError)
		return
	}
	
	// 2. Validar y eliminar expenses anteriores al nuevo start_date
	if err = cleanupExpensesForNewPeriod(db, updateData); err != nil {
		sendErrorResponse(w, "Error cleaning up expenses", http.StatusInternalServerError)
		return
	}
	
	// 3. Actualizar bill_payments según el nuevo periodo
	if err = updateBillPaymentsForNewPeriod(db, updateData); err != nil {
		sendErrorResponse(w, "Error updating bill payments", http.StatusInternalServerError)
		return
	}
	
	// 4. Aplicar el nuevo bill con la información actualizada
	if err = applyNewBillToMonthlyBalance(db, updateData); err != nil {
		sendErrorResponse(w, "Error applying new bill", http.StatusInternalServerError)
		return
	}
	
	// Invalidate bills cache for this user since a bill was updated
	invalidateBillsCache(updateRequest.UserID)
	
	sendSuccessResponse(w, "Bill updated successfully", map[string]interface{}{
		"bill_id": updateRequest.BillID, "status": "updated",
	})
}

// handleUpdateBillCashBank maneja las solicitudes POST para actualizar facturas en monthly_cash_bank_balance
// NUEVO: Implementa el algoritmo específico para monthly_cash_bank_balance
func handleUpdateBillCashBank(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Decodificar solicitud de actualización
	var updateRequest UpdateBillRequest
	if err := json.NewDecoder(r.Body).Decode(&updateRequest); err != nil || updateRequest.UserID == "" || updateRequest.BillID <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Obtener datos actuales de la factura
	oldBillData, err := getBillOldData(db, updateRequest.BillID, updateRequest.UserID)
	if err != nil {
		sendErrorResponse(w, "Bill not found", http.StatusNotFound)
		return
	}
	
	// Actualizar datos básicos en la base de datos
	if err = updateBillInDatabase(db, updateRequest); err != nil {
		sendErrorResponse(w, "Error updating bill", http.StatusInternalServerError)
		return
	}
	
	// Preparar datos para el algoritmo de actualización específico de monthly_cash_bank_balance
	updateData := BillCashBankUpdateData{
		BillID: updateRequest.BillID, 
		UserID: updateRequest.UserID,
		OldAmount: oldBillData.Amount, 
		NewAmount: getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths, 
		NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate: oldBillData.StartDate, 
		NewStartDate: getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod: oldBillData.PaymentMethod, 
		NewPaymentMethod: getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
		OldPaymentDay: oldBillData.PaymentDay,
		NewPaymentDay: getIntValueOrDefault(updateRequest.PaymentDay, oldBillData.PaymentDay),
	}
	
	// CORREGIDO: ALGORITMO ACUMULATIVO PARA MONTHLY_CASH_BANK_BALANCE
	
	// Preparar datos para la nueva arquitectura acumulativa
	oldBillDataStruct := BillData{
		ID: updateData.BillID,
		UserID: updateData.UserID,
		Amount: updateData.OldAmount,
		PaymentMethod: updateData.OldPaymentMethod,
		StartDate: updateData.OldStartDate,
		Duration: updateData.OldDurationMonths,
	}
	
	newBillDataStruct := BillData{
		ID: updateData.BillID,
		UserID: updateData.UserID,
		Amount: updateData.NewAmount,
		PaymentMethod: updateData.NewPaymentMethod,
		StartDate: updateData.NewStartDate,
		Duration: updateData.NewDurationMonths,
	}
	
	// Actualizar usando lógica acumulativa
	if err = updateBillInCashBankBalanceCumulative(db, oldBillDataStruct, newBillDataStruct); err != nil {
		sendErrorResponse(w, "Error updating bill with cumulative logic", http.StatusInternalServerError)
		return
	}
	
	// Actualizar bill_payments según el nuevo periodo
	if err = updateBillPaymentsForNewPeriodCashBank(db, updateData); err != nil {
		sendErrorResponse(w, "Error updating bill payments", http.StatusInternalServerError)
		return
	}
	
	sendSuccessResponse(w, "Bill updated successfully in monthly_cash_bank_balance", map[string]interface{}{
		"bill_id": updateRequest.BillID, 
		"status": "updated",
		"table": "monthly_cash_bank_balance",
	})
}

// handleAddBillCashBank maneja las solicitudes POST para crear nuevas facturas en monthly_cash_bank_balance
// NUEVO: Implementa creación específica para monthly_cash_bank_balance
func handleAddBillCashBank(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Estructura para la solicitud de creación de factura
	var req struct {
		UserID         string  `json:"user_id"`
		Name           string  `json:"name"`
		Amount         float64 `json:"amount"`
		DueDate        string  `json:"due_date"`
		Category       string  `json:"category"`
		Icon           string  `json:"icon"`
		StartDate      string  `json:"start_date"`
		PaymentDay     int     `json:"payment_day"`
		DurationMonths int     `json:"duration_months"`
		Regularity     string  `json:"regularity"`
		PaymentMethod  string  `json:"payment_method"`
	}
	
	// Validar datos de la solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.Name == "" || req.Amount <= 0 {
		sendErrorResponse(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Insertar factura en la base de datos
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?)", req.UserID, req.Name, req.Amount, req.DueDate, req.Category, req.Icon, req.StartDate, req.PaymentDay, req.DurationMonths, req.Regularity, req.PaymentMethod)
	if err != nil {
		sendErrorResponse(w, "Error adding bill", http.StatusInternalServerError)
		return
	}
	
	// Obtener ID de la factura creada
	billID, _ := result.LastInsertId()
	
	// CORREGIDO: Aplicar la factura usando lógica acumulativa
	err = addBillToCashBankBalanceCumulative(db, req.UserID, req.Amount, req.StartDate, req.DurationMonths, req.PaymentMethod)
	if err != nil {
		sendErrorResponse(w, fmt.Sprintf("Error adding bill to cash bank balance: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Crear registros de pago para la factura
	createBillPaymentRecords(db, int(billID), req.UserID, req.StartDate, req.DurationMonths, req.PaymentMethod)
	
	sendSuccessResponse(w, "Bill added successfully to monthly_cash_bank_balance", map[string]interface{}{
		"id": billID, "user_id": req.UserID, "name": req.Name, "amount": req.Amount,
		"table": "monthly_cash_bank_balance",
	})
}
