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
