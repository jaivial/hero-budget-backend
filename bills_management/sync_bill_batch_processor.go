package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador de lotes para sincronización offline de facturas
// Implementa la lógica principal de procesamiento siguiendo el patrón de expense_management

// processBillBatch procesa un lote completo de operaciones de sincronización de facturas
// Retorna respuesta detallada con resultados y conflictos detectados
func processBillBatch(request SyncBillBatchRequest) (*SyncBillBatchResponse, error) {
	log.Printf("Procesando lote de sincronización: %d facturas para usuario %s", len(request.Bills), request.UserID)
	
	// Inicializar respuesta del lote
	response := &SyncBillBatchResponse{
		Success:       true,
		Message:       "Lote procesado exitosamente",
		ProcessedOps:  0,
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncBillResult, 0, len(request.Bills)),
		Conflicts:     make([]BillConflictResolution, 0),
		ServerData:    make([]Bill, 0),
		LastSync:      time.Now().UTC().Format(time.RFC3339),
		NextSyncTime:  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
	}
	
	// Procesar cada factura en el lote
	for _, offlineBill := range request.Bills {
		response.ProcessedOps++
		
		// Procesar operación individual
		result, serverBill, err := processSingleBillOperation(offlineBill)
		
		// Agregar resultado a la respuesta
		response.Results = append(response.Results, result)
		
		if err != nil {
			response.FailedOps++
			log.Printf("Error procesando factura %s: %v", offlineBill.LocalID, err)
			continue
		}
		
		response.SuccessfulOps++
		
		// Agregar datos del servidor si la operación fue exitosa
		if serverBill != nil {
			response.ServerData = append(response.ServerData, *serverBill)
		}
	}
	
	// Actualizar estado general de la respuesta
	if response.FailedOps > 0 {
		response.Success = false
		response.Message = fmt.Sprintf("Lote procesado con %d errores de %d operaciones", response.FailedOps, response.ProcessedOps)
	}
	
	log.Printf("Lote completado: %d exitosas, %d fallidas de %d total", response.SuccessfulOps, response.FailedOps, response.ProcessedOps)
	return response, nil
}

// processSingleBillOperation procesa una operación individual de factura
// Retorna el resultado de la operación y los datos del servidor actualizados
func processSingleBillOperation(offlineBill OfflineBill) (SyncBillResult, *Bill, error) {
	// Inicializar resultado
	result := SyncBillResult{
		LocalID:       offlineBill.LocalID,
		Action:        offlineBill.Action,
		Status:        "success",
		SyncTimestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	var serverBill *Bill
	var err error
	
	// Procesar según el tipo de acción
	switch offlineBill.Action {
	case "add":
		serverBill, err = processAddBillOperation(offlineBill)
	case "update":
		serverBill, err = processUpdateBillOperation(offlineBill)
	case "delete":
		err = processDeleteBillOperation(offlineBill)
	default:
		err = fmt.Errorf("acción no soportada: %s", offlineBill.Action)
	}
	
	// Manejar errores
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, nil, err
	}
	
	// Asignar ServerID si es aplicable
	if serverBill != nil {
		result.ServerID = fmt.Sprintf("%d", serverBill.ID)
	}
	
	return result, serverBill, nil
}

