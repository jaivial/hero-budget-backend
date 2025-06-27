package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Funciones de base de datos para Cash Bank Management - Parte 1
// Implementan operaciones CRUD principales para efectivo y banco
// Incluyen consultas de distribución y funciones de obtención de datos

// fetchCashBankDistribution obtiene la distribución actual de efectivo/banco para un usuario
// Consulta datos más recientes disponibles y calcula porcentajes
// Retorna estructura completa con cantidades y porcentajes calculados
func fetchCashBankDistribution(userID string) (CashBankDistribution, error) {
	var distribution CashBankDistribution
	distribution.UserID = userID

	// Get current month in format YYYY-MM para consulta
	currentMonth := time.Now().Format("2006-01")

	// Query monthly_cash_bank_balance data from database for current month
	// Busca datos del mes actual primero para obtener información más relevante
	err := db.QueryRow(`
		SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, userID, currentMonth).Scan(
		&distribution.Month,
		&distribution.CashAmount,
		&distribution.BankAmount,
		&distribution.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// If no data for current month, try to get the most recent month
		// Si no hay datos para el mes actual, buscar el mes más reciente disponible
		err = db.QueryRow(`
			SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
			FROM monthly_cash_bank_balance
			WHERE user_id = ?
			ORDER BY year_month DESC, updated_at DESC
			LIMIT 1
		`, userID).Scan(
			&distribution.Month,
			&distribution.CashAmount,
			&distribution.BankAmount,
			&distribution.MonthlyTotal,
		)

		if err == sql.ErrNoRows {
			// Return default values if no data found
			// Retornar valores por defecto si no se encuentran datos
			now := time.Now()
			distribution.Month = now.Format("January 2006")
			distribution.CashAmount = 0
			distribution.CashPercent = 0
			distribution.BankAmount = 0
			distribution.BankPercent = 0
			distribution.MonthlyTotal = 0
			return distribution, nil
		} else if err != nil {
			return distribution, err
		}
	} else if err != nil {
		return distribution, err
	}

	// Calculate percentages based on amounts
	// Calcular porcentajes basados en las cantidades obtenidas
	if distribution.MonthlyTotal > 0 {
		distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
		distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
	} else {
		distribution.CashPercent = 0
		distribution.BankPercent = 0
	}

	return distribution, nil
}

// updateCashBankDistribution actualiza distribución en todas las tablas periódicas
// Mantiene consistencia entre tablas diarias, semanales, mensuales, etc.
// Incluye tabla legacy para compatibilidad con versiones anteriores
func updateCashBankDistribution(distribution CashBankDistribution) error {
	// Get current date and time periods for all table updates
	// Obtener fecha actual y todos los períodos para actualizar tablas
	now := time.Now()
	currentDate := now.Format("2006-01-02")
	currentMonth := now.Format("2006-01")
	currentWeek := getWeekPeriod(now)
	currentQuarter := getQuarterPeriod(now)
	currentSemiannual := getSemiannualPeriod(now)
	currentYear := now.Format("2006")

	// Update all period tables with new distribution
	// Actualizar todas las tablas periódicas con nueva distribución
	err := updateAllPeriodTables(distribution, currentDate, currentMonth, currentWeek, currentQuarter, currentSemiannual, currentYear)
	if err != nil {
		return err
	}

	// Also update the legacy cash_bank table for backward compatibility
	// También actualizar tabla legacy cash_bank para compatibilidad
	var legacyCount int
	err2 := db.QueryRow(`
		SELECT COUNT(*) 
		FROM cash_bank 
		WHERE user_id = ?
	`, distribution.UserID).Scan(&legacyCount)

	if err2 == nil {
		if legacyCount > 0 {
			// Update existing cash_bank entry
			// Actualizar entrada existente en cash_bank
			db.Exec(`
				UPDATE cash_bank
				SET month = ?,
					cash_amount = ?,
					cash_percent = ?,
					bank_amount = ?,
					bank_percent = ?,
					monthly_total = ?,
					updated_at = CURRENT_TIMESTAMP
				WHERE user_id = ?
			`,
				distribution.Month,
				distribution.CashAmount,
				distribution.CashPercent,
				distribution.BankAmount,
				distribution.BankPercent,
				distribution.MonthlyTotal,
				distribution.UserID,
			)
		} else {
			// Insert new cash_bank entry
			// Insertar nueva entrada en cash_bank
			db.Exec(`
				INSERT INTO cash_bank (
					user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total
				) VALUES (?, ?, ?, ?, ?, ?, ?)
			`,
				distribution.UserID,
				distribution.Month,
				distribution.CashAmount,
				distribution.CashPercent,
				distribution.BankAmount,
				distribution.BankPercent,
				distribution.MonthlyTotal,
			)
		}
	}

	return err
}

// Helper functions for period calculations
// Funciones auxiliares para cálculos de períodos
// Estas funciones generan identificadores únicos para cada período

// getWeekPeriod retorna período semanal en formato YYYY-WW
func getWeekPeriod(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-%02d", year, week)
}

// getQuarterPeriod retorna período trimestral en formato YYYY-Q
func getQuarterPeriod(t time.Time) string {
	quarter := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d-%d", t.Year(), quarter)
}

// getSemiannualPeriod retorna período semestral en formato YYYY-H
func getSemiannualPeriod(t time.Time) string {
	semiannual := (int(t.Month())-1)/6 + 1
	return fmt.Sprintf("%d-%d", t.Year(), semiannual)
}

// updateAllPeriodTables actualiza todas las tablas periódicas con nueva distribución
// Mantiene consistencia entre todos los períodos de tiempo
// Utiliza función genérica para evitar duplicación de código
func updateAllPeriodTables(distribution CashBankDistribution, currentDate, currentMonth, currentWeek, currentQuarter, currentSemiannual, currentYear string) error {
	// Update daily_cash_bank_balance
	// Actualizar balance diario
	err := updatePeriodTable("daily_cash_bank_balance", "date", currentDate, distribution)
	if err != nil {
		log.Printf("Error updating daily_cash_bank_balance: %v", err)
		return err
	}

	// Update weekly_cash_bank_balance
	// Actualizar balance semanal
	err = updatePeriodTable("weekly_cash_bank_balance", "year_week", currentWeek, distribution)
	if err != nil {
		log.Printf("Error updating weekly_cash_bank_balance: %v", err)
		return err
	}

	// Update monthly_cash_bank_balance
	// Actualizar balance mensual
	err = updatePeriodTable("monthly_cash_bank_balance", "year_month", currentMonth, distribution)
	if err != nil {
		log.Printf("Error updating monthly_cash_bank_balance: %v", err)
		return err
	}

	// Update quarterly_cash_bank_balance
	// Actualizar balance trimestral
	err = updatePeriodTable("quarterly_cash_bank_balance", "year_quarter", currentQuarter, distribution)
	if err != nil {
		log.Printf("Error updating quarterly_cash_bank_balance: %v", err)
		return err
	}

	// Update semiannual_cash_bank_balance
	// Actualizar balance semestral
	err = updatePeriodTable("semiannual_cash_bank_balance", "year_half", currentSemiannual, distribution)
	if err != nil {
		log.Printf("Error updating semiannual_cash_bank_balance: %v", err)
		return err
	}

	// Update annual_cash_bank_balance
	// Actualizar balance anual
	err = updatePeriodTable("annual_cash_bank_balance", "year", currentYear, distribution)
	if err != nil {
		log.Printf("Error updating annual_cash_bank_balance: %v", err)
		return err
	}

	return nil
}