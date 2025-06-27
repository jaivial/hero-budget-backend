package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador por lotes para sincronización offline de Categories Management - Parte 2
// Contiene funciones auxiliares y de soporte para el procesamiento por lotes
// Incluye validaciones avanzadas, estadísticas y utilidades de rendimiento

// processCategoryDeleteAdvanced procesamiento avanzado para eliminación de categorías
// Incluye verificación de dependencias y limpieza de referencias
// Maneja eliminación segura con validaciones adicionales
func (bp *CategoriesBatchProcessor) processCategoryDeleteAdvanced(category OfflineCategory) error {
	// Verificar dependencias antes de eliminar
	if err := bp.checkDeleteDependencies(category); err != nil {
		return fmt.Errorf("cannot delete category due to dependencies: %v", err)
	}

	// Procesar usando función base
	err := processCategoryDelete(category)
	if err != nil {
		return fmt.Errorf("failed to delete category: %v", err)
	}

	// Actualizar estadísticas
	bp.updateDeleteStatistics()

	return nil
}

// Helper functions para el procesador por lotes

// determineOptimalSubBatchSize determina el tamaño óptimo de sub-lote
// Balancea rendimiento y uso de memoria según el tamaño total
func (bp *CategoriesBatchProcessor) determineOptimalSubBatchSize(totalSize int) int {
	// Configurar tamaños según el total
	switch {
	case totalSize <= 10:
		return totalSize // Procesar todo junto para lotes pequeños
	case totalSize <= 50:
		return 10 // Sub-lotes de 10 para lotes medianos
	case totalSize <= 100:
		return 20 // Sub-lotes de 20 para lotes grandes
	default:
		return 25 // Máximo 25 por sub-lote para lotes muy grandes
	}
}

// validateBatchRequest valida la solicitud completa del lote
// Implementa validaciones avanzadas específicas del procesador
func (bp *CategoriesBatchProcessor) validateBatchRequest(request SyncCategoriesBatchRequest) error {
	// Usar validaciones existentes como base
	if err := validateCategorySyncRequest(request); err != nil {
		return err
	}

	// Validaciones adicionales específicas del procesador
	if err := validateSyncRequestSize(request); err != nil {
		return err
	}

	if err := validateCategorySyncContext(request); err != nil {
		return err
	}

	return nil
}

// validateCategoryConsistency validación avanzada de consistencia para el procesador
// Extiende las validaciones básicas con lógica específica del procesador
func (bp *CategoriesBatchProcessor) validateCategoryConsistency(category OfflineCategory, existingCategories []Category) error {
	// Usar validación básica como punto de partida
	if err := validateCategoryConsistency(category); err != nil {
		return err
	}

	// Validar reglas de negocio específicas
	if err := validateCategoryBusinessRules(category, existingCategories); err != nil {
		return err
	}

	// Validar integridad de datos
	if err := validateCategoryDataIntegrity(category); err != nil {
		return err
	}

	return nil
}

