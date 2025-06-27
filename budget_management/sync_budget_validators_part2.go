package main

import (
	"fmt"
	"time"
)

// Validadores para sincronización offline de presupuestos - Parte 2
// Contiene validaciones avanzadas y lógica de períodos

// validateBudgetPeriodLogic valida la lógica específica de períodos de presupuesto
// Verifica que los períodos y fechas sean consistentes entre sí
func validateBudgetPeriodLogic(budget *OfflineBudget, existingBudgets []BudgetData) error {
	// Verificar que no haya solapamiento de períodos para el mismo usuario
	for _, existing := range existingBudgets {
		if existing.UserID == budget.UserID && existing.Period == budget.Period {
			// Si es el mismo período, verificar que no haya conflictos de fechas
			if budget.Date != "" && existing.Date != "" {
				budgetDate, err1 := time.Parse("2006-01-02", budget.Date)
				existingDate, err2 := time.Parse("2006-01-02", existing.Date)
				
				if err1 == nil && err2 == nil {
					// Verificar solapamiento basado en el tipo de período
					if hasDateOverlap(budgetDate, existingDate, budget.Period) {
						return fmt.Errorf("solapamiento de fechas detectado para período %s", budget.Period)
					}
				}
			}
		}
	}
	
	return nil
}

// hasDateOverlap verifica si dos fechas se solapan para un período dado
func hasDateOverlap(date1, date2 time.Time, period string) bool {
	// Implementación simplificada - se puede expandir según necesidades específicas
	switch period {
	case "daily":
		return date1.Format("2006-01-02") == date2.Format("2006-01-02")
	case "weekly":
		// Misma semana del año
		year1, week1 := date1.ISOWeek()
		year2, week2 := date2.ISOWeek()
		return year1 == year2 && week1 == week2
	case "monthly":
		return date1.Year() == date2.Year() && date1.Month() == date2.Month()
	case "quarterly":
		quarter1 := (int(date1.Month())-1)/3 + 1
		quarter2 := (int(date2.Month())-1)/3 + 1
		return date1.Year() == date2.Year() && quarter1 == quarter2
	case "semiannual":
		half1 := (int(date1.Month())-1)/6 + 1
		half2 := (int(date2.Month())-1)/6 + 1
		return date1.Year() == date2.Year() && half1 == half2
	case "annual":
		return date1.Year() == date2.Year()
	}
	
	return false
}

// validateBudgetBusinessRules valida reglas de negocio específicas para presupuestos
// Implementa validaciones que van más allá de la consistencia matemática básica
func validateBudgetBusinessRules(budget *OfflineBudget) error {
	// Regla 1: Un presupuesto no puede tener gastos sin ingresos o herencia
	if budget.SpentAmount > 0 && budget.TotalIncome == 0 && budget.FromPrevious == 0 {
		return fmt.Errorf("no se pueden registrar gastos sin ingresos o herencia del período anterior")
	}
	
	// Regla 2: El período debe ser apropiado para la fecha especificada
	if budget.Date != "" {
		if err := validatePeriodDateAlignment(budget.Period, budget.Date); err != nil {
			return fmt.Errorf("período no alineado con fecha: %v", err)
		}
	}
	
	// Regla 3: Los montos futuros no pueden exceder significativamente el presupuesto
	if budget.UpcomingAmount > budget.TotalAmount * 1.5 && budget.TotalAmount > 0 {
		return fmt.Errorf("upcoming_amount (%.2f) excede significativamente el presupuesto total (%.2f)", 
			budget.UpcomingAmount, budget.TotalAmount)
	}
	
	// Regla 4: Validar límites razonables para diferentes períodos
	if err := validateAmountLimitsForPeriod(budget); err != nil {
		return fmt.Errorf("límites de monto no válidos: %v", err)
	}
	
	return nil
}

// validatePeriodDateAlignment verifica que el período y la fecha estén alineados
func validatePeriodDateAlignment(period, dateStr string) error {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("formato de fecha inválido")
	}
	
	now := time.Now()
	
	switch period {
	case "daily":
		// La fecha debe ser del día actual o pasado reciente
		if date.After(now.AddDate(0, 0, 1)) {
			return fmt.Errorf("fecha muy futura para período diario")
		}
	case "weekly":
		// La fecha debe estar dentro de la semana actual o pasada reciente
		_, currentWeek := now.ISOWeek()
		_, dateWeek := date.ISOWeek()
		if dateWeek > currentWeek+1 {
			return fmt.Errorf("fecha muy futura para período semanal")
		}
	case "monthly":
		// La fecha debe estar dentro del mes actual o pasado reciente
		if date.Year() > now.Year() || (date.Year() == now.Year() && date.Month() > now.Month()+1) {
			return fmt.Errorf("fecha muy futura para período mensual")
		}
	}
	
	return nil
}

