package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

// Operaciones de base de datos para funcionalidad de sincronización offline
// Gestiona procesamiento de lotes, detección de conflictos y resolución

// processSyncBatch procesa un lote completo de operaciones de sincronización
// Ejecuta todas las operaciones en una transacción para mantener consistencia
func processSyncBatch(request *SyncBatchRequest) (*SyncBatchResponse, error) {
	// Abrir conexión a la base de datos
	db, err := openDatabase()
	if err != nil {
		return nil, fmt.Errorf("error abriendo base de datos: %v", err)
	}
	defer db.Close()

	// Iniciar transacción para atomicidad
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error iniciando transacción: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Crear respuesta base
	response := &SyncBatchResponse{
		Success:       true,
		Message:       "Sincronización procesada",
		ProcessedOps:  len(request.Expenses),
		SuccessfulOps: 0,
		FailedOps:     0,
		Results:       make([]SyncResult, 0, len(request.Expenses)),
		Conflicts:     make([]ConflictResolution, 0),
		ServerData:    make([]Expense, 0),
		LastSync:      time.Now().UTC().Format(time.RFC3339),
	}

	// Procesar cada operación individual
	for _, expense := range request.Expenses {
		result, conflict, err := processSingleOperation(tx, &expense, request.UserID)
		
		if err != nil {
			// Error procesando operación
			result = SyncResult{
				LocalID:       expense.LocalID,
				Action:        expense.Action,
				Status:        "error",
				Error:         err.Error(),
				SyncTimestamp: time.Now().UTC().Format(time.RFC3339),
			}
			response.FailedOps++
			log.Printf("Error procesando operación %s: %v", expense.LocalID, err)
		} else if conflict != nil {
			// Conflicto detectado
			result.Status = "conflict"
			result.ConflictType = conflict.ConflictType
			result.RequiresAction = true
			response.Conflicts = append(response.Conflicts, *conflict)
			log.Printf("Conflicto detectado para operación %s: %s", expense.LocalID, conflict.ConflictType)
		} else {
			// Operación exitosa
			response.SuccessfulOps++
			log.Printf("Operación %s procesada exitosamente", expense.LocalID)
		}

		response.Results = append(response.Results, result)
	}

	// Confirmar transacción si no hay errores críticos
	if response.FailedOps == 0 || (response.SuccessfulOps > 0 && response.FailedOps < len(request.Expenses)/2) {
		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("error confirmando transacción: %v", err)
		}
		log.Printf("Transacción de sync confirmada: %d exitosos, %d fallidos", response.SuccessfulOps, response.FailedOps)
	} else {
		tx.Rollback()
		response.Success = false
		response.Message = "Sincronización fallida: demasiados errores"
		log.Printf("Transacción de sync revertida debido a errores")
	}

	// Obtener datos actualizados del servidor para el cliente
	serverData, err := getUpdatedServerData(request.UserID, request.LastSync)
	if err != nil {
		log.Printf("Advertencia: error obteniendo datos del servidor: %v", err)
	} else {
		response.ServerData = serverData
	}

	// Registrar estadísticas de la sincronización
	recordSyncBatchStats(request.UserID, response)

	return response, nil
}

// processSingleOperation procesa una operación individual de sincronización
// Detecta conflictos y ejecuta la operación apropiada
func processSingleOperation(tx *sql.Tx, expense *OfflineExpense, userID string) (SyncResult, *ConflictResolution, error) {
	result := SyncResult{
		LocalID:       expense.LocalID,
		Action:        expense.Action,
		Status:        "success",
		SyncTimestamp: time.Now().UTC().Format(time.RFC3339),
	}

	switch expense.Action {
	case "add":
		// Procesar adición de nuevo gasto
		serverID, conflict, err := processSyncAdd(tx, expense, userID)
		if err != nil {
			return result, nil, err
		}
		if conflict != nil {
			return result, conflict, nil
		}
		result.ServerID = strconv.Itoa(serverID)
		
	case "update":
		// Procesar actualización de gasto existente
		conflict, err := processSyncUpdate(tx, expense, userID)
		if err != nil {
			return result, nil, err
		}
		if conflict != nil {
			return result, conflict, nil
		}
		result.ServerID = strconv.Itoa(expense.ServerID)
		
	case "delete":
		// Procesar eliminación de gasto
		conflict, err := processSyncDelete(tx, expense, userID)
		if err != nil {
			return result, nil, err
		}
		if conflict != nil {
			return result, conflict, nil
		}
		result.ServerID = strconv.Itoa(expense.ServerID)
		
	default:
		return result, nil, fmt.Errorf("acción no soportada: %s", expense.Action)
	}

	return result, nil, nil
}

