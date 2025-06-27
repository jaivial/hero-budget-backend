package main

import (
	"fmt"
	"time"
)

// Estructuras de datos para sincronización offline de Categories Management
// Permite la sincronización bidireccional entre cliente offline y servidor
// Basado en el patrón exitoso implementado en cash_bank_management, bills_management y expense_management
// Adaptado específicamente para las operaciones de categorías de ingresos y gastos

// SyncCategoriesBatchRequest representa una solicitud de sincronización por lotes para Categories
// Contiene todas las operaciones offline realizadas por el cliente relacionadas con categorías
// Incluye creación, modificación y eliminación de categorías
type SyncCategoriesBatchRequest struct {
	UserID       string                `json:"user_id"`       // ID del usuario que realiza la sincronización
	Categories   []OfflineCategory     `json:"categories"`    // Lista de categorías modificadas offline
	LastSync     string                `json:"last_sync"`     // Timestamp del último sync exitoso
	ClientID     string                `json:"client_id"`     // ID único del cliente para evitar duplicados
	DeviceInfo   string                `json:"device_info"`   // Información del dispositivo (opcional)
	AppVersion   string                `json:"app_version"`   // Versión de la app cliente
}

// OfflineCategory representa datos de categoría modificados offline
// Incluye información completa para detección y resolución de conflictos
// Mantiene compatibilidad con la estructura Category existente
type OfflineCategory struct {
	ID               string `json:"id"`                  // ID de la categoría (puede ser local para nuevos)
	LocalID          string `json:"local_id"`            // ID local único en el dispositivo
	ServerID         string `json:"server_id"`           // ID en el servidor (vacío para nuevos registros)
	Action           string `json:"action"`              // "add", "update", "delete"
	UserID           string `json:"user_id"`             // ID del usuario propietario
	Name             string `json:"name"`                // Nombre de la categoría
	Type             string `json:"type"`                // "income" o "expense"
	Emoji            string `json:"emoji"`               // Emoji representativo de la categoría
	OfflineTimestamp string `json:"offline_timestamp"`   // Timestamp cuando se realizó offline
	SyncTimestamp    string `json:"sync_timestamp"`      // Timestamp para sincronización
	Status           string `json:"status"`              // "pending", "synced", "conflict"
	Version          int    `json:"version"`             // Versión para control de concurrencia
}

// SyncCategoriesBatchResponse representa la respuesta del servidor tras sincronización
// Incluye resultados, conflictos detectados y datos actualizados del servidor
// Proporciona feedback detallado sobre cada operación procesada
type SyncCategoriesBatchResponse struct {
	Success       bool                             `json:"success"`        // Indica si la sincronización fue exitosa
	Message       string                           `json:"message"`        // Mensaje descriptivo del resultado
	ProcessedOps  int                              `json:"processed_ops"`  // Número de operaciones procesadas
	SuccessfulOps int                              `json:"successful_ops"` // Operaciones exitosas
	FailedOps     int                              `json:"failed_ops"`     // Operaciones fallidas
	Results       []SyncCategoriesResult           `json:"results"`        // Resultado detallado por operación
	Conflicts     []CategoriesConflictResolution   `json:"conflicts"`      // Conflictos detectados
	ServerData    []Category                       `json:"server_data"`    // Datos actualizados del servidor
	LastSync      string                           `json:"last_sync"`      // Nuevo timestamp de sincronización
	NextSyncTime  string                           `json:"next_sync_time"` // Sugerencia para próximo sync
}

// SyncCategoriesResult representa el resultado de sincronización de una operación individual
// Proporciona feedback detallado sobre cada categoría procesada
// Incluye información sobre conflictos y acciones requeridas
type SyncCategoriesResult struct {
	LocalID         string `json:"local_id"`         // ID local de la operación
	ServerID        string `json:"server_id"`        // ID asignado en el servidor
	Action          string `json:"action"`           // Acción realizada
	OperationType   string `json:"operation_type"`   // "category"
	Status          string `json:"status"`           // "success", "error", "conflict"
	Error           string `json:"error,omitempty"`  // Mensaje de error si aplica
	ConflictType    string `json:"conflict_type,omitempty"` // Tipo de conflicto detectado
	RequiresAction  bool   `json:"requires_action"`  // Si requiere acción del usuario
	SyncTimestamp   string `json:"sync_timestamp"`   // Timestamp de sincronización
}

