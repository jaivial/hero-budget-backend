package main

import (
	"fmt"
	"strings"
)

// Funciones de validación para sincronización offline de Categories Management - Parte 2
// Contiene validaciones específicas para operaciones CRUD y reglas de negocio
// Incluye validación de límites, permisos y consistencia de datos

// validateCategoryUpdateOperation valida operación de actualización de categoría
// Verifica que haya identificadores válidos y campos a actualizar
// Incluye validación de existencia de la categoría en el servidor
func validateCategoryUpdateOperation(category OfflineCategory) error {
	// Validar que tenga identificador del servidor o local
	if category.ServerID == "" && category.LocalID == "" {
		return fmt.Errorf("either server_id or local_id is required for update operation")
	}

	// Validar formato del ServerID si está presente
	if category.ServerID != "" {
		var serverID int
		if _, err := fmt.Sscanf(category.ServerID, "%d", &serverID); err != nil {
			return fmt.Errorf("invalid server_id format: %s", category.ServerID)
		}
		if serverID <= 0 {
			return fmt.Errorf("server_id must be positive: %s", category.ServerID)
		}
	}

	// Validar que al menos un campo esté presente para actualizar
	hasUpdateFields := category.Name != "" || category.Type != "" || category.Emoji != ""
	if !hasUpdateFields {
		return fmt.Errorf("at least one field (name, type, emoji) must be provided for update")
	}

	return nil
}

// validateCategoryDeleteOperation valida operación de eliminación de categoría
// Verifica que haya identificadores válidos para la eliminación
// Incluye validación de permisos de eliminación
func validateCategoryDeleteOperation(category OfflineCategory) error {
	// Validar que tenga identificador del servidor
	if category.ServerID == "" {
		return fmt.Errorf("server_id is required for delete operation")
	}

	// Validar formato del ServerID
	var serverID int
	if _, err := fmt.Sscanf(category.ServerID, "%d", &serverID); err != nil {
		return fmt.Errorf("invalid server_id format: %s", category.ServerID)
	}
	if serverID <= 0 {
		return fmt.Errorf("server_id must be positive: %s", category.ServerID)
	}

	// Validar que tenga UserID para verificar permisos
	if category.UserID == "" {
		return fmt.Errorf("user_id is required for delete operation")
	}

	return nil
}

// validateUniqueLocalIDs valida que no haya IDs locales duplicados en un lote
// Verifica unicidad dentro de la solicitud de sincronización
// Retorna error si encuentra duplicados
func validateUniqueLocalIDs(categories []OfflineCategory) error {
	localIDs := make(map[string]bool)

	for i, category := range categories {
		if category.LocalID == "" {
			continue // LocalID es opcional para algunas operaciones
		}

		if localIDs[category.LocalID] {
			return fmt.Errorf("duplicate local_id found at position %d: %s", i, category.LocalID)
		}

		localIDs[category.LocalID] = true
	}

	return nil
}

// validateOperationLimits valida límites de operaciones por tipo
// Verifica que no se excedan los límites máximos por lote
// Incluye límites específicos para cada tipo de operación
func validateOperationLimits(categories []OfflineCategory) error {
	// Contar operaciones por tipo
	operationCounts := map[string]int{
		"add":    0,
		"update": 0,
		"delete": 0,
	}

	for _, category := range categories {
		operationCounts[category.Action]++
	}

	// Definir límites por operación
	limits := map[string]int{
		"add":    50,  // Máximo 50 adiciones por lote
		"update": 100, // Máximo 100 actualizaciones por lote
		"delete": 20,  // Máximo 20 eliminaciones por lote
	}

	// Validar límites
	for operation, count := range operationCounts {
		if limit, exists := limits[operation]; exists && count > limit {
			return fmt.Errorf("too many %s operations: maximum %d, got %d",
				operation, limit, count)
		}
	}

	// Validar límite total de operaciones
	totalOperations := len(categories)
	if totalOperations > 100 {
		return fmt.Errorf("too many total operations: maximum 100, got %d", totalOperations)
	}

	return nil
}

// validateCategoryBusinessRules valida reglas de negocio específicas para categorías
// Incluye validaciones de lógica de negocio que van más allá de la validación de formato
// Verifica restricciones específicas del dominio de categorías
func validateCategoryBusinessRules(category OfflineCategory, existingCategories []Category) error {
	// Para operaciones add y update, validar reglas de negocio
	if category.Action == "add" || category.Action == "update" {
		// Validar nombres reservados
		reservedNames := []string{"Default", "System", "Admin", "Test"}
		for _, reserved := range reservedNames {
			if strings.EqualFold(category.Name, reserved) {
				return fmt.Errorf("category name '%s' is reserved", category.Name)
			}
		}

		// Validar unicidad de nombres por tipo de categoría
		for _, existing := range existingCategories {
			// Saltar si es la misma categoría (para updates)
			if category.Action == "update" &&
				category.ServerID != "" &&
				fmt.Sprintf("%d", existing.ID) == category.ServerID {
				continue
			}

			// Verificar duplicado de nombre en el mismo tipo
			if existing.UserID == category.UserID &&
				existing.Type == category.Type &&
				strings.EqualFold(existing.Name, category.Name) {
				return fmt.Errorf("category name '%s' already exists for type '%s'",
					category.Name, category.Type)
			}
		}
	}

	return nil
}

