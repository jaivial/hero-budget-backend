package main

import (
	"fmt"
	"time"
)

// Validadores para sincronización offline de presupuestos - Parte 1
// Implementa validaciones específicas para garantizar integridad de datos de presupuestos

// Validate valida un presupuesto offline individual
// Verifica que contiene la información mínima requerida según la acción
func (budget *OfflineBudget) Validate() error {
	// Validar campos básicos siempre requeridos
	if budget.LocalID == "" {
		return fmt.Errorf("local_id es requerido para identificación única")
	}
	if budget.UserID == "" {
		return fmt.Errorf("user_id es requerido para asociación de presupuesto")
	}
	
	// Validar acción específica
	if !isValidBudgetAction(budget.Action) {
		return fmt.Errorf("action debe ser add, update o delete")
	}
	
	// Validaciones específicas según el tipo de acción
	switch budget.Action {
	case "add":
		return validateAddBudgetAction(budget)
	case "update":
		return validateUpdateBudgetAction(budget)
	case "delete":
		return validateDeleteBudgetAction(budget)
	default:
		return fmt.Errorf("acción no reconocida: %s", budget.Action)
	}
}

// isValidBudgetAction verifica si la acción es válida para presupuestos
func isValidBudgetAction(action string) bool {
	validActions := []string{"add", "update", "delete"}
	for _, validAction := range validActions {
		if action == validAction {
			return true
		}
	}
	return false
}

// validateAddBudgetAction valida los datos requeridos para agregar un nuevo presupuesto
func validateAddBudgetAction(budget *OfflineBudget) error {
	if budget.Period == "" {
		return fmt.Errorf("period es requerido para nuevo presupuesto")
	}
	
	// Validar que el período sea uno de los valores permitidos
	if !isValidPeriod(budget.Period) {
		return fmt.Errorf("period debe ser uno de: daily, weekly, monthly, quarterly, semiannual, annual")
	}
	
	// Validar que los montos no sean negativos
	if budget.TotalAmount < 0 {
		return fmt.Errorf("total_amount no puede ser negativo")
	}
	if budget.SpentAmount < 0 {
		return fmt.Errorf("spent_amount no puede ser negativo")
	}
	if budget.UpcomingAmount < 0 {
		return fmt.Errorf("upcoming_amount no puede ser negativo")
	}
	if budget.FromPrevious < 0 {
		return fmt.Errorf("from_previous no puede ser negativo")
	}
	if budget.TotalIncome < 0 {
		return fmt.Errorf("total_income no puede ser negativo")
	}
	
	// Validar que el porcentaje esté en rango válido
	if budget.Percent < 0 || budget.Percent > 100 {
		return fmt.Errorf("percent debe estar entre 0 y 100")
	}
	
	// Validar formato de fecha si está presente
	if budget.Date != "" {
		if _, err := time.Parse("2006-01-02", budget.Date); err != nil {
			return fmt.Errorf("formato de date inválido: debe ser YYYY-MM-DD")
		}
	}
	
	// Validar consistencia matemática básica
	if err := validateBudgetMathConsistency(budget); err != nil {
		return fmt.Errorf("inconsistencia matemática: %v", err)
	}
	
	return nil
}

// validateUpdateBudgetAction valida los datos para actualización de presupuesto existente
func validateUpdateBudgetAction(budget *OfflineBudget) error {
	if budget.ServerID == "" {
		return fmt.Errorf("server_id es requerido para actualizar presupuesto existente")
	}
	
	// Para updates, los campos pueden ser opcionales, pero si están presentes deben ser válidos
	if budget.Period != "" && !isValidPeriod(budget.Period) {
		return fmt.Errorf("period debe ser uno de: daily, weekly, monthly, quarterly, semiannual, annual")
	}
	
	// Validar que los montos no sean negativos si están presentes
	if budget.TotalAmount < 0 {
		return fmt.Errorf("total_amount no puede ser negativo")
	}
	if budget.SpentAmount < 0 {
		return fmt.Errorf("spent_amount no puede ser negativo")
	}
	if budget.UpcomingAmount < 0 {
		return fmt.Errorf("upcoming_amount no puede ser negativo")
	}
	if budget.FromPrevious < 0 {
		return fmt.Errorf("from_previous no puede ser negativo")
	}
	if budget.TotalIncome < 0 {
		return fmt.Errorf("total_income no puede ser negativo")
	}
	
	// Validar que el porcentaje esté en rango válido si está presente
	if budget.Percent != 0 && (budget.Percent < 0 || budget.Percent > 100) {
		return fmt.Errorf("percent debe estar entre 0 y 100")
	}
	
	// Validar formato de fecha si está presente
	if budget.Date != "" {
		if _, err := time.Parse("2006-01-02", budget.Date); err != nil {
			return fmt.Errorf("formato de date inválido: debe ser YYYY-MM-DD")
		}
	}
	
	// Para updates, solo validamos consistencia si tenemos valores no-cero
	if budget.hasNonZeroAmounts() {
		if err := validateBudgetMathConsistency(budget); err != nil {
			return fmt.Errorf("inconsistencia matemática: %v", err)
		}
	}
	
	return nil
}

// validateDeleteBudgetAction valida los datos para eliminación de presupuesto
func validateDeleteBudgetAction(budget *OfflineBudget) error {
	if budget.ServerID == "" {
		return fmt.Errorf("server_id es requerido para eliminar presupuesto existente")
	}
	
	// Para deletes, solo necesitamos IDs básicos
	return nil
}

// isValidPeriod verifica si el período especificado es válido
func isValidPeriod(period string) bool {
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

// validateBudgetMathConsistency valida la consistencia matemática de los montos del presupuesto
// Verifica que las relaciones entre los diferentes montos sean lógicas
func validateBudgetMathConsistency(budget *OfflineBudget) error {
	// El monto total debe ser al menos la suma de los montos base
	expectedTotal := budget.FromPrevious + budget.TotalIncome
	if budget.TotalAmount > 0 && expectedTotal > 0 {
		// Permitir pequeñas diferencias por redondeo
		tolerance := 0.01
		if abs(budget.TotalAmount - expectedTotal) > tolerance {
			return fmt.Errorf("total_amount (%f) no coincide con from_previous + total_income (%f)", 
				budget.TotalAmount, expectedTotal)
		}
	}
	
	// El monto restante debe ser coherente
	expectedRemaining := budget.TotalAmount - budget.SpentAmount - budget.UpcomingAmount
	if budget.RemainingAmount > 0 && expectedRemaining != 0 {
		tolerance := 0.01
		if abs(budget.RemainingAmount - expectedRemaining) > tolerance {
			return fmt.Errorf("remaining_amount (%f) no coincide con el cálculo esperado (%f)", 
				budget.RemainingAmount, expectedRemaining)
		}
	}
	
	// Los gastos no pueden exceder el total disponible
	totalSpent := budget.SpentAmount + budget.UpcomingAmount
	if totalSpent > budget.TotalAmount && budget.TotalAmount > 0 {
		return fmt.Errorf("spent_amount + upcoming_amount (%f) excede total_amount (%f)", 
			totalSpent, budget.TotalAmount)
	}
	
	return nil
}

// hasNonZeroAmounts verifica si el presupuesto tiene montos no-cero
// Utilizado para determinar si debe validarse la consistencia matemática
func (budget *OfflineBudget) hasNonZeroAmounts() bool {
	return budget.TotalAmount != 0 || budget.SpentAmount != 0 || 
		   budget.UpcomingAmount != 0 || budget.FromPrevious != 0 || 
		   budget.TotalIncome != 0
}

// abs retorna el valor absoluto de un número flotante
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}