// CategoriesConflictResolution representa un conflicto detectado durante sincronización
// Proporciona información para resolución manual o automática de conflictos
// Incluye datos tanto del cliente como del servidor para comparación
type CategoriesConflictResolution struct {
	LocalID       string      `json:"local_id"`       // ID local del registro en conflicto
	ServerID      string      `json:"server_id"`      // ID del registro en el servidor
	ConflictType  string      `json:"conflict_type"`  // "version", "timestamp", "data", "duplicate_name"
	OperationType string      `json:"operation_type"` // "category"
	LocalData     interface{} `json:"local_data"`     // Datos del cliente (Category)
	ServerData    interface{} `json:"server_data"`    // Datos del servidor
	Resolution    string      `json:"resolution"`     // "manual", "server_wins", "client_wins"
	Priority      string      `json:"priority"`       // "high", "medium", "low"
	Description   string      `json:"description"`    // Descripción del conflicto
	Suggestions   []string    `json:"suggestions"`    // Sugerencias de resolución
}

// SyncCategoriesChangesRequest solicita cambios del servidor desde último sync
// Permite obtener actualizaciones sin enviar datos del cliente
// Útil para sincronización unidireccional (solo descarga)
type SyncCategoriesChangesRequest struct {
	UserID    string `json:"user_id"`    // ID del usuario
	LastSync  string `json:"last_sync"`  // Timestamp del último sync
	Limit     int    `json:"limit"`      // Límite de registros (opcional)
	Offset    int    `json:"offset"`     // Offset para paginación (opcional)
}

// SyncCategoriesChangesResponse contiene cambios del servidor para sincronización
// Permite al cliente actualizar su base de datos local con cambios del servidor
// Incluye categorías modificadas y eliminadas
type SyncCategoriesChangesResponse struct {
	Success      bool       `json:"success"`       // Éxito de la operación
	Message      string     `json:"message"`       // Mensaje descriptivo
	Categories   []Category `json:"categories"`    // Categorías modificadas en el servidor
	Deletions    []string   `json:"deletions"`     // IDs de categorías eliminadas
	HasMore      bool       `json:"has_more"`      // Indica si hay más cambios
	TotalChanges int        `json:"total_changes"` // Total de cambios disponibles
	LastSync     string     `json:"last_sync"`     // Nuevo timestamp de sync
	ServerTime   string     `json:"server_time"`   // Timestamp actual del servidor
}

// SyncCategoriesConflictRequest solicita resolución de un conflicto específico
// Permite al cliente indicar cómo resolver conflictos detectados durante sync
// Incluye opciones para resolución automática o datos fusionados manualmente
type SyncCategoriesConflictRequest struct {
	UserID         string      `json:"user_id"`         // ID del usuario
	LocalID        string      `json:"local_id"`        // ID local del registro
	ServerID       string      `json:"server_id"`       // ID del servidor
	OperationType  string      `json:"operation_type"`  // "category"
	Resolution     string      `json:"resolution"`      // "server_wins", "client_wins", "merge"
	MergedData     interface{} `json:"merged_data,omitempty"` // Datos fusionados (si aplica)
	ClientChoice   string      `json:"client_choice"`   // Elección específica del usuario
}

// SyncCategoriesStats representa estadísticas de sincronización de Categories
// Proporciona métricas útiles para monitoreo y optimización específicas de categorías
// Incluye información sobre rendimiento y errores de sincronización
type SyncCategoriesStats struct {
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
	
	// Estadísticas específicas de Categories
	IncomeCategoriesSynced  int `json:"income_categories_synced"`  // Categorías de ingresos sincronizadas
	ExpenseCategoriesSynced int `json:"expense_categories_synced"` // Categorías de gastos sincronizadas
	TotalCategoriesManaged  int `json:"total_categories_managed"`  // Total de categorías gestionadas
	DuplicateNamesResolved  int `json:"duplicate_names_resolved"`  // Conflictos de nombres duplicados resueltos
}

