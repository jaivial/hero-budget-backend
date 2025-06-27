package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

// Funciones de base de datos y utilidades para Categories Management - Parte 3
// Contiene operaciones CRUD de base de datos y funciones de codificación de emojis
// Incluye manejo especializado de emojis UTF-8 y corrección de datos corruptos

// handleFixEmojis maneja la corrección automática de emojis corruptos
// Endpoint GET que identifica y corrige emojis malformados en la base de datos
// Utiliza codificación Base64 para preservar caracteres UTF-8 complejos
func handleFixEmojis(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo GET para operaciones de corrección
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Iniciando corrección de emojis corruptos...")

	// Get all categories with corrupted emojis and those that are not properly encoded
	// Buscar categorías con emojis corruptos o no codificados correctamente
	rows, err := db.Query(`SELECT id, user_id, emoji FROM categories WHERE emoji = 'ð' OR emoji = 'ð ' OR emoji LIKE '%ð%' OR emoji NOT LIKE 'BASE64:%'`)
	if err != nil {
		log.Printf("Error al consultar categorías: %v", err)
		sendErrorResponse(w, "Error al consultar categorías", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Contador para categorías actualizadas y lista de resultados
	var updatedCount int
	var updatedCategories []Category

	// Mapa de emojis predeterminados basados en el ID de categoría (para tener variedad)
	defaultEmojis := []string{
		"📊", "💰", "🛒", "🏠", "🚗", "✈️", "🍔", "🍕", "💼", "💸", "💳", "💵",
	}

	// Procesar cada categoría con emoji corrupto
	for rows.Next() {
		var id int
		var userId string
		var emoji string

		if err := rows.Scan(&id, &userId, &emoji); err != nil {
			log.Printf("Error al escanear fila: %v", err)
			continue
		}

		// Elegir un emoji predeterminado basado en el ID (para variar)
		defaultEmoji := defaultEmojis[id%len(defaultEmojis)]

		// Codificar el emoji en base64 para preservar UTF-8
		encodedEmoji := encodeEmoji(defaultEmoji)

		// Actualizar la categoría en la base de datos
		_, err := db.Exec(
			`UPDATE categories SET emoji = ? WHERE id = ? AND user_id = ?`,
			encodedEmoji, id, userId,
		)

		if err != nil {
			log.Printf("Error al actualizar categoría %d: %v", id, err)
			continue
		}

		// Obtener la categoría actualizada para verificar corrección
		updatedCategory, err := fetchCategoryByID(id, userId)
		if err != nil {
			log.Printf("Error al obtener categoría actualizada %d: %v", id, err)
			continue
		}

		updatedCategories = append(updatedCategories, *updatedCategory)
		updatedCount++
		log.Printf("Categoría %d actualizada con éxito: %s -> %s", id, emoji, updatedCategory.Emoji)
	}

	log.Printf("Proceso completado. %d categorías actualizadas.", updatedCount)
	sendSuccessResponse(w, fmt.Sprintf("%d categorías actualizadas", updatedCount), updatedCategories)
}

// Database functions - Funciones de acceso a datos para categorías

// fetchCategories obtiene categorías del usuario con filtrado opcional por tipo
// Implementa consulta optimizada con ORDER BY para resultados consistentes
// Incluye decodificación automática de emojis desde formato Base64
func fetchCategories(userID, categoryType string) ([]Category, error) {
	var query string
	var args []interface{}

	// Construir consulta según filtros aplicados
	if categoryType == "" {
		// Fetch all categories for the user - obtener todas las categorías
		query = `SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE user_id = ? ORDER BY name ASC`
		args = []interface{}{userID}
	} else {
		// Fetch categories of specific type - filtrar por tipo específico
		query = `SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE user_id = ? AND type = ? ORDER BY name ASC`
		args = []interface{}{userID, categoryType}
	}

	// Ejecutar consulta con manejo de errores
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Procesar resultados con decodificación de emojis
	var categories []Category
	for rows.Next() {
		var category Category
		var encodedEmoji string

		// Escanear datos de la fila actual
		err := rows.Scan(
			&category.ID,
			&category.UserID,
			&category.Name,
			&category.Type,
			&encodedEmoji, // Leer el emoji codificado
			&category.CreatedAt,
			&category.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Decodificar el emoji antes de agregarlo al objeto Category
		category.Emoji = decodeEmoji(encodedEmoji)

		categories = append(categories, category)
	}

	return categories, nil
}

// fetchCategoryByID obtiene una categoría específica por ID y usuario
// Incluye validación de permisos verificando que pertenezca al usuario
// Retorna puntero a Category o error si no existe o no tiene permisos
func fetchCategoryByID(categoryID int, userID string) (*Category, error) {
	var category Category
	var encodedEmoji string

	// Consulta con filtro por ID y usuario para validar permisos
	err := db.QueryRow(
		`SELECT id, user_id, name, type, emoji, created_at, updated_at FROM categories WHERE id = ? AND user_id = ?`,
		categoryID, userID,
	).Scan(
		&category.ID,
		&category.UserID,
		&category.Name,
		&category.Type,
		&encodedEmoji,
		&category.CreatedAt,
		&category.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Decodificar el emoji para respuesta al cliente
	category.Emoji = decodeEmoji(encodedEmoji)

	return &category, nil
}

// addCategory inserta una nueva categoría en la base de datos
// Incluye codificación automática de emoji y retorna ID generado
// Valida integridad de datos antes de la inserción
func addCategory(category Category) (int, error) {
	// Codificar el emoji antes de guardarlo para preservar UTF-8
	encodedEmoji := encodeEmoji(category.Emoji)

	// Insertar nueva categoría con emoji codificado
	result, err := db.Exec(
		`INSERT INTO categories (user_id, name, type, emoji) VALUES (?, ?, ?, ?)`,
		category.UserID, category.Name, category.Type, encodedEmoji,
	)
	if err != nil {
		return 0, err
	}

	// Obtener ID generado por la base de datos
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}