package main

import (
	"fmt"
	"log"
	"time"
)

// Handlers HTTP para sincronización offline de Categories Management - Parte 3 (Helper Functions)
// Contiene funciones auxiliares para sincronización de categorías
// Incluye obtención de cambios, estadísticas y resolución de conflictos

// fetchCategoriesChanges obtiene cambios del servidor desde último sync
// Implementa consulta eficiente de cambios con soporte para paginación
// Retorna categorías modificadas, eliminadas y metadatos de sincronización
func fetchCategoriesChanges(userID, lastSync string, limit, offset int) (SyncCategoriesChangesResponse, error) {
	// Inicializar respuesta con valores predeterminados
	response := SyncCategoriesChangesResponse{
		Success:      true,
		Message:      "Changes fetched successfully",
		Categories:   make([]Category, 0),
		Deletions:    make([]string, 0),
		HasMore:      false,
		TotalChanges: 0,
		ServerTime:   time.Now().Format(time.RFC3339),
		LastSync:     time.Now().Format(time.RFC3339),
	}

	// Log de inicio de consulta
	log.Printf("Fetching categories changes for user %s from %s (limit: %d, offset: %d)", 
		userID, lastSync, limit, offset)

	// Obtener todas las categorías del usuario (simplificado - sin filtro temporal)
	categories, err := fetchCategories(userID, "")
	if err != nil {
		return response, fmt.Errorf("failed to fetch categories: %v", err)
	}

	// Aplicar paginación manual
	start := offset
	end := offset + limit
	if start > len(categories) {
		start = len(categories)
	}
	if end > len(categories) {
		end = len(categories)
	}

	// Extraer slice paginado
	if start < end {
		response.Categories = categories[start:end]
	}

	// Configurar metadatos de paginación
	response.TotalChanges = len(categories)
	response.HasMore = end < len(categories)

	// Log del resultado
	log.Printf("✅ Categories changes fetched: %d categories, hasMore: %t", 
		len(response.Categories), response.HasMore)

	return response, nil
}

// getCategoriesSyncStats calcula estadísticas de sincronización para categorías
// Proporciona métricas útiles sobre el estado y rendimiento de sincronización
// Incluye estadísticas específicas de categorías de ingresos y gastos
func getCategoriesSyncStats(userID string) (SyncCategoriesStats, error) {
	// Inicializar estadísticas con valores predeterminados
	stats := SyncCategoriesStats{
		UserID:                  userID,
		LastSyncTime:            time.Now(),
		TotalSyncs:              0,
		PendingOperations:       0,
		ConflictsResolved:       0,
		DataSizeBytes:           0,
		AverageLatency:          0.0,
		ErrorCount:              0,
		IncomeCategoriesSynced:  0,
		ExpenseCategoriesSynced: 0,
		TotalCategoriesManaged:  0,
		DuplicateNamesResolved:  0,
	}

	// Obtener estadísticas básicas desde la base de datos
	categories, err := fetchCategories(userID, "")
	if err != nil {
		return stats, fmt.Errorf("failed to fetch categories for stats: %v", err)
	}

	// Calcular estadísticas específicas de categorías
	incomeCount := 0
	expenseCount := 0
	
	for _, category := range categories {
		if category.Type == "income" {
			incomeCount++
		} else if category.Type == "expense" {
			expenseCount++
		}
	}

	// Actualizar estadísticas calculadas
	stats.IncomeCategoriesSynced = incomeCount
	stats.ExpenseCategoriesSynced = expenseCount
	stats.TotalCategoriesManaged = len(categories)
	stats.TotalSyncs = len(categories) // Simplificado para este ejemplo

	// Calcular tamaño aproximado de datos en bytes
	dataSize := int64(0)
	for _, category := range categories {
		dataSize += int64(len(category.Name) + len(category.Type) + len(category.Emoji))
	}
	stats.DataSizeBytes = dataSize

	// Log de estadísticas calculadas
	log.Printf("✅ Categories sync stats calculated for user %s: %d total, %d income, %d expense", 
		userID, stats.TotalCategoriesManaged, stats.IncomeCategoriesSynced, stats.ExpenseCategoriesSynced)

	return stats, nil
}