// processSyncAdd procesa la adición de un nuevo gasto desde cliente offline
// Verifica duplicados y detecta conflictos potenciales
func processSyncAdd(tx *sql.Tx, expense *OfflineExpense, userID string) (int, *ConflictResolution, error) {
	// Verificar si ya existe un gasto similar (posible duplicado)
	existingID, err := findSimilarExpense(tx, expense, userID)
	if err != nil {
		return 0, nil, fmt.Errorf("error verificando duplicados: %v", err)
	}

	if existingID > 0 {
		// Posible duplicado detectado - crear conflicto
		existingExpense, err := getExpenseByIDFromTx(tx, existingID)
		if err != nil {
			return 0, nil, fmt.Errorf("error obteniendo gasto existente: %v", err)
		}

		conflict := &ConflictResolution{
			LocalID:      expense.LocalID,
			ServerID:     existingID,
			ConflictType: "duplicate",
			ServerData:   *existingExpense,
			Resolution:   "manual",
			Priority:     "medium",
			Description:  "Posible gasto duplicado detectado",
			Suggestions:  []string{"Verificar si es el mismo gasto", "Crear nuevo gasto", "Actualizar gasto existente"},
		}
		return 0, conflict, nil
	}

	// Insertar nuevo gasto
	query := `
		INSERT INTO expenses (user_id, amount, date, category, payment_method, description, created_at, updated_at, sync_timestamp, client_id)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?)
	`
	
	result, err := tx.Exec(query, userID, expense.Amount, expense.Date, expense.Category, 
		expense.PaymentMethod, expense.Description, expense.SyncTimestamp, expense.LocalID)
	if err != nil {
		return 0, nil, fmt.Errorf("error insertando gasto: %v", err)
	}

	// Obtener ID del gasto insertado
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		return 0, nil, fmt.Errorf("error obteniendo ID insertado: %v", err)
	}

	serverID := int(lastInsertID)

	// Actualizar balances en cascada
	if err := updateBalanceAfterExpenseAdd(tx, serverID, userID); err != nil {
		return 0, nil, fmt.Errorf("error actualizando balances: %v", err)
	}

	log.Printf("Nuevo gasto añadido: ID %d para usuario %s", serverID, userID)
	return serverID, nil, nil
}

// processSyncUpdate procesa la actualización de un gasto existente
// Detecta conflictos de versión y aplica estrategias de resolución
func processSyncUpdate(tx *sql.Tx, expense *OfflineExpense, userID string) (*ConflictResolution, error) {
	// Obtener gasto actual del servidor
	serverExpense, err := getExpenseByIDFromTx(tx, expense.ServerID)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo gasto del servidor: %v", err)
	}

	// Verificar que el gasto pertenece al usuario
	if serverExpense.UserID != userID {
		return nil, fmt.Errorf("gasto no pertenece al usuario")
	}

	// Detectar conflictos de timestamp/versión
	if isUpdateConflict(serverExpense, expense) {
		conflict := &ConflictResolution{
			LocalID:      expense.LocalID,
			ServerID:     expense.ServerID,
			ConflictType: "version",
			ServerData:   *serverExpense,
			Resolution:   "manual",
			Priority:     "high",
			Description:  "El gasto fue modificado en el servidor después de la modificación local",
			Suggestions:  []string{"Mantener cambios del servidor", "Aplicar cambios locales", "Fusionar cambios"},
		}
		return conflict, nil
	}

	// Proceder con la actualización
	oldAmount := serverExpense.Amount
	oldPaymentMethod := serverExpense.PaymentMethod
	oldDate := serverExpense.Date

	query := `
		UPDATE expenses 
		SET amount = ?, date = ?, category = ?, payment_method = ?, description = ?, 
		    updated_at = CURRENT_TIMESTAMP, sync_timestamp = ?
		WHERE id = ? AND user_id = ?
	`
	
	_, err = tx.Exec(query, expense.Amount, expense.Date, expense.Category, 
		expense.PaymentMethod, expense.Description, expense.SyncTimestamp, expense.ServerID, userID)
	if err != nil {
		return nil, fmt.Errorf("error actualizando gasto: %v", err)
	}

	// Actualizar balances si cambió el monto, método de pago o fecha
	if oldAmount != expense.Amount || oldPaymentMethod != expense.PaymentMethod || oldDate != expense.Date {
		if err := updateBalanceAfterExpenseUpdate(tx, expense.ServerID, userID, oldAmount, oldPaymentMethod, oldDate); err != nil {
			return nil, fmt.Errorf("error actualizando balances: %v", err)
		}
	}

	log.Printf("Gasto actualizado: ID %d para usuario %s", expense.ServerID, userID)
	return nil, nil
}

