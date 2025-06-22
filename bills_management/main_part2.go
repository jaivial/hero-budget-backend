package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// handleFetchBills maneja las solicitudes GET para obtener facturas
// Soporta filtrado por periodo y fecha para consultas específicas
func handleFetchBills(w http.ResponseWriter, r *http.Request) {
	// Validar que se proporcione user_id
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}
	
	// Verificar si se solicita un periodo específico
	period := r.URL.Query().Get("period")
	date := r.URL.Query().Get("date")
	if period != "" && date != "" {
		// Obtener facturas para un periodo específico
		billsWithStatus, err := fetchBillsForPeriod(userID, period, date)
		if err != nil {
			sendErrorResponse(w, "Error fetching bills for period", http.StatusInternalServerError)
			return
		}
		// Convertir a formato Bill estándar
		var bills []Bill
		for _, billWithStatus := range billsWithStatus {
			bills = append(bills, convertBillWithPeriodStatusToBill(billWithStatus))
		}
		sendSuccessResponse(w, "Bills fetched successfully", bills)
		return
	}
	
	// Obtener todas las facturas del usuario
	bills, err := fetchBills(userID)
	if err != nil {
		sendErrorResponse(w, "Error fetching bills", http.StatusInternalServerError)
		return
	}
	sendSuccessResponse(w, "Bills fetched successfully", bills)
}

// handleAddBill maneja las solicitudes POST para crear nuevas facturas
// Valida los datos y crea la factura con sus registros de pago asociados
func handleAddBill(w http.ResponseWriter, r *http.Request) {
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
	
	// Aplicar la factura al sistema de balance mensual
	addNewBillToMonthlyBalance(db, req.UserID, req.Amount, req.StartDate, req.DurationMonths, req.PaymentMethod)
	
	// Crear registros de pago para la factura
	createBillPaymentRecords(db, int(billID), req.UserID, req.StartDate, req.DurationMonths, req.PaymentMethod)
	
	sendSuccessResponse(w, "Bill added successfully", map[string]interface{}{
		"id": billID, "user_id": req.UserID, "name": req.Name, "amount": req.Amount,
	})
}

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
	
	sendSuccessResponse(w, "Bill updated successfully", map[string]interface{}{
		"bill_id": updateRequest.BillID, "status": "updated",
	})
}

// handlePayBill maneja las solicitudes POST para procesar pagos de facturas
// Utiliza la función markBillPaid para procesar el pago de manera transaccional
func handlePayBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Estructura para la solicitud de pago
	var req struct {
		UserID        string `json:"user_id"`
		BillID        int    `json:"bill_id"`
		YearMonth     string `json:"year_month"`
		PaymentDate   string `json:"payment_date"`
		PaymentMethod string `json:"payment_method"`
	}

	// Decodificar solicitud
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validar campos requeridos
	if req.UserID == "" || req.BillID <= 0 || req.YearMonth == "" {
		sendErrorResponse(w, "Missing required fields: user_id, bill_id, year_month", http.StatusBadRequest)
		return
	}

	// Procesar pago usando la función dedicada
	paymentResponse, err := markBillPaid(db, req.BillID, req.UserID, req.YearMonth, req.PaymentDate)
	if err != nil {
		log.Printf("Error marking bill as paid: %v", err)
		sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Bill payment processed successfully", paymentResponse)
}

// handleDebugAddBill endpoint de depuración para probar funcionalidades
// Útil durante el desarrollo para verificar el comportamiento del sistema
func handleDebugAddBill(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔥 DEBUG: handleDebugAddBill llamada")

	// Parámetros fijos para depuración
	userID := "14"
	amount := 100.0
	startDate := "2025-01-15T00:00:00Z"
	durationMonths := 6
	paymentMethod := "bank"

	log.Printf("🔥 DEBUG: Llamando a addNewBillToMonthlyBalance directamente...")
	err := addNewBillToMonthlyBalance(db, userID, amount, startDate, durationMonths, paymentMethod)

	if err != nil {
		log.Printf("🔥 DEBUG: Error en addNewBillToMonthlyBalance: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("🔥 DEBUG: addNewBillToMonthlyBalance completada sin errores")
	sendSuccessResponse(w, "Debug completed", map[string]interface{}{
		"user_id":         userID,
		"amount":          amount,
		"duration_months": durationMonths,
		"payment_method":  paymentMethod,
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
	
	// ALGORITMO ESPECÍFICO PARA MONTHLY_CASH_BANK_BALANCE:
	
	// 1. Revertir efectos del bill anterior en monthly_cash_bank_balance
	if err = revertOldBillEffectsInCashBank(db, updateData); err != nil {
		sendErrorResponse(w, "Error reverting old bill effects in cash bank", http.StatusInternalServerError)
		return
	}
	
	// 2. Limpiar expenses anteriores al nuevo start_date
	if err = cleanupExpensesForNewPeriod(db, BillUpdateData{
		BillID: updateData.BillID,
		UserID: updateData.UserID,
		NewStartDate: updateData.NewStartDate,
	}); err != nil {
		sendErrorResponse(w, "Error cleaning up expenses", http.StatusInternalServerError)
		return
	}
	
	// 3. Actualizar bill_payments según el nuevo periodo
	if err = updateBillPaymentsForNewPeriodCashBank(db, updateData); err != nil {
		sendErrorResponse(w, "Error updating bill payments", http.StatusInternalServerError)
		return
	}
	
	// 4. Aplicar el nuevo bill a monthly_cash_bank_balance
	if err = applyNewBillToCashBank(db, updateData); err != nil {
		sendErrorResponse(w, "Error applying new bill to cash bank", http.StatusInternalServerError)
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
	
	// Aplicar la factura al sistema monthly_cash_bank_balance
	err = addBillToCashBankBalance(db, req.UserID, req.Amount, req.StartDate, req.DurationMonths, req.PaymentMethod)
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
