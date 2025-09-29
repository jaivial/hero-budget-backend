package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

	// Record sync operation with auto-generated operation_id (consistent pattern from implementation guide)
	log.Printf("Recording sync operation for category creation with auto-generated operation_id")

	// Create sync operation data with created category structure
	syncData := map[string]interface{}{
		"id":         createdCategory.ID,
		"user_id":    createdCategory.UserID,
		"name":       createdCategory.Name,
		"type":       createdCategory.Type,
		"emoji":      createdCategory.Emoji,
		"created_at": createdCategory.CreatedAt,
		"updated_at": createdCategory.UpdatedAt,
	}

	// Always add sync operation record to database - auto-generate operation_id if not provided
	err = addSyncOperation(
		addRequest.UserID,
		addRequest.OperationID, // Will auto-generate if empty or invalid
		"create",
		"categories",
		strconv.Itoa(createdCategory.ID),
		syncData,
		addRequest.DeviceID,  // Use device_id from request (can be empty)
		addRequest.Timestamp, // Use timestamp from request (can be 0)
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for category creation: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for category creation: ID %d", createdCategory.ID)
	}

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

	// Store original category name for transaction update comparison
	originalCategoryName := existingCategory.Name

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

	// Update transaction category names if category name changed
	if updateRequest.Name != "" && originalCategoryName != updatedCategory.Name {
		log.Printf("🔄 Category name changed from '%s' to '%s', updating transaction category names...",
			originalCategoryName, updatedCategory.Name)

		updateResult := updateTransactionsCategoryName(
			updatedCategory.ID,
			originalCategoryName,
			updatedCategory.Name,
			updatedCategory.Type,
			updatedCategory.UserID,
		)

		if updateResult.Success {
			log.Printf("✅ Successfully updated %d transaction(s) with new category name",
				updateResult.UpdatedCount)
		} else {
			log.Printf("⚠️ Failed to update transaction category names: %s", updateResult.Error)
			// Note: We don't fail the category update if transaction update fails
			// This allows the category update to succeed even if transaction update has issues
		}
	} else {
		log.Printf("💡 Category name unchanged ('%s'), skipping transaction updates", updatedCategory.Name)
	}

	// Record sync operation with auto-generated operation_id (consistent pattern from implementation guide)
	log.Printf("Recording sync operation for category update with auto-generated operation_id")

	// Create sync operation data with updated category structure
	syncData := map[string]interface{}{
		"id":         updatedCategory.ID,
		"user_id":    updatedCategory.UserID,
		"name":       updatedCategory.Name,
		"type":       updatedCategory.Type,
		"emoji":      updatedCategory.Emoji,
		"created_at": updatedCategory.CreatedAt,
		"updated_at": updatedCategory.UpdatedAt,
	}

	// Always add sync operation record to database - auto-generate operation_id if not provided
	err = addSyncOperation(
		updateRequest.UserID,
		updateRequest.OperationID, // Will auto-generate if empty or invalid
		"update",
		"categories",
		strconv.Itoa(updateRequest.CategoryID),
		syncData,
		updateRequest.DeviceID,  // Use device_id from request (can be empty)
		updateRequest.Timestamp, // Use timestamp from request (can be 0)
	)

	if err != nil {
		log.Printf("❌ ERROR: Failed to record sync operation for category update: %v", err)
		// Don't fail the main operation for sync errors, just log warning
	} else {
		log.Printf("✅ SUCCESS: Successfully recorded sync operation for category update: ID %d", updateRequest.CategoryID)
	}

	// Invalidate cache after updating category para mantener consistencia
	invalidateCategoriesCache(updateRequest.UserID)

	// Return success response with the updated category
	sendSuccessResponse(w, "Category updated successfully", updatedCategory)
}

// TransactionUpdateResult represents the result of updating transaction category names
type TransactionUpdateResult struct {
	Success      bool   `json:"success"`
	UpdatedCount int    `json:"updated_count"`
	Error        string `json:"error,omitempty"`
}