// processSyncDelete procesa la eliminación de un gasto
// Verifica existencia y maneja conflictos de eliminación
func processSyncDelete(tx *sql.Tx, expense *OfflineExpense, userID string) (*ConflictResolution, error) {
	// Verificar que el gasto existe y pertenece al usuario
	serverExpense, err := getExpenseByIDFromTx(tx, expense.ServerID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Gasto ya fue eliminado - no es un error grave
			log.Printf("Gasto %d ya fue eliminado del servidor", expense.ServerID)
			return nil, nil
		}
		return nil, fmt.Errorf("error verificando gasto a eliminar: %v", err)
	}

	if serverExpense.UserID != userID {
		return nil, fmt.Errorf("gasto no pertenece al usuario")
	}

	// Verificar si hay conflicto de eliminación (gasto modificado recientemente)
	if isDeleteConflict(serverExpense, expense) {
		conflict := &ConflictResolution{
			LocalID:      expense.LocalID,
			ServerID:     expense.ServerID,
			ConflictType: "delete_conflict",
			ServerData:   *serverExpense,
			Resolution:   "manual",
			Priority:     "medium",
			Description:  "El gasto fue modificado en el servidor antes de la eliminación",
			Suggestions:  []string{"Proceder con eliminación", "Mantener gasto actualizado", "Revisar cambios"},
		}
		return conflict, nil
	}

	// Proceder con la eliminación
	query := `DELETE FROM expenses WHERE id = ? AND user_id = ?`
	result, err := tx.Exec(query, expense.ServerID, userID)
	if err != nil {
		return nil, fmt.Errorf("error eliminando gasto: %v", err)
	}

	// Verificar que se eliminó exactamente un registro
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("error verificando eliminación: %v", err)
	}
	if rowsAffected == 0 {
		return nil, fmt.Errorf("gasto no encontrado para eliminación")
	}

	// Actualizar balances después de la eliminación
	if err := updateBalanceAfterExpenseDelete(tx, serverExpense, userID); err != nil {
		return nil, fmt.Errorf("error actualizando balances: %v", err)
	}

	log.Printf("Gasto eliminado: ID %d para usuario %s", expense.ServerID, userID)
	return nil, nil
}

// Funciones auxiliares para detección de conflictos

// findSimilarExpense busca gastos similares para detectar duplicados
func findSimilarExpense(tx *sql.Tx, expense *OfflineExpense, userID string) (int, error) {
	query := `
		SELECT id FROM expenses 
		WHERE user_id = ? AND amount = ? AND date = ? AND category = ? 
		AND payment_method = ? AND ABS(julianday('now') - julianday(created_at)) < 1
		LIMIT 1
	`
	
	var id int
	err := tx.QueryRow(query, userID, expense.Amount, expense.Date, 
		expense.Category, expense.PaymentMethod).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil // No hay duplicado
	}
	if err != nil {
		return 0, err
	}
	
	return id, nil
}

// isUpdateConflict detecta si hay conflicto en una actualización
func isUpdateConflict(serverExpense *Expense, clientExpense *OfflineExpense) bool {
	// Convertir timestamps para comparación
	serverTime, err := time.Parse(time.RFC3339, serverExpense.UpdatedAt)
	if err != nil {
		// Si no se puede parsear, asumir conflicto por seguridad
		return true
	}
	
	clientTime, err := time.Parse(time.RFC3339, clientExpense.OfflineTimestamp)
	if err != nil {
		return true
	}
	
	// Hay conflicto si el servidor fue actualizado después del cambio offline
	return serverTime.After(clientTime)
}

// isDeleteConflict detecta si hay conflicto en una eliminación
func isDeleteConflict(serverExpense *Expense, clientExpense *OfflineExpense) bool {
	// Similar a update conflict pero con umbral más permisivo
	serverTime, err := time.Parse(time.RFC3339, serverExpense.UpdatedAt)
	if err != nil {
		return false // Permitir eliminación si hay dudas
	}
	
	clientTime, err := time.Parse(time.RFC3339, clientExpense.OfflineTimestamp)
	if err != nil {
		return false
	}
	
	// Conflicto solo si fue actualizado muy recientemente (menos de 1 hora)
	return serverTime.After(clientTime) && serverTime.Sub(clientTime) < time.Hour
}

// getUpdatedServerData obtiene datos actualizados del servidor para el cliente
func getUpdatedServerData(userID, lastSync string) ([]Expense, error) {
	db, err := openDatabase()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	var args []interface{}

	if lastSync != "" {
		// Obtener solo los cambios desde el último sync
		query = `
			SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
			FROM expenses 
			WHERE user_id = ? AND (created_at > ? OR updated_at > ?)
			ORDER BY updated_at DESC
		`
		args = []interface{}{userID, lastSync, lastSync}
	} else {
		// Primera sincronización - obtener todos los gastos
		query = `
			SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
			FROM expenses 
			WHERE user_id = ?
			ORDER BY updated_at DESC
		`
		args = []interface{}{userID}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []Expense
	for rows.Next() {
		var expense Expense
		err := rows.Scan(&expense.ID, &expense.UserID, &expense.Amount, &expense.Date,
			&expense.Category, &expense.PaymentMethod, &expense.Description,
			&expense.CreatedAt, &expense.UpdatedAt)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

// getExpenseByIDFromTx obtiene un gasto por ID usando una transacción existente
func getExpenseByIDFromTx(tx *sql.Tx, expenseID int) (*Expense, error) {
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM expenses WHERE id = ?
	`
	
	var expense Expense
	err := tx.QueryRow(query, expenseID).Scan(&expense.ID, &expense.UserID, &expense.Amount,
		&expense.Date, &expense.Category, &expense.PaymentMethod, &expense.Description,
		&expense.CreatedAt, &expense.UpdatedAt)
	if err != nil {
		return nil, err
	}
	
	return &expense, nil
}