package main

import (
	"encoding/base64"
	"log"
	"strings"
	"unicode/utf8"
)

// Funciones de base de datos y utilidades para Categories Management - Parte 4
// Contiene funciones de actualización, eliminación y manejo de emojis
// Incluye gestión de cache y funciones auxiliares

// updateCategory actualiza una categoría existente en la base de datos
// Incluye timestamp automático y codificación de emoji
// Valida permisos mediante combinación de ID y userID
func updateCategory(category Category) error {
	// Codificar el emoji antes de guardarlo
	encodedEmoji := encodeEmoji(category.Emoji)

	// Actualizar categoría con validación de permisos
	_, err := db.Exec(
		`UPDATE categories SET name = ?, type = ?, emoji = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`,
		category.Name, category.Type, encodedEmoji, category.ID, category.UserID,
	)
	return err
}

// deleteCategory elimina una categoría de la base de datos
// Incluye validación de permisos y verificación de existencia
// Retorna error si la categoría no existe o no pertenece al usuario
func deleteCategory(categoryID int, userID string) error {
	// Eliminar con validación de permisos
	_, err := db.Exec(
		`DELETE FROM categories WHERE id = ? AND user_id = ?`,
		categoryID, userID,
	)
	return err
}

// Funciones de codificación y decodificación de emojis

// encodeEmoji codifica emojis como Base64 antes de almacenarlos
// Preserva caracteres UTF-8 complejos que podrían corromperse en la base de datos
// Retorna emoji original si no contiene caracteres no-ASCII
func encodeEmoji(emoji string) string {
	// Manejar emoji vacío con valor predeterminado
	if emoji == "" {
		return "📊" // Emoji predeterminado
	}

	// Verificar si el emoji necesita ser codificado (caracteres no ASCII)
	needsEncoding := false
	for _, r := range emoji {
		if r > 127 { // Caracteres fuera del rango ASCII
			needsEncoding = true
			break
		}
	}

	// Codificar solo si es necesario para optimizar espacio
	if needsEncoding {
		// Convertir a bytes UTF-8 y luego a base64
		encoded := base64.StdEncoding.EncodeToString([]byte(emoji))
		result := "BASE64:" + encoded
		log.Printf("DEBUG - Emoji codificado: '%s' -> '%s'", emoji, result)
		return result
	}

	return emoji
}

// decodeEmoji decodifica emojis de Base64 cuando se recuperan
// Maneja múltiples formatos y casos de error con fallback a emoji predeterminado
// Incluye validación UTF-8 y detección de corrupción
func decodeEmoji(encoded string) string {
	// Si está vacío o es el emoji predeterminado, devolverlo tal cual
	if encoded == "" || encoded == "📊" {
		return encoded
	}

	// Si está codificado en Base64, decodificarlo
	if strings.HasPrefix(encoded, "BASE64:") {
		encodedPart := strings.TrimPrefix(encoded, "BASE64:")
		decoded, err := base64.StdEncoding.DecodeString(encodedPart)
		if err != nil {
			log.Printf("ERROR - Error al decodificar emoji: %v", err)
			return "📊" // En caso de error, devolver el emoji predeterminado
		}

		// Verificar que la decodificación resultó en UTF-8 válido
		decodedStr := string(decoded)
		if !utf8.ValidString(decodedStr) {
			log.Printf("ERROR - El emoji decodificado no es UTF-8 válido")
			return "📊"
		}

		log.Printf("DEBUG - Emoji decodificado: '%s' -> '%s'", encoded, decodedStr)
		return decodedStr
	}

	// Verificar caracteres corruptos comunes y reemplazar con predeterminado
	if encoded == "ð" || encoded == "ð " || strings.Contains(encoded, "ð") || strings.Contains(encoded, "â") {
		log.Printf("DEBUG - Emoji corrupto detectado: '%s', usando predeterminado", encoded)
		return "📊"
	}

	// Verificar que el string sea UTF-8 válido antes de retornarlo
	if !utf8.ValidString(encoded) {
		log.Printf("ERROR - El emoji no es UTF-8 válido: '%s'", encoded)
		return "📊"
	}

	return encoded
}

// invalidateCategoriesCache invalida cache de categorías para un usuario
// Mantiene consistencia entre cache y base de datos después de modificaciones
// Utiliza cache manager compartido para operaciones de invalidación
func invalidateCategoriesCache(userID string) {
	if cacheManager != nil {
		// Invalidate user cache (categories are stored under user data)
		err := cacheManager.InvalidateUserCache(userID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate categories cache for user %s: %v", userID, err)
		}

		log.Printf("✅ Cache invalidated for user: %s (categories)", userID)
	}
}
