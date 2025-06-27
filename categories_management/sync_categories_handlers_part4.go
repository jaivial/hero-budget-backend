package main

import (
	"fmt"
	"log"
)

// Handlers HTTP para sincronización offline de Categories Management - Parte 4 (Processing Functions)
// Contiene funciones de procesamiento específicas para operaciones CRUD de categorías
// Incluye validación de consistencia y manejo de errores

// processCategoryAdd procesa la adición de una nueva categoría desde sync offline
// Crea la categoría en la base de datos y retorna el ID asignado por el servidor
// Maneja validación de nombres duplicados y codificación de emojis
func processCategoryAdd(category OfflineCategory) (string, error) {
	// Log de inicio de adición de categoría
	log.Printf("Processing category add for user %s: %s (%s)", 
		category.UserID, category.Name, category.Type)

	// Crear estructura de categoría compatible con función existente
	newCategory := Category{
		UserID: category.UserID,
		Name:   category.Name,
		Type:   category.Type,
		Emoji:  category.Emoji,
	}

	// Usar función existente para agregar categoría
	categoryID, err := addCategory(newCategory)
	if err != nil {
		return "", fmt.Errorf("failed to add category: %v", err)
	}

	// Convertir ID entero a string para compatibilidad
	serverID := fmt.Sprintf("%d", categoryID)
	
	// Log de adición exitosa
	log.Printf("✅ Category added successfully: %s -> server ID %s", category.LocalID, serverID)
	
	return serverID, nil
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

// detectCategoryConflicts detecta conflictos potenciales en operaciones de categorías
// Compara datos del cliente con el servidor para identificar inconsistencias
// Retorna lista de conflictos encontrados para resolución manual o automática
func detectCategoryConflicts(localCategory OfflineCategory, serverCategory *Category) []CategoriesConflictResolution {
	conflicts := make([]CategoriesConflictResolution, 0)

	// Solo verificar conflictos si existe una categoría en el servidor
	if serverCategory == nil {
		return conflicts
	}

	// Detectar conflictos de nombre
	if localCategory.Name != serverCategory.Name {
		conflict := CategoriesConflictResolution{
			LocalID:       localCategory.LocalID,
			ServerID:      fmt.Sprintf("%d", serverCategory.ID),
			ConflictType:  "data",
			OperationType: "category",
			LocalData:     localCategory,
			ServerData:    serverCategory,
			Resolution:    "manual",
			Priority:      "medium",
			Description:   fmt.Sprintf("Category name differs: local='%s', server='%s'", localCategory.Name, serverCategory.Name),
			Suggestions:   []string{"Keep local name", "Keep server name", "Merge names"},
		}
		conflicts = append(conflicts, conflict)
	}

	// Detectar conflictos de tipo
	if localCategory.Type != serverCategory.Type {
		conflict := CategoriesConflictResolution{
			LocalID:       localCategory.LocalID,
			ServerID:      fmt.Sprintf("%d", serverCategory.ID),
			ConflictType:  "data",
			OperationType: "category",
			LocalData:     localCategory,
			ServerData:    serverCategory,
			Resolution:    "manual",
			Priority:      "high",
			Description:   fmt.Sprintf("Category type differs: local='%s', server='%s'", localCategory.Type, serverCategory.Type),
			Suggestions:   []string{"Keep local type", "Keep server type"},
		}
		conflicts = append(conflicts, conflict)
	}

	// Detectar conflictos de emoji
	if localCategory.Emoji != serverCategory.Emoji {
		conflict := CategoriesConflictResolution{
			LocalID:       localCategory.LocalID,
			ServerID:      fmt.Sprintf("%d", serverCategory.ID),
			ConflictType:  "data",
			OperationType: "category",
			LocalData:     localCategory,
			ServerData:    serverCategory,
			Resolution:    "manual",
			Priority:      "low",
			Description:   fmt.Sprintf("Category emoji differs: local='%s', server='%s'", localCategory.Emoji, serverCategory.Emoji),
			Suggestions:   []string{"Keep local emoji", "Keep server emoji"},
		}
		conflicts = append(conflicts, conflict)
	}

	// Log de conflictos detectados
	if len(conflicts) > 0 {
		log.Printf("⚠️ Detected %d conflicts for category %s", len(conflicts), localCategory.LocalID)
	}

	return conflicts
}

// applyCategoryConflictResolution aplica la resolución de un conflicto específico
// Implementa las diferentes estrategias de resolución disponibles
// Actualiza la base de datos según la estrategia seleccionada
func applyCategoryConflictResolution(conflict CategoriesConflictResolution, resolution string) error {
	// Log de aplicación de resolución
	log.Printf("Applying conflict resolution '%s' for category %s", resolution, conflict.LocalID)

	// Aplicar resolución según la estrategia especificada
	switch resolution {
	case "server_wins":
		// No hacer nada, mantener datos del servidor
		log.Printf("Server wins: keeping server data for category %s", conflict.LocalID)
		return nil
		
	case "client_wins":
		// Aplicar datos del cliente
		if localData, ok := conflict.LocalData.(OfflineCategory); ok {
			return processCategoryUpdate(localData)
		}
		return fmt.Errorf("invalid local data type for category %s", conflict.LocalID)
		
	case "merge":
		// Implementar lógica de fusión específica para categorías
		log.Printf("Merge resolution not yet implemented for category %s", conflict.LocalID)
		return fmt.Errorf("merge resolution not implemented")
		
	default:
		return fmt.Errorf("unsupported resolution strategy: %s", resolution)
	}
}

// validateCategorySyncPermissions valida permisos para operaciones de sincronización
// Verifica que el usuario tenga autorización para realizar las operaciones solicitadas
// Incluye validación de límites de categorías y restricciones específicas
func validateCategorySyncPermissions(userID string, operations []OfflineCategory) error {
	// Validar que el usuario esté autenticado
	if userID == "" {
		return fmt.Errorf("user authentication required")
	}

	// Validar límites de operaciones por lote
	if len(operations) > 100 {
		return fmt.Errorf("maximum 100 operations per batch, received %d", len(operations))
	}

	// Contar operaciones por tipo para validar límites
	addCount := 0
	updateCount := 0
	deleteCount := 0

	for _, op := range operations {
		// Validar que la operación pertenezca al usuario
		if op.UserID != userID {
			return fmt.Errorf("operation for different user not allowed: %s", op.LocalID)
		}

		// Contar operaciones por tipo
		switch op.Action {
		case "add":
			addCount++
		case "update":
			updateCount++
		case "delete":
			deleteCount++
		default:
			return fmt.Errorf("invalid operation type: %s", op.Action)
		}
	}

	// Validar límites específicos (ejemplo: máximo 50 adiciones por lote)
	if addCount > 50 {
		return fmt.Errorf("maximum 50 add operations per batch, received %d", addCount)
	}

	return nil
}