// SyncCategoriesConfig almacena configuración para el sistema de sincronización
// Permite ajustar comportamiento según necesidades específicas del sistema de categorías
// Incluye configuraciones específicas para operaciones de categorías
type SyncCategoriesConfig struct {
	MaxBatchSize        int                      `json:"max_batch_size"`        // Máximo número de operaciones por lote
	ConflictResolution  string                   `json:"conflict_resolution"`   // Estrategia predeterminada: "manual", "server_wins", "client_wins"
	SyncIntervalMinutes int                      `json:"sync_interval_minutes"` // Intervalo sugerido entre syncs
	RetryAttempts       int                      `json:"retry_attempts"`        // Intentos de reintento para operaciones fallidas
	TimeoutSeconds      int                      `json:"timeout_seconds"`       // Timeout para operaciones de sync
	CompressionEnabled  bool                     `json:"compression_enabled"`   // Habilitar compresión de datos
	EncryptionEnabled   bool                     `json:"encryption_enabled"`    // Habilitar encriptación de datos
	CategoriesOptions   CategoriesSyncOptions    `json:"categories_options"`    // Opciones específicas para categorías
}

// CategoriesSyncOptions contiene opciones específicas para sincronización de categorías
// Personaliza el comportamiento de sync para las características únicas de categorías
// Incluye configuraciones para validación de nombres y manejo de emojis
type CategoriesSyncOptions struct {
	ValidateUniqueNames     bool `json:"validate_unique_names"`     // Validar nombres únicos por tipo
	SyncEmojiEncoding       bool `json:"sync_emoji_encoding"`       // Sincronizar codificación de emojis
	HandleEmojiConflicts    bool `json:"handle_emoji_conflicts"`    // Manejar conflictos de emojis
	AllowDuplicateNames     bool `json:"allow_duplicate_names"`     // Permitir nombres duplicados entre tipos
	PreserveCreationOrder   bool `json:"preserve_creation_order"`   // Preservar orden de creación
	AutoFixEmojiCorruption  bool `json:"auto_fix_emoji_corruption"` // Corregir automáticamente emojis corruptos
}

// Validate método básico para validar solicitud de sincronización
// Verifica que los campos obligatorios estén presentes y sean válidos
// Retorna error específico si encuentra problemas en la validación
func (req *SyncCategoriesBatchRequest) Validate() error {
	// Validar que el ID de usuario esté presente
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	
	// Validar que el ID de cliente esté presente para evitar duplicados
	if req.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	
	// Validar que haya al menos una categoría para procesar
	if len(req.Categories) == 0 {
		return fmt.Errorf("at least one category operation is required")
	}
	
	// Validar cada categoría individual
	for i, category := range req.Categories {
		if err := category.Validate(); err != nil {
			return fmt.Errorf("category %d validation failed: %v", i, err)
		}
	}
	
	return nil
}

// Validate método para validar una categoría offline individual
// Verifica que los campos de la categoría sean válidos según las reglas de negocio
// Incluye validación específica según la acción a realizar
func (cat *OfflineCategory) Validate() error {
	// Validar acción requerida
	if cat.Action == "" {
		return fmt.Errorf("action is required")
	}
	
	// Validar acciones permitidas
	validActions := map[string]bool{"add": true, "update": true, "delete": true}
	if !validActions[cat.Action] {
		return fmt.Errorf("invalid action: %s", cat.Action)
	}
	
	// Para operaciones add y update, validar campos adicionales
	if cat.Action == "add" || cat.Action == "update" {
		if cat.UserID == "" {
			return fmt.Errorf("user_id is required for %s action", cat.Action)
		}
		
		if cat.Name == "" {
			return fmt.Errorf("name is required for %s action", cat.Action)
		}
		
		// Validar tipo de categoría
		if cat.Type != "income" && cat.Type != "expense" {
			return fmt.Errorf("type must be 'income' or 'expense'")
		}
	}
	
	// Para operaciones update y delete, validar que haya identificador
	if (cat.Action == "update" || cat.Action == "delete") && cat.LocalID == "" && cat.ServerID == "" {
		return fmt.Errorf("either local_id or server_id is required for %s action", cat.Action)
	}
	
	return nil
}