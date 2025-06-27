package main

import (
	"fmt"
	"log"
	"time"
)

// Procesador de lotes para sincronización offline de ahorros - Parte 2
// Contiene funciones de utilidad y resolución de conflictos específicas para savings

// validateSavingsDataConsistency valida la consistencia de los datos de ahorro
// Verifica que los cálculos sean correctos antes de persistir
func validateSavingsDataConsistency(savings *SavingsData) error {
	// Validar que los montos no sean negativos
	if savings.Available < 0 {
		return fmt.Errorf("available no puede ser negativo: %f", savings.Available)
	}
	if savings.Goal < 0 {
		return fmt.Errorf("goal no puede ser negativo: %f", savings.Goal)
	}
	if savings.NeedToSave < 0 {
		return fmt.Errorf("need_to_save no puede ser negativo: %f", savings.NeedToSave)
	}
	if savings.DailyTarget < 0 {
		return fmt.Errorf("daily_target no puede ser negativo: %f", savings.DailyTarget)
	}
	
	// Validar que el porcentaje esté en rango válido
	if savings.Percent < 0 || savings.Percent > 100 {
		return fmt.Errorf("percent debe estar entre 0 y 100: %f", savings.Percent)
	}
	
	// Validar consistencia matemática básica para porcentaje
	if savings.Goal > 0 {
		expectedPercent := (savings.Available / savings.Goal) * 100
		if expectedPercent > 100 {
			expectedPercent = 100
		}
		
		tolerance := 1.0 // Tolerancia de 1% para errores de redondeo
		if absSavings(savings.Percent - expectedPercent) > tolerance {
			return fmt.Errorf("percent (%f) no coincide con available/goal*100 (%f)", 
				savings.Percent, expectedPercent)
		}
	}
	
	// Validar need_to_save
	expectedNeedToSave := savings.Goal - savings.Available
	if expectedNeedToSave < 0 {
		expectedNeedToSave = 0
	}
	
	tolerance := 0.01 // Tolerancia para errores de redondeo
	if absSavings(savings.NeedToSave - expectedNeedToSave) > tolerance {
		return fmt.Errorf("need_to_save (%f) no coincide con goal - available (%f)", 
			savings.NeedToSave, expectedNeedToSave)
	}
	
	// Validar daily_target
	expectedDailyTarget := savings.NeedToSave / 30 // Asumiendo 30 días por mes
	if absSavings(savings.DailyTarget - expectedDailyTarget) > tolerance {
		return fmt.Errorf("daily_target (%f) no coincide con need_to_save/30 (%f)", 
			savings.DailyTarget, expectedDailyTarget)
	}
	
	return nil
}

// logSavingsOperation registra una operación de ahorro para auditoría
// Mantiene registro de todas las operaciones realizadas
func logSavingsOperation(userID, operation string, oldValues, newValues *SavingsData) {
	log.Printf("AUDIT: Usuario %s - Operación %s en savings", userID, operation)
	
	if oldValues != nil {
		log.Printf("AUDIT: Valores anteriores - Available: %.2f, Goal: %.2f, Percent: %.2f%%", 
			oldValues.Available, oldValues.Goal, oldValues.Percent)
	}
	
	if newValues != nil {
		log.Printf("AUDIT: Valores nuevos - Available: %.2f, Goal: %.2f, Percent: %.2f%%", 
			newValues.Available, newValues.Goal, newValues.Percent)
	}
}

