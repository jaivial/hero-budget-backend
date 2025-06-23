package common

import (
	"fmt"
)

// Archivo de invalidación de cache - contiene métodos especializados para invalidar cache
// Se encarga de eliminar datos específicos del cache Redis cuando se actualizan

// InvalidateUserCache invalida cache relacionado con un usuario específico
func (cm *CacheManager) InvalidateUserCache(userID string) error {
	patterns := []string{
		fmt.Sprintf("user:%s*", userID),
		fmt.Sprintf("income:%s*", userID),
		fmt.Sprintf("expense:%s*", userID),
		fmt.Sprintf("bills:%s*", userID),
		fmt.Sprintf("dashboard:%s*", userID),
		fmt.Sprintf("savings:%s*", userID),
		fmt.Sprintf("cashbank:%s*", userID),
	}

	for _, pattern := range patterns {
		err := cm.redis.DeletePattern(pattern)
		if err != nil {
			return fmt.Errorf("error invalidando cache del usuario %s: %v", userID, err)
		}
	}

	return nil
}

// InvalidateIncomeCache invalida cache de ingresos específico
func (cm *CacheManager) InvalidateIncomeCache(userID string, periods ...string) error {
	if len(periods) == 0 {
		// Invalidar todos los períodos de ingresos
		pattern := fmt.Sprintf("income:%s*", userID)
		return cm.redis.DeletePattern(pattern)
	}

	// Invalidar períodos específicos
	for _, period := range periods {
		key := KB.BuildIncomeKey(userID, period)
		err := cm.redis.Delete(key)
		if err != nil {
			return fmt.Errorf("error invalidando cache de ingresos: %v", err)
		}
	}

	// También invalidar dashboard relacionado
	return cm.InvalidateDashboardCache(userID, periods...)
}

// InvalidateDashboardCache invalida cache del dashboard específico
func (cm *CacheManager) InvalidateDashboardCache(userID string, periods ...string) error {
	if len(periods) == 0 {
		// Invalidar todos los períodos del dashboard
		pattern := fmt.Sprintf("dashboard:%s*", userID)
		return cm.redis.DeletePattern(pattern)
	}

	// Invalidar períodos específicos
	for _, period := range periods {
		key := KB.BuildDashboardKey(userID, period)
		err := cm.redis.Delete(key)
		if err != nil {
			return fmt.Errorf("error invalidando cache del dashboard: %v", err)
		}
	}

	return nil
}

// InvalidateExpenseCache invalida cache de gastos específico
func (cm *CacheManager) InvalidateExpenseCache(userID string, periods ...string) error {
	if len(periods) == 0 {
		pattern := fmt.Sprintf("expense:%s*", userID)
		return cm.redis.DeletePattern(pattern)
	}

	for _, period := range periods {
		key := KB.BuildExpenseKey(userID, period)
		err := cm.redis.Delete(key)
		if err != nil {
			return fmt.Errorf("error invalidando cache de gastos: %v", err)
		}
	}

	// También invalidar dashboard relacionado
	return cm.InvalidateDashboardCache(userID, periods...)
}

// InvalidateBillsCache invalida cache de facturas específico
func (cm *CacheManager) InvalidateBillsCache(userID string, periods ...string) error {
	if len(periods) == 0 {
		pattern := fmt.Sprintf("bills:%s*", userID)
		return cm.redis.DeletePattern(pattern)
	}

	for _, period := range periods {
		key := KB.BuildBillsKey(userID, period)
		err := cm.redis.Delete(key)
		if err != nil {
			return fmt.Errorf("error invalidando cache de facturas: %v", err)
		}
	}

	// También invalidar dashboard relacionado
	return cm.InvalidateDashboardCache(userID, periods...)
}

// InvalidateSavingsCache invalida cache de ahorros específico
// Elimina datos de ahorros del cache para un usuario específico
func (cm *CacheManager) InvalidateSavingsCache(userID string) error {
	pattern := fmt.Sprintf("savings:%s*", userID)
	return cm.redis.DeletePattern(pattern)
}

// InvalidateCashBankCache invalida cache de distribución cash/bank específico
// Elimina datos de distribución cash/bank del cache para un usuario específico
func (cm *CacheManager) InvalidateCashBankCache(userID string) error {
	pattern := fmt.Sprintf("cashbank:%s*", userID)
	return cm.redis.DeletePattern(pattern)
}