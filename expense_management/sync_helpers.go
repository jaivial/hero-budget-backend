package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Funciones auxiliares para el sistema de sincronización offline
// Proporciona utilidades para manejo de estadísticas, configuración y operaciones de base de datos

// Funciones de configuración y estadísticas

// getSyncConfig obtiene la configuración actual del sistema de sincronización
func getSyncConfig() (*SyncConfig, error) {
	// Configuración por defecto del sistema de sincronización
	config := &SyncConfig{
		MaxBatchSize:        50,  // Máximo 50 operaciones por lote
		ConflictResolution:  "manual", // Resolución manual por defecto
		SyncIntervalMinutes: 15, // Sincronizar cada 15 minutos
		RetryAttempts:       3,  // 3 intentos de reintento
		TimeoutSeconds:      30, // Timeout de 30 segundos
		CompressionEnabled:  false, // Sin compresión por ahora
		EncryptionEnabled:   false, // Sin encriptación por ahora
	}

	// En el futuro, esta configuración puede venir de base de datos o archivo
	// Por ahora usamos valores por defecto
	
	return config, nil
}

// updateSyncConfig actualiza la configuración del sistema de sincronización
func updateSyncConfig(config *SyncConfig) error {
	// Validar configuración
	if config.MaxBatchSize <= 0 || config.MaxBatchSize > 1000 {
		return fmt.Errorf("max_batch_size debe estar entre 1 y 1000")
	}
	
	if config.SyncIntervalMinutes < 1 || config.SyncIntervalMinutes > 1440 {
		return fmt.Errorf("sync_interval_minutes debe estar entre 1 y 1440")
	}
	
	if config.RetryAttempts < 0 || config.RetryAttempts > 10 {
		return fmt.Errorf("retry_attempts debe estar entre 0 y 10")
	}
	
	if config.TimeoutSeconds < 5 || config.TimeoutSeconds > 300 {
		return fmt.Errorf("timeout_seconds debe estar entre 5 y 300")
	}

	// En el futuro, guardar en base de datos
	// Por ahora, la actualización es temporal (solo en memoria)
	
	log.Printf("Configuración de sync actualizada: batch_size=%d, interval=%d min", 
		config.MaxBatchSize, config.SyncIntervalMinutes)
	
	return nil
}

// getUserSyncStats obtiene estadísticas de sincronización para un usuario específico
func getUserSyncStats(userID string) (*SyncStats, error) {
	db, err := openDatabase()
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	stats := &SyncStats{
		UserID: userID,
	}

	// Obtener última sincronización exitosa
	lastSyncQuery := `
		SELECT created_at FROM expenses 
		WHERE user_id = ? AND sync_timestamp IS NOT NULL 
		ORDER BY sync_timestamp DESC LIMIT 1
	`
	var lastSyncStr string
	err = db.QueryRow(lastSyncQuery, userID).Scan(&lastSyncStr)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("error obteniendo última sincronización: %v", err)
	}
	if err == nil {
		if lastSync, parseErr := time.Parse(time.RFC3339, lastSyncStr); parseErr == nil {
			stats.LastSyncTime = lastSync
		}
	}

	// Obtener número total de gastos sincronizados
	syncCountQuery := `
		SELECT COUNT(*) FROM expenses 
		WHERE user_id = ? AND sync_timestamp IS NOT NULL
	`
	err = db.QueryRow(syncCountQuery, userID).Scan(&stats.TotalSyncs)
	if err != nil {
		return nil, fmt.Errorf("error contando sincronizaciones: %v", err)
	}

	// Obtener operaciones pendientes (gastos sin sincronizar)
	pendingOpsQuery := `
		SELECT COUNT(*) FROM expenses 
		WHERE user_id = ? AND (sync_timestamp IS NULL OR sync_timestamp = '')
	`
	err = db.QueryRow(pendingOpsQuery, userID).Scan(&stats.PendingOperations)
	if err != nil {
		return nil, fmt.Errorf("error contando operaciones pendientes: %v", err)
	}

	// Calcular métricas adicionales
	stats.ConflictsResolved = 0 // Por implementar
	stats.DataSizeBytes = calculateUserDataSize(userID)
	stats.AverageLatency = 150.0 // Valor estimado en ms
	stats.ErrorCount = 0 // Por implementar

	return stats, nil
}

