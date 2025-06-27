package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
	
	"github.com/herobudget/backend/common"
)

// Handlers HTTP para funcionalidad de sincronización offline
// Gestiona la sincronización bidireccional entre cliente y servidor

// handleSyncBatch procesa una solicitud de sincronización por lotes
// Endpoint: POST /sync/batch
// Procesa múltiples operaciones offline en una sola transacción
func handleSyncBatch(w http.ResponseWriter, r *http.Request) {
	// Verificar método HTTP
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear solicitud JSON
	var request SyncBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decodificando solicitud de sync: %v", err)
		sendErrorResponse(w, "Formato JSON inválido", http.StatusBadRequest)
		return
	}

	// Validar solicitud
	if err := request.Validate(); err != nil {
		log.Printf("Solicitud de sync inválida: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Solicitud inválida: %v", err), http.StatusBadRequest)
		return
	}

	// Registrar solicitud de sincronización
	log.Printf("Procesando sync batch para usuario %s: %d operaciones", request.UserID, len(request.Expenses))

	// Procesar sincronización en base de datos
	response, err := processSyncBatch(&request)
	if err != nil {
		log.Printf("Error procesando sync batch: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error de sincronización: %v", err), http.StatusInternalServerError)
		return
	}

	// Invalidar caché de gastos del usuario
	invalidateExpenseCache(request.UserID)

	// Registrar estadísticas de sincronización
	updateSyncStats(request.UserID, len(request.Expenses), response.SuccessfulOps, response.FailedOps)

	// Enviar respuesta exitosa
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de sync: %v", err)
	}

	log.Printf("Sync batch completado para usuario %s: %d exitosos, %d fallidos, %d conflictos", 
		request.UserID, response.SuccessfulOps, response.FailedOps, len(response.Conflicts))
}

// handleSyncChanges obtiene cambios del servidor desde último sync
// Endpoint: GET /sync/changes?user_id=X&last_sync=Y
// Permite al cliente obtener actualizaciones sin enviar datos
func handleSyncChanges(w http.ResponseWriter, r *http.Request) {
	// Verificar método HTTP
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Obtener parámetros de la URL
	userID := r.URL.Query().Get("user_id")
	lastSync := r.URL.Query().Get("last_sync")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Validar parámetros requeridos
	if userID == "" {
		sendErrorResponse(w, "user_id es requerido", http.StatusBadRequest)
		return
	}

	// Parsear límite y offset (valores por defecto)
	limit := 100 // Default
	offset := 0  // Default
	
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}
	
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Crear solicitud de cambios
	request := SyncChangesRequest{
		UserID:   userID,
		LastSync: lastSync,
		Limit:    limit,
		Offset:   offset,
	}

	log.Printf("Obteniendo cambios para usuario %s desde %s (limit: %d, offset: %d)", 
		userID, lastSync, limit, offset)

	// Obtener cambios de la base de datos
	response, err := getChangesAfterTimestamp(&request)
	if err != nil {
		log.Printf("Error obteniendo cambios: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error obteniendo cambios: %v", err), http.StatusInternalServerError)
		return
	}

	// Actualizar timestamp de última consulta
	updateLastSyncQuery(userID)

	// Enviar respuesta
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de cambios: %v", err)
	}

	log.Printf("Cambios enviados para usuario %s: %d cambios, %d eliminaciones", 
		userID, len(response.Changes), len(response.Deletions))
}

// handleResolveConflicts resuelve conflictos de sincronización específicos
// Endpoint: POST /sync/resolve-conflicts
// Permite resolución manual de conflictos detectados
func handleResolveConflicts(w http.ResponseWriter, r *http.Request) {
	// Verificar método HTTP
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Parsear solicitud JSON
	var request SyncConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("Error decodificando solicitud de resolución de conflictos: %v", err)
		sendErrorResponse(w, "Formato JSON inválido", http.StatusBadRequest)
		return
	}

	// Validar solicitud
	if request.UserID == "" {
		sendErrorResponse(w, "user_id es requerido", http.StatusBadRequest)
		return
	}
	if request.LocalID == "" {
		sendErrorResponse(w, "local_id es requerido", http.StatusBadRequest)
		return
	}
	if request.Resolution == "" {
		sendErrorResponse(w, "resolution es requerido", http.StatusBadRequest)
		return
	}

	// Validar tipo de resolución
	validResolutions := map[string]bool{
		"server_wins": true,
		"client_wins": true,
		"merge":       true,
	}
	if !validResolutions[request.Resolution] {
		sendErrorResponse(w, "resolution debe ser server_wins, client_wins o merge", http.StatusBadRequest)
		return
	}

	log.Printf("Resolviendo conflicto para usuario %s, local_id %s, resolución: %s", 
		request.UserID, request.LocalID, request.Resolution)

	// Procesar resolución del conflicto
	result, err := resolveConflict(&request)
	if err != nil {
		log.Printf("Error resolviendo conflicto: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Error resolviendo conflicto: %v", err), http.StatusInternalServerError)
		return
	}

	// Invalidar caché después de resolver conflicto
	invalidateExpenseCache(request.UserID)

	// Enviar respuesta exitosa
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("Error enviando respuesta de resolución: %v", err)
	}

	log.Printf("Conflicto resuelto exitosamente para usuario %s", request.UserID)
}

