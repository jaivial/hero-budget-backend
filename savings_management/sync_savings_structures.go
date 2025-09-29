package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de ahorros
// Permite la sincronización bidireccional entre cliente offline y servidor
// Basado en el patrón exitoso implementado en budget_management, bills_management y expense_management

// SyncSavingsBatchRequest representa una solicitud de sincronización por lotes para ahorros
// Contiene todas las operaciones offline realizadas por el cliente relacionadas con savings
type SyncSavingsBatchRequest struct {
	UserID     string           `json:"user_id"`     // ID del usuario que realiza la sincronización
	Savings    []OfflineSavings `json:"savings"`     // Lista de ahorros modificados offline
	LastSync   string           `json:"last_sync"`   // Timestamp del último sync exitoso
	ClientID   string           `json:"client_id"`   // ID único del cliente para evitar duplicados
	DeviceInfo string           `json:"device_info"` // Información del dispositivo (opcional)
	AppVersion string           `json:"app_version"` // Versión de la app cliente
}

// OfflineSavings representa datos de ahorro modificados offline
// Incluye información completa para detección y resolución de conflictos
// Mantiene compatibilidad con la estructura SavingsData existente
type OfflineSavings struct {
	ID               string  `json:"id"`                // ID del ahorro (puede ser local para nuevos)
	LocalID          string  `json:"local_id"`          // ID local único en el dispositivo
	ServerID         string  `json:"server_id"`         // ID en el servidor (vacío para nuevos ahorros)
	Action           string  `json:"action"`            // "add", "update", "delete"
	UserID           string  `json:"user_id"`           // ID del usuario propietario
	Available        float64 `json:"available"`         // Cantidad disponible ahorrada
	Goal             float64 `json:"goal"`              // Meta de ahorro establecida
	Period           string  `json:"period"`            // Período para alcanzar la meta
	Percent          float64 `json:"percent"`           // Porcentaje de progreso
	NeedToSave       float64 `json:"need_to_save"`      // Cantidad faltante para la meta
	DailyTarget      float64 `json:"daily_target"`      // Meta diaria calculada
	OfflineTimestamp string  `json:"offline_timestamp"` // Timestamp cuando se realizó offline
	SyncTimestamp    string  `json:"sync_timestamp"`    // Timestamp para sincronización
	Status           string  `json:"status"`            // "pending", "synced", "conflict"
	Version          int     `json:"version"`           // Versión para control de concurrencia
}