// validateAmountLimitsForPeriod valida que los montos sean razonables para el período
func validateAmountLimitsForPeriod(budget *OfflineBudget) error {
	// Definir límites máximos razonables por período (en unidades monetarias)
	var maxAmount float64
	
	switch budget.Period {
	case "daily":
		maxAmount = 10000 // Límite diario
	case "weekly":
		maxAmount = 50000 // Límite semanal
	case "monthly":
		maxAmount = 200000 // Límite mensual
	case "quarterly":
		maxAmount = 600000 // Límite trimestral
	case "semiannual":
		maxAmount = 1200000 // Límite semestral
	case "annual":
		maxAmount = 2400000 // Límite anual
	default:
		return nil // No validar períodos no estándar
	}
	
	// Validar que ningún monto exceda el límite
	if budget.TotalAmount > maxAmount {
		return fmt.Errorf("total_amount (%.2f) excede el límite para período %s (%.2f)", 
			budget.TotalAmount, budget.Period, maxAmount)
	}
	if budget.SpentAmount > maxAmount {
		return fmt.Errorf("spent_amount (%.2f) excede el límite para período %s (%.2f)", 
			budget.SpentAmount, budget.Period, maxAmount)
	}
	if budget.TotalIncome > maxAmount {
		return fmt.Errorf("total_income (%.2f) excede el límite para período %s (%.2f)", 
			budget.TotalIncome, budget.Period, maxAmount)
	}
	
	return nil
}

// validateBudgetIntegrity realiza validaciones de integridad completas
// Combina todas las validaciones para asegurar datos consistentes
func validateBudgetIntegrity(budget *OfflineBudget, context ValidationContext) error {
	// Validaciones básicas
	if err := budget.Validate(); err != nil {
		return fmt.Errorf("validación básica fallida: %v", err)
	}
	
	// Validaciones de reglas de negocio
	if err := validateBudgetBusinessRules(budget); err != nil {
		return fmt.Errorf("reglas de negocio fallidas: %v", err)
	}
	
	// Validaciones de período si hay contexto
	if len(context.ExistingBudgets) > 0 {
		if err := validateBudgetPeriodLogic(budget, context.ExistingBudgets); err != nil {
			return fmt.Errorf("lógica de período fallida: %v", err)
		}
	}
	
	return nil
}

// ValidationContext proporciona contexto adicional para validaciones avanzadas
type ValidationContext struct {
	ExistingBudgets []BudgetData // Presupuestos existentes para validar conflictos
	UserPreferences UserBudgetPreferences // Preferencias del usuario
	SystemLimits    SystemBudgetLimits // Límites del sistema
}

// UserBudgetPreferences define preferencias específicas del usuario para presupuestos
type UserBudgetPreferences struct {
	DefaultPeriod        string  `json:"default_period"`        // Período predeterminado
	MaxBudgetAmount      float64 `json:"max_budget_amount"`     // Límite máximo de presupuesto
	AllowOverBudget      bool    `json:"allow_over_budget"`     // Permitir sobrepresupuesto
	NotificationThreshold float64 `json:"notification_threshold"` // Umbral para notificaciones
}

// SystemBudgetLimits define límites globales del sistema para presupuestos
type SystemBudgetLimits struct {
	MaxBudgetsPerUser    int     `json:"max_budgets_per_user"`    // Máximo presupuestos por usuario
	MaxAmountPerPeriod   float64 `json:"max_amount_per_period"`   // Monto máximo por período
	MinSyncInterval      int     `json:"min_sync_interval"`       // Intervalo mínimo de sincronización
	MaxBatchSize         int     `json:"max_batch_size"`          // Tamaño máximo de lote
}

// validateUserLimits valida que el presupuesto respete los límites del usuario
func validateUserLimits(budget *OfflineBudget, preferences UserBudgetPreferences) error {
	// Validar límite máximo del usuario
	if preferences.MaxBudgetAmount > 0 && budget.TotalAmount > preferences.MaxBudgetAmount {
		return fmt.Errorf("presupuesto (%.2f) excede el límite del usuario (%.2f)", 
			budget.TotalAmount, preferences.MaxBudgetAmount)
	}
	
	// Validar sobrepresupuesto si no está permitido
	if !preferences.AllowOverBudget && budget.RemainingAmount < 0 {
		return fmt.Errorf("sobrepresupuesto no permitido para este usuario")
	}
	
	return nil
}