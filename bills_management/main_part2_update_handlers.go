package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
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
		BillID:            updateRequest.BillID,
		UserID:            updateRequest.UserID,
		OldAmount:         oldBillData.Amount,
		NewAmount:         getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths,
		NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate:      oldBillData.StartDate,
		NewStartDate:      getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod:  oldBillData.PaymentMethod,
		NewPaymentMethod:  getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
	}

	// NUEVO ALGORITMO CORREGIDO: Usar lógica acumulativa

	// Preparar datos para la nueva arquitectura acumulativa
	oldBillDataStruct := BillData{
		ID:            updateData.BillID,
		UserID:        updateData.UserID,
		Amount:        updateData.OldAmount,
		PaymentMethod: updateData.OldPaymentMethod,
		StartDate:     updateData.OldStartDate,
		Duration:      updateData.OldDurationMonths,
	}

	newBillDataStruct := BillData{
		ID:            updateData.BillID,
		UserID:        updateData.UserID,
		Amount:        updateData.NewAmount,
		PaymentMethod: updateData.NewPaymentMethod,
		StartDate:     updateData.NewStartDate,
		Duration:      updateData.NewDurationMonths,
	}

	// Actualizar usando lógica acumulativa
	if err = updateBillInCashBankBalanceCumulative(db, oldBillDataStruct, newBillDataStruct); err != nil {
		sendErrorResponse(w, "Error updating bill with cumulative logic", http.StatusInternalServerError)
		return
	}

	// Actualizar bill_payments según el nuevo periodo
	if err = updateBillPaymentsForNewPeriodCashBank(db, BillCashBankUpdateData{
		BillID:            updateData.BillID,
		UserID:            updateData.UserID,
		OldAmount:         updateData.OldAmount,
		NewAmount:         updateData.NewAmount,
		OldDurationMonths: updateData.OldDurationMonths,
		NewDurationMonths: updateData.NewDurationMonths,
		OldStartDate:      updateData.OldStartDate,
		NewStartDate:      updateData.NewStartDate,
		OldPaymentMethod:  updateData.OldPaymentMethod,
		NewPaymentMethod:  updateData.NewPaymentMethod,
	}); err != nil {
		sendErrorResponse(w, "Error updating bill payments", http.StatusInternalServerError)
		return
	}

	// Always record sync operation with auto-generated operation_id (like add handler)
	log.Printf("Recording sync operation for bill update with auto-generated operation_id")

	// Get updated bill data for sync operation
	updatedBillData, err := getBillOldData(db, updateRequest.BillID, updateRequest.UserID)
	if err != nil {
		log.Printf("Warning: Could not fetch updated bill data for sync operation: %v", err)
		// Still try to record sync operation with basic data
		syncData := map[string]interface{}{
			"id":         updateRequest.BillID,
			"user_id":    updateRequest.UserID,
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
		}

		// Add sync operation record to database
		err = addSyncOperation(
			updateRequest.UserID,
			"", // Empty operation_id will trigger auto-generation
			"update",
			"bills",
			strconv.Itoa(updateRequest.BillID),
			syncData,
			updateRequest.DeviceID, // Use device_id from request
			0,                      // Timestamp will be auto-generated
		)

		if err != nil {
			log.Printf("Warning: Failed to record sync operation with basic data: %v", err)
		} else {
			log.Printf("Successfully recorded sync operation with basic data for bill ID: %d", updateRequest.BillID)
		}
	} else {
		// Create complete sync operation data with updated bill structure
		syncData := map[string]interface{}{
			"id":              updateRequest.BillID,
			"user_id":         updateRequest.UserID,
			"name":            updatedBillData.Name,
			"amount":          updatedBillData.Amount,
			"due_date":        updatedBillData.DueDate,
			"category":        updatedBillData.Category,
			"icon":            updatedBillData.Icon,
			"start_date":      updatedBillData.StartDate,
			"payment_day":     updatedBillData.PaymentDay,
			"duration_months": updatedBillData.DurationMonths,
			"regularity":      updatedBillData.Regularity,
			"payment_method":  updatedBillData.PaymentMethod,
			"created_at":      updatedBillData.CreatedAt,
			"updated_at":      time.Now().Format("2006-01-02 15:04:05"),
		}

		// Add sync operation record to database with device_id from request
		err = addSyncOperation(
			updateRequest.UserID,
			"", // Empty operation_id will trigger auto-generation
			"update",
			"bills",
			strconv.Itoa(updateRequest.BillID),
			syncData,
			updateRequest.DeviceID, // Use device_id from request
			0,                      // Timestamp will be auto-generated
		)

		if err != nil {
			log.Printf("Warning: Failed to record sync operation: %v", err)
			// Don't fail the bill update for sync errors, just log warning
		} else {
			log.Printf("Successfully recorded sync operation for bill ID: %d", updateRequest.BillID)
		}
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
		BillID:            updateRequest.BillID,
		UserID:            updateRequest.UserID,
		OldAmount:         oldBillData.Amount,
		NewAmount:         getValueOrDefault(updateRequest.Amount, oldBillData.Amount),
		OldDurationMonths: oldBillData.DurationMonths,
		NewDurationMonths: getIntValueOrDefault(updateRequest.DurationMonths, oldBillData.DurationMonths),
		OldStartDate:      oldBillData.StartDate,
		NewStartDate:      getStringValueOrDefault(updateRequest.StartDate, oldBillData.StartDate),
		OldPaymentMethod:  oldBillData.PaymentMethod,
		NewPaymentMethod:  getStringValueOrDefault(updateRequest.PaymentMethod, oldBillData.PaymentMethod),
		OldPaymentDay:     oldBillData.PaymentDay,
		NewPaymentDay:     getIntValueOrDefault(updateRequest.PaymentDay, oldBillData.PaymentDay),
	}

	// CORREGIDO: ALGORITMO ACUMULATIVO PARA MONTHLY_CASH_BANK_BALANCE

	// Preparar datos para la nueva arquitectura acumulativa
	oldBillDataStruct := BillData{
		ID:            updateData.BillID,
		UserID:        updateData.UserID,
		Amount:        updateData.OldAmount,
		PaymentMethod: updateData.OldPaymentMethod,
		StartDate:     updateData.OldStartDate,
		Duration:      updateData.OldDurationMonths,
	}

	newBillDataStruct := BillData{
		ID:            updateData.BillID,
		UserID:        updateData.UserID,
		Amount:        updateData.NewAmount,
		PaymentMethod: updateData.NewPaymentMethod,
		StartDate:     updateData.NewStartDate,
		Duration:      updateData.NewDurationMonths,
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
		"status":  "updated",
		"table":   "monthly_cash_bank_balance",
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
		CategoryID     *int    `json:"category_id,omitempty"`
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

	// Insertar factura en la base de datos con category_id
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, category_id, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?, ?)", req.UserID, req.Name, req.Amount, req.DueDate, req.Category, req.CategoryID, req.Icon, req.StartDate, req.PaymentDay, req.DurationMonths, req.Regularity, req.PaymentMethod)
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