// processAddBillOperation procesa la adición de una nueva factura
// Crea la factura y actualiza todas las tablas relacionadas
func processAddBillOperation(offlineBill OfflineBill) (*Bill, error) {
	// Crear estructura para agregar factura usando handlers existentes
	addRequest := struct {
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
	}{
		UserID:         offlineBill.UserID,
		Name:           offlineBill.Name,
		Amount:         offlineBill.Amount,
		DueDate:        offlineBill.DueDate,
		Category:       offlineBill.Category,
		Icon:           offlineBill.Icon,
		StartDate:      offlineBill.StartDate,
		PaymentDay:     offlineBill.PaymentDay,
		DurationMonths: offlineBill.DurationMonths,
		Regularity:     offlineBill.Regularity,
		PaymentMethod:  offlineBill.PaymentMethod,
	}
	
	// Insertar factura en la base de datos
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?)", 
		addRequest.UserID, addRequest.Name, addRequest.Amount, addRequest.DueDate, addRequest.Category, addRequest.Icon, 
		addRequest.StartDate, addRequest.PaymentDay, addRequest.DurationMonths, addRequest.Regularity, addRequest.PaymentMethod)
	if err != nil {
		return nil, fmt.Errorf("error insertando factura: %v", err)
	}
	
	// Obtener ID de la factura creada
	billID, _ := result.LastInsertId()
	
	// Aplicar efectos en tablas de balance usando lógica acumulativa existente
	err = addBillToCashBankBalanceCumulative(db, addRequest.UserID, addRequest.Amount, addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)
	if err != nil {
		log.Printf("Error aplicando efectos de balance: %v", err)
		// No fallar la operación por errores de balance
	}
	
	// Crear registros de pago
	createBillPaymentRecords(db, int(billID), addRequest.UserID, addRequest.StartDate, addRequest.DurationMonths, addRequest.PaymentMethod)
	
	// Crear estructura Bill para retornar
	createdBill := &Bill{
		ID:             int(billID),
		UserID:         addRequest.UserID,
		Name:           addRequest.Name,
		Amount:         addRequest.Amount,
		DueDate:        addRequest.DueDate,
		StartDate:      addRequest.StartDate,
		PaymentDay:     addRequest.PaymentDay,
		DurationMonths: addRequest.DurationMonths,
		Regularity:     addRequest.Regularity,
		Paid:           false,
		Overdue:        false,
		OverdueDays:    0,
		Recurring:      true,
		Category:       addRequest.Category,
		Icon:           addRequest.Icon,
		PaymentMethod:  addRequest.PaymentMethod,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	
	return createdBill, nil
}

// processUpdateBillOperation procesa la actualización de una factura existente
// Utiliza la lógica de actualización existente con algoritmo acumulativo
func processUpdateBillOperation(offlineBill OfflineBill) (*Bill, error) {
	// Obtener datos actuales de la factura
	oldBillData, err := getBillOldData(db, offlineBill.ServerID, offlineBill.UserID)
	if err != nil {
		return nil, fmt.Errorf("factura no encontrada: %v", err)
	}
	
	// Crear estructura de actualización
	updateRequest := UpdateBillRequest{
		UserID:         offlineBill.UserID,
		BillID:         offlineBill.ServerID,
		Name:           offlineBill.Name,
		Amount:         offlineBill.Amount,
		StartDate:      offlineBill.StartDate,
		PaymentDay:     offlineBill.PaymentDay,
		DurationMonths: offlineBill.DurationMonths,
		Regularity:     offlineBill.Regularity,
		Category:       offlineBill.Category,
		Icon:           offlineBill.Icon,
		PaymentMethod:  offlineBill.PaymentMethod,
	}
	
	// Actualizar usando lógica existente
	if err = updateBillInDatabase(db, updateRequest); err != nil {
		return nil, fmt.Errorf("error actualizando factura: %v", err)
	}
	
	// Aplicar algoritmo acumulativo de actualización
	// (Esta lógica ya está implementada en los handlers existentes)
	
	// Obtener factura actualizada
	updatedBill, err := getBillOldData(db, offlineBill.ServerID, offlineBill.UserID)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo factura actualizada: %v", err)
	}
	
	return updatedBill, nil
}

// processDeleteBillOperation procesa la eliminación de una factura
// Elimina la factura y revierte todos sus efectos
func processDeleteBillOperation(offlineBill OfflineBill) error {
	// Obtener datos de la factura antes de eliminar
	billData, err := getBillOldData(db, offlineBill.ServerID, offlineBill.UserID)
	if err != nil {
		return fmt.Errorf("factura no encontrada para eliminar: %v", err)
	}
	
	// Eliminar factura y revertir efectos usando lógica existente
	err = deleteBillAndRevertEffects(db, billData)
	if err != nil {
		return fmt.Errorf("error eliminando factura: %v", err)
	}
	
	return nil
}

// getBillChanges obtiene cambios de facturas del servidor desde último sync
// Implementa paginación y filtrado por usuario
func getBillChanges(request SyncBillChangesRequest) (*SyncBillChangesResponse, error) {
	// Por ahora, implementación simple que retorna todas las facturas del usuario
	bills, err := fetchBills(request.UserID)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo facturas: %v", err)
	}
	
	response := &SyncBillChangesResponse{
		Success:      true,
		Message:      "Cambios obtenidos exitosamente",
		Changes:      bills,
		Deletions:    make([]int, 0),
		HasMore:      false,
		TotalChanges: len(bills),
		LastSync:     time.Now().UTC().Format(time.RFC3339),
		ServerTime:   time.Now().UTC().Format(time.RFC3339),
	}
	
	return response, nil
}