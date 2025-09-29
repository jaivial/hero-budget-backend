package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador de lotes para sincronización offline de ahorros - Parte 1
// Implementa la lógica principal de procesamiento siguiendo el patrón de budget_management

// processSavingsBatch procesa un lote completo de operaciones de sincronización de ahorros
// Retorna respuesta detallada con resultados y conflictos detectados
func processSavingsBatch(request SyncSavingsBatchRequest) (*SyncSavingsBatchResponse, error) {
	log.Printf("Procesando lote de sincronización: %d registros de ahorros para usuario %s", len(request.Savings), request.UserID)

	// Inicializar respuesta del lote
	response := &SyncSavingsBatchResponse{
		Success:       true,
		Message:       "Lote procesado exitosamente",
		ProcessedOps:  0,
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncSavingsResult, 0, len(request.Savings)),
		Conflicts:     make([]SavingsConflictResolution, 0),
		ServerData:    make([]SavingsData, 0),
		LastSync:      time.Now().UTC().Format(time.RFC3339),
		NextSyncTime:  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
	}

	// Procesar cada registro de ahorro en el lote
	for _, offlineSavings := range request.Savings {
		response.ProcessedOps++

		// Procesar operación individual
		result, serverSavings, err := processSingleSavingsOperation(offlineSavings)

		// Agregar resultado a la respuesta
		response.Results = append(response.Results, result)

		if err != nil {
			response.FailedOps++
			log.Printf("Error procesando ahorro %s: %v", offlineSavings.LocalID, err)
			continue
		}

		response.SuccessfulOps++

		// Agregar datos del servidor si la operación fue exitosa
		if serverSavings != nil {
			response.ServerData = append(response.ServerData, *serverSavings)
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

// processSingleSavingsOperation procesa una operación individual de ahorro
// Retorna el resultado de la operación y los datos del servidor actualizados
func processSingleSavingsOperation(offlineSavings OfflineSavings) (SyncSavingsResult, *SavingsData, error) {
	// Inicializar resultado
	result := SyncSavingsResult{
		LocalID:       offlineSavings.LocalID,
		Action:        offlineSavings.Action,
		Status:        "success",
		SyncTimestamp: time.Now().UTC().Format(time.RFC3339),
	}

	var serverSavings *SavingsData
	var err error

	// Procesar según el tipo de acción
	switch offlineSavings.Action {
	case "add":
		serverSavings, err = processAddSavingsOperation(offlineSavings)
	case "update":
		serverSavings, err = processUpdateSavingsOperation(offlineSavings)
	case "delete":
		err = processDeleteSavingsOperation(offlineSavings)
	default:
		err = fmt.Errorf("acción no soportada: %s", offlineSavings.Action)
	}

	// Manejar errores
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, nil, err
	}

	// Asignar ServerID si es aplicable
	if serverSavings != nil {
		result.ServerID = fmt.Sprintf("savings_%s", serverSavings.UserID)
	}

	return result, serverSavings, nil
}

// processAddSavingsOperation procesa la adición de un nuevo registro de ahorro
// Crea el registro aplicando validaciones y cálculos necesarios
func processAddSavingsOperation(offlineSavings OfflineSavings) (*SavingsData, error) {
	// Crear estructura para agregar ahorro
	newSavings := SavingsData{
		UserID:      offlineSavings.UserID,
		Available:   offlineSavings.Available,
		Goal:        offlineSavings.Goal,
		Period:      offlineSavings.Period,
		Percent:     offlineSavings.Percent,
		NeedToSave:  offlineSavings.NeedToSave,
		DailyTarget: offlineSavings.DailyTarget,
	}

	// Si no se proporciona período, usar mensual por defecto
	if newSavings.Period == "" {
		newSavings.Period = "monthly"
	}

	// Recalcular valores derivados para asegurar consistencia
	if err := recalculateSavingsValues(&newSavings); err != nil {
		return nil, fmt.Errorf("error recalculando valores: %v", err)
	}

	// Verificar si ya existe un registro de ahorro para este usuario
	existingSavings, err := fetchSavingsData(newSavings.UserID)
	if err != nil {
		return nil, fmt.Errorf("error verificando ahorro existente: %v", err)
	}

	// Si ya existe y tiene datos significativos, es un conflicto
	if existingSavings.Goal > 0 || existingSavings.Available > 0 {
		return nil, fmt.Errorf("ya existe un registro de ahorro para el usuario %s", newSavings.UserID)
	}

	// Insertar nuevo registro de ahorro en la base de datos
	err = updateSavingsData(newSavings)
	if err != nil {
		return nil, fmt.Errorf("error insertando registro de ahorro: %v", err)
	}

	log.Printf("Registro de ahorro creado exitosamente para usuario %s", newSavings.UserID)
	return &newSavings, nil
}

// processUpdateSavingsOperation procesa la actualización de un registro de ahorro existente
// Actualiza los campos especificados manteniendo consistencia de datos
func processUpdateSavingsOperation(offlineSavings OfflineSavings) (*SavingsData, error) {
	// Obtener datos actuales del ahorro
	existingSavings, err := fetchSavingsData(offlineSavings.UserID)
	if err != nil {
		return nil, fmt.Errorf("registro de ahorro no encontrado: %v", err)
	}

	// Actualizar campos especificados
	updatedSavings := existingSavings

	// Solo actualizar campos que no sean cero (para permitir updates parciales)
	if offlineSavings.Available > 0 {
		updatedSavings.Available = offlineSavings.Available
	}
	if offlineSavings.Goal > 0 {
		updatedSavings.Goal = offlineSavings.Goal
	}
	if offlineSavings.Period != "" {
		updatedSavings.Period = offlineSavings.Period
	}
	// Para percent, need_to_save y daily_target se recalculan automáticamente

	// Recalcular valores derivados
	if err := recalculateSavingsValues(&updatedSavings); err != nil {
		return nil, fmt.Errorf("error recalculando valores: %v", err)
	}

	// Actualizar en la base de datos
	err = updateSavingsData(updatedSavings)
	if err != nil {
		return nil, fmt.Errorf("error actualizando registro de ahorro: %v", err)
	}

	log.Printf("Registro de ahorro actualizado exitosamente para usuario %s", updatedSavings.UserID)
	return &updatedSavings, nil
}

// processDeleteSavingsOperation procesa la eliminación de un registro de ahorro
// Elimina el registro de la base de datos
func processDeleteSavingsOperation(offlineSavings OfflineSavings) error {
	// Verificar que el registro de ahorro existe
	_, err := fetchSavingsData(offlineSavings.UserID)
	if err != nil {
		return fmt.Errorf("registro de ahorro no encontrado para eliminar: %v", err)
	}

	// Eliminar registro de ahorro de la base de datos
	err = deleteSavingsData(offlineSavings.UserID)
	if err != nil {
		return fmt.Errorf("error eliminando registro de ahorro: %v", err)
	}

	log.Printf("Registro de ahorro eliminado exitosamente para usuario %s", offlineSavings.UserID)
	return nil
}

// recalculateSavingsValues recalcula los valores derivados de un registro de ahorro
// Asegura la consistencia matemática entre los diferentes campos
func recalculateSavingsValues(savings *SavingsData) error {
	// Calcular el porcentaje de progreso hacia la meta
	if savings.Goal > 0 {
		savings.Percent = (savings.Available / savings.Goal) * 100
		if savings.Percent > 100 {
			savings.Percent = 100 // Máximo 100%
		}
	} else {
		savings.Percent = 0
	}

	// Calcular cantidad necesaria para alcanzar la meta
	savings.NeedToSave = savings.Goal - savings.Available
	if savings.NeedToSave < 0 {
		savings.NeedToSave = 0 // No puede ser negativo
	}

	// Calcular target diario asumiendo 30 días por mes
	if savings.NeedToSave > 0 {
		savings.DailyTarget = savings.NeedToSave / 30
	} else {
		savings.DailyTarget = 0
	}

	// Validar que los valores recalculados sean coherentes
	if savings.Available < 0 {
		return fmt.Errorf("available no puede ser negativo después del recálculo")
	}
	if savings.Goal < 0 {
		return fmt.Errorf("goal no puede ser negativo después del recálculo")
	}
	if savings.Percent < 0 || savings.Percent > 100 {
		return fmt.Errorf("percent debe estar entre 0 y 100 después del recálculo")
	}

	return nil
}

// getSavingsChanges obtiene cambios de ahorros desde el último sync
// Implementa lógica para obtener actualizaciones del servidor
func getSavingsChanges(request SyncSavingsChangesRequest) (*SyncSavingsChangesResponse, error) {
	log.Printf("Obteniendo cambios de ahorros para usuario %s desde %s", request.UserID, request.LastSync)

	// Inicializar respuesta
	response := &SyncSavingsChangesResponse{
		Success:      true,
		Message:      "Cambios obtenidos exitosamente",
		Changes:      make([]SavingsData, 0),
		Deletions:    make([]string, 0),
		HasMore:      false,
		TotalChanges: 0,
		LastSync:     time.Now().UTC().Format(time.RFC3339),
		ServerTime:   time.Now().UTC().Format(time.RFC3339),
	}

	// Obtener datos actuales del usuario
	currentSavings, err := fetchSavingsData(request.UserID)
	if err != nil {
		// Si no hay datos, no es un error para sync
		log.Printf("No hay datos de ahorros para usuario %s", request.UserID)
		return response, nil
	}

	// Si hay datos, incluirlos en los cambios
	response.Changes = append(response.Changes, currentSavings)
	response.TotalChanges = 1

	return response, nil
}
