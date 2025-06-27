package main

import (
	"fmt"
	"time"
)

// Validadores para sincronización offline de ahorros - Parte 2
// Contiene validaciones avanzadas y lógica específica de metas de ahorro

// validateSavingsGoalLogic valida la lógica específica de metas de ahorro
// Verifica que las metas y períodos sean consistentes entre sí
func validateSavingsGoalLogic(savings *OfflineSavings, existingSavings []SavingsData) error {
	// Verificar que no haya metas conflictivas para el mismo usuario
	for _, existing := range existingSavings {
		if existing.UserID == savings.UserID && existing.Period == savings.Period {
			// Si es el mismo período, verificar que las metas sean compatibles
			if savings.Goal > 0 && existing.Goal > 0 {
				// Verificar que no haya metas excesivamente diferentes
				goalDifference := absSavings(savings.Goal - existing.Goal)
				if goalDifference > existing.Goal * 0.5 { // Más del 50% de diferencia
					return fmt.Errorf("meta de ahorro muy diferente a la existente para período %s", savings.Period)
				}
			}
		}
	}
	
	return nil
}

// validateSavingsBusinessRules valida reglas de negocio específicas para ahorros
// Implementa validaciones que van más allá de la consistencia matemática básica
func validateSavingsBusinessRules(savings *OfflineSavings) error {
	// Regla 1: Una meta debe ser alcanzable en un tiempo razonable
	if savings.Goal > 0 && savings.DailyTarget > 0 {
		daysToGoal := savings.NeedToSave / savings.DailyTarget
		if daysToGoal > 3650 { // Más de 10 años
			return fmt.Errorf("meta de ahorro no alcanzable en tiempo razonable (%.0f días)", daysToGoal)
		}
	}
	
	// Regla 2: El período debe ser apropiado para la meta
	if err := validateGoalPeriodAlignment(savings.Goal, savings.Period); err != nil {
		return fmt.Errorf("período no apropiado para la meta: %v", err)
	}
	
	// Regla 3: El progreso no puede ser negativo
	if savings.Available < 0 {
		return fmt.Errorf("el monto ahorrado no puede ser negativo")
	}
	
	// Regla 4: Validar límites razonables para diferentes períodos
	if err := validateSavingsAmountLimitsForPeriod(savings); err != nil {
		return fmt.Errorf("límites de monto no válidos: %v", err)
	}
	
	// Regla 5: El target diario debe ser realista
	if savings.DailyTarget > savings.Goal * 0.1 { // No más del 10% diario
		return fmt.Errorf("target diario muy alto (%.2f) para meta total (%.2f)", 
			savings.DailyTarget, savings.Goal)
	}
	
	return nil
}

// validateGoalPeriodAlignment verifica que la meta y el período estén alineados
func validateGoalPeriodAlignment(goal float64, period string) error {
	// Definir rangos apropiados de metas por período
	var minGoal, maxGoal float64
	
	switch period {
	case "daily":
		minGoal, maxGoal = 1, 1000     // Rango diario
	case "weekly":
		minGoal, maxGoal = 10, 5000    // Rango semanal
	case "monthly":
		minGoal, maxGoal = 50, 50000   // Rango mensual
	case "quarterly":
		minGoal, maxGoal = 200, 150000 // Rango trimestral
	case "semiannual":
		minGoal, maxGoal = 500, 300000 // Rango semestral
	case "annual":
		minGoal, maxGoal = 1000, 600000 // Rango anual
	default:
		return nil // No validar períodos no estándar
	}
	
	if goal < minGoal {
		return fmt.Errorf("meta (%.2f) muy baja para período %s (mínimo: %.2f)", 
			goal, period, minGoal)
	}
	if goal > maxGoal {
		return fmt.Errorf("meta (%.2f) muy alta para período %s (máximo: %.2f)", 
			goal, period, maxGoal)
	}
	
	return nil
}

// validateSavingsAmountLimitsForPeriod valida que los montos sean razonables para el período
func validateSavingsAmountLimitsForPeriod(savings *OfflineSavings) error {
	// Definir límites máximos razonables por período (en unidades monetarias)
	var maxAmount float64
	
	switch savings.Period {
	case "daily":
		maxAmount = 5000   // Límite diario para ahorros
	case "weekly":
		maxAmount = 25000  // Límite semanal
	case "monthly":
		maxAmount = 100000 // Límite mensual
	case "quarterly":
		maxAmount = 300000 // Límite trimestral
	case "semiannual":
		maxAmount = 600000 // Límite semestral
	case "annual":
		maxAmount = 1200000 // Límite anual
	default:
		return nil // No validar períodos no estándar
	}
	
	// Validar que ningún monto exceda el límite
	if savings.Available > maxAmount {
		return fmt.Errorf("available (%.2f) excede el límite para período %s (%.2f)", 
			savings.Available, savings.Period, maxAmount)
	}
	if savings.Goal > maxAmount {
		return fmt.Errorf("goal (%.2f) excede el límite para período %s (%.2f)", 
			savings.Goal, savings.Period, maxAmount)
	}
	
	return nil
}

