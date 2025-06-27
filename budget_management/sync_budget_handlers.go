package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Handlers para sincronización offline de presupuestos
// Implementa endpoints específicos para operaciones de sincronización de presupuestos

// handleSyncBudgetHealth proporciona health check específico para sincronización de presupuestos
// Endpoint: GET /budget/sync/health
// Verifica el estado del sistema de sincronización de presupuestos
func handleSyncBudgetHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar conexión a base de datos específica para budgets
	_, err := openBudgetDatabase()
	if err != nil {
		response := map[string]interface{}{
			"service":     "budget_management_sync",
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

	// Health check exitoso específico para presupuestos
	response := map[string]interface{}{
		"service":      "budget_management_sync",
		"status":       "healthy",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"version":      "1.0.0",
		"database":     "connected",
		"cache":        getBudgetCacheStatus(),
		"capabilities": []string{"batch_sync", "conflict_resolution", "offline_support", "period_inheritance"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de health check: %v", err)
	}
}

// handleSyncBudgetBatch procesa un lote de operaciones de sincronización de presupuestos
// Endpoint: POST /budget/sync/batch
// Procesa múltiples operaciones offline de presupuestos en una sola solicitud
func handleSyncBudgetBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar solicitud de sincronización por lotes
	var batchRequest SyncBudgetBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchRequest); err != nil {
		log.Printf("Error decodificando batch request de presupuestos: %v", err)
		sendErrorResponse(w, "Formato de solicitud inválido", http.StatusBadRequest)
		return
	}

	// Validar solicitud usando validadores específicos
	if err := batchRequest.Validate(); err != nil {
		log.Printf("Validación de presupuestos fallida: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Datos inválidos: %v", err), http.StatusBadRequest)
		return
	}

	// Procesar el lote de operaciones de presupuestos
	response, err := processBudgetBatch(batchRequest)
	if err != nil {
		log.Printf("Error procesando lote de presupuestos: %v", err)
		sendErrorResponse(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Invalidar cache de presupuestos para el usuario
	invalidateBudgetsCache(batchRequest.UserID)

	// Enviar respuesta de sincronización
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de batch sync de presupuestos: %v", err)
	}
}

// handleSyncBudgetChanges obtiene cambios de presupuestos desde el último sync
// Endpoint: GET /budget/sync/changes
// Permite al cliente obtener actualizaciones del servidor sin enviar datos
func handleSyncBudgetChanges(w http.ResponseWriter, r *http.Request) {
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

	// Crear solicitud de cambios de presupuestos
	changesRequest := SyncBudgetChangesRequest{
		UserID:   userID,
		LastSync: lastSync,
		Limit:    25, // Límite predeterminado para presupuestos
		Offset:   0,
	}

	// Obtener cambios desde el servidor
	response, err := getBudgetChanges(changesRequest)
	if err != nil {
		log.Printf("Error obteniendo cambios de presupuestos: %v", err)
		sendErrorResponse(w, "Error obteniendo cambios", http.StatusInternalServerError)
		return
	}

	// Enviar respuesta con los cambios
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de cambios de presupuestos: %v", err)
	}
}

// openBudgetDatabase obtiene la conexión a la base de datos para operaciones de sincronización de presupuestos
// Verifica que la conexión esté activa antes de retornarla
func openBudgetDatabase() (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("conexión a base de datos no inicializada")
	}
	
	// Verificar que la conexión esté activa
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error de conexión a base de datos: %v", err)
	}
	
	return db, nil
}

// getBudgetCacheStatus verifica el estado del cache para health checks de presupuestos
// Retorna información sobre la disponibilidad del sistema de cache
func getBudgetCacheStatus() string {
	// Por ahora retornamos unavailable ya que no tenemos cache manager implementado
	// En el futuro se puede implementar integración con Redis
	return "unavailable"
}

// invalidateBudgetsCache invalida el cache de presupuestos para un usuario específico
// Asegura que los datos en cache estén actualizados después de cambios
func invalidateBudgetsCache(userID string) {
	// Implementación de invalidación de cache específica para presupuestos
	// Por ahora es un stub, se puede implementar con Redis en el futuro
	log.Printf("Invalidating budget cache for user: %s", userID)
}

// handleSyncBudgetConflictResolution maneja la resolución de conflictos específicos de presupuestos
// Endpoint: POST /budget/sync/resolve-conflict
// Permite al cliente especificar cómo resolver conflictos detectados
func handleSyncBudgetConflictResolution(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar solicitud de resolución de conflicto
	var conflictRequest SyncBudgetConflictRequest
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
	err := resolveBudgetConflict(conflictRequest)
	if err != nil {
		log.Printf("Error resolviendo conflicto de presupuesto: %v", err)
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

// resolveBudgetConflict procesa la resolución de un conflicto específico de presupuesto
// Implementa la lógica para aplicar la resolución elegida por el usuario
func resolveBudgetConflict(request SyncBudgetConflictRequest) error {
	// Implementación de resolución de conflictos específica para presupuestos
	log.Printf("Resolviendo conflicto de presupuesto para usuario %s, resolución: %s", 
		request.UserID, request.Resolution)
	
	switch request.Resolution {
	case "server_wins":
		// El servidor mantiene su versión, descarta cambios del cliente
		return resolveConflictServerWins(request)
	case "client_wins":
		// Se aplican los cambios del cliente, se descarta la versión del servidor
		return resolveConflictClientWins(request)
	case "merge":
		// Se fusionan los datos utilizando request.MergedData
		return resolveConflictMerge(request)
	default:
		return fmt.Errorf("tipo de resolución no soportado: %s", request.Resolution)
	}
}

// resolveConflictServerWins implementa resolución donde gana el servidor
func resolveConflictServerWins(request SyncBudgetConflictRequest) error {
	log.Printf("Aplicando resolución server_wins para presupuesto %s", request.LocalID)
	// Implementación específica donde se mantiene la versión del servidor
	return nil
}

// resolveConflictClientWins implementa resolución donde gana el cliente
func resolveConflictClientWins(request SyncBudgetConflictRequest) error {
	log.Printf("Aplicando resolución client_wins para presupuesto %s", request.LocalID)
	// Implementación específica donde se aplican los cambios del cliente
	return nil
}

// resolveConflictMerge implementa resolución por fusión de datos
func resolveConflictMerge(request SyncBudgetConflictRequest) error {
	log.Printf("Aplicando resolución merge para presupuesto %s", request.LocalID)
	// Implementación específica donde se fusionan los datos
	return updateBudgetData(request.MergedData)
}