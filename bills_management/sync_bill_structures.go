package main

import (
	"time"
)

// Estructuras de datos para sincronización offline de facturas
// Permite la sincronización bidireccional entre cliente offline y servidor
// Basado en el patrón exitoso implementado en expense_management

// SyncBillBatchRequest representa una solicitud de sincronización por lotes para facturas
// Contiene todas las operaciones offline realizadas por el cliente relacionadas con facturas
type SyncBillBatchRequest struct {
	UserID     string        `json:"user_id"`     // ID del usuario que realiza la sincronización
	Bills      []OfflineBill `json:"bills"`       // Lista de facturas modificadas offline
	LastSync   string        `json:"last_sync"`   // Timestamp del último sync exitoso
	ClientID   string        `json:"client_id"`   // ID único del cliente para evitar duplicados
	DeviceInfo string        `json:"device_info"` // Información del dispositivo (opcional)
	AppVersion string        `json:"app_version"` // Versión de la app cliente
}

// OfflineBill representa una factura modificada offline
// Incluye información completa para detección y resolución de conflictos
// Mantiene compatibilidad con la estructura Bill existente
type OfflineBill struct {
	ID               string  `json:"id"`                // ID de la factura (puede ser local para nuevas)
	LocalID          string  `json:"local_id"`          // ID local único en el dispositivo
	ServerID         int     `json:"server_id"`         // ID en el servidor (0 para nuevas facturas)
	Action           string  `json:"action"`            // "add", "update", "delete"
	UserID           string  `json:"user_id"`           // ID del usuario propietario
	Name             string  `json:"name"`              // Nombre de la factura
	Amount           float64 `json:"amount"`            // Cantidad de la factura
	DueDate          string  `json:"due_date"`          // Fecha de vencimiento
	StartDate        string  `json:"start_date"`        // Fecha de inicio
	PaymentDay       int     `json:"payment_day"`       // Día de pago mensual
	DurationMonths   int     `json:"duration_months"`   // Duración en meses
	Regularity       string  `json:"regularity"`        // Regularidad de la factura
	Paid             bool    `json:"paid"`              // Estado de pago
	Overdue          bool    `json:"overdue"`           // Estado de atraso
	OverdueDays      int     `json:"overdue_days"`      // Días de atraso
	Recurring        bool    `json:"recurring"`         // Factura recurrente
	Category         string  `json:"category"`          // Categoría de la factura
	Icon             string  `json:"icon"`              // Icono asociado
	PaymentMethod    string  `json:"payment_method"`    // "cash" o "bank"
	OfflineTimestamp string  `json:"offline_timestamp"` // Timestamp cuando se realizó offline
	SyncTimestamp    string  `json:"sync_timestamp"`    // Timestamp para sincronización
	Status           string  `json:"status"`            // "pending", "synced", "conflict"
	Version          int     `json:"version"`           // Versión para control de concurrencia
}

