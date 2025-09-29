package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de Transaction Delete Service
// Permite la sincronización bidireccional entre cliente offline y servidor para eliminación de transacciones
// Basado en el patrón exitoso implementado en otros servicios del sistema
// Adaptado específicamente para operaciones de eliminación segura de transacciones

// SyncTransactionDeleteBatchRequest representa una solicitud de sincronización por lotes para eliminación de transacciones
// Contiene todas las operaciones de eliminación offline realizadas por el cliente
// Incluye validaciones de integridad y detección de conflictos durante la eliminación
type SyncTransactionDeleteBatchRequest struct {
	UserID          string                       `json:"user_id"`          // ID del usuario que realiza la sincronización
	Deletions       []OfflineTransactionDeletion `json:"deletions"`        // Lista de transacciones eliminadas offline
	LastSync        string                       `json:"last_sync"`        // Timestamp del último sync exitoso
	ClientID        string                       `json:"client_id"`        // ID único del cliente para evitar duplicados
	DeviceInfo      string                       `json:"device_info"`      // Información del dispositivo (opcional)
	AppVersion      string                       `json:"app_version"`      // Versión de la app cliente
	ValidateBalance bool                         `json:"validate_balance"` // Indica si validar balance tras eliminación
}

// OfflineTransactionDeletion representa datos de transacciones eliminadas offline
// Incluye información completa para validación y detección de conflictos
// Mantiene referencia al estado original para verificación de integridad
type OfflineTransactionDeletion struct {
	ID                 string  `json:"id"`                   // ID de la eliminación (puede ser local)
	LocalID            string  `json:"local_id"`             // ID local único en el dispositivo
	ServerID           string  `json:"server_id"`            // ID en el servidor (vacío para nuevos)
	TransactionID      string  `json:"transaction_id"`       // ID de la transacción a eliminar
	TransactionLocalID string  `json:"transaction_local_id"` // ID local de la transacción
	Action             string  `json:"action"`               // Siempre "delete" para este servicio
	UserID             string  `json:"user_id"`              // ID del usuario propietario
	TransactionType    string  `json:"transaction_type"`     // Tipo de transacción eliminada
	OriginalAmount     float64 `json:"original_amount"`      // Cantidad original de la transacción
	OriginalDate       string  `json:"original_date"`        // Fecha original de la transacción
	DeletionReason     string  `json:"deletion_reason"`      // Razón de la eliminación (opcional)
	OfflineTimestamp   string  `json:"offline_timestamp"`    // Timestamp cuando se eliminó offline
	SyncTimestamp      string  `json:"sync_timestamp"`       // Timestamp para sincronización
	Status             string  `json:"status"`               // "pending", "synced", "conflict", "cancelled"
	Version            int     `json:"version"`              // Versión para control de concurrencia
	RequiresValidation bool    `json:"requires_validation"`  // Si requiere validación adicional
}

// SyncTransactionDeleteBatchResponse representa la respuesta del servidor tras sincronización de eliminaciones
// Incluye resultados, conflictos detectados y efectos en el balance
// Proporciona feedback detallado sobre cada operación de eliminación procesada
type SyncTransactionDeleteBatchResponse struct {
	Success          bool                                  `json:"success"`           // Indica si la sincronización fue exitosa
	Message          string                                `json:"message"`           // Mensaje descriptivo del resultado
	ProcessedOps     int                                   `json:"processed_ops"`     // Número de operaciones procesadas
	SuccessfulOps    int                                   `json:"successful_ops"`    // Operaciones exitosas
	FailedOps        int                                   `json:"failed_ops"`        // Operaciones fallidas
	Results          []SyncTransactionDeleteResult         `json:"results"`           // Resultado detallado por operación
	Conflicts        []TransactionDeleteConflictResolution `json:"conflicts"`         // Conflictos detectados
	BalanceImpacts   []TransactionDeleteBalanceImpact      `json:"balance_impacts"`   // Impactos en el balance
	LastSync         string                                `json:"last_sync"`         // Nuevo timestamp de sincronización
	NextSyncTime     string                                `json:"next_sync_time"`    // Sugerencia para próximo sync
	ValidationErrors []string                              `json:"validation_errors"` // Errores de validación detectados
}

