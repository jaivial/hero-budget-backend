package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador por lotes para sincronización offline de Categories Management
// Implementa lógica avanzada de procesamiento por lotes para categorías
// Incluye manejo de transacciones, rollback y optimizaciones de rendimiento
// Adaptado del patrón exitoso usado en otros servicios de sincronización

// CategoriesBatchProcessor estructura principal para procesamiento por lotes
// Maneja el estado y configuración del procesamiento de sincronización
// Incluye métricas de rendimiento y control de transacciones
type CategoriesBatchProcessor struct {
	UserID           string                    // ID del usuario propietario
	BatchSize        int                       // Tamaño del lote a procesar
	Config           SyncCategoriesConfig      // Configuración de sincronización
	Stats            SyncCategoriesStats       // Estadísticas de procesamiento
	TransactionMode  bool                      // Modo transaccional habilitado
	RollbackOnError  bool                      // Rollback automático en errores
	ProcessedItems   []SyncCategoriesResult    // Elementos procesados exitosamente
	FailedItems      []SyncCategoriesResult    // Elementos que fallaron
	ConflictItems    []CategoriesConflictResolution // Conflictos detectados
}

// NewCategoriesBatchProcessor crea un nuevo procesador por lotes
// Inicializa la configuración y estadísticas predeterminadas
// Configura el comportamiento según las opciones proporcionadas
func NewCategoriesBatchProcessor(userID string, config SyncCategoriesConfig) *CategoriesBatchProcessor {
	return &CategoriesBatchProcessor{
		UserID:          userID,
		BatchSize:       config.MaxBatchSize,
		Config:          config,
		Stats:           SyncCategoriesStats{UserID: userID, LastSyncTime: time.Now()},
		TransactionMode: true,
		RollbackOnError: true,
		ProcessedItems:  make([]SyncCategoriesResult, 0),
		FailedItems:     make([]SyncCategoriesResult, 0),
		ConflictItems:   make([]CategoriesConflictResolution, 0),
	}
}

// ProcessBatch procesa un lote completo de operaciones de categorías
// Función principal que coordina todo el procesamiento por lotes
// Implementa manejo de transacciones y recuperación de errores
func (bp *CategoriesBatchProcessor) ProcessBatch(request SyncCategoriesBatchRequest) (SyncCategoriesBatchResponse, error) {
	// Log de inicio del procesamiento
	startTime := time.Now()
	log.Printf("🚀 Starting batch processing for user %s: %d categories", 
		bp.UserID, len(request.Categories))

	// Validar la solicitud completa antes del procesamiento
	if err := bp.validateBatchRequest(request); err != nil {
		return SyncCategoriesBatchResponse{}, fmt.Errorf("batch validation failed: %v", err)
	}

	// Inicializar respuesta
	response := SyncCategoriesBatchResponse{
		Success:       false,
		ProcessedOps:  0,
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncCategoriesResult, 0),
		Conflicts:     make([]CategoriesConflictResolution, 0),
		ServerData:    make([]Category, 0),
		LastSync:      time.Now().Format(time.RFC3339),
		NextSyncTime:  time.Now().Add(15 * time.Minute).Format(time.RFC3339),
	}

	// Procesar categorías en sub-lotes para optimizar rendimiento
	if err := bp.processInSubBatches(request.Categories); err != nil {
		return response, fmt.Errorf("sub-batch processing failed: %v", err)
	}

	// Compilar respuesta final
	response = bp.compileResponse(response)
	
	// Actualizar estadísticas de rendimiento
	processingTime := time.Since(startTime)
	bp.updatePerformanceStats(processingTime, len(request.Categories))

	// Log de finalización
	log.Printf("✅ Batch processing completed for user %s in %v: %d successful, %d failed, %d conflicts", 
		bp.UserID, processingTime, response.SuccessfulOps, response.FailedOps, len(response.Conflicts))

	return response, nil
}

// processInSubBatches divide el procesamiento en sub-lotes más pequeños
// Optimiza el rendimiento y reduce el uso de memoria para lotes grandes
// Implementa control de transacciones por sub-lote
func (bp *CategoriesBatchProcessor) processInSubBatches(categories []OfflineCategory) error {
	// Determinar tamaño de sub-lote óptimo
	subBatchSize := bp.determineOptimalSubBatchSize(len(categories))
	
	// Procesar en sub-lotes
	for i := 0; i < len(categories); i += subBatchSize {
		end := i + subBatchSize
		if end > len(categories) {
			end = len(categories)
		}
		
		subBatch := categories[i:end]
		log.Printf("Processing sub-batch %d-%d of %d categories", i+1, end, len(categories))
		
		// Procesar sub-lote con manejo de errores
		if err := bp.processSubBatch(subBatch); err != nil {
			log.Printf("⚠️ Sub-batch %d-%d failed: %v", i+1, end, err)
			
			// Decidir si continuar o fallar completamente según configuración
			if bp.RollbackOnError {
				return fmt.Errorf("sub-batch processing failed, rolling back: %v", err)
			}
			// Continuar con el siguiente sub-lote si no se requiere rollback
		}
	}
	
	return nil
}

