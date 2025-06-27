package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de presupuestos
// Permite la sincronización bidireccional entre cliente offline y servidor
// Basado en el patrón exitoso implementado en bills_management y expense_management

// SyncBudgetBatchRequest representa una solicitud de sincronización por lotes para presupuestos
// Contiene todas las operaciones offline realizadas por el cliente relacionadas con presupuestos
type SyncBudgetBatchRequest struct {
	UserID         string             `json:"user_id"`         // ID del usuario que realiza la sincronización
	Budgets        []OfflineBudget    `json:"budgets"`         // Lista de presupuestos modificados offline
	LastSync       string             `json:"last_sync"`       // Timestamp del último sync exitoso
	ClientID       string             `json:"client_id"`       // ID único del cliente para evitar duplicados
	DeviceInfo     string             `json:"device_info"`     // Información del dispositivo (opcional)
	AppVersion     string             `json:"app_version"`     // Versión de la app cliente
}

// OfflineBudget representa un presupuesto modificado offline
// Incluye información completa para detección y resolución de conflictos
// Mantiene compatibilidad con la estructura BudgetData existente
type OfflineBudget struct {
	ID               string  `json:"id"`                  // ID del presupuesto (puede ser local para nuevos)
	LocalID          string  `json:"local_id"`            // ID local único en el dispositivo
	ServerID         string  `json:"server_id"`           // ID en el servidor (vacío para nuevos presupuestos)
	Action           string  `json:"action"`              // "add", "update", "delete"
	UserID           string  `json:"user_id"`             // ID del usuario propietario
	Period           string  `json:"period"`              // Período del presupuesto
	Date             string  `json:"date"`                // Fecha del presupuesto
	TotalAmount      float64 `json:"total_amount"`        // Monto total disponible
	RemainingAmount  float64 `json:"remaining_amount"`    // Monto restante disponible
	SpentAmount      float64 `json:"spent_amount"`        // Monto ya gastado
	UpcomingAmount   float64 `json:"upcoming_amount"`     // Monto comprometido en gastos futuros
	FromPrevious     float64 `json:"from_previous"`       // Monto heredado del período anterior
	Percent          float64 `json:"percent"`             // Porcentaje de presupuesto utilizado
	TotalIncome      float64 `json:"total_income"`        // Total de ingresos del período
	OfflineTimestamp string  `json:"offline_timestamp"`   // Timestamp cuando se realizó offline
	SyncTimestamp    string  `json:"sync_timestamp"`      // Timestamp para sincronización
	Status           string  `json:"status"`              // "pending", "synced", "conflict"
	Version          int     `json:"version"`             // Versión para control de concurrencia
}