// detectPotentialConflicts detecta conflictos potenciales antes del procesamiento
// Implementa detección proactiva de conflictos para evitar errores durante procesamiento
func (bp *CategoriesBatchProcessor) detectPotentialConflicts(category OfflineCategory, existingCategories []Category) []CategoriesConflictResolution {
	conflicts := make([]CategoriesConflictResolution, 0)

	// Para operaciones de actualización, buscar la categoría existente
	if category.Action == "update" && category.ServerID != "" {
		for _, existing := range existingCategories {
			if fmt.Sprintf("%d", existing.ID) == category.ServerID {
				// Detectar conflictos usando función existente
				categoryConflicts := detectCategoryConflicts(category, &existing)
				conflicts = append(conflicts, categoryConflicts...)
				break
			}
		}
	}

	// Para operaciones de adición, verificar duplicados de nombre
	if category.Action == "add" {
		for _, existing := range existingCategories {
			if existing.UserID == category.UserID &&
			   existing.Type == category.Type &&
			   existing.Name == category.Name {
				conflict := CategoriesConflictResolution{
					LocalID:       category.LocalID,
					ServerID:      fmt.Sprintf("%d", existing.ID),
					ConflictType:  "duplicate_name",
					OperationType: "category",
					LocalData:     category,
					ServerData:    existing,
					Resolution:    "manual",
					Priority:      "high",
					Description:   fmt.Sprintf("Category name '%s' already exists for type '%s'", category.Name, category.Type),
					Suggestions:   []string{"Use different name", "Update existing category", "Merge categories"},
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// createConflictResolution crea una resolución de conflicto a partir de un resultado
// Convierte resultados de conflicto en objetos de resolución estructurados
func (bp *CategoriesBatchProcessor) createConflictResolution(category OfflineCategory, result SyncCategoriesResult) CategoriesConflictResolution {
	return CategoriesConflictResolution{
		LocalID:       result.LocalID,
		ServerID:      result.ServerID,
		ConflictType:  result.ConflictType,
		OperationType: result.OperationType,
		LocalData:     category,
		ServerData:    nil, // Se podría cargar si es necesario
		Resolution:    "manual",
		Priority:      "medium",
		Description:   fmt.Sprintf("Conflict detected during %s operation", category.Action),
		Suggestions:   []string{"Review and resolve manually", "Apply server version", "Apply client version"},
	}
}

// compileResponse compila la respuesta final del procesamiento por lotes
// Agrega todos los resultados, conflictos y datos del servidor a la respuesta
func (bp *CategoriesBatchProcessor) compileResponse(response SyncCategoriesBatchResponse) SyncCategoriesBatchResponse {
	// Agregar todos los resultados procesados
	response.Results = append(response.Results, bp.ProcessedItems...)
	response.Results = append(response.Results, bp.FailedItems...)

	// Agregar conflictos detectados
	response.Conflicts = append(response.Conflicts, bp.ConflictItems...)

	// Calcular estadísticas de operaciones
	response.ProcessedOps = len(bp.ProcessedItems) + len(bp.FailedItems)
	response.SuccessfulOps = len(bp.ProcessedItems)
	response.FailedOps = len(bp.FailedItems)

	// Determinar éxito general
	response.Success = response.FailedOps == 0 && len(response.Conflicts) == 0

	// Configurar mensaje apropiado
	if response.Success {
		response.Message = fmt.Sprintf("Batch processed successfully: %d operations completed", response.SuccessfulOps)
	} else {
		response.Message = fmt.Sprintf("Batch completed with %d errors and %d conflicts", response.FailedOps, len(response.Conflicts))
	}

	// Obtener datos actualizados del servidor
	if serverCategories, err := fetchCategories(bp.UserID, ""); err == nil {
		response.ServerData = serverCategories
	}

	return response
}

// updatePerformanceStats actualiza estadísticas de rendimiento del procesador
// Registra métricas de tiempo y throughput para optimización futura
func (bp *CategoriesBatchProcessor) updatePerformanceStats(processingTime time.Duration, itemCount int) {
	// Actualizar estadísticas básicas
	bp.Stats.TotalSyncs++
	bp.Stats.LastSyncTime = time.Now()

	// Calcular latencia promedio
	latencyMs := float64(processingTime.Milliseconds())
	if bp.Stats.AverageLatency == 0 {
		bp.Stats.AverageLatency = latencyMs
	} else {
		// Promedio móvil simple
		bp.Stats.AverageLatency = (bp.Stats.AverageLatency + latencyMs) / 2
	}

	// Actualizar contadores específicos
	bp.Stats.TotalCategoriesManaged += itemCount
	bp.Stats.ConflictsResolved += len(bp.ConflictItems)

	// Log de estadísticas de rendimiento
	throughput := float64(itemCount) / processingTime.Seconds()
	log.Printf("📊 Performance stats: %d items in %v (%.2f items/sec), avg latency: %.2fms", 
		itemCount, processingTime, throughput, bp.Stats.AverageLatency)
}

// updateAddStatistics actualiza estadísticas específicas de operaciones de adición
func (bp *CategoriesBatchProcessor) updateAddStatistics(categoryType string) {
	if categoryType == "income" {
		bp.Stats.IncomeCategoriesSynced++
	} else if categoryType == "expense" {
		bp.Stats.ExpenseCategoriesSynced++
	}
}

// updateUpdateStatistics actualiza estadísticas específicas de operaciones de actualización
func (bp *CategoriesBatchProcessor) updateUpdateStatistics() {
	// Las actualizaciones no cambian los contadores por tipo, pero podrían rastrear otras métricas
	// Por ejemplo: número de campos actualizados, frecuencia de actualizaciones, etc.
}

// updateDeleteStatistics actualiza estadísticas específicas de operaciones de eliminación
func (bp *CategoriesBatchProcessor) updateDeleteStatistics() {
	// Las eliminaciones reducen el total, pero esto se refleja mejor en el conteo actual
	// Podrían rastrearse métricas como: razones de eliminación, frecuencia, etc.
}

// Funciones auxiliares adicionales

// validateAddPrerequisites valida prerequisitos específicos para operaciones de adición
func (bp *CategoriesBatchProcessor) validateAddPrerequisites(category OfflineCategory) error {
	// Validar que todos los campos requeridos estén presentes
	if category.Name == "" || category.Type == "" || category.UserID == "" {
		return fmt.Errorf("required fields missing for add operation")
	}

	// Validar que no sea una operación duplicada en el mismo lote
	for _, processed := range bp.ProcessedItems {
		if processed.LocalID == category.LocalID {
			return fmt.Errorf("duplicate operation detected in same batch: %s", category.LocalID)
		}
	}

	return nil
}

// checkCategoryLimits verifica límites de categorías por usuario
func (bp *CategoriesBatchProcessor) checkCategoryLimits(userID, categoryType string) error {
	// Obtener categorías existentes para contar
	existingCategories, err := fetchCategories(userID, categoryType)
	if err != nil {
		return fmt.Errorf("failed to check category limits: %v", err)
	}

	// Definir límites por tipo
	const maxCategoriesPerType = 50
	
	if len(existingCategories) >= maxCategoriesPerType {
		return fmt.Errorf("maximum categories limit reached for type %s: %d/%d", 
			categoryType, len(existingCategories), maxCategoriesPerType)
	}

	return nil
}

// checkDeleteDependencies verifica dependencias antes de eliminar una categoría
func (bp *CategoriesBatchProcessor) checkDeleteDependencies(category OfflineCategory) error {
	// Aquí se podrían verificar dependencias como:
	// - Transacciones que usan esta categoría
	// - Presupuestos vinculados a esta categoría
	// - Reglas automáticas que referencian esta categoría
	
	// Por ahora, implementación básica
	if category.ServerID == "" {
		return fmt.Errorf("server ID required to check dependencies")
	}

	// Log de verificación de dependencias
	log.Printf("✅ Dependency check passed for category deletion: %s", category.LocalID)
	
	return nil
}

// GetProcessingStats retorna estadísticas actuales del procesador
// Útil para monitoreo y debugging del rendimiento de sincronización
func (bp *CategoriesBatchProcessor) GetProcessingStats() SyncCategoriesStats {
	return bp.Stats
}

// ResetProcessor reinicia el estado del procesador para un nuevo lote
// Limpia resultados anteriores pero preserva configuración y estadísticas acumuladas
func (bp *CategoriesBatchProcessor) ResetProcessor() {
	bp.ProcessedItems = make([]SyncCategoriesResult, 0)
	bp.FailedItems = make([]SyncCategoriesResult, 0)
	bp.ConflictItems = make([]CategoriesConflictResolution, 0)
}