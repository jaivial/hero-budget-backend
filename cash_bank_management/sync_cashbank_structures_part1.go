package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de Cash Bank Management - Parte 1
// Permite la sincronización bidireccional entre cliente offline y servidor
// Basado en el patrón exitoso implementado en budget_management, bills_management y expense_management
// Adaptado específicamente para las operaciones de efectivo y banco

// SyncCashBankBatchRequest representa una solicitud de sincronización por lotes para Cash Bank
// Contiene todas las operaciones offline realizadas por el cliente relacionadas con efectivo/banco
// Incluye distribuciones, transferencias y actualizaciones de cantidades
type SyncCashBankBatchRequest struct {
	UserID         string                      `json:"user_id"`         // ID del usuario que realiza la sincronización
	Distributions  []OfflineCashBankDistribution `json:"distributions"`  // Lista de distribuciones modificadas offline
	Transfers      []OfflineCashBankTransfer    `json:"transfers"`      // Lista de transferencias realizadas offline
	LastSync       string                      `json:"last_sync"`       // Timestamp del último sync exitoso
	ClientID       string                      `json:"client_id"`       // ID único del cliente para evitar duplicados
	DeviceInfo     string                      `json:"device_info"`     // Información del dispositivo (opcional)
	AppVersion     string                      `json:"app_version"`     // Versión de la app cliente
}

// OfflineCashBankDistribution representa datos de distribución efectivo/banco modificados offline
// Incluye información completa para detección y resolución de conflictos
// Mantiene compatibilidad con la estructura CashBankDistribution existente
type OfflineCashBankDistribution struct {
	ID               string  `json:"id"`                  // ID de la distribución (puede ser local para nuevos)
	LocalID          string  `json:"local_id"`            // ID local único en el dispositivo
	ServerID         string  `json:"server_id"`           // ID en el servidor (vacío para nuevos registros)
	Action           string  `json:"action"`              // "add", "update", "delete"
	UserID           string  `json:"user_id"`             // ID del usuario propietario
	Month            string  `json:"month"`               // Mes de la distribución
	CashAmount       float64 `json:"cash_amount"`         // Cantidad en efectivo
	CashPercent      float64 `json:"cash_percent"`        // Porcentaje de efectivo
	BankAmount       float64 `json:"bank_amount"`         // Cantidad en banco
	BankPercent      float64 `json:"bank_percent"`        // Porcentaje bancario
	MonthlyTotal     float64 `json:"monthly_total"`       // Total mensual combinado
	OfflineTimestamp string  `json:"offline_timestamp"`   // Timestamp cuando se realizó offline
	SyncTimestamp    string  `json:"sync_timestamp"`      // Timestamp para sincronización
	Status           string  `json:"status"`              // "pending", "synced", "conflict"
	Version          int     `json:"version"`             // Versión para control de concurrencia
}

// OfflineCashBankTransfer representa transferencias efectivo/banco realizadas offline
// Incluye tanto transferencias de efectivo a banco como de banco a efectivo
// Mantiene integridad referencial con las distribuciones afectadas
type OfflineCashBankTransfer struct {
	ID               string  `json:"id"`                  // ID de la transferencia
	LocalID          string  `json:"local_id"`            // ID local único en el dispositivo
	ServerID         string  `json:"server_id"`           // ID en el servidor
	Action           string  `json:"action"`              // "add", "update", "delete"
	UserID           string  `json:"user_id"`             // ID del usuario
	TransferType     string  `json:"transfer_type"`       // "cash_to_bank", "bank_to_cash"
	Amount           float64 `json:"amount"`              // Cantidad transferida
	Date             string  `json:"date"`                // Fecha de la transferencia
	OfflineTimestamp string  `json:"offline_timestamp"`   // Timestamp offline
	SyncTimestamp    string  `json:"sync_timestamp"`      // Timestamp de sync
	Status           string  `json:"status"`              // Estado de sincronización
	Version          int     `json:"version"`             // Versión para control
}