// detectSavingsConflicts detecta conflictos entre datos locales y del servidor
// Identifica discrepancias que requieren resolución manual
func detectSavingsConflicts(localSavings OfflineSavings, serverSavings SavingsData) []SavingsConflictResolution {
	var conflicts []SavingsConflictResolution
	
	// Verificar conflictos de meta de ahorro
	if localSavings.Goal != serverSavings.Goal && localSavings.Goal != 0 {
		conflicts = append(conflicts, SavingsConflictResolution{
			LocalID:      localSavings.LocalID,
			ServerID:     fmt.Sprintf("savings_%s", serverSavings.UserID),
			ConflictType: "goal_mismatch",
			Priority:     "medium",
			Description:  fmt.Sprintf("Meta de ahorro difiere: local %.2f vs servidor %.2f", localSavings.Goal, serverSavings.Goal),
			Suggestions:  []string{"Usar meta del servidor", "Usar meta local", "Promediar metas"},
		})
	}
	
	// Verificar conflictos de cantidad disponible
	if localSavings.Available != serverSavings.Available && localSavings.Available != 0 {
		conflicts = append(conflicts, SavingsConflictResolution{
			LocalID:      localSavings.LocalID,
			ServerID:     fmt.Sprintf("savings_%s", serverSavings.UserID),
			ConflictType: "available_mismatch",
			Priority:     "high",
			Description:  fmt.Sprintf("Cantidad ahorrada difiere: local %.2f vs servidor %.2f", localSavings.Available, serverSavings.Available),
			Suggestions:  []string{"Sincronizar transacciones", "Usar valor más alto", "Verificar manualmente"},
		})
	}
	
	// Verificar conflictos de período
	if localSavings.Period != serverSavings.Period && localSavings.Period != "" {
		conflicts = append(conflicts, SavingsConflictResolution{
			LocalID:      localSavings.LocalID,
			ServerID:     fmt.Sprintf("savings_%s", serverSavings.UserID),
			ConflictType: "period_mismatch",
			Priority:     "low",
			Description:  fmt.Sprintf("Período difiere: local %s vs servidor %s", localSavings.Period, serverSavings.Period),
			Suggestions:  []string{"Usar período del servidor", "Usar período local", "Mantener ambos"},
		})
	}
	
	return conflicts
}

// applySavingsConflictResolution aplica la resolución de un conflicto específico
// Implementa diferentes estrategias de resolución según el tipo de conflicto
func applySavingsConflictResolution(conflict SavingsConflictResolution, resolution string) error {
	log.Printf("Aplicando resolución '%s' para conflicto %s", resolution, conflict.ConflictType)
	
	switch conflict.ConflictType {
	case "goal_mismatch":
		return resolveSavingsGoalConflict(conflict, resolution)
	case "available_mismatch":
		return resolveSavingsAvailableConflict(conflict, resolution)
	case "period_mismatch":
		return resolveSavingsPeriodConflict(conflict, resolution)
	default:
		return fmt.Errorf("tipo de conflicto no soportado: %s", conflict.ConflictType)
	}
}

// resolveSavingsGoalConflict resuelve conflictos de meta de ahorro
func resolveSavingsGoalConflict(conflict SavingsConflictResolution, resolution string) error {
	switch resolution {
	case "server_wins":
		log.Printf("Manteniendo meta del servidor para conflicto %s", conflict.LocalID)
		// No se requiere acción adicional, el servidor ya tiene la versión correcta
		return nil
	case "client_wins":
		log.Printf("Aplicando meta del cliente para conflicto %s", conflict.LocalID)
		// Actualizar con datos del cliente (ya está en LocalData)
		return updateSavingsData(conflict.LocalData)
	case "merge":
		log.Printf("Promediando metas para conflicto %s", conflict.LocalID)
		// Crear datos fusionados promediando las metas
		mergedData := conflict.ServerData
		mergedData.Goal = (conflict.LocalData.Goal + conflict.ServerData.Goal) / 2
		// Recalcular valores derivados
		return recalculateSavingsValues(&mergedData)
	default:
		return fmt.Errorf("estrategia de resolución no soportada: %s", resolution)
	}
}

// resolveSavingsAvailableConflict resuelve conflictos de cantidad disponible
func resolveSavingsAvailableConflict(conflict SavingsConflictResolution, resolution string) error {
	switch resolution {
	case "server_wins":
		log.Printf("Manteniendo cantidad del servidor para conflicto %s", conflict.LocalID)
		return nil
	case "client_wins":
		log.Printf("Aplicando cantidad del cliente para conflicto %s", conflict.LocalID)
		return updateSavingsData(conflict.LocalData)
	case "use_higher":
		log.Printf("Usando cantidad más alta para conflicto %s", conflict.LocalID)
		mergedData := conflict.ServerData
		if conflict.LocalData.Available > conflict.ServerData.Available {
			mergedData.Available = conflict.LocalData.Available
		}
		return recalculateSavingsValues(&mergedData)
	default:
		return fmt.Errorf("estrategia de resolución no soportada: %s", resolution)
	}
}

