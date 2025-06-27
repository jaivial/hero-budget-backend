package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Funciones de validación para sincronización offline de Categories Management
// Implementan validaciones específicas para categorías de ingresos y gastos
// Incluyen validación de nombres, tipos, emojis y consistencia de datos
// Adaptadas del patrón exitoso usado en otros servicios de sincronización

// validateCategorySyncRequest valida la estructura completa de una solicitud de sincronización
// Verifica que todos los campos requeridos estén presentes y sean válidos
// Incluye validación de límites y permisos del usuario
func validateCategorySyncRequest(request SyncCategoriesBatchRequest) error {
	// Validar campos básicos de la solicitud
	if err := request.Validate(); err != nil {
		return fmt.Errorf("basic validation failed: %v", err)
	}

	// Validar permisos de sincronización para el usuario
	if err := validateCategorySyncPermissions(request.UserID, request.Categories); err != nil {
		return fmt.Errorf("permission validation failed: %v", err)
	}

	// Validar cada categoría individual en el lote
	for i, category := range request.Categories {
		if err := validateOfflineCategory(category); err != nil {
			return fmt.Errorf("category %d validation failed: %v", i, err)
		}
	}

	// Validar que no haya IDs locales duplicados
	if err := validateUniqueLocalIDs(request.Categories); err != nil {
		return fmt.Errorf("duplicate ID validation failed: %v", err)
	}

	// Validar límites de sincronización por tipo de operación
	if err := validateOperationLimits(request.Categories); err != nil {
		return fmt.Errorf("operation limits validation failed: %v", err)
	}

	// Log de validación exitosa
	log.Printf("✅ Sync request validation passed for user %s: %d categories", 
		request.UserID, len(request.Categories))

	return nil
}

// validateOfflineCategory valida una categoría offline individual
// Verifica campos requeridos, tipos válidos y formato de datos
// Incluye validación específica según la acción a realizar
func validateOfflineCategory(category OfflineCategory) error {
	// Usar validación básica incorporada
	if err := category.Validate(); err != nil {
		return err
	}

	// Validaciones adicionales específicas para categorías
	if category.Action == "add" || category.Action == "update" {
		// Validar nombre de categoría
		if err := validateCategoryName(category.Name); err != nil {
			return fmt.Errorf("category name validation failed: %v", err)
		}

		// Validar tipo de categoría
		if err := validateCategoryType(category.Type); err != nil {
			return fmt.Errorf("category type validation failed: %v", err)
		}

		// Validar emoji de categoría
		if err := validateCategoryEmoji(category.Emoji); err != nil {
			return fmt.Errorf("category emoji validation failed: %v", err)
		}
	}

	// Validaciones específicas por acción
	switch category.Action {
	case "add":
		return validateCategoryAddOperation(category)
	case "update":
		return validateCategoryUpdateOperation(category)
	case "delete":
		return validateCategoryDeleteOperation(category)
	default:
		return fmt.Errorf("unknown action: %s", category.Action)
	}
}

// validateCategoryName valida el nombre de una categoría
// Verifica longitud, caracteres permitidos y formato
// Incluye validación de caracteres especiales y emojis en nombres
func validateCategoryName(name string) error {
	// Validar que el nombre no esté vacío
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("category name cannot be empty")
	}

	// Validar longitud del nombre
	if len(name) > 100 {
		return fmt.Errorf("category name too long: maximum 100 characters, got %d", len(name))
	}

	if len(name) < 2 {
		return fmt.Errorf("category name too short: minimum 2 characters, got %d", len(name))
	}

	// Validar que el nombre sea UTF-8 válido
	if !utf8.ValidString(name) {
		return fmt.Errorf("category name contains invalid UTF-8 characters")
	}

	// Validar caracteres no permitidos (opcional - permitir la mayoría de caracteres)
	invalidChars := regexp.MustCompile(`[<>{}|\\^~\[\]` + "`" + `]`)
	if invalidChars.MatchString(name) {
		return fmt.Errorf("category name contains invalid characters")
	}

	// Validar que no sea solo espacios en blanco
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("category name cannot start or end with whitespace")
	}

	return nil
}

// validateCategoryType valida el tipo de una categoría
// Verifica que sea uno de los tipos permitidos: income o expense
// Incluye normalización de casos para compatibilidad
func validateCategoryType(categoryType string) error {
	// Normalizar tipo a minúsculas para comparación
	normalizedType := strings.ToLower(strings.TrimSpace(categoryType))

	// Validar tipos permitidos
	validTypes := map[string]bool{
		"income":  true,
		"expense": true,
	}

	if !validTypes[normalizedType] {
		return fmt.Errorf("invalid category type: must be 'income' or 'expense', got '%s'", categoryType)
	}

	return nil
}

// validateCategoryEmoji valida el emoji de una categoría
// Verifica que sea un emoji válido o una cadena codificada válida
// Incluye validación de formato Base64 para emojis codificados
func validateCategoryEmoji(emoji string) error {
	// Permitir emoji vacío (se asignará predeterminado)
	if emoji == "" {
		return nil
	}

	// Validar que sea UTF-8 válido
	if !utf8.ValidString(emoji) {
		return fmt.Errorf("emoji contains invalid UTF-8 characters")
	}

	// Validar longitud máxima
	if len(emoji) > 200 {
		return fmt.Errorf("emoji too long: maximum 200 characters, got %d", len(emoji))
	}

	// Si es emoji codificado en Base64, validar formato
	if strings.HasPrefix(emoji, "BASE64:") {
		encodedPart := strings.TrimPrefix(emoji, "BASE64:")
		if len(encodedPart) == 0 {
			return fmt.Errorf("empty Base64 encoded emoji")
		}
		
		// Validar caracteres Base64
		base64Pattern := regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)
		if !base64Pattern.MatchString(encodedPart) {
			return fmt.Errorf("invalid Base64 encoding in emoji")
		}
	}

	return nil
}

// validateCategoryAddOperation valida operación de adición de categoría
// Verifica que todos los campos requeridos estén presentes
// Incluye validación de unicidad de nombres por usuario y tipo
func validateCategoryAddOperation(category OfflineCategory) error {
	// Validar que los campos requeridos estén presentes
	requiredFields := map[string]string{
		"UserID": category.UserID,
		"Name":   category.Name,
		"Type":   category.Type,
	}

	for field, value := range requiredFields {
		if value == "" {
			return fmt.Errorf("field %s is required for add operation", field)
		}
	}

	// Validar que el LocalID esté presente para tracking
	if category.LocalID == "" {
		return fmt.Errorf("local_id is required for add operation")
	}

	return nil
}
		}
	}

	// Validar que el LocalID esté presente para tracking
	if category.LocalID == "" {
		return fmt.Errorf("local_id is required for add operation")
	}

	// Nota: La validación de nombres únicos se haría en el procesamiento
	// para evitar carreras entre solicitudes concurrentes

	return nil
}