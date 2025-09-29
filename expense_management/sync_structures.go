package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de gastos
// Permite la sincronización bidireccional entre cliente offline y servidor

// SyncBatchRequest representa una solicitud de sincronización por lotes
// Contiene todas las operaciones offline realizadas por el cliente
type SyncBatchRequest struct {
	UserID     string           `json:"user_id"`     // ID del usuario que realiza la sincronización
	Expenses   []OfflineExpense `json:"expenses"`    // Lista de gastos modificados offline
	LastSync   string           `json:"last_sync"`   // Timestamp del último sync exitoso
	ClientID   string           `json:"client_id"`   // ID único del cliente para evitar duplicados
	DeviceInfo string           `json:"device_info"` // Información del dispositivo (opcional)
	AppVersion string           `json:"app_version"` // Versión de la app cliente
}

// OfflineExpense representa un gasto modificado offline
// Incluye información completa para detección y resolución de conflictos
type OfflineExpense struct {
	ID               string  `json:"id"`                // ID del gasto (puede ser local para nuevos)
	LocalID          string  `json:"local_id"`          // ID local único en el dispositivo
	ServerID         int     `json:"server_id"`         // ID en el servidor (0 para nuevos gastos)
	Action           string  `json:"action"`            // "add", "update", "delete"
	UserID           string  `json:"user_id"`           // ID del usuario propietario
	Amount           float64 `json:"amount"`            // Cantidad del gasto
	Date             string  `json:"date"`              // Fecha del gasto (YYYY-MM-DD)
	Category         string  `json:"category"`          // Categoría del gasto
	PaymentMethod    string  `json:"payment_method"`    // "cash" o "bank"
	Description      string  `json:"description"`       // Descripción opcional del gasto
	OfflineTimestamp string  `json:"offline_timestamp"` // Timestamp cuando se realizó offline
	SyncTimestamp    string  `json:"sync_timestamp"`    // Timestamp para sincronización
	Status           string  `json:"status"`            // "pending", "synced", "conflict"
	Version          int     `json:"version"`           // Versión para control de concurrencia
}

// SyncBatchResponse representa la respuesta del servidor tras sincronización
// Incluye resultados, conflictos detectados y datos actualizados del servidor
type SyncBatchResponse struct {
	Success       bool                 `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string               `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                  `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                  `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                  `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncResult         `json:"results"`        // Resultado detallado por operación
	Conflicts     []ConflictResolution `json:"conflicts"`      // Conflictos detectados
	ServerData    []Expense            `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string               `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string               `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncResult representa el resultado de sincronización de una operación individual
// Proporciona feedback detallado sobre cada gasto procesado
type SyncResult struct {
	LocalID        string `json:"local_id"`                // ID local del gasto
	ServerID       string `json:"server_id"`               // ID asignado en el servidor
	Action         string `json:"action"`                  // Acción realizada
	Status         string `json:"status"`                  // "success", "error", "conflict"
	Error          string `json:"error,omitempty"`         // Mensaje de error si aplica
	ConflictType   string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction bool   `json:"requires_action"`         // Si requiere acción del usuario
	SyncTimestamp  string `json:"sync_timestamp"`          // Timestamp de sincronización
}

// ConflictResolution representa un conflicto detectado durante sincronización
// Proporciona información para resolución manual o automática
type ConflictResolution struct {
	LocalID      string   `json:"local_id"`      // ID local del gasto en conflicto
	ServerID     int      `json:"server_id"`     // ID del gasto en el servidor
	ConflictType string   `json:"conflict_type"` // "version", "timestamp", "data"
	LocalData    Expense  `json:"local_data"`    // Datos del cliente
	ServerData   Expense  `json:"server_data"`   // Datos del servidor
	Resolution   string   `json:"resolution"`    // "manual", "server_wins", "client_wins"
	Priority     string   `json:"priority"`      // "high", "medium", "low"
	Description  string   `json:"description"`   // Descripción del conflicto
	Suggestions  []string `json:"suggestions"`   // Sugerencias de resolución
}

// SyncChangesRequest solicita cambios del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
type SyncChangesRequest struct {
	UserID   string `json:"user_id"`   // ID del usuario
	LastSync string `json:"last_sync"` // Timestamp del último sync
	Limit    int    `json:"limit"`     // Límite de registros (opcional)
	Offset   int    `json:"offset"`    // Offset para paginación (opcional)
}

// SyncChangesResponse contiene cambios del servidor para sincronización
// Permite al cliente actualizar su base de datos local
type SyncChangesResponse struct {
	Success      bool      `json:"success"`       // Éxito de la operación
	Message      string    `json:"message"`       // Mensaje descriptivo
	Changes      []Expense `json:"changes"`       // Gastos modificados en el servidor
	Deletions    []int     `json:"deletions"`     // IDs de gastos eliminados
	HasMore      bool      `json:"has_more"`      // Indica si hay más cambios
	TotalChanges int       `json:"total_changes"` // Total de cambios disponibles
	LastSync     string    `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime   string    `json:"server_time"`   // Timestamp actual del servidor
}

// SyncConflictRequest solicita resolución de un conflicto específico
// Permite al cliente indicar cómo resolver conflictos detectados
type SyncConflictRequest struct {
	UserID       string  `json:"user_id"`               // ID del usuario
	LocalID      string  `json:"local_id"`              // ID local del gasto
	ServerID     int     `json:"server_id"`             // ID del servidor
	Resolution   string  `json:"resolution"`            // "server_wins", "client_wins", "merge"
	MergedData   Expense `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice string  `json:"client_choice"`         // Elección específica del usuario
}

// SyncStats representa estadísticas de sincronización
// Proporciona métricas útiles para monitoreo y optimización
type SyncStats struct {
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

// SyncConfig almacena configuración para el sistema de sincronización
// Permite ajustar comportamiento según necesidades del sistema
type SyncConfig struct {
	MaxBatchSize        int    `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution  string `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes int    `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int    `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds      int    `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled  bool   `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled   bool   `json:"encryption_enabled"`    // Habilitar encriptación de datos
}

// validateSyncRequest valida la estructura de una solicitud de sincronización
// Asegura que todos los campos requeridos están presentes y son válidos
func (req *SyncBatchRequest) Validate() error {
	if req.UserID == "" {
		return fmt.Errorf("user_id es requerido")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id es requerido")
	}
	if len(req.Expenses) == 0 {
		return fmt.Errorf("no hay gastos para sincronizar")
	}

	// Validar cada gasto individual
	for i, expense := range req.Expenses {
		if err := expense.Validate(); err != nil {
			return fmt.Errorf("gasto %d inválido: %v", i, err)
		}
	}

	return nil
}

// validateOfflineExpense valida un gasto offline individual
// Verifica que contiene la información mínima requerida
func (expense *OfflineExpense) Validate() error {
	if expense.LocalID == "" {
		return fmt.Errorf("local_id es requerido")
	}
	if expense.Action != "add" && expense.Action != "update" && expense.Action != "delete" {
		return fmt.Errorf("action debe ser add, update o delete")
	}
	if expense.UserID == "" {
		return fmt.Errorf("user_id es requerido")
	}
	if expense.Action != "delete" {
		if expense.Amount <= 0 {
			return fmt.Errorf("amount debe ser mayor que 0")
		}
		if expense.Date == "" {
			return fmt.Errorf("date es requerido")
		}
		if expense.Category == "" {
			return fmt.Errorf("category es requerido")
		}
		if expense.PaymentMethod != "cash" && expense.PaymentMethod != "bank" {
			return fmt.Errorf("payment_method debe ser cash o bank")
		}
	}

	return nil
}
