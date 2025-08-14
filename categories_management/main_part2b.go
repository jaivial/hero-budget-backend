package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// Handlers HTTP para gestión de categorías - Parte 2B
// Contiene handlers adicionales y funciones de respuesta estandarizadas
// Incluye manejo de eliminación y funciones auxiliares de respuesta

// handleDeleteCategory maneja la eliminación de categorías
// Endpoint POST que valida permisos y elimina categoría del sistema
// Incluye verificación de dependencias y limpieza de cache
func handleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - POST para operaciones de eliminación
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body con validación de formato
	var deleteRequest DeleteCategoryRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request con verificación de permisos
	if deleteRequest.UserID == "" {
		sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	if deleteRequest.CategoryID <= 0 {
		sendErrorResponse(w, "Valid category ID is required", http.StatusBadRequest)
		return
	}

	// Fetch category data before deletion for sync operations
	categoryToDelete, err := fetchCategoryByID(deleteRequest.CategoryID, deleteRequest.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(w, "Category not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching category before deletion: %v", err)
			sendErrorResponse(w, "Error fetching category", http.StatusInternalServerError)
		}
		return
	}

	// Delete category from database con validación de existencia
	err = deleteCategory(deleteRequest.CategoryID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting category: %v", err)
		sendErrorResponse(w, "Error deleting category", http.StatusInternalServerError)
		return
	}

	// Record sync operation if sync parameters are provided
	if deleteRequest.OperationID != "" && deleteRequest.DeviceID != "" && deleteRequest.Timestamp > 0 {
		log.Printf("Recording sync operation for category deletion: operation_id=%s, device_id=%s, timestamp=%d", 
			deleteRequest.OperationID, deleteRequest.DeviceID, deleteRequest.Timestamp)
		
		// Create sync operation data with deleted category structure
		syncData := map[string]interface{}{
			"id":         categoryToDelete.ID,
			"user_id":    categoryToDelete.UserID,
			"name":       categoryToDelete.Name,
			"type":       categoryToDelete.Type,
			"emoji":      categoryToDelete.Emoji,
			"created_at": categoryToDelete.CreatedAt,
			"updated_at": categoryToDelete.UpdatedAt,
			"deleted_at": "now", // Placeholder since category is already deleted
		}
		
		// Add sync operation record to database
		err = addSyncOperation(
			deleteRequest.UserID,
			deleteRequest.OperationID,
			"delete",
			"categories",
			strconv.Itoa(deleteRequest.CategoryID),
			syncData,
			deleteRequest.DeviceID,
			deleteRequest.Timestamp,
		)
		
		if err != nil {
			log.Printf("Warning: Failed to record sync operation: %v", err)
			// Don't fail the category deletion for sync errors, just log warning
		} else {
			log.Printf("Successfully recorded sync operation for category deletion ID: %d", deleteRequest.CategoryID)
		}
	} else {
		log.Printf("Sync parameters not provided or incomplete, skipping sync operation recording")
	}

	// Invalidate cache after deleting category para mantener consistencia
	invalidateCategoriesCache(deleteRequest.UserID)

	// Return success response con confirmación de eliminación
	sendSuccessResponse(w, "Category deleted successfully", nil)
}

// sendSuccessResponse envía respuesta de éxito estandarizada
// Utiliza estructura ApiResponse para consistencia en todas las respuestas
// Incluye mensaje descriptivo y datos opcionales
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	// Configurar header de contenido JSON
	w.Header().Set("Content-Type", "application/json")
	
	// Codificar y enviar respuesta con formato estándar
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// sendErrorResponse envía respuesta de error estandarizada
// Configura código de estado HTTP apropiado y mensaje descriptivo
// Utiliza estructura ApiResponse para consistencia en manejo de errores
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	// Configurar headers para respuesta de error
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Codificar y enviar respuesta de error con formato estándar
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}