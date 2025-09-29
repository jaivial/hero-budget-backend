package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// Handlers HTTP para sincronización offline de Categories Management - Parte 1
// Implementan endpoints principales para sincronización bidireccional
// Incluyen procesamiento por lotes y resolución de conflictos para categorías
// Adaptados del patrón exitoso usado en cash_bank_management

// handleSyncCategoriesBatch procesa sincronización por lotes de operaciones offline de categorías
// Endpoint principal para sincronización de categorías modificadas offline
// Acepta operaciones de crear, actualizar y eliminar categorías en un solo lote
func handleSyncCategoriesBatch(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo POST permitido para operaciones de sincronización
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parsear la solicitud JSON del cliente
	var syncRequest SyncCategoriesBatchRequest
	err := json.NewDecoder(r.Body).Decode(&syncRequest)
	if err != nil {
		log.Printf("Error parsing sync batch request: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validar la estructura de la solicitud usando método incorporado
	if err := syncRequest.Validate(); err != nil {
		log.Printf("Validation failed for sync batch request: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Log de información sobre la sincronización para seguimiento
	log.Printf("Processing categories sync batch for user %s: %d categories",
		syncRequest.UserID, len(syncRequest.Categories))

	// Procesar la solicitud de sincronización usando función especializada
	response, err := processCategoriesSyncBatch(syncRequest)
	if err != nil {
		log.Printf("Error processing categories sync batch: %v", err)
		sendErrorResponse(w, "Error processing sync batch", http.StatusInternalServerError)
		return
	}

	// Invalidar caches relacionados si la sincronización fue exitosa
	// Esto asegura que las consultas futuras obtengan datos actualizados
	if cacheManager != nil && response.Success {
		err := cacheManager.InvalidateUserCache(syncRequest.UserID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate categories cache for user %s: %v",
				syncRequest.UserID, err)
		}
		log.Printf("✅ Categories cache invalidated after sync for user: %s", syncRequest.UserID)
	}

	// Enviar respuesta exitosa con los resultados de la sincronización
	sendSuccessResponse(w, "Categories sync batch processed successfully", response)
}

// handleSyncCategoriesChanges obtiene cambios del servidor desde último sync
// Permite al cliente descargar categorías modificadas en el servidor
// Útil para sincronización unidireccional y obtener actualizaciones remotas
func handleSyncCategoriesChanges(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo GET para consultas de cambios
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extraer parámetros de consulta requeridos
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}

	// Extraer parámetros opcionales para paginación y filtrado
	lastSync := r.URL.Query().Get("last_sync")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Configurar valores predeterminados para paginación
	limit := 100 // Límite predeterminado de registros por consulta
	offset := 0  // Inicio predeterminado para paginación

	// Parsear límite si se proporciona
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			// Aplicar límite máximo de seguridad para evitar consultas excesivas
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	// Parsear offset si se proporciona
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Log de la consulta para seguimiento
	log.Printf("Fetching categories changes for user %s, lastSync: %s, limit: %d, offset: %d",
		userID, lastSync, limit, offset)

	// Obtener cambios del servidor usando función especializada
	changesResponse, err := fetchCategoriesChanges(userID, lastSync, limit, offset)
	if err != nil {
		log.Printf("Error fetching categories changes: %v", err)
		sendErrorResponse(w, "Error fetching changes", http.StatusInternalServerError)
		return
	}

	// Log del resultado para seguimiento
	log.Printf("✅ Categories changes fetched for user %s: %d categories, %d deletions",
		userID, len(changesResponse.Categories), len(changesResponse.Deletions))

	// Enviar respuesta con los cambios encontrados
	sendSuccessResponse(w, "Categories changes fetched successfully", changesResponse)
}

// handleSyncCategoriesStats proporciona estadísticas de sincronización para categorías
// Retorna métricas útiles sobre el estado de sincronización del usuario
// Incluye información sobre rendimiento, errores y estadísticas específicas de categorías
func handleSyncCategoriesStats(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - solo GET para consultas de estadísticas
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extraer ID de usuario requerido
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		sendErrorResponse(w, "user_id parameter is required", http.StatusBadRequest)
		return
	}

	// Log de consulta de estadísticas
	log.Printf("Fetching categories sync stats for user %s", userID)

	// Obtener estadísticas usando función especializada
	stats, err := getCategoriesSyncStats(userID)
	if err != nil {
		log.Printf("Error fetching categories sync stats: %v", err)
		sendErrorResponse(w, "Error fetching sync stats", http.StatusInternalServerError)
		return
	}

	// Log del resultado
	log.Printf("✅ Categories sync stats fetched for user %s: %d total syncs, %d conflicts resolved",
		userID, stats.TotalSyncs, stats.ConflictsResolved)

	// Enviar estadísticas como respuesta
	sendSuccessResponse(w, "Categories sync stats fetched successfully", stats)
}

// handleSyncCategoriesConflictResolution maneja resolución manual de conflictos
// Permite al cliente especificar cómo resolver conflictos detectados durante sincronización
// Acepta diferentes estrategias de resolución: server_wins, client_wins, merge
func handleSyncCategoriesConflictResolution(w http.ResponseWriter, r *http.Request) {
	// Validar método HTTP - POST para envío de resolución de conflictos
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parsear solicitud de resolución de conflicto
	var conflictRequest SyncCategoriesConflictRequest
	err := json.NewDecoder(r.Body).Decode(&conflictRequest)
	if err != nil {
		log.Printf("Error parsing conflict resolution request: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validar campos requeridos para resolución de conflictos
	if conflictRequest.UserID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}

	if conflictRequest.LocalID == "" && conflictRequest.ServerID == "" {
		sendErrorResponse(w, "either local_id or server_id is required", http.StatusBadRequest)
		return
	}

	// Validar estrategia de resolución
	validResolutions := map[string]bool{
		"server_wins": true,
		"client_wins": true,
		"merge":       true,
	}
	if !validResolutions[conflictRequest.Resolution] {
		sendErrorResponse(w, "invalid resolution strategy", http.StatusBadRequest)
		return
	}

	// Log de resolución de conflicto
	log.Printf("Processing conflict resolution for user %s, resolution: %s",
		conflictRequest.UserID, conflictRequest.Resolution)

	// Procesar resolución usando función especializada
	result, err := resolveCategoriesConflict(conflictRequest)
	if err != nil {
		log.Printf("Error resolving categories conflict: %v", err)
		sendErrorResponse(w, "Error resolving conflict", http.StatusInternalServerError)
		return
	}

	// Invalidar cache después de resolver conflicto
	if cacheManager != nil {
		cacheManager.InvalidateUserCache(conflictRequest.UserID)
		log.Printf("✅ Cache invalidated after conflict resolution for user: %s", conflictRequest.UserID)
	}

	// Enviar resultado de la resolución
	sendSuccessResponse(w, "Conflict resolved successfully", result)
}
