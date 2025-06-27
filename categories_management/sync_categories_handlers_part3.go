package main

import (
	"fmt"
	"log"
	"time"
)

// Handlers HTTP para sincronización offline de Categories Management - Parte 3 (Helper Functions)
// Contiene funciones auxiliares para sincronización de categorías
// Incluye obtención de cambios, estadísticas y resolución de conflictos


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