package main

import (
	"encoding/json"
	"log"
	"net/http"
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

	// Delete category from database con validación de existencia
	err = deleteCategory(deleteRequest.CategoryID, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error deleting category: %v", err)
		sendErrorResponse(w, "Error deleting category", http.StatusInternalServerError)
		return
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