// SyncTransactionDeleteResult representa el resultado de sincronización de una eliminación individual
// Proporciona feedback detallado sobre cada transacción eliminada
// Incluye información sobre efectos en el balance y acciones requeridas
type SyncTransactionDeleteResult struct {
	LocalID           string  `json:"local_id"`                // ID local de la operación
	ServerID          string  `json:"server_id"`               // ID asignado en el servidor
	TransactionID     string  `json:"transaction_id"`          // ID de la transacción eliminada
	Action            string  `json:"action"`                  // Acción realizada (delete)
	Status            string  `json:"status"`                  // "success", "error", "conflict", "cancelled"
	Error             string  `json:"error,omitempty"`         // Mensaje de error si aplica
	ConflictType      string  `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction    bool    `json:"requires_action"`         // Si requiere acción del usuario
	SyncTimestamp     string  `json:"sync_timestamp"`          // Timestamp de sincronización
	BalanceAdjustment float64 `json:"balance_adjustment"`      // Ajuste realizado al balance
	ValidationPassed  bool    `json:"validation_passed"`       // Si pasó las validaciones
}

// TransactionDeleteConflictResolution representa un conflicto detectado durante eliminación
// Proporciona información para resolución manual o automática de conflictos
// Incluye datos de la transacción original para verificación
type TransactionDeleteConflictResolution struct {
	LocalID              string      `json:"local_id"`               // ID local del registro en conflicto
	ServerID             string      `json:"server_id"`              // ID del registro en el servidor
	TransactionID        string      `json:"transaction_id"`         // ID de la transacción en conflicto
	ConflictType         string      `json:"conflict_type"`          // "already_deleted", "modified", "not_found", "balance_conflict"
	LocalData            interface{} `json:"local_data"`             // Datos del cliente
	ServerData           interface{} `json:"server_data"`            // Datos del servidor
	Resolution           string      `json:"resolution"`             // "manual", "server_wins", "client_wins", "cancel_deletion"
	Priority             string      `json:"priority"`               // "high", "medium", "low"
	Description          string      `json:"description"`            // Descripción del conflicto
	Suggestions          []string    `json:"suggestions"`            // Sugerencias de resolución
	BalanceImpact        float64     `json:"balance_impact"`         // Impacto potencial en el balance
	RequiresUserDecision bool        `json:"requires_user_decision"` // Si requiere decisión del usuario
}

// TransactionDeleteBalanceImpact representa el impacto de una eliminación en el balance
// Ayuda a comprender los efectos financieros de la eliminación de transacciones
// Incluye cálculos antes y después para verificación de integridad
type TransactionDeleteBalanceImpact struct {
	TransactionID         string  `json:"transaction_id"`         // ID de la transacción eliminada
	TransactionType       string  `json:"transaction_type"`       // Tipo de transacción eliminada
	Amount                float64 `json:"amount"`                 // Cantidad de la transacción eliminada
	BalanceBefore         float64 `json:"balance_before"`         // Balance antes de la eliminación
	BalanceAfter          float64 `json:"balance_after"`          // Balance después de la eliminación
	AdjustmentMade        float64 `json:"adjustment_made"`        // Ajuste realizado al balance
	CashImpact            float64 `json:"cash_impact"`            // Impacto en efectivo
	BankImpact            float64 `json:"bank_impact"`            // Impacto en banco
	RequiresRecalculation bool    `json:"requires_recalculation"` // Si requiere recálculo de porcentajes
}

// SyncTransactionDeleteChangesRequest solicita cambios del servidor relacionados con eliminaciones
// Permite obtener actualizaciones de eliminaciones sin enviar datos del cliente
// Útil para sincronización unidireccional de eliminaciones realizadas en otros dispositivos
type SyncTransactionDeleteChangesRequest struct {
	UserID                string `json:"user_id"`                 // ID del usuario
	LastSync              string `json:"last_sync"`               // Timestamp del último sync
	Limit                 int    `json:"limit"`                   // Límite de registros (opcional)
	Offset                int    `json:"offset"`                  // Offset para paginación (opcional)
	IncludeBalanceImpacts bool   `json:"include_balance_impacts"` // Incluir impactos en balance
}

// SyncTransactionDeleteChangesResponse contiene cambios del servidor para sincronización de eliminaciones
// Permite al cliente actualizar su base de datos local con eliminaciones del servidor
// Incluye información sobre transacciones eliminadas e impactos en balance
type SyncTransactionDeleteChangesResponse struct {
	Success        bool                             `json:"success"`         // Éxito de la operación
	Message        string                           `json:"message"`         // Mensaje descriptivo
	Deletions      []TransactionDeletionRecord      `json:"deletions"`       // Eliminaciones realizadas en el servidor
	BalanceImpacts []TransactionDeleteBalanceImpact `json:"balance_impacts"` // Impactos en balance de eliminaciones
	Restorations   []string                         `json:"restorations"`    // IDs de transacciones restauradas
	HasMore        bool                             `json:"has_more"`        // Indica si hay más cambios
	TotalChanges   int                              `json:"total_changes"`   // Total de cambios disponibles
	LastSync       string                           `json:"last_sync"`       // Nuevo timestamp de sync
	ServerTime     string                           `json:"server_time"`     // Timestamp actual del servidor
}

// TransactionDeletionRecord representa un registro de eliminación de transacción
// Utilizado en respuestas de sincronización para mantener histórico completo
// Incluye metadatos sobre la eliminación para auditoría
type TransactionDeletionRecord struct {
	ID              string    `json:"id"`               // ID único del registro de eliminación
	UserID          string    `json:"user_id"`          // ID del usuario
	TransactionID   string    `json:"transaction_id"`   // ID de la transacción eliminada
	TransactionType string    `json:"transaction_type"` // Tipo de transacción eliminada
	OriginalAmount  float64   `json:"original_amount"`  // Cantidad original
	OriginalDate    string    `json:"original_date"`    // Fecha original
	DeletionReason  string    `json:"deletion_reason"`  // Razón de eliminación
	DeletedAt       time.Time `json:"deleted_at"`       // Timestamp de eliminación
	DeletedBy       string    `json:"deleted_by"`       // Usuario o dispositivo que eliminó
	BalanceAdjusted bool      `json:"balance_adjusted"` // Si se ajustó el balance
	CanBeRestored   bool      `json:"can_be_restored"`  // Si puede ser restaurada
}

// SyncTransactionDeleteConflictRequest solicita resolución de un conflicto específico de eliminación
// Permite al cliente indicar cómo resolver conflictos detectados durante eliminaciones
// Incluye opciones para cancelar eliminación o proceder con ajustes
type SyncTransactionDeleteConflictRequest struct {
	UserID          string   `json:"user_id"`                    // ID del usuario
	LocalID         string   `json:"local_id"`                   // ID local del registro
	ServerID        string   `json:"server_id"`                  // ID del servidor
	TransactionID   string   `json:"transaction_id"`             // ID de la transacción
	Resolution      string   `json:"resolution"`                 // "proceed", "cancel", "restore", "force_delete"
	ForceBalance    bool     `json:"force_balance"`              // Forzar ajuste de balance
	UserDecision    string   `json:"user_decision"`              // Decisión específica del usuario
	BalanceOverride *float64 `json:"balance_override,omitempty"` // Sobrescribir balance (si aplica)
}

// SyncTransactionDeleteStats representa estadísticas de sincronización de eliminaciones
// Proporciona métricas útiles para monitoreo y optimización específicas de eliminaciones
// Incluye información sobre rendimiento y errores de sincronización de eliminaciones
type SyncTransactionDeleteStats struct {
	UserID            string     `json:"user_id"`                   // ID del usuario
	LastSyncTime      time.Time  `json:"last_sync_time"`            // Última sincronización exitosa
	TotalSyncs        int        `json:"total_syncs"`               // Total de sincronizaciones
	PendingDeletions  int        `json:"pending_deletions"`         // Eliminaciones pendientes
	ConflictsResolved int        `json:"conflicts_resolved"`        // Conflictos resueltos
	DataSizeBytes     int64      `json:"data_size_bytes"`           // Tamaño de datos sincronizados
	AverageLatency    float64    `json:"avg_latency_ms"`            // Latencia promedio en ms
	ErrorCount        int        `json:"error_count"`               // Número de errores
	LastErrorTime     *time.Time `json:"last_error_time,omitempty"` // Último error
	LastErrorMessage  string     `json:"last_error_msg,omitempty"`  // Mensaje del último error

	// Estadísticas específicas de eliminación de transacciones
	TotalDeletions       int     `json:"total_deletions"`       // Total de eliminaciones sincronizadas
	CancelledDeletions   int     `json:"cancelled_deletions"`   // Eliminaciones canceladas
	RestoredTransactions int     `json:"restored_transactions"` // Transacciones restauradas
	BalanceAdjustments   int     `json:"balance_adjustments"`   // Ajustes de balance realizados
	TotalAmountDeleted   float64 `json:"total_amount_deleted"`  // Cantidad total eliminada
	AverageConflictTime  float64 `json:"avg_conflict_time_ms"`  // Tiempo promedio de resolución de conflictos
}

// SyncTransactionDeleteConfig almacena configuración para el sistema de sincronización de eliminaciones
// Permite ajustar comportamiento según necesidades específicas del sistema de eliminaciones
// Incluye configuraciones de seguridad y validación específicas para eliminaciones
type SyncTransactionDeleteConfig struct {
	MaxBatchSize        int                          `json:"max_batch_size"`        // Máximo número de eliminaciones por lote
	ConflictResolution  string                       `json:"conflict_resolution"`   // Estrategia predeterminada
	SyncIntervalMinutes int                          `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int                          `json:"retry_attempts"`        // Intentos de reintento
	TimeoutSeconds      int                          `json:"timeout_seconds"`       // Timeout para operaciones
	RequireValidation   bool                         `json:"require_validation"`    // Requerir validación de balance
	AllowForceDelete    bool                         `json:"allow_force_delete"`    // Permitir eliminación forzada
	DeleteOptions       TransactionDeleteSyncOptions `json:"delete_options"`        // Opciones específicas de eliminación
}

