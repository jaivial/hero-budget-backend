package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador de lotes para sincronización offline de presupuestos - Parte 2
// Contiene funciones de utilidad y procesamiento de cambios

// recalculateBudgetValues recalcula los valores derivados del presupuesto
// Asegura consistencia matemática entre los diferentes campos
func recalculateBudgetValues(budget *BudgetData) error {
	// Recalcular monto total disponible
	budget.TotalAmount = budget.FromPrevious + budget.TotalIncome
	
	// Recalcular monto restante
	budget.RemainingAmount = budget.TotalAmount - budget.SpentAmount - budget.UpcomingAmount
	
	// Recalcular porcentaje utilizado
	if budget.TotalAmount > 0 {
		budget.Percent = ((budget.SpentAmount + budget.UpcomingAmount) / budget.TotalAmount) * 100
	} else {
		budget.Percent = 0
	}
	
	// Validar que los valores sean lógicos
	if budget.RemainingAmount < 0 {
		log.Printf("Advertencia: Presupuesto excedido para usuario %s, período %s (restante: %.2f)", 
			budget.UserID, budget.Period, budget.RemainingAmount)
	}
	
	if budget.Percent > 100 {
		log.Printf("Advertencia: Presupuesto excede 100%% para usuario %s, período %s (%.2f%%)", 
			budget.UserID, budget.Period, budget.Percent)
	}
	
	return nil
}

// getBudgetChanges obtiene cambios de presupuestos del servidor desde último sync
// Implementa paginación y filtrado por usuario
func getBudgetChanges(request SyncBudgetChangesRequest) (*SyncBudgetChangesResponse, error) {
	// Query para obtener presupuestos del usuario
	query := `
		SELECT user_id, period, date, total_amount, remaining_amount, spent_amount, 
		       upcoming_amount, from_previous, percent, COALESCE(total_income, 0)
		FROM budget
		WHERE user_id = ?
	`
	
	// Agregar filtro de fecha si se proporciona last_sync
	args := []interface{}{request.UserID}
	if request.LastSync != "" {
		query += " AND updated_at > ?"
		args = append(args, request.LastSync)
	}
	
	// Agregar orden y límites
	query += " ORDER BY updated_at DESC"
	if request.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, request.Limit)
	}
	if request.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, request.Offset)
	}
	
	// Ejecutar query
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("error consultando presupuestos: %v", err)
	}
	defer rows.Close()
	
	// Procesar resultados
	var budgets []BudgetData
	for rows.Next() {
		var budget BudgetData
		err := rows.Scan(
			&budget.UserID,
			&budget.Period,
			&budget.Date,
			&budget.TotalAmount,
			&budget.RemainingAmount,
			&budget.SpentAmount,
			&budget.UpcomingAmount,
			&budget.FromPrevious,
			&budget.Percent,
			&budget.TotalIncome,
		)
		if err != nil {
			log.Printf("Error escaneando presupuesto: %v", err)
			continue
		}
		budgets = append(budgets, budget)
	}
	
	// Crear respuesta
	response := &SyncBudgetChangesResponse{
		Success:      true,
		Message:      "Cambios obtenidos exitosamente",
		Changes:      budgets,
		Deletions:    make([]string, 0), // Por ahora no implementamos tracking de eliminaciones
		HasMore:      len(budgets) == request.Limit, // Simple check para paginación
		TotalChanges: len(budgets),
		LastSync:     time.Now().UTC().Format(time.RFC3339),
		ServerTime:   time.Now().UTC().Format(time.RFC3339),
	}
	
	return response, nil
}