// SyncCashBankBatchResponse representa la respuesta del servidor tras sincronización
// Incluye resultados, conflictos detectados y datos actualizados del servidor
// Proporciona feedback detallado sobre cada operación procesada
type SyncCashBankBatchResponse struct {
	Success       bool                              `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string                            `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                               `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                               `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                               `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncCashBankResult              `json:"results"`        // Resultado detallado por operación
	Conflicts     []CashBankConflictResolution      `json:"conflicts"`      // Conflictos detectados
	ServerData    []CashBankDistribution            `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string                            `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string                            `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncCashBankResult representa el resultado de sincronización de una operación individual
// Proporciona feedback detallado sobre cada distribución o transferencia procesada
// Incluye información sobre conflictos y acciones requeridas
type SyncCashBankResult struct {
	LocalID         string `json:"local_id"`         // ID local de la operación
	ServerID        string `json:"server_id"`        // ID asignado en el servidor
	Action          string `json:"action"`           // Acción realizada
	OperationType   string `json:"operation_type"`   // "distribution", "transfer"
	Status          string `json:"status"`           // "success", "error", "conflict"
	Error           string `json:"error,omitempty"`  // Mensaje de error si aplica
	ConflictType    string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction  bool   `json:"requires_action"`  // Si requiere acción del usuario
	SyncTimestamp   string `json:"sync_timestamp"`   // Timestamp de sincronización
}

// CashBankConflictResolution representa un conflicto detectado durante sincronización
// Proporciona información para resolución manual o automática de conflictos
// Incluye datos tanto del cliente como del servidor para comparación
type CashBankConflictResolution struct {
	LocalID       string                    `json:"local_id"`       // ID local del registro en conflicto
	ServerID      string                    `json:"server_id"`      // ID del registro en el servidor
	ConflictType  string                    `json:"conflict_type"`  // "version", "timestamp", "data", "balance"
	OperationType string                    `json:"operation_type"` // "distribution", "transfer"
	LocalData     interface{}               `json:"local_data"`     // Datos del cliente (Distribution o Transfer)
	ServerData    interface{}               `json:"server_data"`    // Datos del servidor
	Resolution    string                    `json:"resolution"`     // "manual", "server_wins", "client_wins"
	Priority      string                    `json:"priority"`       // "high", "medium", "low"
	Description   string                    `json:"description"`    // Descripción del conflicto
	Suggestions   []string                  `json:"suggestions"`    // Sugerencias de resolución
}

// SyncCashBankChangesRequest solicita cambios del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
// Útil para sincronización unidireccional (solo descarga)
type SyncCashBankChangesRequest struct {
	UserID    string `json:"user_id"`    // ID del usuario
	LastSync  string `json:"last_sync"`  // Timestamp del último sync
	Limit     int    `json:"limit"`      // Límite de registros (opcional)
	Offset    int    `json:"offset"`     // Offset para paginación (opcional)
}

// SyncCashBankChangesResponse contiene cambios del servidor para sincronización
// Permite al cliente actualizar su base de datos local con cambios del servidor
// Incluye tanto distribuciones como transferencias modificadas
type SyncCashBankChangesResponse struct {
	Success       bool                    `json:"success"`       // Éxito de la operación
	Message       string                  `json:"message"`       // Mensaje descriptivo
	Distributions []CashBankDistribution  `json:"distributions"` // Distribuciones modificadas en el servidor
	Transfers     []CashBankTransfer      `json:"transfers"`     // Transferencias modificadas en el servidor
	Deletions     []string                `json:"deletions"`     // IDs de registros eliminados
	HasMore       bool                    `json:"has_more"`      // Indica si hay más cambios
	TotalChanges  int                     `json:"total_changes"` // Total de cambios disponibles
	LastSync      string                  `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime    string                  `json:"server_time"`   // Timestamp actual del servidor
}

// CashBankTransfer representa una transferencia entre efectivo y banco
// Utilizada en respuestas de sincronización para mantener histórico completo
// Complementa la información de distribuciones con detalles de transferencias
type CashBankTransfer struct {
	ID           string    `json:"id"`            // ID único de la transferencia
	UserID       string    `json:"user_id"`       // ID del usuario
	TransferType string    `json:"transfer_type"` // "cash_to_bank", "bank_to_cash"
	Amount       float64   `json:"amount"`        // Cantidad transferida
	Date         string    `json:"date"`          // Fecha de la transferencia
	CreatedAt    time.Time `json:"created_at"`    // Timestamp de creación
	UpdatedAt    time.Time `json:"updated_at"`    // Timestamp de última actualización
}

// SyncCashBankConflictRequest solicita resolución de un conflicto específico
// Permite al cliente indicar cómo resolver conflictos detectados durante sync
// Incluye opciones para resolución automática o datos fusionados manualmente
type SyncCashBankConflictRequest struct {
	UserID         string      `json:"user_id"`         // ID del usuario
	LocalID        string      `json:"local_id"`        // ID local del registro
	ServerID       string      `json:"server_id"`       // ID del servidor
	OperationType  string      `json:"operation_type"`  // "distribution", "transfer"
	Resolution     string      `json:"resolution"`      // "server_wins", "client_wins", "merge"
	MergedData     interface{} `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice   string      `json:"client_choice"`   // Elección específica del usuario
}

// SyncCashBankStats representa estadísticas de sincronización de Cash Bank
// Proporciona métricas útiles para monitoreo y optimización específicas de efectivo/banco
// Incluye información sobre rendimiento y errores de sincronización
type SyncCashBankStats struct {
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
	
	// Estadísticas específicas de Cash Bank
	DistributionSyncs int     `json:"distribution_syncs"`  // Sincronizaciones de distribución
	TransferSyncs     int     `json:"transfer_syncs"`      // Sincronizaciones de transferencia
	TotalCashSynced   float64 `json:"total_cash_synced"`   // Total de efectivo sincronizado
	TotalBankSynced   float64 `json:"total_bank_synced"`   // Total bancario sincronizado
}

// SyncCashBankConfig almacena configuración para el sistema de sincronización
// Permite ajustar comportamiento según necesidades específicas del sistema de efectivo/banco
// Incluye configuraciones específicas para operaciones de cash bank
type SyncCashBankConfig struct {
	MaxBatchSize        int                     `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution  string                  `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes int                     `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int                     `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds      int                     `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled  bool                    `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled   bool                    `json:"encryption_enabled"`    // Habilitar encriptación de datos
	CashBankOptions     CashBankSyncOptions     `json:"cashbank_options"`      // Opciones específicas para cash bank
}

// CashBankSyncOptions contiene opciones específicas para sincronización de cash bank
// Personaliza el comportamiento de sync para las características únicas de efectivo/banco
// Incluye configuraciones para validación de balances y resolución de conflictos
type CashBankSyncOptions struct {
	SyncCalculatedFields  bool `json:"sync_calculated_fields"`  // Sincronizar campos calculados (porcentajes)
	ValidateBalances      bool `json:"validate_balances"`       // Validar balances durante sync
	HandleBalanceConflicts bool `json:"handle_balance_conflicts"` // Manejar conflictos de balance
	AutoRecalculatePercents bool `json:"auto_recalculate_percents"` // Recalcular automáticamente porcentajes
	SyncTransferHistory   bool `json:"sync_transfer_history"`   // Sincronizar historial de transferencias
	ConsolidateTransfers  bool `json:"consolidate_transfers"`   // Consolidar transferencias múltiples
}

// Validate método básico para validar solicitud de sincronización
func (req *SyncCashBankBatchRequest) Validate() error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	return nil
}