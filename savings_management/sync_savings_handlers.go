package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Handlers para sincronización offline de ahorros
// Implementa endpoints específicos para operaciones de sincronización de savings

// handleSyncSavingsHealth proporciona health check específico para sincronización de ahorros
// Endpoint: GET /savings/sync/health
// Verifica el estado del sistema de sincronización de ahorros
func handleSyncSavingsHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar conexión a base de datos específica para savings
	_, err := openSavingsDatabase()
	if err != nil {
		response := map[string]interface{}{
			"service":     "savings_management_sync",
			"status":      "unhealthy",
			"error":       err.Error(),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
			"version":     "1.0.0",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Health check exitoso específico para ahorros
	response := map[string]interface{}{
		"service":      "savings_management_sync",
		"status":       "healthy",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"version":      "1.0.0",
		"database":     "connected",
		"cache":        getSavingsCacheStatus(),
		"capabilities": []string{"batch_sync", "conflict_resolution", "offline_support", "goal_tracking"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de health check: %v", err)
	}
}

// handleSyncSavingsBatch procesa un lote de operaciones de sincronización de ahorros
// Endpoint: POST /savings/sync/batch
// Procesa múltiples operaciones offline de savings en una sola solicitud
func handleSyncSavingsBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar solicitud de sincronización por lotes
	var batchRequest SyncSavingsBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchRequest); err != nil {
		log.Printf("Error decodificando batch request de ahorros: %v", err)
		sendErrorResponse(w, "Formato de solicitud inválido", http.StatusBadRequest)
		return
	}

	// Validar solicitud usando validadores específicos
	if err := batchRequest.Validate(); err != nil {
		log.Printf("Validación de ahorros fallida: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Datos inválidos: %v", err), http.StatusBadRequest)
		return
	}

	// Procesar el lote de operaciones de ahorros
	response, err := processSavingsBatch(batchRequest)
	if err != nil {
		log.Printf("Error procesando lote de ahorros: %v", err)
		sendErrorResponse(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Invalidar cache de ahorros para el usuario
	invalidateSavingsCache(batchRequest.UserID)

	// Enviar respuesta de sincronización
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de batch sync de ahorros: %v", err)
	}
}

// handleSyncSavingsChanges obtiene cambios de ahorros desde el último sync
// Endpoint: GET /savings/sync/changes
// Permite al cliente obtener actualizaciones del servidor sin enviar datos
func handleSyncSavingsChanges(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Extraer parámetros de la consulta
	userID := r.URL.Query().Get("user_id")
	lastSync := r.URL.Query().Get("last_sync")
	
	if userID == "" {
		sendErrorResponse(w, "user_id es requerido", http.StatusBadRequest)
		return
	}

	// Crear solicitud de cambios de ahorros
	changesRequest := SyncSavingsChangesRequest{
		UserID:   userID,
		LastSync: lastSync,
		Limit:    25, // Límite predeterminado para ahorros
		Offset:   0,
	}

	// Obtener cambios desde el servidor
	response, err := getSavingsChanges(changesRequest)
	if err != nil {
		log.Printf("Error obteniendo cambios de ahorros: %v", err)
		sendErrorResponse(w, "Error obteniendo cambios", http.StatusInternalServerError)
		return
	}

	// Enviar respuesta con los cambios
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de cambios de ahorros: %v", err)
	}
}

// handleSyncSavingsConflictResolution maneja la resolución de conflictos específicos de ahorros
// Endpoint: POST /savings/sync/resolve-conflict
// Permite al cliente especificar cómo resolver conflictos detectados
func handleSyncSavingsConflictResolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar solicitud de resolución de conflicto
	var conflictRequest SyncSavingsConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&conflictRequest); err != nil {
		log.Printf("Error decodificando conflict resolution request: %v", err)
		sendErrorResponse(w, "Formato de solicitud inválido", http.StatusBadRequest)
		return
	}

	// Validar campos requeridos
	if conflictRequest.UserID == "" || conflictRequest.LocalID == "" || conflictRequest.Resolution == "" {
		sendErrorResponse(w, "user_id, local_id y resolution son requeridos", http.StatusBadRequest)
		return
	}

	// Procesar resolución de conflicto
	err := resolveSavingsConflict(conflictRequest)
	if err != nil {
		log.Printf("Error resolviendo conflicto de ahorro: %v", err)
		sendErrorResponse(w, "Error resolviendo conflicto", http.StatusInternalServerError)
		return
	}

	// Respuesta exitosa
	response := map[string]interface{}{
		"success":   true,
		"message":   "Conflicto resuelto exitosamente",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// openSavingsDatabase obtiene la conexión a la base de datos para operaciones de sincronización de ahorros
// Verifica que la conexión esté activa antes de retornarla
func openSavingsDatabase() (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("conexión a base de datos no inicializada")
	}
	
	// Verificar que la conexión esté activa
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error de conexión a base de datos: %v", err)
	}
	
	return db, nil
}