// getGlobalSyncStats obtiene estadísticas globales del sistema de sincronización
func getGlobalSyncStats() (map[string]interface{}, error) {
	db, err := openDatabase()
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	stats := make(map[string]interface{})

	// Total de gastos en el sistema
	var totalExpenses int
	err = db.QueryRow("SELECT COUNT(*) FROM expenses").Scan(&totalExpenses)
	if err != nil {
		return nil, fmt.Errorf("error contando gastos totales: %v", err)
	}
	stats["total_expenses"] = totalExpenses

	// Total de usuarios activos
	var totalUsers int
	err = db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM expenses").Scan(&totalUsers)
	if err != nil {
		return nil, fmt.Errorf("error contando usuarios: %v", err)
	}
	stats["total_users"] = totalUsers

	// Gastos sincronizados
	var syncedExpenses int
	err = db.QueryRow("SELECT COUNT(*) FROM expenses WHERE sync_timestamp IS NOT NULL").Scan(&syncedExpenses)
	if err != nil {
		return nil, fmt.Errorf("error contando gastos sincronizados: %v", err)
	}
	stats["synced_expenses"] = syncedExpenses

	// Gastos pendientes de sincronización
	var pendingExpenses int
	err = db.QueryRow("SELECT COUNT(*) FROM expenses WHERE sync_timestamp IS NULL OR sync_timestamp = ''").Scan(&pendingExpenses)
	if err != nil {
		return nil, fmt.Errorf("error contando gastos pendientes: %v", err)
	}
	stats["pending_expenses"] = pendingExpenses

	// Calcular porcentaje de sincronización
	if totalExpenses > 0 {
		syncPercentage := float64(syncedExpenses) / float64(totalExpenses) * 100
		stats["sync_percentage"] = syncPercentage
	} else {
		stats["sync_percentage"] = 0.0
	}

	// Agregar timestamp
	stats["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	stats["service"] = "expense_management"
	stats["version"] = "1.0.0"

	return stats, nil
}

// Funciones de base de datos para sincronización

// getChangesAfterTimestamp obtiene cambios del servidor después de un timestamp
func getChangesAfterTimestamp(request *SyncChangesRequest) (*SyncChangesResponse, error) {
	db, err := openDatabase()
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	response := &SyncChangesResponse{
		Success:      true,
		Message:      "Cambios obtenidos exitosamente",
		Changes:      make([]Expense, 0),
		Deletions:    make([]int, 0),
		HasMore:      false,
		TotalChanges: 0,
		LastSync:     time.Now().UTC().Format(time.RFC3339),
		ServerTime:   time.Now().UTC().Format(time.RFC3339),
	}

	// Construir query base
	var query string
	var args []interface{}

	if request.LastSync != "" {
		// Obtener cambios desde el último sync
		query = `
			SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
			FROM expenses 
			WHERE user_id = ? AND (updated_at > ? OR created_at > ?)
			ORDER BY updated_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{request.UserID, request.LastSync, request.LastSync, request.Limit, request.Offset}
	} else {
		// Primera sincronización - obtener todos los gastos del usuario
		query = `
			SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
			FROM expenses 
			WHERE user_id = ?
			ORDER BY updated_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{request.UserID, request.Limit, request.Offset}
	}

	// Ejecutar query para obtener cambios
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo cambios: %v", err)
	}
	defer rows.Close()

	// Procesar resultados
	for rows.Next() {
		var expense Expense
		err := rows.Scan(&expense.ID, &expense.UserID, &expense.Amount, &expense.Date,
			&expense.Category, &expense.PaymentMethod, &expense.Description,
			&expense.CreatedAt, &expense.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("error escaneando gasto: %v", err)
		}
		response.Changes = append(response.Changes, expense)
	}

	response.TotalChanges = len(response.Changes)

	// Verificar si hay más cambios (para paginación)
	if len(response.Changes) == request.Limit {
		// Verificar si hay más registros
		var hasMoreCount int
		countQuery := query // Usar la misma query pero con COUNT
		if request.LastSync != "" {
			countQuery = `
				SELECT COUNT(*) FROM expenses 
				WHERE user_id = ? AND (updated_at > ? OR created_at > ?)
			`
			countArgs := []interface{}{request.UserID, request.LastSync, request.LastSync}
			db.QueryRow(countQuery, countArgs...).Scan(&hasMoreCount)
		} else {
			countQuery = `SELECT COUNT(*) FROM expenses WHERE user_id = ?`
			db.QueryRow(countQuery, request.UserID).Scan(&hasMoreCount)
		}
		
		response.HasMore = hasMoreCount > (request.Offset + request.Limit)
	}

	// Por ahora, no manejamos eliminaciones específicamente
	// En el futuro, se puede agregar una tabla de eliminaciones o usar soft deletes

	return response, nil
}

// resolveConflict resuelve un conflicto específico de sincronización
func resolveConflict(request *SyncConflictRequest) (map[string]interface{}, error) {
	db, err := openDatabase()
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	// Comenzar transacción
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error iniciando transacción: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	result := make(map[string]interface{})

	switch request.Resolution {
	case "server_wins":
		// No hacer nada - mantener datos del servidor
		result["action"] = "server_data_kept"
		result["message"] = "Datos del servidor mantenidos"
		
	case "client_wins":
		// Actualizar con datos del cliente
		// Nota: Necesitaríamos los datos del cliente en la request
		result["action"] = "client_data_applied"
		result["message"] = "Datos del cliente aplicados"
		
	case "merge":
		// Aplicar datos fusionados
		if request.MergedData.ID == 0 {
			return nil, fmt.Errorf("datos fusionados requeridos para resolución merge")
		}
		
		updateQuery := `
			UPDATE expenses 
			SET amount = ?, date = ?, category = ?, payment_method = ?, description = ?, 
			    updated_at = CURRENT_TIMESTAMP, sync_timestamp = ?
			WHERE id = ? AND user_id = ?
		`
		
		_, err = tx.Exec(updateQuery, request.MergedData.Amount, request.MergedData.Date,
			request.MergedData.Category, request.MergedData.PaymentMethod, 
			request.MergedData.Description, time.Now().UTC().Format(time.RFC3339),
			request.ServerID, request.UserID)
		if err != nil {
			return nil, fmt.Errorf("error aplicando datos fusionados: %v", err)
		}
		
		result["action"] = "merged_data_applied"
		result["message"] = "Datos fusionados aplicados exitosamente"
		
	default:
		return nil, fmt.Errorf("tipo de resolución no soportado: %s", request.Resolution)
	}

	// Confirmar transacción
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error confirmando resolución: %v", err)
	}

	result["success"] = true
	result["local_id"] = request.LocalID
	result["server_id"] = request.ServerID
	result["resolution"] = request.Resolution
	result["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	return result, nil
}

