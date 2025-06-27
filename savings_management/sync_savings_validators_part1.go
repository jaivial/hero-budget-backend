package main

import (
	"fmt"
	"time"
)

// Validadores para sincronización offline de ahorros - Parte 1
// Implementa validaciones específicas para garantizar integridad de datos de savings

// Validate valida un registro de ahorro offline individual
// Verifica que contiene la información mínima requerida según la acción
func (savings *OfflineSavings) ValidateExtended() error {
	// Validar campos básicos siempre requeridos
	if savings.LocalID == "" {
		return fmt.Errorf("local_id es requerido para identificación única")
	}
	if savings.UserID == "" {
		return fmt.Errorf("user_id es requerido para asociación de ahorro")
	}
	
	// Validar acción específica
	if !isValidSavingsAction(savings.Action) {
		return fmt.Errorf("action debe ser add, update o delete")
	}
	
	// Validaciones específicas según el tipo de acción
	switch savings.Action {
	case "add":
		return validateAddSavingsAction(savings)
	case "update":
		return validateUpdateSavingsAction(savings)
	case "delete":
		return validateDeleteSavingsAction(savings)
	default:
		return fmt.Errorf("acción no reconocida: %s", savings.Action)
	}
}

// isValidSavingsAction verifica si la acción es válida para ahorros
func isValidSavingsAction(action string) bool {
	validActions := []string{"add", "update", "delete"}
	for _, validAction := range validActions {
		if action == validAction {
			return true
		}
	}
	return false
}

// validateAddSavingsAction valida los datos requeridos para agregar un nuevo ahorro
func validateAddSavingsAction(savings *OfflineSavings) error {
	if savings.Period == "" {
		return fmt.Errorf("period es requerido para nuevo ahorro")
	}
	
	// Validar que el período sea uno de los valores permitidos
	if !isValidSavingsPeriod(savings.Period) {
		return fmt.Errorf("period debe ser uno de: daily, weekly, monthly, quarterly, semiannual, annual")
	}
	
	// Validar que los montos no sean negativos
	if savings.Available < 0 {
		return fmt.Errorf("available no puede ser negativo")
	}
	if savings.Goal < 0 {
		return fmt.Errorf("goal no puede ser negativo")
	}
	if savings.NeedToSave < 0 {
		return fmt.Errorf("need_to_save no puede ser negativo")
	}
	if savings.DailyTarget < 0 {
		return fmt.Errorf("daily_target no puede ser negativo")
	}
	
	// Validar que el porcentaje esté en rango válido
	if savings.Percent < 0 || savings.Percent > 100 {
		return fmt.Errorf("percent debe estar entre 0 y 100")
	}
	
	// Validar que tenga sentido establecer una meta
	if savings.Goal > 0 && savings.Available > savings.Goal {
		return fmt.Errorf("available (%f) no puede ser mayor que goal (%f) para nuevo ahorro", 
			savings.Available, savings.Goal)
	}
	
	// Validar consistencia matemática básica
	if err := validateSavingsMathConsistency(savings); err != nil {
		return fmt.Errorf("inconsistencia matemática: %v", err)
	}
	
	return nil
}

// validateUpdateSavingsAction valida los datos para actualización de ahorro existente
func validateUpdateSavingsAction(savings *OfflineSavings) error {
	if savings.ServerID == "" {
		return fmt.Errorf("server_id es requerido para actualizar ahorro existente")
	}
	
	// Para updates, los campos pueden ser opcionales, pero si están presentes deben ser válidos
	if savings.Period != "" && !isValidSavingsPeriod(savings.Period) {
		return fmt.Errorf("period debe ser uno de: daily, weekly, monthly, quarterly, semiannual, annual")
	}
	
	// Validar que los montos no sean negativos si están presentes
	if savings.Available < 0 {
		return fmt.Errorf("available no puede ser negativo")
	}
	if savings.Goal < 0 {
		return fmt.Errorf("goal no puede ser negativo")
	}
	if savings.NeedToSave < 0 {
		return fmt.Errorf("need_to_save no puede ser negativo")
	}
	if savings.DailyTarget < 0 {
		return fmt.Errorf("daily_target no puede ser negativo")
	}
	
	// Validar que el porcentaje esté en rango válido si está presente
	if savings.Percent != 0 && (savings.Percent < 0 || savings.Percent > 100) {
		return fmt.Errorf("percent debe estar entre 0 y 100")
	}
	
	// Para updates, solo validamos consistencia si tenemos valores significativos
	if savings.hasSignificantValues() {
		if err := validateSavingsMathConsistency(savings); err != nil {
			return fmt.Errorf("inconsistencia matemática: %v", err)
		}
	}
	
	return nil
}