// handleSyncHealth verifica el estado del sistema de sincronización
// Endpoint: GET /sync/health
// Proporciona información sobre la salud del servicio de sync
func handleSyncHealth(w http.ResponseWriter, r *http.Request) {
	// Verificar método HTTP
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar estado de la base de datos
	dbStatus := "healthy"
	dbError := ""
	if err := testDatabaseConnection(); err != nil {
		dbStatus = "unhealthy"
		dbError = err.Error()
		log.Printf("Database health check failed: %v", err)
	}

	// Verificar estado de Redis
	redisStatus := "healthy"
	redisError := ""
	if err := testRedisConnection(); err != nil {
		redisStatus = "unhealthy"
		redisError = err.Error()
		log.Printf("Redis health check failed: %v", err)
	}

	// Crear respuesta de health check
	healthResponse := map[string]interface{}{
		"service":    "expense_management_sync",
		"status":     "healthy",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"version":    "1.0.0",
		"components": map[string]interface{}{
			"database": map[string]string{
				"status": dbStatus,
				"error":  dbError,
			},
			"redis": map[string]string{
				"status": redisStatus,
				"error":  redisError,
			},
		},
		"metrics": map[string]interface{}{
			"uptime_seconds": time.Since(startTime).Seconds(),
			"sync_operations_total": getSyncOperationsCount(),
			"active_connections": getActiveConnectionsCount(),
		},
	}

	// Determinar estado general
	if dbStatus != "healthy" || redisStatus != "healthy" {
		healthResponse["status"] = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	// Enviar respuesta
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		log.Printf("Error enviando respuesta de health: %v", err)
	}
}

// handleSyncStats proporciona estadísticas detalladas de sincronización
// Endpoint: GET /sync/stats?user_id=X
// Útil para monitoreo y debugging
func handleSyncStats(w http.ResponseWriter, r *http.Request) {
	// Verificar método HTTP
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Obtener user_id (opcional para stats globales)
	userID := r.URL.Query().Get("user_id")

	var stats interface{}
	var err error

	if userID != "" {
		// Obtener estadísticas específicas del usuario
		stats, err = getUserSyncStats(userID)
		if err != nil {
			log.Printf("Error obteniendo stats del usuario %s: %v", userID, err)
			sendErrorResponse(w, fmt.Sprintf("Error obteniendo estadísticas: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Stats solicitadas para usuario: %s", userID)
	} else {
		// Obtener estadísticas globales del sistema
		stats, err = getGlobalSyncStats()
		if err != nil {
			log.Printf("Error obteniendo stats globales: %v", err)
			sendErrorResponse(w, fmt.Sprintf("Error obteniendo estadísticas: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Stats globales solicitadas")
	}

	// Enviar respuesta
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error enviando respuesta de stats: %v", err)
	}
}

// handleSyncConfig maneja configuración del sistema de sincronización
// Endpoint: GET/POST /sync/config
// Permite consultar y actualizar configuración de sync
func handleSyncConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Obtener configuración actual
		config, err := getSyncConfig()
		if err != nil {
			log.Printf("Error obteniendo configuración de sync: %v", err)
			sendErrorResponse(w, fmt.Sprintf("Error obteniendo configuración: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(config); err != nil {
			log.Printf("Error enviando configuración: %v", err)
		}

	case "POST":
		// Actualizar configuración
		var config SyncConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			log.Printf("Error decodificando configuración: %v", err)
			sendErrorResponse(w, "Formato JSON inválido", http.StatusBadRequest)
			return
		}

		if err := updateSyncConfig(&config); err != nil {
			log.Printf("Error actualizando configuración: %v", err)
			sendErrorResponse(w, fmt.Sprintf("Error actualizando configuración: %v", err), http.StatusInternalServerError)
			return
		}

		// Enviar configuración actualizada
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(config); err != nil {
			log.Printf("Error enviando configuración actualizada: %v", err)
		}

		log.Printf("Configuración de sync actualizada")

	default:
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

// Variables globales para métricas y monitoreo
var (
	startTime = time.Now() // Tiempo de inicio del servicio
)

// Funciones auxiliares para health checks y métricas

// testDatabaseConnection verifica que la conexión a la base de datos funciona
func testDatabaseConnection() error {
	db, err := openDatabase()
	if err != nil {
		return fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("error ping base de datos: %v", err)
	}

	return nil
}

// testRedisConnection verifica que la conexión a Redis funciona
func testRedisConnection() error {
	// Usar el cliente Redis común
	client := common.GetRedisClient()
	if client == nil {
		return fmt.Errorf("cliente Redis no disponible")
	}

	// Test ping
	if err := client.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("error ping Redis: %v", err)
	}

	return nil
}

// getSyncOperationsCount obtiene el número total de operaciones de sync procesadas
func getSyncOperationsCount() int64 {
	// Implementar contador de operaciones (puede usar Redis o DB)
	// Por ahora retornar valor placeholder
	return 0
}

// getActiveConnectionsCount obtiene el número de conexiones activas
func getActiveConnectionsCount() int {
	// Implementar contador de conexiones activas
	// Por ahora retornar valor placeholder
	return 1
}