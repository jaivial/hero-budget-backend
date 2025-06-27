package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador de lotes para sincronización offline de presupuestos - Parte 1
// Implementa la lógica principal de procesamiento siguiendo el patrón de bills_management

// processBudgetBatch procesa un lote completo de operaciones de sincronización de presupuestos
// Retorna respuesta detallada con resultados y conflictos detectados
func processBudgetBatch(request SyncBudgetBatchRequest) (*SyncBudgetBatchResponse, error) {
	log.Printf("Procesando lote de sincronización: %d presupuestos para usuario %s", len(request.Budgets), request.UserID)
	
	// Inicializar respuesta del lote
	response := &SyncBudgetBatchResponse{
		Success:       true,
		Message:       "Lote procesado exitosamente",
		ProcessedOps:  0,
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncBudgetResult, 0, len(request.Budgets)),
		Conflicts:     make([]BudgetConflictResolution, 0),
		ServerData:    make([]BudgetData, 0),
		LastSync:      time.Now().UTC().Format(time.RFC3339),
		NextSyncTime:  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
	}
	
	// Procesar cada presupuesto en el lote
	for _, offlineBudget := range request.Budgets {
		response.ProcessedOps++
		
		// Procesar operación individual
		result, serverBudget, err := processSingleBudgetOperation(offlineBudget)
		
		// Agregar resultado a la respuesta
		response.Results = append(response.Results, result)
		
		if err != nil {
			response.FailedOps++
			log.Printf("Error procesando presupuesto %s: %v", offlineBudget.LocalID, err)
			continue
		}
		
		response.SuccessfulOps++
		
		// Agregar datos del servidor si la operación fue exitosa
		if serverBudget != nil {
			response.ServerData = append(response.ServerData, *serverBudget)
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

// processSingleBudgetOperation procesa una operación individual de presupuesto
// Retorna el resultado de la operación y los datos del servidor actualizados
func processSingleBudgetOperation(offlineBudget OfflineBudget) (SyncBudgetResult, *BudgetData, error) {
	// Inicializar resultado
	result := SyncBudgetResult{
		LocalID:       offlineBudget.LocalID,
		Action:        offlineBudget.Action,
		Status:        "success",
		SyncTimestamp: time.Now().UTC().Format(time.RFC3339),
	}
	
	var serverBudget *BudgetData
	var err error
	
	// Procesar según el tipo de acción
	switch offlineBudget.Action {
	case "add":
		serverBudget, err = processAddBudgetOperation(offlineBudget)
	case "update":
		serverBudget, err = processUpdateBudgetOperation(offlineBudget)
	case "delete":
		err = processDeleteBudgetOperation(offlineBudget)
	default:
		err = fmt.Errorf("acción no soportada: %s", offlineBudget.Action)
	}
	
	// Manejar errores
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, nil, err
	}
	
	// Asignar ServerID si es aplicable
	if serverBudget != nil {
		result.ServerID = fmt.Sprintf("%s_%s", serverBudget.UserID, serverBudget.Period)
	}
	
	return result, serverBudget, nil
}

// processAddBudgetOperation procesa la adición de un nuevo presupuesto
// Crea el presupuesto aplicando validaciones y cálculos necesarios
func processAddBudgetOperation(offlineBudget OfflineBudget) (*BudgetData, error) {
	// Crear estructura para agregar presupuesto
	newBudget := BudgetData{
		UserID:          offlineBudget.UserID,
		Period:          offlineBudget.Period,
		Date:            offlineBudget.Date,
		TotalAmount:     offlineBudget.TotalAmount,
		RemainingAmount: offlineBudget.RemainingAmount,
		SpentAmount:     offlineBudget.SpentAmount,
		UpcomingAmount:  offlineBudget.UpcomingAmount,
		FromPrevious:    offlineBudget.FromPrevious,
		Percent:         offlineBudget.Percent,
		TotalIncome:     offlineBudget.TotalIncome,
	}
	
	// Si no se proporciona fecha, usar la actual
	if newBudget.Date == "" {
		newBudget.Date = time.Now().Format("2006-01-02")
	}
	
	// Recalcular valores derivados para asegurar consistencia
	if err := recalculateBudgetValues(&newBudget); err != nil {
		return nil, fmt.Errorf("error recalculando valores: %v", err)
	}
	
	// Verificar si ya existe un presupuesto para este usuario y período
	existingBudget, err := fetchBudgetData(newBudget.UserID, newBudget.Period)
	if err != nil {
		return nil, fmt.Errorf("error verificando presupuesto existente: %v", err)
	}
	
	// Si ya existe y tiene datos, es un conflicto
	if existingBudget.TotalAmount > 0 || existingBudget.SpentAmount > 0 {
		return nil, fmt.Errorf("ya existe un presupuesto para el período %s", newBudget.Period)
	}
	
	// Insertar nuevo presupuesto en la base de datos
	err = updateBudgetData(newBudget)
	if err != nil {
		return nil, fmt.Errorf("error insertando presupuesto: %v", err)
	}
	
	log.Printf("Presupuesto creado exitosamente para usuario %s, período %s", newBudget.UserID, newBudget.Period)
	return &newBudget, nil
}

// processUpdateBudgetOperation procesa la actualización de un presupuesto existente
// Actualiza los campos especificados manteniendo consistencia de datos
func processUpdateBudgetOperation(offlineBudget OfflineBudget) (*BudgetData, error) {
	// Obtener datos actuales del presupuesto
	existingBudget, err := fetchBudgetData(offlineBudget.UserID, offlineBudget.Period)
	if err != nil {
		return nil, fmt.Errorf("presupuesto no encontrado: %v", err)
	}
	
	// Actualizar campos especificados
	updatedBudget := existingBudget
	
	// Solo actualizar campos que no sean cero (para permitir updates parciales)
	if offlineBudget.TotalAmount != 0 {
		updatedBudget.TotalAmount = offlineBudget.TotalAmount
	}
	if offlineBudget.SpentAmount != 0 {
		updatedBudget.SpentAmount = offlineBudget.SpentAmount
	}
	if offlineBudget.UpcomingAmount != 0 {
		updatedBudget.UpcomingAmount = offlineBudget.UpcomingAmount
	}
	if offlineBudget.FromPrevious != 0 {
		updatedBudget.FromPrevious = offlineBudget.FromPrevious
	}
	if offlineBudget.TotalIncome != 0 {
		updatedBudget.TotalIncome = offlineBudget.TotalIncome
	}
	if offlineBudget.Date != "" {
		updatedBudget.Date = offlineBudget.Date
	}
	
	// Recalcular valores derivados
	if err := recalculateBudgetValues(&updatedBudget); err != nil {
		return nil, fmt.Errorf("error recalculando valores: %v", err)
	}
	
	// Actualizar en la base de datos
	err = updateBudgetData(updatedBudget)
	if err != nil {
		return nil, fmt.Errorf("error actualizando presupuesto: %v", err)
	}
	
	log.Printf("Presupuesto actualizado exitosamente para usuario %s, período %s", updatedBudget.UserID, updatedBudget.Period)
	return &updatedBudget, nil
}

// processDeleteBudgetOperation procesa la eliminación de un presupuesto
// Elimina el presupuesto de la base de datos
func processDeleteBudgetOperation(offlineBudget OfflineBudget) error {
	// Verificar que el presupuesto existe
	_, err := fetchBudgetData(offlineBudget.UserID, offlineBudget.Period)
	if err != nil {
		return fmt.Errorf("presupuesto no encontrado para eliminar: %v", err)
	}
	
	// Eliminar presupuesto de la base de datos
	_, err = db.Exec(`
		DELETE FROM budget 
		WHERE user_id = ? AND period = ?
	`, offlineBudget.UserID, offlineBudget.Period)
	
	if err != nil {
		return fmt.Errorf("error eliminando presupuesto: %v", err)
	}
	
	log.Printf("Presupuesto eliminado exitosamente para usuario %s, período %s", offlineBudget.UserID, offlineBudget.Period)
	return nil
}