// updateTransactionsCategoryName updates transaction category names when a category name changes
// Updates expenses, incomes, and bills tables where category_id matches the updated category
// Implements triple-entity update strategy: expenses, incomes, AND bills
// Only updates if the category name actually changed to avoid unnecessary operations
//
// Parameters:
//   - categoryId: ID of the category that was updated
//   - oldCategoryName: Previous category name
//   - newCategoryName: New category name
//   - categoryType: Type of category ('income' or 'expense')
//   - userId: User ID for permission validation
//
// Returns: TransactionUpdateResult with update status and total count (transactions + bills)
func updateTransactionsCategoryName(categoryId int, oldCategoryName, newCategoryName, categoryType, userId string) TransactionUpdateResult {
	log.Printf("🔄 Starting transaction category name update for category ID %d...", categoryId)
	log.Printf("📝 Update details: User=%s, Type=%s, Old='%s', New='%s'",
		userId, categoryType, oldCategoryName, newCategoryName)

	// Skip update if names are the same
	if oldCategoryName == newCategoryName {
		log.Printf("💡 Category names are identical, skipping transaction updates")
		return TransactionUpdateResult{
			Success:      true,
			UpdatedCount: 0,
		}
	}

	// Get database connection - use production database path
	dbPath := "../database/hero_budget.db"

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("❌ Failed to open database: %v", err)
		return TransactionUpdateResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to open database: %v", err),
		}
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Printf("❌ Failed to ping database: %v", err)
		return TransactionUpdateResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to ping database: %v", err),
		}
	}

	// Determine which table to update based on category type
	tableName := "expenses"
	if categoryType == "income" {
		tableName = "incomes"
	}

	log.Printf("📝 Updating %s transactions with category_id: %d", tableName, categoryId)

	totalUpdated := 0

	// Update transactions where category_id matches (primary method)
	updateSql := fmt.Sprintf(`
		UPDATE %s
		SET category = ?, updated_at = datetime('now', 'localtime')
		WHERE user_id = ? AND category_id = ?
	`, tableName)

	result, err := db.Exec(updateSql, newCategoryName, userId, categoryId)
	if err != nil {
		log.Printf("❌ Error updating %s transactions: %v", tableName, err)
		return TransactionUpdateResult{
			Success: false,
			Error:   fmt.Sprintf("Error updating %s transactions: %v", tableName, err),
		}
	}

	primaryUpdated, _ := result.RowsAffected()
	totalUpdated += int(primaryUpdated)

	log.Printf("✅ Primary update: %d %s transactions updated via category_id", primaryUpdated, tableName)

	// Also update any transactions that still use the old category name (fallback method)
	fallbackUpdateSql := fmt.Sprintf(`
		UPDATE %s
		SET category = ?, updated_at = datetime('now', 'localtime')
		WHERE user_id = ? AND category = ? AND (category_id IS NULL OR category_id != ?)
	`, tableName)

	fallbackResult, err := db.Exec(fallbackUpdateSql, newCategoryName, userId, oldCategoryName, categoryId)
	if err != nil {
		log.Printf("⚠️ Error in fallback %s transaction update: %v", tableName, err)
		// Don't fail the entire operation if fallback fails
	} else {
		fallbackUpdated, _ := fallbackResult.RowsAffected()
		totalUpdated += int(fallbackUpdated)
		log.Printf("✅ Fallback update: %d additional %s transactions updated via category name", fallbackUpdated, tableName)
	}

	log.Printf("🎉 Total transactions updated: %d in %s table", totalUpdated, tableName)
	log.Printf("📊 Transaction update summary: CategoryID=%d, Type=%s, Updated=%d",
		categoryId, categoryType, totalUpdated)

	// Update bills table if category type is expense (bills are always expenses)
	if categoryType == "expense" {
		log.Printf("📝 Updating bills with category_id: %d", categoryId)

		// Update bills where category_id matches
		billsUpdateSql := `
			UPDATE bills
			SET category = ?, updated_at = datetime('now', 'localtime')
			WHERE user_id = ? AND category_id = ?
		`

		billsResult, err := db.Exec(billsUpdateSql, newCategoryName, userId, categoryId)
		if err != nil {
			log.Printf("⚠️ Error updating bills: %v", err)
			// Don't fail the entire operation if bills update fails
		} else {
			billsUpdated, _ := billsResult.RowsAffected()
			totalUpdated += int(billsUpdated)
			log.Printf("✅ Primary update: %d bills updated via category_id", billsUpdated)
		}

		// Fallback update for bills with old category name
		billsFallbackUpdateSql := `
			UPDATE bills
			SET category = ?, updated_at = datetime('now', 'localtime')
			WHERE user_id = ? AND category = ? AND (category_id IS NULL OR category_id != ?)
		`

		billsFallbackResult, err := db.Exec(billsFallbackUpdateSql, newCategoryName, userId, oldCategoryName, categoryId)
		if err != nil {
			log.Printf("⚠️ Error in fallback bills update: %v", err)
			// Don't fail the entire operation if fallback fails
		} else {
			billsFallbackUpdated, _ := billsFallbackResult.RowsAffected()
			totalUpdated += int(billsFallbackUpdated)
			log.Printf("✅ Fallback update: %d additional bills updated via category name", billsFallbackUpdated)
		}
	}

	log.Printf("🎉 Total entities updated: %d (transactions + bills)", totalUpdated)

	return TransactionUpdateResult{
		Success:      true,
		UpdatedCount: totalUpdated,
	}
}