// resolveSavingsPeriodConflict resuelve conflictos de período
func resolveSavingsPeriodConflict(conflict SavingsConflictResolution, resolution string) error {
	switch resolution {
	case "server_wins":
		log.Printf("Manteniendo período del servidor para conflicto %s", conflict.LocalID)
		return nil
	case "client_wins":
		log.Printf("Aplicando período del cliente para conflicto %s", conflict.LocalID)
		return updateSavingsData(conflict.LocalData)
	default:
		return fmt.Errorf("estrategia de resolución no soportada: %s", resolution)
	}
}

// validateSavingsBusinessLogic aplica validaciones de lógica de negocio específicas
// Verifica que los datos de ahorro cumplan con las reglas establecidas
func validateSavingsBusinessLogic(savings *SavingsData) error {
	// Regla 1: No se puede tener más del 100% de la meta alcanzada
	if savings.Available > savings.Goal && savings.Goal > 0 {
		log.Printf("Advertencia: Cantidad ahorrada (%.2f) excede la meta (%.2f) para usuario %s", 
			savings.Available, savings.Goal, savings.UserID)
	}
	
	// Regla 2: El target diario debe ser realista
	if savings.DailyTarget > savings.Goal * 0.1 && savings.Goal > 0 {
		return fmt.Errorf("target diario (%.2f) muy alto para meta (%.2f)", 
			savings.DailyTarget, savings.Goal)
	}
	
	// Regla 3: El período debe ser válido
	validPeriods := []string{"daily", "weekly", "monthly", "quarterly", "semiannual", "annual"}
	isValidPeriod := false
	for _, period := range validPeriods {
		if savings.Period == period {
			isValidPeriod = true
			break
		}
	}
	if !isValidPeriod {
		return fmt.Errorf("período inválido: %s", savings.Period)
	}
	
	return nil
}

// optimizeSavingsSync optimiza el proceso de sincronización para mejorar rendimiento
// Aplica estrategias de optimización específicas para datos de ahorro
func optimizeSavingsSync(operations []OfflineSavings) []OfflineSavings {
	// Eliminar operaciones duplicadas (mismo user_id y action)
	seenOperations := make(map[string]bool)
	var optimizedOps []OfflineSavings
	
	// Procesar en orden inverso para mantener la operación más reciente
	for i := len(operations) - 1; i >= 0; i-- {
		op := operations[i]
		key := fmt.Sprintf("%s_%s", op.UserID, op.Action)
		
		if !seenOperations[key] {
			optimizedOps = append([]OfflineSavings{op}, optimizedOps...) // Prepend
			seenOperations[key] = true
		}
	}
	
	log.Printf("Optimización de sync: %d operaciones reducidas a %d", len(operations), len(optimizedOps))
	return optimizedOps
}

// calculateSavingsMetrics calcula métricas útiles para el usuario
// Proporciona información adicional sobre el progreso de ahorro
func calculateSavingsMetrics(savings SavingsData) map[string]interface{} {
	metrics := make(map[string]interface{})
	
	// Calcular días para alcanzar la meta al ritmo actual
	if savings.DailyTarget > 0 && savings.NeedToSave > 0 {
		daysToGoal := savings.NeedToSave / savings.DailyTarget
		metrics["days_to_goal"] = int(daysToGoal)
	}
	
	// Calcular fecha estimada de completación
	if savings.DailyTarget > 0 && savings.NeedToSave > 0 {
		daysToGoal := savings.NeedToSave / savings.DailyTarget
		estimatedDate := time.Now().AddDate(0, 0, int(daysToGoal))
		metrics["estimated_completion"] = estimatedDate.Format("2006-01-02")
	}
	
	// Calcular progreso semanal recomendado
	if savings.DailyTarget > 0 {
		weeklyTarget := savings.DailyTarget * 7
		metrics["weekly_target"] = weeklyTarget
	}
	
	// Calcular progreso mensual recomendado
	if savings.DailyTarget > 0 {
		monthlyTarget := savings.DailyTarget * 30
		metrics["monthly_target"] = monthlyTarget
	}
	
	// Determinar estado del progreso
	switch {
	case savings.Percent >= 100: metrics["status"] = "completed"
	case savings.Percent >= 75: metrics["status"] = "on_track"
	case savings.Percent >= 50: metrics["status"] = "moderate_progress"
	case savings.Percent >= 25: metrics["status"] = "slow_progress"
	default: metrics["status"] = "just_started"
	}
	
	return metrics
}