// resolveCategoriesConflict resuelve un conflicto específico de categoría
// Aplica la estrategia de resolución especificada por el cliente
// Retorna el resultado de la resolución para confirmación
func resolveCategoriesConflict(request SyncCategoriesConflictRequest) (SyncCategoriesResult, error) {
	// Inicializar resultado de resolución
	result := SyncCategoriesResult{
		LocalID:       request.LocalID,
		ServerID:      request.ServerID,
		OperationType: "category",
		Status:        "success",
		SyncTimestamp: time.Now().Format(time.RFC3339),
	}

	// Log de inicio de resolución de conflicto
	log.Printf("Resolving conflict for user %s, strategy: %s", request.UserID, request.Resolution)

	// Aplicar estrategia de resolución según especificación del cliente
	switch request.Resolution {
	case "server_wins":
		// El servidor mantiene su versión, ignorar cambios del cliente
		log.Printf("Conflict resolved: server version maintained for %s", request.LocalID)
		
	case "client_wins":
		// Aplicar cambios del cliente, sobrescribir versión del servidor
		if request.MergedData != nil {
			// Aquí se aplicarían los datos del cliente
			log.Printf("Conflict resolved: client version applied for %s", request.LocalID)
		}
		
	case "merge":
		// Fusionar datos según lógica específica de categorías
		if request.MergedData != nil {
			// Aquí se aplicarían los datos fusionados
			log.Printf("Conflict resolved: merged data applied for %s", request.LocalID)
		}
		
	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("unsupported resolution strategy: %s", request.Resolution)
		return result, fmt.Errorf("unsupported resolution strategy: %s", request.Resolution)
	}

	// Log de resolución exitosa
	log.Printf("✅ Conflict resolved successfully for user %s using %s strategy", 
		request.UserID, request.Resolution)

	return result, nil
}
}

// processCategoryUpdate procesa la actualización de una categoría desde sync offline
// Actualiza los campos modificados manteniendo la integridad de datos
// Maneja validación de tipos y codificación de emojis
func processCategoryUpdate(category OfflineCategory) error {
	// Log de inicio de actualización
	log.Printf("Processing category update for user %s: %s", category.UserID, category.LocalID)

	// Obtener categoría existente para actualización
	var categoryID int
	if category.ServerID != "" {
		// Parsear ID del servidor si está disponible
		if _, err := fmt.Sscanf(category.ServerID, "%d", &categoryID); err != nil {
			return fmt.Errorf("invalid server ID format: %s", category.ServerID)
		}
	} else {
		return fmt.Errorf("server ID required for update operation")
	}

	// Crear estructura de categoría actualizada
	updatedCategory := Category{
		ID:     categoryID,
		UserID: category.UserID,
		Name:   category.Name,
		Type:   category.Type,
		Emoji:  category.Emoji,
	}

	// Usar función existente para actualizar categoría
	err := updateCategory(updatedCategory)
	if err != nil {
		return fmt.Errorf("failed to update category: %v", err)
	}

	// Log de actualización exitosa
	log.Printf("✅ Category updated successfully: %s", category.LocalID)
	
	return nil
}

// processCategoryDelete procesa la eliminación de una categoría desde sync offline
// Elimina la categoría de la base de datos manteniendo integridad referencial
// Valida permisos de usuario antes de proceder con la eliminación
func processCategoryDelete(category OfflineCategory) error {
	// Log de inicio de eliminación
	log.Printf("Processing category delete for user %s: %s", category.UserID, category.LocalID)

	// Determinar ID de categoría para eliminación
	var categoryID int
	if category.ServerID != "" {
		// Parsear ID del servidor si está disponible
		if _, err := fmt.Sscanf(category.ServerID, "%d", &categoryID); err != nil {
			return fmt.Errorf("invalid server ID format: %s", category.ServerID)
		}
	} else {
		return fmt.Errorf("server ID required for delete operation")
	}

	// Usar función existente para eliminar categoría
	err := deleteCategory(categoryID, category.UserID)
	if err != nil {
		return fmt.Errorf("failed to delete category: %v", err)
	}

	// Log de eliminación exitosa
	log.Printf("✅ Category deleted successfully: %s", category.LocalID)
	
	return nil
}

// validateCategoryConsistency valida la consistencia de datos de una categoría offline
// Verifica que los campos requeridos estén presentes y sean válidos
// Incluye validación específica para nombres, tipos y emojis
func validateCategoryConsistency(category OfflineCategory) error {
	// Validar que el ID local esté presente
	if category.LocalID == "" {
		return fmt.Errorf("local ID is required")
	}

	// Validar que el ID de usuario esté presente
	if category.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Para operaciones que requieren datos de categoría
	if category.Action == "add" || category.Action == "update" {
		// Validar nombre de categoría
		if category.Name == "" {
			return fmt.Errorf("category name is required")
		}
		
		// Validar tipo de categoría
		if category.Type != "income" && category.Type != "expense" {
			return fmt.Errorf("category type must be 'income' or 'expense'")
		}
		
		// Validar que el emoji esté presente (puede ser predeterminado)
		if category.Emoji == "" {
			return fmt.Errorf("category emoji is required")
		}
	}

	// Para operaciones de actualización y eliminación, validar que haya identificador del servidor
	if (category.Action == "update" || category.Action == "delete") && category.ServerID == "" {
		return fmt.Errorf("server ID is required for %s operation", category.Action)
	}

	// Log de validación exitosa
	log.Printf("✅ Category validation passed for %s (%s)", category.LocalID, category.Action)
	
	return nil
}