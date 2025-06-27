package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Handlers HTTP para gestión de categorías - Parte 2
// Contiene los handlers principales para operaciones CRUD de categorías
// Incluye manejo de cache, validación de datos y respuestas estandarizadas

// handleFetchCategories maneja la obtención de categorías del usuario
// Endpoint GET que retorna categorías filtradas por usuario y opcionalmente por tipo
// Implementa cache para optimizar rendimiento en consultas frecuentes
func handleFetchCategories(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo GET permitido para consultas
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from query parameter - requerido para filtrar categorías del usuario
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Get optional type filter para filtrar por tipo de categoría
	categoryType := r.URL.Query().Get("type") // "income", "expense", or empty for all

	// Try cache first for categories data para optimizar consultas repetidas
	if cacheManager != nil {
		var cachedCategories []Category
		cacheKey := "categories_" + categoryType
		if categoryType == "" {
			cacheKey = "categories_all"
		}
		err := cacheManager.GetUserData(userID, &cachedCategories)
		if err == nil {
			log.Printf("✅ Cache HIT: categories for user %s, type %s", userID, cacheKey)
			sendSuccessResponse(w, "Categories fetched successfully from cache", cachedCategories)
			return
		}
		log.Printf("🔍 Cache MISS: categories for user %s, type %s", userID, cacheKey)
	}

	// Get categories from database cuando no están en cache
	categories, err := fetchCategories(userID, categoryType)
	if err != nil {
		log.Printf("Error fetching categories: %v", err)
		sendErrorResponse(w, "Error fetching categories", http.StatusInternalServerError)
		return
	}

	// Cache the result for future requests para optimizar consultas futuras
	if cacheManager != nil {
		err = cacheManager.CacheUserData(userID, categories)
		if err != nil {
			log.Printf("Warning: Failed to cache categories for user %s: %v", userID, err)
		}
	}

	// Return categories as JSON con respuesta estandarizada
	sendSuccessResponse(w, "Categories fetched successfully", categories)
}

// handleAddCategory maneja la creación de nuevas categorías
// Endpoint POST que valida datos, crea categoría y retorna información completa
// Incluye asignación de emojis predeterminados y validación de tipos
func handleAddCategory(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo POST para creación de recursos
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body con validación de formato JSON
	var addRequest AddCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&addRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request con validación de campos requeridos
	if addRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if addRequest.Name == "" {
		sendErrorResponse(w, "Category name is required", http.StatusBadRequest)
		return
	}

	if addRequest.Type != "income" && addRequest.Type != "expense" {
		sendErrorResponse(w, "Category type must be 'income' or 'expense'", http.StatusBadRequest)
		return
	}

	// Set default emoji if not provided según el tipo de categoría
	if addRequest.Emoji == "" {
		if addRequest.Type == "income" {
			addRequest.Emoji = "💰"
		} else {
			addRequest.Emoji = "🛒"
		}
	}

	// Create category object con estructura completa
	category := Category{
		UserID: addRequest.UserID,
		Name:   addRequest.Name,
		Type:   addRequest.Type,
		Emoji:  addRequest.Emoji,
	}

	// Add category to database con manejo de errores
	categoryID, err := addCategory(category)
	if err != nil {
		log.Printf("Error adding category: %v", err)
		sendErrorResponse(w, "Error adding category", http.StatusInternalServerError)
		return
	}

	// Recuperar la categoría recién creada para asegurar integridad de datos
	createdCategory, err := fetchCategoryByID(categoryID, addRequest.UserID)
	if err != nil {
		log.Printf("Error fetching created category: %v", err)
		sendErrorResponse(w, "Error fetching created category", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG - Emoji después de creación: %s", createdCategory.Emoji)

	// Invalidate cache after adding category para mantener consistencia
	invalidateCategoriesCache(addRequest.UserID)

	// Return success response with the created category
	sendSuccessResponse(w, "Category added successfully", createdCategory)
}

// handleUpdateCategory maneja la actualización de categorías existentes
// Endpoint POST que permite actualización parcial de campos
// Incluye validación de permisos y verificación de existencia
func handleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - POST para modificación de recursos
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body con validación de estructura
	var updateRequest UpdateCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request con verificación de campos requeridos
	if updateRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if updateRequest.CategoryID <= 0 {
		sendErrorResponse(w, "Valid category ID is required", http.StatusBadRequest)
		return
	}

	// Validar tipo de categoría si se proporciona
	if updateRequest.Type != "" && updateRequest.Type != "income" && updateRequest.Type != "expense" {
		sendErrorResponse(w, "Category type must be 'income' or 'expense'", http.StatusBadRequest)
		return
	}

	// Fetch existing category para verificar existencia y permisos
	existingCategory, err := fetchCategoryByID(updateRequest.CategoryID, updateRequest.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(w, "Category not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching category: %v", err)
			sendErrorResponse(w, "Error fetching category", http.StatusInternalServerError)
		}
		return
	}

	// Update fields if provided - actualización parcial preservando valores existentes
	if updateRequest.Name != "" {
		existingCategory.Name = updateRequest.Name
	}
	if updateRequest.Type != "" {
		existingCategory.Type = updateRequest.Type
	}
	if updateRequest.Emoji != "" {
		existingCategory.Emoji = updateRequest.Emoji
	}

	// Update category in database con validación de integridad
	err = updateCategory(*existingCategory)
	if err != nil {
		log.Printf("Error updating category: %v", err)
		sendErrorResponse(w, "Error updating category", http.StatusInternalServerError)
		return
	}

	// Recuperar la categoría actualizada para obtener información correcta
	updatedCategory, err := fetchCategoryByID(updateRequest.CategoryID, updateRequest.UserID)
	if err != nil {
		log.Printf("Error fetching updated category: %v", err)
		sendErrorResponse(w, "Error fetching updated category", http.StatusInternalServerError)
		return
	}

	log.Printf("DEBUG - Emoji después de actualización: %s", updatedCategory.Emoji)

	// Invalidate cache after updating category para mantener consistencia
	invalidateCategoriesCache(updateRequest.UserID)

	// Return success response with the updated category
	sendSuccessResponse(w, "Category updated successfully", updatedCategory)
}