package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

// SyncService maneja la sincronización de datos offline
// Proporciona endpoints para sincronización masiva de operaciones pendientes
// y resolución de conflictos entre datos locales y del servidor
type SyncService struct {
	db *sql.DB
}

// SyncRequest representa una solicitud de sincronización del cliente
// Contiene las operaciones offline que necesitan ser procesadas
type SyncRequest struct {
	UserID             string             `json:"user_id"`
	DeviceID           string             `json:"device_id"`
	LastSyncTimestamp  string             `json:"last_sync_timestamp"`
	Operations         []OfflineOperation `json:"operations"`
	ClientTimestamp    string             `json:"client_timestamp"`
}

// OfflineOperation representa una operación offline a sincronizar
// Estructura que coincide con el modelo Flutter para compatibilidad
type OfflineOperation struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"operation_type"`
	TableName     string                 `json:"table_name"`
	Data          map[string]interface{} `json:"data"`
	SequenceNumber int                   `json:"sequence_number"`
	CreatedAt     string                 `json:"created_at"`
	Dependencies  []string               `json:"dependencies"`
}

// SyncResponse respuesta del servidor tras procesar sincronización
// Proporciona resultados detallados y datos actualizados para el cliente
type SyncResponse struct {
	Success           bool                     `json:"success"`
	Message           string                   `json:"message"`
	ProcessedOps      int                      `json:"processed_operations"`
	SuccessfulOps     int                      `json:"successful_operations"`
	FailedOps         int                      `json:"failed_operations"`
	ServerUpdates     []map[string]interface{} `json:"server_updates"`
	ConflictResolutions []ConflictResolution   `json:"conflict_resolutions"`
	NewSyncTimestamp  string                   `json:"new_sync_timestamp"`
	Errors            []string                 `json:"errors,omitempty"`
}

// ConflictResolution información sobre resolución de conflictos
// Documenta cómo se resolvieron los conflictos durante la sincronización
type ConflictResolution struct {
	OperationID string `json:"operation_id"`
	TableName   string `json:"table_name"`
	Resolution  string `json:"resolution"`
	Reason      string `json:"reason"`
}

// NewSyncService crea una nueva instancia del servicio de sincronización
// Inicializa conexión a base de datos y configura tablas necesarias
func NewSyncService() (*SyncService, error) {
	// Conectar a la base de datos principal del sistema
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "../budget_data.db" // Ruta por defecto
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %v", err)
	}

	// Verificar conexión
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	// Crear tablas de sincronización si no existen
	service := &SyncService{db: db}
	if err := service.createSyncTables(); err != nil {
		return nil, fmt.Errorf("error creating sync tables: %v", err)
	}

	return service, nil
}

// createSyncTables crea las tablas necesarias para el proceso de sincronización
// Configura metadatos de sincronización y logs de operaciones
func (s *SyncService) createSyncTables() error {
	// Tabla para metadatos de sincronización por usuario
	createSyncMetadata := `
	CREATE TABLE IF NOT EXISTS sync_metadata (
		user_id TEXT PRIMARY KEY,
		device_id TEXT,
		last_sync_timestamp TEXT,
		last_client_timestamp TEXT,
		total_operations_processed INTEGER DEFAULT 0,
		last_sync_device TEXT,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);`

	// Tabla para log de operaciones de sincronización
	createSyncLog := `
	CREATE TABLE IF NOT EXISTS sync_operations_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		operation_id TEXT NOT NULL,
		operation_type TEXT NOT NULL,
		table_name TEXT NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT,
		processed_at TEXT DEFAULT CURRENT_TIMESTAMP,
		server_id INTEGER,
		conflict_resolution TEXT
	);`

	// Ejecutar creación de tablas
	if _, err := s.db.Exec(createSyncMetadata); err != nil {
		return fmt.Errorf("error creating sync_metadata table: %v", err)
	}

	if _, err := s.db.Exec(createSyncLog); err != nil {
		return fmt.Errorf("error creating sync_operations_log table: %v", err)
	}

	return nil
}

// handleBatchSync maneja solicitudes de sincronización masiva
// Endpoint principal para procesar múltiples operaciones offline
func (s *SyncService) handleBatchSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var syncReq SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&syncReq); err != nil {
		log.Printf("Error decoding sync request: %v", err)
		s.sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validar datos requeridos
	if syncReq.UserID == "" {
		s.sendErrorResponse(w, "User ID is required", http.StatusBadRequest)
		return
	}

	log.Printf("Processing sync request for user %s with %d operations", 
		syncReq.UserID, len(syncReq.Operations))

	// Procesar sincronización
	response := s.processBatchSync(syncReq)
	
	// Enviar respuesta
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// processBatchSync procesa todas las operaciones de sincronización
// Ejecuta operaciones en orden secuencial y maneja conflictos
func (s *SyncService) processBatchSync(req SyncRequest) SyncResponse {
	response := SyncResponse{
		Success:             true,
		ProcessedOps:        0,
		SuccessfulOps:       0,
		FailedOps:           0,
		ServerUpdates:       []map[string]interface{}{},
		ConflictResolutions: []ConflictResolution{},
		NewSyncTimestamp:    time.Now().UTC().Format(time.RFC3339),
		Errors:              []string{},
	}

	// Ordenar operaciones por número de secuencia
	operations := req.Operations
	for i := 0; i < len(operations)-1; i++ {
		for j := i + 1; j < len(operations); j++ {
			if operations[i].SequenceNumber > operations[j].SequenceNumber {
				operations[i], operations[j] = operations[j], operations[i]
			}
		}
	}

	// Procesar cada operación en orden secuencial
	for _, operation := range operations {
		response.ProcessedOps++
		
		success, serverID, conflictRes, err := s.processOperation(req.UserID, operation)
		
		if success {
			response.SuccessfulOps++
			
			// Registrar éxito en log
			s.logOperation(req.UserID, operation, "completed", "", serverID, conflictRes)
			
			// Agregar resolución de conflicto si existe
			if conflictRes != "" {
				response.ConflictResolutions = append(response.ConflictResolutions, ConflictResolution{
					OperationID: operation.ID,
					TableName:   operation.TableName,
					Resolution:  conflictRes,
					Reason:      "Automatic conflict resolution applied",
				})
			}
			
		} else {
			response.FailedOps++
			response.Success = false
			
			errorMsg := "Unknown error"
			if err != nil {
				errorMsg = err.Error()
			}
			response.Errors = append(response.Errors, 
				fmt.Sprintf("Operation %s failed: %s", operation.ID, errorMsg))
			
			// Registrar fallo en log
			s.logOperation(req.UserID, operation, "failed", errorMsg, 0, "")
		}
	}

	// Actualizar metadatos de sincronización
	s.updateSyncMetadata(req.UserID, req.DeviceID, response.NewSyncTimestamp, 
		req.ClientTimestamp, response.SuccessfulOps)

	// Configurar mensaje de respuesta
	if response.Success {
		response.Message = fmt.Sprintf("Sync completed successfully. Processed %d operations.", 
			response.SuccessfulOps)
	} else {
		response.Message = fmt.Sprintf("Sync completed with errors. %d successful, %d failed.", 
			response.SuccessfulOps, response.FailedOps)
	}

	return response
}

// processOperation procesa una operación individual
// Maneja crear, actualizar y eliminar records según el tipo de operación
func (s *SyncService) processOperation(userID string, op OfflineOperation) (bool, int, string, error) {
	log.Printf("Processing operation %s: %s on %s", op.ID, op.Type, op.TableName)

	switch op.Type {
	case "create":
		return s.processCreateOperation(userID, op)
	case "update":
		return s.processUpdateOperation(userID, op)
	case "delete":
		return s.processDeleteOperation(userID, op)
	default:
		return false, 0, "", fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

// processCreateOperation procesa operación de creación
// Inserta nuevo record en la tabla correspondiente del servidor
func (s *SyncService) processCreateOperation(userID string, op OfflineOperation) (bool, int, string, error) {
	// Mapear tabla local a tabla del servidor
	serverTable := s.mapToServerTable(op.TableName)
	if serverTable == "" {
		return false, 0, "", fmt.Errorf("unknown table: %s", op.TableName)
	}

	// Preparar datos para inserción
	op.Data["user_id"] = userID
	op.Data["created_at"] = time.Now().UTC().Format(time.RFC3339)
	op.Data["updated_at"] = time.Now().UTC().Format(time.RFC3339)

	// Construir query de inserción dinámicamente
	columns := []string{}
	placeholders := []string{}
	values := []interface{}{}

	for key, value := range op.Data {
		if key != "id" && key != "local_id" { // Excluir IDs locales
			columns = append(columns, key)
			placeholders = append(placeholders, "?")
			values = append(values, value)
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		serverTable, 
		joinStrings(columns, ", "),
		joinStrings(placeholders, ", "))

	result, err := s.db.Exec(query, values...)
	if err != nil {
		return false, 0, "", fmt.Errorf("error inserting into %s: %v", serverTable, err)
	}

	// Obtener ID del servidor
	serverID, err := result.LastInsertId()
	if err != nil {
		return false, 0, "", fmt.Errorf("error getting server ID: %v", err)
	}

	return true, int(serverID), "", nil
}

// processUpdateOperation procesa operación de actualización
// Actualiza record existente en la tabla del servidor
func (s *SyncService) processUpdateOperation(userID string, op OfflineOperation) (bool, int, string, error) {
	serverTable := s.mapToServerTable(op.TableName)
	if serverTable == "" {
		return false, 0, "", fmt.Errorf("unknown table: %s", op.TableName)
	}

	// Obtener server_id del registro
	serverIDInterface, exists := op.Data["server_id"]
	if !exists {
		return false, 0, "", fmt.Errorf("server_id required for update operation")
	}

	serverID, err := strconv.Atoi(fmt.Sprintf("%v", serverIDInterface))
	if err != nil {
		return false, 0, "", fmt.Errorf("invalid server_id: %v", serverIDInterface)
	}

	// Preparar datos para actualización
	op.Data["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	
	// Construir query de actualización dinámicamente
	setParts := []string{}
	values := []interface{}{}

	for key, value := range op.Data {
		if key != "id" && key != "local_id" && key != "server_id" && key != "created_at" {
			setParts = append(setParts, key+" = ?")
			values = append(values, value)
		}
	}

	// Agregar WHERE clause
	values = append(values, serverID, userID)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = ? AND user_id = ?",
		serverTable, joinStrings(setParts, ", "))

	result, err := s.db.Exec(query, values...)
	if err != nil {
		return false, 0, "", fmt.Errorf("error updating %s: %v", serverTable, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return false, 0, "", fmt.Errorf("no rows updated - record may not exist")
	}

	return true, serverID, "", nil
}

// processDeleteOperation procesa operación de eliminación
// Elimina record de la tabla del servidor
func (s *SyncService) processDeleteOperation(userID string, op OfflineOperation) (bool, int, string, error) {
	serverTable := s.mapToServerTable(op.TableName)
	if serverTable == "" {
		return false, 0, "", fmt.Errorf("unknown table: %s", op.TableName)
	}

	// Obtener server_id del registro
	serverIDInterface, exists := op.Data["server_id"]
	if !exists {
		return false, 0, "", fmt.Errorf("server_id required for delete operation")
	}

	serverID, err := strconv.Atoi(fmt.Sprintf("%v", serverIDInterface))
	if err != nil {
		return false, 0, "", fmt.Errorf("invalid server_id: %v", serverIDInterface)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ? AND user_id = ?", serverTable)
	result, err := s.db.Exec(query, serverID, userID)
	if err != nil {
		return false, 0, "", fmt.Errorf("error deleting from %s: %v", serverTable, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return false, 0, "", fmt.Errorf("no rows deleted - record may not exist")
	}

	return true, serverID, "", nil
}

// mapToServerTable mapea nombres de tabla local a tabla del servidor
// Convierte nombres de tabla de Flutter a nombres del esquema del servidor
func (s *SyncService) mapToServerTable(localTable string) string {
	mapping := map[string]string{
		"expenses_local":   "expenses",
		"incomes_local":    "incomes", 
		"bills_local":      "bills",
		"categories_local": "categories",
	}
	
	return mapping[localTable]
}

// logOperation registra el resultado de una operación en el log
// Mantiene auditoría completa de todas las operaciones de sincronización
func (s *SyncService) logOperation(userID string, op OfflineOperation, status, errorMsg string, serverID int, conflictRes string) {
	query := `INSERT INTO sync_operations_log 
	          (user_id, operation_id, operation_type, table_name, status, error_message, server_id, conflict_resolution)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, userID, op.ID, op.Type, op.TableName, status, errorMsg, serverID, conflictRes)
	if err != nil {
		log.Printf("Error logging operation: %v", err)
	}
}