// validateDeleteSavingsAction valida los datos para eliminación de ahorro
func validateDeleteSavingsAction(savings *OfflineSavings) error {
	if savings.ServerID == "" {
		return fmt.Errorf("server_id es requerido para eliminar ahorro existente")
	}
	
	// Para deletes, solo necesitamos IDs básicos
	return nil
}

// isValidSavingsPeriod verifica si el período especificado es válido para ahorros
func isValidSavingsPeriod(period string) bool {
	validPeriods := []string{
		"daily", "weekly", "monthly", "quarterly", "semiannual", "annual",
	}
	for _, validPeriod := range validPeriods {
		if period == validPeriod {
			return true
		}
	}
	return false
}

// validateSavingsMathConsistency valida la consistencia matemática de los montos del ahorro
// Verifica que las relaciones entre los diferentes montos sean lógicas
func validateSavingsMathConsistency(savings *OfflineSavings) error {
	// Validar que la cantidad necesaria para ahorrar sea coherente
	if savings.Goal > 0 && savings.Available >= 0 {
		expectedNeedToSave := savings.Goal - savings.Available
		if expectedNeedToSave < 0 {
			expectedNeedToSave = 0 // No puede ser negativo
		}
		
		// Permitir pequeñas diferencias por redondeo
		tolerance := 0.01
		if savings.NeedToSave > 0 && absSavings(savings.NeedToSave - expectedNeedToSave) > tolerance {
			return fmt.Errorf("need_to_save (%f) no coincide con goal - available (%f)", 
				savings.NeedToSave, expectedNeedToSave)
		}
	}
	
	// Validar que el porcentaje sea coherente con available y goal
	if savings.Goal > 0 && savings.Available >= 0 {
		expectedPercent := (savings.Available / savings.Goal) * 100
		if expectedPercent > 100 {
			expectedPercent = 100 // Máximo 100%
		}
		
		tolerance := 1.0 // Tolerancia de 1% para redondeo
		if savings.Percent > 0 && absSavings(savings.Percent - expectedPercent) > tolerance {
			return fmt.Errorf("percent (%f) no coincide con el cálculo esperado (%f)", 
				savings.Percent, expectedPercent)
		}
	}
	
	// Validar que el target diario sea razonable
	if savings.NeedToSave > 0 && savings.DailyTarget > 0 {
		// Asumiendo 30 días por mes como estándar
		expectedDailyTarget := savings.NeedToSave / 30
		tolerance := 0.01
		if absSavings(savings.DailyTarget - expectedDailyTarget) > tolerance {
			return fmt.Errorf("daily_target (%f) no coincide con need_to_save/30 (%f)", 
				savings.DailyTarget, expectedDailyTarget)
		}
	}
	
	// El available nunca debería exceder significativamente el goal
	if savings.Goal > 0 && savings.Available > (savings.Goal * 1.1) {
		return fmt.Errorf("available (%f) excede significativamente goal (%f)", 
			savings.Available, savings.Goal)
	}
	
	return nil
}

// hasSignificantValues verifica si el ahorro tiene valores significativos
// Utilizado para determinar si debe validarse la consistencia matemática
func (savings *OfflineSavings) hasSignificantValues() bool {
	return savings.Available > 0 || savings.Goal > 0 || 
		   savings.NeedToSave > 0 || savings.DailyTarget > 0
}

// absSavings retorna el valor absoluto de un número flotante
func absSavings(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// validateSavingsTimestamps valida los timestamps relacionados con sincronización
func validateSavingsTimestamps(savings *OfflineSavings) error {
	// Validar offline timestamp si está presente
	if savings.OfflineTimestamp != "" {
		if _, err := time.Parse(time.RFC3339, savings.OfflineTimestamp); err != nil {
			return fmt.Errorf("formato de offline_timestamp inválido: debe ser RFC3339")
		}
	}
	
	// Validar sync timestamp si está presente
	if savings.SyncTimestamp != "" {
		if _, err := time.Parse(time.RFC3339, savings.SyncTimestamp); err != nil {
			return fmt.Errorf("formato de sync_timestamp inválido: debe ser RFC3339")
		}
	}
	
	// Si ambos timestamps están presentes, sync debe ser posterior a offline
	if savings.OfflineTimestamp != "" && savings.SyncTimestamp != "" {
		offlineTime, _ := time.Parse(time.RFC3339, savings.OfflineTimestamp)
		syncTime, _ := time.Parse(time.RFC3339, savings.SyncTimestamp)
		
		if syncTime.Before(offlineTime) {
			return fmt.Errorf("sync_timestamp no puede ser anterior a offline_timestamp")
		}
	}
	
	return nil
}