// SyncSavingsBatchResponse representa la respuesta del servidor tras sincronización de ahorros
// Incluye resultados, conflictos detectados y datos actualizados del servidor
type SyncSavingsBatchResponse struct {
	Success       bool                        `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string                      `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                         `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                         `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                         `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncSavingsResult         `json:"results"`        // Resultado detallado por operación
	Conflicts     []SavingsConflictResolution `json:"conflicts"`      // Conflictos detectados
	ServerData    []SavingsData               `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string                      `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string                      `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncSavingsResult representa el resultado de sincronización de una operación individual de ahorro
// Proporciona feedback detallado sobre cada ahorro procesado
type SyncSavingsResult struct {
	LocalID        string `json:"local_id"`                // ID local del ahorro
	ServerID       string `json:"server_id"`               // ID asignado en el servidor
	Action         string `json:"action"`                  // Acción realizada
	Status         string `json:"status"`                  // "success", "error", "conflict"
	Error          string `json:"error,omitempty"`         // Mensaje de error si aplica
	ConflictType   string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction bool   `json:"requires_action"`         // Si requiere acción del usuario
	SyncTimestamp  string `json:"sync_timestamp"`          // Timestamp de sincronización
}

// SavingsConflictResolution representa un conflicto detectado durante sincronización de ahorros
// Proporciona información para resolución manual o automática
type SavingsConflictResolution struct {
	LocalID      string      `json:"local_id"`      // ID local del ahorro en conflicto
	ServerID     string      `json:"server_id"`     // ID del ahorro en el servidor
	ConflictType string      `json:"conflict_type"` // "version", "timestamp", "data"
	LocalData    SavingsData `json:"local_data"`    // Datos del cliente
	ServerData   SavingsData `json:"server_data"`   // Datos del servidor
	Resolution   string      `json:"resolution"`    // "manual", "server_wins", "client_wins"
	Priority     string      `json:"priority"`      // "high", "medium", "low"
	Description  string      `json:"description"`   // Descripción del conflicto
	Suggestions  []string    `json:"suggestions"`   // Sugerencias de resolución
}

// SyncSavingsChangesRequest solicita cambios de ahorros del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
type SyncSavingsChangesRequest struct {
	UserID   string `json:"user_id"`   // ID del usuario
	LastSync string `json:"last_sync"` // Timestamp del último sync
	Limit    int    `json:"limit"`     // Límite de registros (opcional)
	Offset   int    `json:"offset"`    // Offset para paginación (opcional)
}

// SyncSavingsChangesResponse contiene cambios de ahorros del servidor para sincronización
// Permite al cliente actualizar su base de datos local
type SyncSavingsChangesResponse struct {
	Success      bool          `json:"success"`       // Éxito de la operación
	Message      string        `json:"message"`       // Mensaje descriptivo
	Changes      []SavingsData `json:"changes"`       // Ahorros modificados en el servidor
	Deletions    []string      `json:"deletions"`     // IDs de ahorros eliminados
	HasMore      bool          `json:"has_more"`      // Indica si hay más cambios
	TotalChanges int           `json:"total_changes"` // Total de cambios disponibles
	LastSync     string        `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime   string        `json:"server_time"`   // Timestamp actual del servidor
}

// SyncSavingsConflictRequest solicita resolución de un conflicto específico de ahorro
// Permite al cliente indicar cómo resolver conflictos detectados
type SyncSavingsConflictRequest struct {
	UserID       string      `json:"user_id"`               // ID del usuario
	LocalID      string      `json:"local_id"`              // ID local del ahorro
	ServerID     string      `json:"server_id"`             // ID del servidor
	Resolution   string      `json:"resolution"`            // "server_wins", "client_wins", "merge"
	MergedData   SavingsData `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice string      `json:"client_choice"`         // Elección específica del usuario
}

// SyncSavingsStats representa estadísticas de sincronización de ahorros
// Proporciona métricas útiles para monitoreo y optimización específicas de savings
type SyncSavingsStats struct {
	UserID            string     `json:"user_id"`                   // ID del usuario
	LastSyncTime      time.Time  `json:"last_sync_time"`            // Última sincronización exitosa
	TotalSyncs        int        `json:"total_syncs"`               // Total de sincronizaciones
	PendingOperations int        `json:"pending_ops"`               // Operaciones pendientes
	ConflictsResolved int        `json:"conflicts_resolved"`        // Conflictos resueltos
	DataSizeBytes     int64      `json:"data_size_bytes"`           // Tamaño de datos sincronizados
	AverageLatency    float64    `json:"avg_latency_ms"`            // Latencia promedio en ms
	ErrorCount        int        `json:"error_count"`               // Número de errores
	LastErrorTime     *time.Time `json:"last_error_time,omitempty"` // Último error
	LastErrorMessage  string     `json:"last_error_msg,omitempty"`  // Mensaje del último error
}

// SyncSavingsConfig almacena configuración para el sistema de sincronización de ahorros
// Permite ajustar comportamiento según necesidades específicas del sistema de savings
type SyncSavingsConfig struct {
	MaxBatchSize           int                `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution     string             `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes    int                `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts          int                `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds         int                `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled     bool               `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled      bool               `json:"encryption_enabled"`    // Habilitar encriptación de datos
	SavingsSpecificOptions SavingsSyncOptions `json:"savings_options"`       // Opciones específicas para ahorros
}

// SavingsSyncOptions contiene opciones específicas para sincronización de ahorros
// Personaliza el comportamiento de sync para las características únicas de savings
type SavingsSyncOptions struct {
	SyncCalculatedFields   bool `json:"sync_calculated_fields"`   // Sincronizar campos calculados (porcentajes, metas diarias)
	SyncGoalTracking       bool `json:"sync_goal_tracking"`       // Sincronizar seguimiento de metas
	HandleGoalConflicts    bool `json:"handle_goal_conflicts"`    // Manejar conflicts en metas
	AutoRecalculateTargets bool `json:"auto_recalculate_targets"` // Recalcular automáticamente targets
}

// Validate valida la estructura de una solicitud de sincronización por lotes de ahorros
// Asegura que todos los campos requeridos están presentes y son válidos
func (req *SyncSavingsBatchRequest) Validate() error {
	// Validar campos básicos requeridos
	if req.UserID == "" {
		return fmt.Errorf("user_id es requerido para sincronización de ahorros")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id es requerido para evitar duplicados")
	}
	if len(req.Savings) == 0 {
		return fmt.Errorf("no hay datos de ahorros para sincronizar")
	}

	// Validar límite de lote para evitar sobrecarga del servidor
	if len(req.Savings) > 50 {
		return fmt.Errorf("el lote excede el límite máximo de 50 registros de ahorros")
	}

	// Validar cada registro de ahorro individual en el lote
	for i, savings := range req.Savings {
		if err := savings.Validate(); err != nil {
			return fmt.Errorf("registro de ahorro %d inválido: %v", i, err)
		}
	}

	// Validar formato de timestamps si están presentes
	if req.LastSync != "" {
		if _, err := time.Parse(time.RFC3339, req.LastSync); err != nil {
			return fmt.Errorf("formato de last_sync inválido: debe ser RFC3339")
		}
	}

	return nil
}

// Validate valida un registro individual de ahorro offline
// Verifica que los campos obligatorios estén presentes y los valores sean consistentes
func (s *OfflineSavings) Validate() error {
	// Validar campos básicos requeridos
	if s.UserID == "" {
		return fmt.Errorf("user_id es requerido")
	}
	if s.LocalID == "" {
		return fmt.Errorf("local_id es requerido")
	}
	if s.Action == "" {
		return fmt.Errorf("action es requerida")
	}

	// Validar valores de acción permitidos
	validActions := map[string]bool{"add": true, "update": true, "delete": true}
	if !validActions[s.Action] {
		return fmt.Errorf("action inválida: %s (debe ser add, update o delete)", s.Action)
	}

	// Validar valores numéricos no negativos para add/update
	if s.Action != "delete" {
		if s.Available < 0 {
			return fmt.Errorf("available no puede ser negativo")
		}
		if s.Goal < 0 {
			return fmt.Errorf("goal no puede ser negativo")
		}
		if s.Percent < 0 || s.Percent > 100 {
			return fmt.Errorf("percent debe estar entre 0 y 100")
		}
	}

	// Validar timestamps si están presentes
	if s.OfflineTimestamp != "" {
		if _, err := time.Parse(time.RFC3339, s.OfflineTimestamp); err != nil {
			return fmt.Errorf("formato de offline_timestamp inválido: debe ser RFC3339")
		}
	}

	return nil
}