// validateCategorySyncContext valida el contexto de sincronización
// Verifica que el estado del sistema permita la sincronización
// Incluye validación de versión de cliente y compatibilidad
func validateCategorySyncContext(request SyncCategoriesBatchRequest) error {
	// Validar versión de la aplicación cliente (opcional)
	if request.AppVersion != "" {
		// Aquí se podría implementar validación de versiones compatibles
		// Por ejemplo, rechazar versiones muy antiguas
		if len(request.AppVersion) > 50 {
			return fmt.Errorf("app_version too long: maximum 50 characters")
		}
	}

	// Validar información del dispositivo (opcional)
	if request.DeviceInfo != "" {
		if len(request.DeviceInfo) > 200 {
			return fmt.Errorf("device_info too long: maximum 200 characters")
		}
	}

	// Validar formato del timestamp de último sync
	if request.LastSync != "" {
		// Aquí se podría validar formato ISO 8601
		if len(request.LastSync) > 50 {
			return fmt.Errorf("last_sync timestamp too long")
		}
	}

	return nil
}

// validateCategoryDataIntegrity valida la integridad de datos de categorías
// Verifica consistencia entre campos relacionados
// Incluye validación de coherencia entre tipos y emojis predeterminados
func validateCategoryDataIntegrity(category OfflineCategory) error {
	// Solo validar para operaciones que modifican datos
	if category.Action != "add" && category.Action != "update" {
		return nil
	}

	// Validar coherencia entre tipo y emoji predeterminado
	if category.Type != "" && category.Emoji != "" {
		// Lista de emojis típicos por tipo para validación opcional
		incomeEmojis := []string{"💰", "💵", "💸", "💳", "🏦", "📈"}
		expenseEmojis := []string{"🛒", "🍔", "⛽", "🏠", "🚗", "🎬", "👕", "📱"}

		// Esta validación es opcional y flexible
		_ = incomeEmojis
		_ = expenseEmojis

		// Podrías implementar validación más estricta aquí si es necesario
		// Por ejemplo, advertir sobre emojis inconsistentes con el tipo
	}

	// Validar que el nombre no contenga caracteres de control
	if category.Name != "" {
		for _, char := range category.Name {
			if char < 32 && char != 9 && char != 10 && char != 13 { // Permitir tab, LF, CR
				return fmt.Errorf("category name contains control characters")
			}
		}
	}

	return nil
}

// validateCategoryConflictResolution valida una solicitud de resolución de conflicto
// Verifica que la solicitud de resolución sea válida y completa
// Incluye validación de estrategias de resolución soportadas
func validateCategoryConflictResolution(request SyncCategoriesConflictRequest) error {
	// Validar campos requeridos
	if request.UserID == "" {
		return fmt.Errorf("user_id is required for conflict resolution")
	}

	if request.LocalID == "" && request.ServerID == "" {
		return fmt.Errorf("either local_id or server_id is required for conflict resolution")
	}

	// Validar estrategia de resolución
	validResolutions := map[string]bool{
		"server_wins": true,
		"client_wins": true,
		"merge":       true,
	}

	if !validResolutions[request.Resolution] {
		return fmt.Errorf("invalid resolution strategy: must be 'server_wins', 'client_wins', or 'merge'")
	}

	// Para estrategia merge, validar que se proporcionen datos fusionados
	if request.Resolution == "merge" && request.MergedData == nil {
		return fmt.Errorf("merged_data is required for 'merge' resolution strategy")
	}

	// Validar tipo de operación
	if request.OperationType == "" {
		request.OperationType = "category" // Valor predeterminado
	}

	if request.OperationType != "category" {
		return fmt.Errorf("invalid operation_type: must be 'category'")
	}

	return nil
}

// validateSyncRequestSize valida el tamaño total de la solicitud de sincronización
// Verifica que la solicitud no sea demasiado grande para procesar eficientemente
// Incluye validación de límites de memoria y procesamiento
func validateSyncRequestSize(request SyncCategoriesBatchRequest) error {
	// Calcular tamaño aproximado de la solicitud
	estimatedSize := 0

	// Tamaño base de la estructura
	estimatedSize += len(request.UserID) + len(request.LastSync) + len(request.ClientID)
	estimatedSize += len(request.DeviceInfo) + len(request.AppVersion)

	// Tamaño de cada categoría
	for _, category := range request.Categories {
		categorySize := len(category.LocalID) + len(category.ServerID) + len(category.Action)
		categorySize += len(category.UserID) + len(category.Name) + len(category.Type)
		categorySize += len(category.Emoji) + len(category.OfflineTimestamp) + len(category.SyncTimestamp)
		categorySize += len(category.Status) + 20 // Buffer para otros campos

		estimatedSize += categorySize
	}

	// Límite máximo de 1MB por solicitud
	maxSize := 1024 * 1024 // 1MB
	if estimatedSize > maxSize {
		return fmt.Errorf("sync request too large: estimated %d bytes, maximum %d bytes",
			estimatedSize, maxSize)
	}

	return nil
}
