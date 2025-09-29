package main

import (
	"fmt"
	"time"
)

// Funciones auxiliares de validación para sincronización offline
// Proporcionan validación básica para estructuras de sincronización
// Mantienen consistencia de datos durante operaciones offline

// validateCashBankDistributionConsistency valida consistencia básica de distribución
// Verifica que los datos de efectivo y banco sean matemáticamente consistentes
func validateCashBankDistributionConsistency(distribution OfflineCashBankDistribution) error {
	// Validar consistencia matemática entre cantidades y total
	expectedTotal := distribution.CashAmount + distribution.BankAmount
	tolerance := 0.01 // Tolerancia para errores de punto flotante

	if abs(expectedTotal-distribution.MonthlyTotal) > tolerance {
		return fmt.Errorf("inconsistencia en totales: suma(%f + %f = %f) != monthly_total(%f)",
			distribution.CashAmount, distribution.BankAmount, expectedTotal, distribution.MonthlyTotal)
	}

	// Validar que las cantidades no sean negativas
	if distribution.CashAmount < 0 {
		return fmt.Errorf("cash_amount no puede ser negativo: %f", distribution.CashAmount)
	}

	if distribution.BankAmount < 0 {
		return fmt.Errorf("bank_amount no puede ser negativo: %f", distribution.BankAmount)
	}

	// Validar consistencia de porcentajes si hay total diferente de cero
	if distribution.MonthlyTotal > 0 {
		expectedCashPercent := (distribution.CashAmount / distribution.MonthlyTotal) * 100
		expectedBankPercent := (distribution.BankAmount / distribution.MonthlyTotal) * 100

		// Tolerancia para porcentajes (0.5%)
		percentTolerance := 0.5

		if abs(expectedCashPercent-distribution.CashPercent) > percentTolerance {
			return fmt.Errorf("inconsistencia en cash_percent: esperado %f, recibido %f",
				expectedCashPercent, distribution.CashPercent)
		}

		if abs(expectedBankPercent-distribution.BankPercent) > percentTolerance {
			return fmt.Errorf("inconsistencia en bank_percent: esperado %f, recibido %f",
				expectedBankPercent, distribution.BankPercent)
		}
	}

	return nil
}

// abs retorna el valor absoluto de un número flotante
// Función auxiliar para validaciones de tolerancia
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// validateUserAccess valida que el usuario existe y tiene permisos básicos
// Verificación simplificada para demo purposes
func validateUserAccess(userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// Aquí se pueden agregar validaciones adicionales
	// Por ahora, validación básica de formato
	if len(userID) < 3 {
		return fmt.Errorf("user ID too short")
	}

	return nil
}

// validateTimestamp valida formato de timestamp RFC3339
func validateTimestamp(timestamp string) error {
	if timestamp == "" {
		return nil // Timestamp vacío es válido (opcional)
	}

	_, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %s", timestamp)
	}

	return nil
}