// updateSyncMetadata actualiza metadatos de última sincronización
// Registra información para futuras sincronizaciones incrementales
func (s *SyncService) updateSyncMetadata(userID, deviceID, serverTimestamp, clientTimestamp string, opsProcessed int) {
	query := `INSERT OR REPLACE INTO sync_metadata 
	          (user_id, device_id, last_sync_timestamp, last_client_timestamp, total_operations_processed, last_sync_device, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	
	_, err := s.db.Exec(query, userID, deviceID, serverTimestamp, clientTimestamp, opsProcessed, deviceID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		log.Printf("Error updating sync metadata: %v", err)
	}
}

// sendErrorResponse envía respuesta de error estandarizada
// Proporciona formato consistente para errores del API
func (s *SyncService) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"success": false,
		"message": message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	json.NewEncoder(w).Encode(response)
}

// joinStrings une slice de strings con separador
// Utilidad para construcción dinámica de queries SQL
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// main función principal del servicio de sincronización
// Configura servidor HTTP y endpoints para sincronización
func main() {
	// Crear instancia del servicio
	service, err := NewSyncService()
	if err != nil {
		log.Fatalf("Error creating sync service: %v", err)
	}
	defer service.db.Close()

	// Configurar router
	router := mux.NewRouter()
	
	// Endpoint principal de sincronización
	router.HandleFunc("/sync/batch", service.handleBatchSync).Methods("POST", "OPTIONS")
	
	// Endpoint de salud del servicio
	router.HandleFunc("/sync/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "healthy",
			"service": "sync-service",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}).Methods("GET")

	// Configurar puerto del servidor
	port := os.Getenv("SYNC_SERVICE_PORT")
	if port == "" {
		port = "8101" // Puerto por defecto
	}

	log.Printf("🔄 Sync Service starting on port %s", port)
	log.Printf("📊 Available endpoints:")
	log.Printf("  POST /sync/batch - Batch synchronization")
	log.Printf("  GET  /sync/health - Health check")

	// Iniciar servidor
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}