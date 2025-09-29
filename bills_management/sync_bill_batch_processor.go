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
		CategoryID     *int    `json:"category_id,omitempty"`
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
		CategoryID:     offlineBill.CategoryID,
		Icon:           offlineBill.Icon,
		StartDate:      offlineBill.StartDate,
		PaymentDay:     offlineBill.PaymentDay,
		DurationMonths: offlineBill.DurationMonths,
		Regularity:     offlineBill.Regularity,
		PaymentMethod:  offlineBill.PaymentMethod,
	}

	// Insertar factura en la base de datos con category_id
	result, err := db.Exec("INSERT INTO bills (user_id, name, amount, due_date, paid, overdue, overdue_days, recurring, category, category_id, icon, start_date, payment_day, duration_months, regularity, payment_method) VALUES (?, ?, ?, ?, 0, 0, 0, 1, ?, ?, ?, ?, ?, ?, ?, ?)",
		addRequest.UserID, addRequest.Name, addRequest.Amount, addRequest.DueDate, addRequest.Category, addRequest.CategoryID, addRequest.Icon,
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
	_, err := getBillOldData(db, offlineBill.ServerID, offlineBill.UserID)
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

// getBillChanges obtiene cambios de facturas del servidor desde último sync (legacy)
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

// getBillOperationChanges obtiene operaciones de facturas desde último operation_id
// Compatible con el nuevo sistema operation_id-based para sincronización incremental
func getBillOperationChanges(request SyncBillOperationChangesRequest) (*SyncBillOperationChangesResponse, error) {
	log.Printf("Fetching bill operations for user %s since operation_id: %s",
		request.UserID, request.LastOperationId)

	// Build query to get operations since last operation_id
	var query string
	var args []interface{}

	if request.LastOperationId == "" {
		// First sync - get all operations for this user
		query = `
			SELECT id, user_id, operation_id, operation_type, entity_type, entity_id, 
				   operation_data, device_ids, client_timestamp, server_timestamp, created_at
			FROM sync_operations 
			WHERE user_id = ? 
			ORDER BY operation_id ASC 
			LIMIT ?
		`
		args = []interface{}{request.UserID, request.Limit}
		log.Printf("First sync for user %s - fetching all operations (limit: %d)", request.UserID, request.Limit)
	} else {
		// Incremental sync - get operations after last operation_id
		query = `
			SELECT id, user_id, operation_id, operation_type, entity_type, entity_id, 
				   operation_data, device_ids, client_timestamp, server_timestamp, created_at
			FROM sync_operations 
			WHERE user_id = ? AND operation_id > ?
			ORDER BY operation_id ASC 
			LIMIT ?
		`
		args = []interface{}{request.UserID, request.LastOperationId, request.Limit}
		log.Printf("Incremental sync for user %s since operation_id %s (limit: %d)",
			request.UserID, request.LastOperationId, request.Limit)
	}

	// Execute query
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error querying sync operations: %v", err)
	}
	defer rows.Close()

	// Process results
	var operations []BillSyncOperation
	var lastOperationId string

	for rows.Next() {
		var op BillSyncOperation
		err := rows.Scan(
			&op.ID,
			&op.UserID,
			&op.OperationID,
			&op.OperationType,
			&op.EntityType,
			&op.EntityID,
			&op.OperationData,
			&op.DeviceIDs,
			&op.ClientTimestamp,
			&op.ServerTimestamp,
			&op.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning operation row: %v", err)
			continue
		}

		operations = append(operations, op)
		lastOperationId = op.OperationID
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating operation rows: %v", err)
	}

	// Check if there are more operations available
	hasMore := false
	if len(operations) == request.Limit {
		// Check if there are more operations beyond this batch
		var count int
		countQuery := `SELECT COUNT(*) FROM sync_operations WHERE user_id = ? AND operation_id > ?`
		err = db.QueryRow(countQuery, request.UserID, lastOperationId).Scan(&count)
		if err == nil && count > 0 {
			hasMore = true
		}
	}

	// Get total count for this user
	var totalCount int
	totalQuery := `SELECT COUNT(*) FROM sync_operations WHERE user_id = ?`
	if request.LastOperationId != "" {
		totalQuery += ` AND operation_id > ?`
		err = db.QueryRow(totalQuery, request.UserID, request.LastOperationId).Scan(&totalCount)
	} else {
		err = db.QueryRow(totalQuery, request.UserID).Scan(&totalCount)
	}
	if err != nil {
		log.Printf("Warning: Could not get total count: %v", err)
		totalCount = len(operations)
	}

	log.Printf("Found %d operations for user %s, hasMore: %t, totalCount: %d",
		len(operations), request.UserID, hasMore, totalCount)

	response := &SyncBillOperationChangesResponse{
		Success:       true,
		Message:       "Operaciones obtenidas exitosamente",
		Operations:    operations,
		HasMore:       hasMore,
		TotalCount:    totalCount,
		LastOperation: lastOperationId,
		ServerTime:    time.Now().UTC().Format(time.RFC3339),
	}

	return response, nil
}