// SyncBudgetBatchResponse representa la respuesta del servidor tras sincronización de presupuestos
// Incluye resultados, conflictos detectados y datos actualizados del servidor
type SyncBudgetBatchResponse struct {
	Success       bool                         `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string                       `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                          `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                          `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                          `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncBudgetResult           `json:"results"`        // Resultado detallado por operación
	Conflicts     []BudgetConflictResolution   `json:"conflicts"`      // Conflictos detectados
	ServerData    []BudgetData                 `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string                       `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string                       `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncBudgetResult representa el resultado de sincronización de una operación individual de presupuesto
// Proporciona feedback detallado sobre cada presupuesto procesado
type SyncBudgetResult struct {
	LocalID         string `json:"local_id"`         // ID local del presupuesto
	ServerID        string `json:"server_id"`        // ID asignado en el servidor
	Action          string `json:"action"`           // Acción realizada
	Status          string `json:"status"`           // "success", "error", "conflict"
	Error           string `json:"error,omitempty"`  // Mensaje de error si aplica
	ConflictType    string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction  bool   `json:"requires_action"`  // Si requiere acción del usuario
	SyncTimestamp   string `json:"sync_timestamp"`   // Timestamp de sincronización
}

// BudgetConflictResolution representa un conflicto detectado durante sincronización de presupuestos
// Proporciona información para resolución manual o automática
type BudgetConflictResolution struct {
	LocalID       string     `json:"local_id"`       // ID local del presupuesto en conflicto
	ServerID      string     `json:"server_id"`      // ID del presupuesto en el servidor
	ConflictType  string     `json:"conflict_type"`  // "version", "timestamp", "data"
	LocalData     BudgetData `json:"local_data"`     // Datos del cliente
	ServerData    BudgetData `json:"server_data"`    // Datos del servidor
	Resolution    string     `json:"resolution"`     // "manual", "server_wins", "client_wins"
	Priority      string     `json:"priority"`       // "high", "medium", "low"
	Description   string     `json:"description"`    // Descripción del conflicto
	Suggestions   []string   `json:"suggestions"`    // Sugerencias de resolución
}

// SyncBudgetChangesRequest solicita cambios de presupuestos del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
type SyncBudgetChangesRequest struct {
	UserID    string `json:"user_id"`    // ID del usuario
	LastSync  string `json:"last_sync"`  // Timestamp del último sync
	Limit     int    `json:"limit"`      // Límite de registros (opcional)
	Offset    int    `json:"offset"`     // Offset para paginación (opcional)
}

// SyncBudgetChangesResponse contiene cambios de presupuestos del servidor para sincronización
// Permite al cliente actualizar su base de datos local
type SyncBudgetChangesResponse struct {
	Success      bool         `json:"success"`       // Éxito de la operación
	Message      string       `json:"message"`       // Mensaje descriptivo
	Changes      []BudgetData `json:"changes"`       // Presupuestos modificados en el servidor
	Deletions    []string     `json:"deletions"`     // IDs de presupuestos eliminados
	HasMore      bool         `json:"has_more"`      // Indica si hay más cambios
	TotalChanges int          `json:"total_changes"` // Total de cambios disponibles
	LastSync     string       `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime   string       `json:"server_time"`   // Timestamp actual del servidor
}

// SyncBudgetConflictRequest solicita resolución de un conflicto específico de presupuesto
// Permite al cliente indicar cómo resolver conflictos detectados
type SyncBudgetConflictRequest struct {
	UserID       string     `json:"user_id"`       // ID del usuario
	LocalID      string     `json:"local_id"`      // ID local del presupuesto
	ServerID     string     `json:"server_id"`     // ID del servidor
	Resolution   string     `json:"resolution"`    // "server_wins", "client_wins", "merge"
	MergedData   BudgetData `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice string     `json:"client_choice"` // Elección específica del usuario
}

// SyncBudgetStats representa estadísticas de sincronización de presupuestos
// Proporciona métricas útiles para monitoreo y optimización específicas de budgets
type SyncBudgetStats struct {
	UserID           string    `json:"user_id"`           // ID del usuario
	LastSyncTime     time.Time `json:"last_sync_time"`    // Última sincronización exitosa
	TotalSyncs       int       `json:"total_syncs"`       // Total de sincronizaciones
	PendingOperations int      `json:"pending_ops"`       // Operaciones pendientes
	ConflictsResolved int      `json:"conflicts_resolved"` // Conflictos resueltos
	DataSizeBytes    int64     `json:"data_size_bytes"`   // Tamaño de datos sincronizados
	AverageLatency   float64   `json:"avg_latency_ms"`    // Latencia promedio en ms
	ErrorCount       int       `json:"error_count"`       // Número de errores
	LastErrorTime    *time.Time `json:"last_error_time,omitempty"` // Último error
	LastErrorMessage string    `json:"last_error_msg,omitempty"`  // Mensaje del último error
}

// SyncBudgetConfig almacena configuración para el sistema de sincronización de presupuestos
// Permite ajustar comportamiento según necesidades específicas del sistema de budgets
type SyncBudgetConfig struct {
	MaxBatchSize        int                 `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution  string              `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes int                 `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int                 `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds      int                 `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled  bool                `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled   bool                `json:"encryption_enabled"`    // Habilitar encriptación de datos
	BudgetSpecificOptions BudgetSyncOptions `json:"budget_options"`        // Opciones específicas para presupuestos
}

// BudgetSyncOptions contiene opciones específicas para sincronización de presupuestos
// Personaliza el comportamiento de sync para las características únicas de budgets
type BudgetSyncOptions struct {
	SyncCalculatedFields  bool `json:"sync_calculated_fields"`  // Sincronizar campos calculados (porcentajes, etc.)
	SyncPeriodInheritance bool `json:"sync_period_inheritance"` // Sincronizar herencia entre períodos
	HandlePeriodOverlaps  bool `json:"handle_period_overlaps"`  // Manejar solapamientos de períodos
	AutoRecalculateAmounts bool `json:"auto_recalculate_amounts"` // Recalcular automáticamente montos derivados
}

// Validate valida la estructura de una solicitud de sincronización por lotes de presupuestos
// Asegura que todos los campos requeridos están presentes y son válidos
func (req *SyncBudgetBatchRequest) Validate() error {
	// Validar campos básicos requeridos
	if req.UserID == "" {
		return fmt.Errorf("user_id es requerido para sincronización de presupuestos")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id es requerido para evitar duplicados")
	}
	if len(req.Budgets) == 0 {
		return fmt.Errorf("no hay presupuestos para sincronizar")
	}
	
	// Validar límite de lote para evitar sobrecarga del servidor
	if len(req.Budgets) > 50 {
		return fmt.Errorf("el lote excede el límite máximo de 50 presupuestos")
	}
	
	// Validar cada presupuesto individual en el lote
	for i, budget := range req.Budgets {
		if err := budget.Validate(); err != nil {
			return fmt.Errorf("presupuesto %d inválido: %v", i, err)
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