// validateBudgetDataConsistency valida la consistencia de los datos de presupuesto
// Verifica que los cálculos sean correctos antes de persistir
func validateBudgetDataConsistency(budget *BudgetData) error {
	// Validar que los montos no sean negativos (excepto remaining que puede ser negativo)
	if budget.TotalAmount < 0 {
		return fmt.Errorf("total_amount no puede ser negativo: %f", budget.TotalAmount)
	}
	if budget.SpentAmount < 0 {
		return fmt.Errorf("spent_amount no puede ser negativo: %f", budget.SpentAmount)
	}
	if budget.UpcomingAmount < 0 {
		return fmt.Errorf("upcoming_amount no puede ser negativo: %f", budget.UpcomingAmount)
	}
	if budget.FromPrevious < 0 {
		return fmt.Errorf("from_previous no puede ser negativo: %f", budget.FromPrevious)
	}
	if budget.TotalIncome < 0 {
		return fmt.Errorf("total_income no puede ser negativo: %f", budget.TotalIncome)
	}
	
	// Validar que el porcentaje esté en rango razonable (puede exceder 100% si está sobrepresupuestado)
	if budget.Percent < 0 {
		return fmt.Errorf("percent no puede ser negativo: %f", budget.Percent)
	}
	
	// Validar consistencia matemática básica
	expectedTotal := budget.FromPrevious + budget.TotalIncome
	tolerance := 0.01 // Tolerancia para errores de redondeo
	
	if budget.TotalAmount > 0 && expectedTotal > 0 {
		if abs(budget.TotalAmount - expectedTotal) > tolerance {
			return fmt.Errorf("total_amount (%f) no coincide con from_previous + total_income (%f)", 
				budget.TotalAmount, expectedTotal)
		}
	}
	
	// Validar remaining amount
	expectedRemaining := budget.TotalAmount - budget.SpentAmount - budget.UpcomingAmount
	if abs(budget.RemainingAmount - expectedRemaining) > tolerance {
		return fmt.Errorf("remaining_amount (%f) no coincide con el cálculo esperado (%f)", 
			budget.RemainingAmount, expectedRemaining)
	}
	
	return nil
}

// abs retorna el valor absoluto de un número flotante
// Función auxiliar para cálculos de tolerancia
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// logBudgetOperation registra una operación de presupuesto para auditoría
// Mantiene registro de todas las operaciones realizadas
func logBudgetOperation(userID, operation, period string, oldValues, newValues *BudgetData) {
	log.Printf("AUDIT: Usuario %s - Operación %s en período %s", userID, operation, period)
	
	if oldValues != nil {
		log.Printf("AUDIT: Valores anteriores - Total: %.2f, Gastado: %.2f, Restante: %.2f", 
			oldValues.TotalAmount, oldValues.SpentAmount, oldValues.RemainingAmount)
	}
	
	if newValues != nil {
		log.Printf("AUDIT: Valores nuevos - Total: %.2f, Gastado: %.2f, Restante: %.2f", 
			newValues.TotalAmount, newValues.SpentAmount, newValues.RemainingAmount)
	}
}

// detectBudgetConflicts detecta conflictos entre datos locales y del servidor
// Identifica discrepancias que requieren resolución manual
func detectBudgetConflicts(localBudget OfflineBudget, serverBudget BudgetData) []BudgetConflictResolution {
	var conflicts []BudgetConflictResolution
	
	// Verificar conflictos de montos
	if localBudget.TotalAmount != serverBudget.TotalAmount && localBudget.TotalAmount != 0 {
		conflicts = append(conflicts, BudgetConflictResolution{
			LocalID:      localBudget.LocalID,
			ServerID:     fmt.Sprintf("%s_%s", serverBudget.UserID, serverBudget.Period),
			ConflictType: "amount_mismatch",
			Priority:     "medium",
			Description:  fmt.Sprintf("Monto total difiere: local %.2f vs servidor %.2f", localBudget.TotalAmount, serverBudget.TotalAmount),
			Suggestions:  []string{"Usar valor del servidor", "Usar valor local", "Promediar valores"},
		})
	}
	
	// Verificar conflictos de gastos
	if localBudget.SpentAmount != serverBudget.SpentAmount && localBudget.SpentAmount != 0 {
		conflicts = append(conflicts, BudgetConflictResolution{
			LocalID:      localBudget.LocalID,
			ServerID:     fmt.Sprintf("%s_%s", serverBudget.UserID, serverBudget.Period),
			ConflictType: "spent_mismatch",
			Priority:     "high",
			Description:  fmt.Sprintf("Monto gastado difiere: local %.2f vs servidor %.2f", localBudget.SpentAmount, serverBudget.SpentAmount),
			Suggestions:  []string{"Sincronizar transacciones", "Usar valor más alto", "Verificar manualmente"},
		})
	}
	
	return conflicts
}