// getSavingsCacheStatus verifica el estado del cache para health checks de ahorros
// Retorna información sobre la disponibilidad del sistema de cache
func getSavingsCacheStatus() string {
	// Verificar si el cache manager está disponible
	if cacheManager != nil {
		return "available"
	}
	return "unavailable"
}

// invalidateSavingsCache invalida el cache de ahorros para un usuario específico
// Asegura que los datos en cache estén actualizados después de cambios
func invalidateSavingsCache(userID string) {
	// Implementación de invalidación de cache específica para ahorros
	if cacheManager != nil {
		err := cacheManager.InvalidateSavingsCache(userID)
		if err != nil {
			log.Printf("Warning: Failed to invalidate savings cache for user %s: %v", userID, err)
		}
		
		// También invalidar cache del dashboard ya que savings afecta el dashboard
		err = cacheManager.InvalidateDashboardCache(userID, "monthly")
		if err != nil {
			log.Printf("Warning: Failed to invalidate dashboard cache for user %s: %v", userID, err)
		}
		
		log.Printf("✅ Cache invalidated for user: %s (savings and dashboard)", userID)
	} else {
		log.Printf("Cache invalidation for savings - user: %s (cache manager unavailable)", userID)
	}
}

// resolveSavingsConflict procesa la resolución de un conflicto específico de ahorro
// Implementa la lógica para aplicar la resolución elegida por el usuario
func resolveSavingsConflict(request SyncSavingsConflictRequest) error {
	// Implementación de resolución de conflictos específica para ahorros
	log.Printf("Resolviendo conflicto de ahorro para usuario %s, resolución: %s", 
		request.UserID, request.Resolution)
	
	switch request.Resolution {
	case "server_wins":
		// El servidor mantiene su versión, descarta cambios del cliente
		return resolveSavingsConflictServerWins(request)
	case "client_wins":
		// Se aplican los cambios del cliente, se descarta la versión del servidor
		return resolveSavingsConflictClientWins(request)
	case "merge":
		// Se fusionan los datos utilizando request.MergedData
		return resolveSavingsConflictMerge(request)
	default:
		return fmt.Errorf("tipo de resolución no soportado: %s", request.Resolution)
	}
}

// resolveSavingsConflictServerWins implementa resolución donde gana el servidor
func resolveSavingsConflictServerWins(request SyncSavingsConflictRequest) error {
	log.Printf("Aplicando resolución server_wins para ahorro %s", request.LocalID)
	// Implementación específica donde se mantiene la versión del servidor
	// No se requiere acción adicional ya que el servidor ya tiene la versión correcta
	return nil
}

// resolveSavingsConflictClientWins implementa resolución donde gana el cliente
func resolveSavingsConflictClientWins(request SyncSavingsConflictRequest) error {
	log.Printf("Aplicando resolución client_wins para ahorro %s", request.LocalID)
	// Implementación específica donde se aplican los cambios del cliente
	// Se actualizará la base de datos del servidor con los datos del cliente
	return updateSavingsData(request.MergedData)
}

// resolveSavingsConflictMerge implementa resolución por fusión de datos
func resolveSavingsConflictMerge(request SyncSavingsConflictRequest) error {
	log.Printf("Aplicando resolución merge para ahorro %s", request.LocalID)
	// Implementación específica donde se fusionan los datos
	// Los datos fusionados ya vienen en request.MergedData
	return updateSavingsData(request.MergedData)
}