// validateSavingsIntegrity realiza validaciones de integridad completas para ahorros
// Combina todas las validaciones para asegurar datos consistentes
func validateSavingsIntegrity(savings *OfflineSavings, context SavingsValidationContext) error {
	// Validaciones básicas
	if err := savings.Validate(); err != nil {
		return fmt.Errorf("validación básica fallida: %v", err)
	}
	
	// Validaciones extendidas
	if err := savings.ValidateExtended(); err != nil {
		return fmt.Errorf("validación extendida fallida: %v", err)
	}
	
	// Validaciones de reglas de negocio
	if err := validateSavingsBusinessRules(savings); err != nil {
		return fmt.Errorf("reglas de negocio fallidas: %v", err)
	}
	
	// Validaciones de metas si hay contexto
	if len(context.ExistingSavings) > 0 {
		if err := validateSavingsGoalLogic(savings, context.ExistingSavings); err != nil {
			return fmt.Errorf("lógica de metas fallida: %v", err)
		}
	}
	
	// Validaciones de timestamps
	if err := validateSavingsTimestamps(savings); err != nil {
		return fmt.Errorf("validación de timestamps fallida: %v", err)
	}
	
	return nil
}

// SavingsValidationContext proporciona contexto adicional para validaciones avanzadas
type SavingsValidationContext struct {
	ExistingSavings []SavingsData           // Ahorros existentes para validar conflictos
	UserPreferences UserSavingsPreferences // Preferencias del usuario
	SystemLimits    SystemSavingsLimits    // Límites del sistema
}

// UserSavingsPreferences define preferencias específicas del usuario para ahorros
type UserSavingsPreferences struct {
	DefaultPeriod         string  `json:"default_period"`         // Período predeterminado
	MaxGoalAmount         float64 `json:"max_goal_amount"`        // Meta máxima permitida
	PreferredDailyTarget  float64 `json:"preferred_daily_target"` // Target diario preferido
	NotificationThreshold float64 `json:"notification_threshold"` // Umbral para notificaciones
	AllowOverGoal         bool    `json:"allow_over_goal"`        // Permitir sobrepasar metas
}

// SystemSavingsLimits define límites globales del sistema para ahorros
type SystemSavingsLimits struct {
	MaxGoalsPerUser       int     `json:"max_goals_per_user"`      // Máximo metas por usuario
	MaxGoalAmountPerType  float64 `json:"max_goal_amount_per_type"` // Monto máximo por tipo de meta
	MinSyncInterval       int     `json:"min_sync_interval"`       // Intervalo mínimo de sincronización
	MaxBatchSize          int     `json:"max_batch_size"`          // Tamaño máximo de lote
	MaxDailyTargetPercent float64 `json:"max_daily_target_percent"` // Porcentaje máximo del target diario
}

// validateUserSavingsLimits valida que el ahorro respete los límites del usuario
func validateUserSavingsLimits(savings *OfflineSavings, preferences UserSavingsPreferences) error {
	// Validar límite máximo de meta del usuario
	if preferences.MaxGoalAmount > 0 && savings.Goal > preferences.MaxGoalAmount {
		return fmt.Errorf("meta (%.2f) excede el límite del usuario (%.2f)", 
			savings.Goal, preferences.MaxGoalAmount)
	}
	
	// Validar target diario preferido
	if preferences.PreferredDailyTarget > 0 && savings.DailyTarget > preferences.PreferredDailyTarget * 2 {
		return fmt.Errorf("target diario (%.2f) muy alto comparado con preferencia (%.2f)", 
			savings.DailyTarget, preferences.PreferredDailyTarget)
	}
	
	// Validar sobrepaso de meta si no está permitido
	if !preferences.AllowOverGoal && savings.Available > savings.Goal && savings.Goal > 0 {
		return fmt.Errorf("sobrepasar meta no permitido para este usuario")
	}
	
	return nil
}

// validateSystemSavingsLimits valida que el ahorro respete los límites del sistema
func validateSystemSavingsLimits(savings *OfflineSavings, limits SystemSavingsLimits) error {
	// Validar monto máximo por tipo
	if limits.MaxGoalAmountPerType > 0 && savings.Goal > limits.MaxGoalAmountPerType {
		return fmt.Errorf("meta (%.2f) excede límite del sistema (%.2f)", 
			savings.Goal, limits.MaxGoalAmountPerType)
	}
	
	// Validar porcentaje máximo de target diario
	if limits.MaxDailyTargetPercent > 0 && savings.Goal > 0 {
		maxDailyTarget := savings.Goal * limits.MaxDailyTargetPercent / 100
		if savings.DailyTarget > maxDailyTarget {
			return fmt.Errorf("target diario (%.2f) excede límite del sistema (%.2f%%)", 
				savings.DailyTarget, limits.MaxDailyTargetPercent)
		}
	}
	
	return nil
}

// validateSavingsProgressionLogic valida la lógica de progresión de ahorros
func validateSavingsProgressionLogic(savings *OfflineSavings) error {
	// Validar que el progreso sea coherente con el tiempo
	if savings.OfflineTimestamp != "" {
		offlineTime, err := time.Parse(time.RFC3339, savings.OfflineTimestamp)
		if err == nil {
			// Calcular días desde el timestamp offline
			daysSince := time.Since(offlineTime).Hours() / 24
			
			// Validar que el progreso sea realista basado en el tiempo transcurrido
			if savings.DailyTarget > 0 && daysSince > 0 {
				expectedProgress := savings.DailyTarget * daysSince
				actualProgress := savings.Available
				
				// Permitir hasta 50% de variación
				if actualProgress > expectedProgress * 1.5 {
					return fmt.Errorf("progreso de ahorro (%.2f) demasiado alto para el tiempo transcurrido", 
						actualProgress)
				}
			}
		}
	}
	
	return nil
}