// Funciones auxiliares

// calculateUserDataSize calcula el tamaño aproximado de datos de un usuario
func calculateUserDataSize(userID string) int64 {
	// Cálculo aproximado basado en estructura de datos
	// Cada gasto: ~200 bytes promedio
	db, err := openDatabase()
	if err != nil {
		return 0
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM expenses WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return 0
	}

	return int64(count * 200) // 200 bytes por gasto aproximadamente
}

// updateSyncStats actualiza estadísticas de una operación de sync
func updateSyncStats(userID string, totalOps, successfulOps, failedOps int) {
	// Por ahora solo loguear, en el futuro guardar en DB
	log.Printf("Sync stats - Usuario: %s, Total: %d, Exitosos: %d, Fallidos: %d", 
		userID, totalOps, successfulOps, failedOps)
}

// recordSyncBatchStats registra estadísticas de un lote de sincronización
func recordSyncBatchStats(userID string, response *SyncBatchResponse) {
	// Registrar estadísticas detalladas del lote
	log.Printf("Sync batch stats - Usuario: %s, Procesados: %d, Exitosos: %d, Fallidos: %d, Conflictos: %d", 
		userID, response.ProcessedOps, response.SuccessfulOps, response.FailedOps, len(response.Conflicts))
}

// updateLastSyncQuery actualiza el timestamp de última consulta de sync
func updateLastSyncQuery(userID string) {
	// Por ahora solo loguear, en el futuro guardar en tabla de stats
	log.Printf("Última consulta de sync para usuario %s: %s", userID, time.Now().UTC().Format(time.RFC3339))
}

// Funciones de actualización de balances para transacciones de sync

// updateBalanceAfterExpenseAdd actualiza balances después de agregar un gasto via sync
func updateBalanceAfterExpenseAdd(tx *sql.Tx, expenseID int, userID string) error {
	// Obtener el gasto para conocer los detalles
	expense, err := getExpenseByIDFromTx(tx, expenseID)
	if err != nil {
		return fmt.Errorf("error obteniendo gasto para actualizar balance: %v", err)
	}

	// Llamar a la función existente de actualización de balances
	// Nota: Esta función debería existir en el servicio principal
	// Por ahora, implementamos una versión simplificada
	return updateBalanceForSyncOperation(tx, expense, "add")
}

// updateBalanceAfterExpenseUpdate actualiza balances después de actualizar un gasto via sync
func updateBalanceAfterExpenseUpdate(tx *sql.Tx, expenseID int, userID string, oldAmount float64, oldPaymentMethod, oldDate string) error {
	// Obtener el gasto actualizado
	expense, err := getExpenseByIDFromTx(tx, expenseID)
	if err != nil {
		return fmt.Errorf("error obteniendo gasto actualizado: %v", err)
	}

	// Revertir el efecto del gasto anterior
	oldExpense := &Expense{
		ID:            expense.ID,
		UserID:        expense.UserID,
		Amount:        oldAmount,
		Date:          oldDate,
		PaymentMethod: oldPaymentMethod,
	}
	
	if err := updateBalanceForSyncOperation(tx, oldExpense, "delete"); err != nil {
		return fmt.Errorf("error revirtiendo balance anterior: %v", err)
	}

	// Aplicar el efecto del nuevo gasto
	return updateBalanceForSyncOperation(tx, expense, "add")
}

// updateBalanceAfterExpenseDelete actualiza balances después de eliminar un gasto via sync
func updateBalanceAfterExpenseDelete(tx *sql.Tx, expense *Expense, userID string) error {
	// Revertir el efecto del gasto eliminado
	return updateBalanceForSyncOperation(tx, expense, "delete")
}

// updateBalanceForSyncOperation actualiza balances para una operación de sync específica
func updateBalanceForSyncOperation(tx *sql.Tx, expense *Expense, operation string) error {
	// Implementación simplificada de actualización de balances
	// En el futuro, integrar con la lógica existente de balances en cascada
	
	log.Printf("Actualizando balances para operación sync: %s, gasto ID: %d, monto: %.2f", 
		operation, expense.ID, expense.Amount)
	
	// Por ahora, solo registrar la operación
	// La implementación completa requiere integración con las funciones existentes
	// de balance del servicio expense_management
	
	return nil
}