package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleSyncBillHealth proporciona health check específico para sincronización de facturas
// Endpoint: GET /bills/sync/health
// Verifica el estado del sistema de sincronización de facturas
func handleSyncBillHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar conexión a base de datos específica para bills
	_, err := openBillDatabase()
	if err != nil {
		response := map[string]interface{}{
			"service":     "bills_management_sync",
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

	// Health check exitoso
	response := map[string]interface{}{
		"service":      "bills_management_sync",
		"status":       "healthy",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"version":      "1.0.0",
		"database":     "connected",
		"cache":        getCacheStatus(),
		"capabilities": []string{"batch_sync", "conflict_resolution", "offline_support"},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de health check: %v", err)
	}
}

// handleSyncBillBatch procesa un lote de operaciones de sincronización de facturas
// Endpoint: POST /bills/sync/batch
// Procesa múltiples operaciones offline de facturas en una sola solicitud
func handleSyncBillBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Decodificar solicitud de sincronización por lotes
	var batchRequest SyncBillBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&batchRequest); err != nil {
		log.Printf("Error decodificando batch request: %v", err)
		sendErrorResponse(w, "Formato de solicitud inválido", http.StatusBadRequest)
		return
	}

	// Validar solicitud usando validadores específicos
	if err := batchRequest.Validate(); err != nil {
		log.Printf("Validación fallida: %v", err)
		sendErrorResponse(w, fmt.Sprintf("Datos inválidos: %v", err), http.StatusBadRequest)
		return
	}

	// Procesar el lote de operaciones
	response, err := processBillBatch(batchRequest)
	if err != nil {
		log.Printf("Error procesando lote de facturas: %v", err)
		sendErrorResponse(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}

	// Invalidar cache de facturas para el usuario
	invalidateBillsCache(batchRequest.UserID)

	// Enviar respuesta de sincronización
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de batch sync: %v", err)
	}
}

// handleSyncBillChanges obtiene cambios de facturas desde el último sync
// Endpoint: GET /bills/sync/changes
// Permite al cliente obtener actualizaciones del servidor sin enviar datos
func handleSyncBillChanges(w http.ResponseWriter, r *http.Request) {
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

	// Crear solicitud de cambios
	changesRequest := SyncBillChangesRequest{
		UserID:   userID,
		LastSync: lastSync,
		Limit:    50, // Límite predeterminado
		Offset:   0,
	}

	// Obtener cambios desde el servidor
	response, err := getBillChanges(changesRequest)
	if err != nil {
		log.Printf("Error obteniendo cambios de facturas: %v", err)
		sendErrorResponse(w, "Error obteniendo cambios", http.StatusInternalServerError)
		return
	}

	// Enviar respuesta con los cambios
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error enviando respuesta de cambios: %v", err)
	}
}

// openBillDatabase obtiene la conexión a la base de datos para operaciones de sincronización de facturas
// Verifica que la conexión esté activa antes de retornarla
func openBillDatabase() (*sql.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("conexión a base de datos no inicializada")
	}
	
	// Verificar que la conexión esté activa
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error de conexión a base de datos: %v", err)
	}
	
	return db, nil
}

// getCacheStatus verifica el estado del cache para health checks
// Retorna información sobre la disponibilidad del sistema de cache
func getCacheStatus() string {
	if cacheManager != nil {
		return "available"
	}
	return "unavailable"
}