// TransactionDeleteSyncOptions contiene opciones específicas para sincronización de eliminaciones
// Personaliza el comportamiento de sync para las características únicas de eliminaciones
// Incluye configuraciones de seguridad y auditoría específicas
type TransactionDeleteSyncOptions struct {
	ValidateBalanceImpact    bool `json:"validate_balance_impact"`     // Validar impacto en balance
	RequireDeletionReason    bool `json:"require_deletion_reason"`     // Requerir razón de eliminación
	AllowBulkDelete          bool `json:"allow_bulk_delete"`           // Permitir eliminación masiva
	AutoAdjustBalance        bool `json:"auto_adjust_balance"`         // Ajustar balance automáticamente
	CreateDeletionAuditTrail bool `json:"create_deletion_audit_trail"` // Crear rastro de auditoría
	EnableDeletionRecovery   bool `json:"enable_deletion_recovery"`    // Habilitar recuperación de eliminaciones
	MaxDeletionsPerBatch     int  `json:"max_deletions_per_batch"`     // Máximo eliminaciones por lote
	ConfirmCriticalDeletions bool `json:"confirm_critical_deletions"`  // Confirmar eliminaciones críticas
}

// Validate método básico para validar solicitud de sincronización de eliminaciones
// Verifica que todos los campos requeridos estén presentes y sean válidos
// Incluye validaciones específicas para operaciones de eliminación
func (req *SyncTransactionDeleteBatchRequest) Validate() error {
	// Validar campos básicos requeridos
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}

	// Validar que hay eliminaciones para procesar
	if len(req.Deletions) == 0 {
		return fmt.Errorf("at least one deletion is required")
	}

	// Validar cada eliminación individual
	for i, deletion := range req.Deletions {
		if deletion.TransactionID == "" && deletion.TransactionLocalID == "" {
			return fmt.Errorf("deletion at index %d requires transaction_id or transaction_local_id", i)
		}
		if deletion.Action != "delete" {
			return fmt.Errorf("deletion at index %d has invalid action: %s (must be 'delete')", i, deletion.Action)
		}
		if deletion.UserID != req.UserID {
			return fmt.Errorf("deletion at index %d has mismatched user_id", i)
		}
	}

	return nil
}