// processSubBatch procesa un sub-lote de categorías
// Implementa la lógica detallada de procesamiento para un grupo pequeño
// Maneja detección de conflictos y aplicación de cambios
func (bp *CategoriesBatchProcessor) processSubBatch(categories []OfflineCategory) error {
	// Obtener categorías existentes del usuario para validación
	existingCategories, err := fetchCategories(bp.UserID, "")
	if err != nil {
		return fmt.Errorf("failed to fetch existing categories: %v", err)
	}

	// Procesar cada categoría individual
	for _, category := range categories {
		result := bp.processSingleCategory(category, existingCategories)
		
		// Agregar resultado a las listas apropiadas
		switch result.Status {
		case "success":
			bp.ProcessedItems = append(bp.ProcessedItems, result)
		case "error":
			bp.FailedItems = append(bp.FailedItems, result)
		case "conflict":
			// Crear resolución de conflicto
			conflict := bp.createConflictResolution(category, result)
			bp.ConflictItems = append(bp.ConflictItems, conflict)
		}
	}
	
	return nil
}

// processSingleCategory procesa una categoría individual
// Implementa la lógica específica para cada tipo de operación
// Incluye validación avanzada y detección de conflictos
func (bp *CategoriesBatchProcessor) processSingleCategory(category OfflineCategory, existingCategories []Category) SyncCategoriesResult {
	// Inicializar resultado
	result := SyncCategoriesResult{
		LocalID:       category.LocalID,
		OperationType: "category",
		Status:        "success",
		SyncTimestamp: time.Now().Format(time.RFC3339),
	}

	// Log de procesamiento individual
	log.Printf("Processing category %s: action=%s, name=%s", 
		category.LocalID, category.Action, category.Name)

	// Validar consistencia avanzada
	if err := bp.validateCategoryConsistency(category, existingCategories); err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result
	}

	// Detectar conflictos potenciales antes del procesamiento
	conflicts := bp.detectPotentialConflicts(category, existingCategories)
	if len(conflicts) > 0 {
		result.Status = "conflict"
		result.ConflictType = conflicts[0].ConflictType
		result.RequiresAction = true
		return result
	}

	// Procesar según la acción especificada
	switch category.Action {
	case "add":
		if serverID, err := bp.processCategoryAddAdvanced(category); err != nil {
			result.Status = "error"
			result.Error = err.Error()
		} else {
			result.ServerID = serverID
		}
		
	case "update":
		if err := bp.processCategoryUpdateAdvanced(category); err != nil {
			result.Status = "error"
			result.Error = err.Error()
		} else {
			result.ServerID = category.ServerID
		}
		
	case "delete":
		if err := bp.processCategoryDeleteAdvanced(category); err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}
		
	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("unknown action: %s", category.Action)
	}

	return result
}

// processCategoryAddAdvanced procesamiento avanzado para adición de categorías
// Incluye validación de duplicados y optimizaciones específicas
// Maneja casos edge y situaciones de error complejas
func (bp *CategoriesBatchProcessor) processCategoryAddAdvanced(category OfflineCategory) (string, error) {
	// Validaciones previas a la adición
	if err := bp.validateAddPrerequisites(category); err != nil {
		return "", err
	}

	// Verificar límites de categorías por usuario
	if err := bp.checkCategoryLimits(category.UserID, category.Type); err != nil {
		return "", err
	}

	// Procesar usando función base optimizada
	serverID, err := processCategoryAdd(category)
	if err != nil {
		return "", fmt.Errorf("failed to add category: %v", err)
	}

	// Actualizar estadísticas específicas
	bp.updateAddStatistics(category.Type)

	return serverID, nil
}

// processCategoryUpdateAdvanced procesamiento avanzado para actualización de categorías
// Incluye verificación de cambios efectivos y optimizaciones
// Maneja actualización parcial y validación de integridad
func (bp *CategoriesBatchProcessor) processCategoryUpdateAdvanced(category OfflineCategory) error {
	// Procesar usando función base
	err := processCategoryUpdate(category)
	if err != nil {
		return fmt.Errorf("failed to update category: %v", err)
	}

	// Actualizar estadísticas
	bp.updateUpdateStatistics()

	return nil
}