// SyncBillBatchResponse representa la respuesta del servidor tras sincronización de facturas
// Incluye resultados, conflictos detectados y datos actualizados del servidor
type SyncBillBatchResponse struct {
	Success       bool                     `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string                   `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                      `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                      `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                      `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncBillResult         `json:"results"`        // Resultado detallado por operación
	Conflicts     []BillConflictResolution `json:"conflicts"`      // Conflictos detectados
	ServerData    []Bill                   `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string                   `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string                   `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncBillResult representa el resultado de sincronización de una operación individual de factura
// Proporciona feedback detallado sobre cada factura procesada
type SyncBillResult struct {
	LocalID        string `json:"local_id"`                // ID local de la factura
	ServerID       string `json:"server_id"`               // ID asignado en el servidor
	Action         string `json:"action"`                  // Acción realizada
	Status         string `json:"status"`                  // "success", "error", "conflict"
	Error          string `json:"error,omitempty"`         // Mensaje de error si aplica
	ConflictType   string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction bool   `json:"requires_action"`         // Si requiere acción del usuario
	SyncTimestamp  string `json:"sync_timestamp"`          // Timestamp de sincronización
}

// BillConflictResolution representa un conflicto detectado durante sincronización de facturas
// Proporciona información para resolución manual o automática
type BillConflictResolution struct {
	LocalID      string   `json:"local_id"`      // ID local de la factura en conflicto
	ServerID     int      `json:"server_id"`     // ID de la factura en el servidor
	ConflictType string   `json:"conflict_type"` // "version", "timestamp", "data"
	LocalData    Bill     `json:"local_data"`    // Datos del cliente
	ServerData   Bill     `json:"server_data"`   // Datos del servidor
	Resolution   string   `json:"resolution"`    // "manual", "server_wins", "client_wins"
	Priority     string   `json:"priority"`      // "high", "medium", "low"
	Description  string   `json:"description"`   // Descripción del conflicto
	Suggestions  []string `json:"suggestions"`   // Sugerencias de resolución
}

// SyncBillChangesRequest solicita cambios de facturas del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
type SyncBillChangesRequest struct {
	UserID   string `json:"user_id"`   // ID del usuario
	LastSync string `json:"last_sync"` // Timestamp del último sync
	Limit    int    `json:"limit"`     // Límite de registros (opcional)
	Offset   int    `json:"offset"`    // Offset para paginación (opcional)
}

// SyncBillChangesResponse contiene cambios de facturas del servidor para sincronización
// Permite al cliente actualizar su base de datos local
type SyncBillChangesResponse struct {
	Success      bool   `json:"success"`       // Éxito de la operación
	Message      string `json:"message"`       // Mensaje descriptivo
	Changes      []Bill `json:"changes"`       // Facturas modificadas en el servidor
	Deletions    []int  `json:"deletions"`     // IDs de facturas eliminadas
	HasMore      bool   `json:"has_more"`      // Indica si hay más cambios
	TotalChanges int    `json:"total_changes"` // Total de cambios disponibles
	LastSync     string `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime   string `json:"server_time"`   // Timestamp actual del servidor
}

// SyncBillConflictRequest solicita resolución de un conflicto específico de factura
// Permite al cliente indicar cómo resolver conflictos detectados
type SyncBillConflictRequest struct {
	UserID       string `json:"user_id"`               // ID del usuario
	LocalID      string `json:"local_id"`              // ID local de la factura
	ServerID     int    `json:"server_id"`             // ID del servidor
	Resolution   string `json:"resolution"`            // "server_wins", "client_wins", "merge"
	MergedData   Bill   `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice string `json:"client_choice"`         // Elección específica del usuario
}

// SyncBillStats representa estadísticas de sincronización de facturas
// Proporciona métricas útiles para monitoreo y optimización específicas de bills
type SyncBillStats struct {
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

// SyncBillConfig almacena configuración para el sistema de sincronización de facturas
// Permite ajustar comportamiento según necesidades específicas del sistema de bills
type SyncBillConfig struct {
	MaxBatchSize        int             `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution  string          `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes int             `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int             `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds      int             `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled  bool            `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled   bool            `json:"encryption_enabled"`    // Habilitar encriptación de datos
	BillSpecificOptions BillSyncOptions `json:"bill_options"`          // Opciones específicas para facturas
}

// BillSyncOptions contiene opciones específicas para sincronización de facturas
// Personaliza el comportamiento de sync para las características únicas de bills
type BillSyncOptions struct {
	SyncPaymentStatus   bool `json:"sync_payment_status"`   // Sincronizar estados de pago
	SyncRecurrenceData  bool `json:"sync_recurrence_data"`  // Sincronizar datos de recurrencia
	HandleOverdueStatus bool `json:"handle_overdue_status"` // Manejar estados de atraso
	AutoResolvePayments bool `json:"auto_resolve_payments"` // Resolver automáticamente conflictos de pago
}

// New operation-based sync structures for bills management

// SyncBillOperationChangesRequest solicita operaciones de facturas desde operation_id
// Utiliza el nuevo sistema de operation_id para sincronización incremental
type SyncBillOperationChangesRequest struct {
	UserID          string `json:"user_id"`           // ID del usuario
	LastOperationId string `json:"last_operation_id"` // Último operation_id procesado (puede ser null)
	Limit           int    `json:"limit"`             // Límite de operaciones a retornar
	Offset          int    `json:"offset"`            // Offset para paginación
}

// SyncBillOperationChangesResponse contiene operaciones de facturas para sincronización
// Compatible con el sistema operation_id-based usado por delta_sync
type SyncBillOperationChangesResponse struct {
	Success       bool                `json:"success"`        // Éxito de la operación
	Message       string              `json:"message"`        // Mensaje descriptivo
	Operations    []BillSyncOperation `json:"operations"`     // Operaciones de facturas
	HasMore       bool                `json:"has_more"`       // Indica si hay más operaciones
	TotalCount    int                 `json:"total_count"`    // Total de operaciones disponibles
	LastOperation string              `json:"last_operation"` // Último operation_id incluido
	ServerTime    string              `json:"server_time"`    // Timestamp actual del servidor
}

// BillSyncOperation representa una operación de sincronización para facturas
// Estructura compatible con delta_sync para mantener consistencia
type BillSyncOperation struct {
	ID              int    `json:"id"`               // ID único de la operación
	UserID          string `json:"user_id"`          // ID del usuario
	OperationID     string `json:"operation_id"`     // Operation ID timestamp-based
	OperationType   string `json:"operation_type"`   // "create", "update", "delete", "pay"
	EntityType      string `json:"entity_type"`      // "bills", "bill_payments"
	EntityID        string `json:"entity_id"`        // ID de la entidad afectada
	OperationData   string `json:"operation_data"`   // Datos JSON de la operación
	DeviceIDs       string `json:"device_ids"`       // JSON array de device IDs
	ClientTimestamp int64  `json:"client_timestamp"` // Timestamp del cliente
	ServerTimestamp int64  `json:"server_timestamp"` // Timestamp del servidor
	CreatedAt       int64  `json:"created_at"`       // Timestamp extraído del operation_id
}
