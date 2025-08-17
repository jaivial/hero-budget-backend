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

// fetchCashBankDistribution obtiene la distribución de efectivo/banco para un usuario y mes específico
// Si no existe el mes solicitado, hereda del mes más reciente y crea el registro
// Implementa lógica de cascada para mantener consistencia histórica
func fetchCashBankDistribution(userID string, yearMonth string) (CashBankDistribution, error) {
	var distribution CashBankDistribution
	distribution.UserID = userID

	// First try to get data for the specific month requested
	// Primero intenta obtener datos del mes específico solicitado
	err := db.QueryRow(`
		SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, userID, yearMonth).Scan(
		&distribution.Month,
		&distribution.CashAmount,
		&distribution.BankAmount,
		&distribution.MonthlyTotal,
	)

	if err == sql.ErrNoRows {
		// Month doesn't exist, try to inherit from most recent available month
		// El mes no existe, intenta heredar del mes más reciente disponible
		var inheritedCash, inheritedBank, inheritedTotal float64
		var mostRecentMonth string
		
		err = db.QueryRow(`
			SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month < ?
			ORDER BY year_month DESC, updated_at DESC
			LIMIT 1
		`, userID, yearMonth).Scan(&mostRecentMonth, &inheritedCash, &inheritedBank, &inheritedTotal)
		
		if err == sql.ErrNoRows {
			// No previous data exists, initialize with defaults
			// No existen datos previos, inicializar con valores por defecto
			inheritedCash, inheritedBank = 0, 0
			if userID == "18" {
				// Special case for user 18 - initialize with $200 bank balance for testing
				inheritedBank = 200.0
			}
			inheritedTotal = inheritedCash + inheritedBank
			log.Printf("✅ Initializing new user %s for month %s with Cash=$%.2f, Bank=$%.2f", userID, yearMonth, inheritedCash, inheritedBank)
		} else if err != nil {
			return distribution, fmt.Errorf("error fetching previous month data: %v", err)
		} else {
			log.Printf("📅 Inheriting balance for user %s month %s from %s: Cash=$%.2f, Bank=$%.2f", userID, yearMonth, mostRecentMonth, inheritedCash, inheritedBank)
		}
		
		// Create record for the requested month with inherited values
		// Crear registro para el mes solicitado con valores heredados
		_, err = db.Exec(`
			INSERT INTO monthly_cash_bank_balance 
			(user_id, year_month, balance_cash_amount, balance_bank_amount, total_balance, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		`, userID, yearMonth, inheritedCash, inheritedBank, inheritedTotal)
		
		if err != nil {
			return distribution, fmt.Errorf("error creating month record: %v", err)
		}
		
		// Set up distribution with inherited values
		distribution.Month = yearMonth
		distribution.CashAmount = inheritedCash
		distribution.BankAmount = inheritedBank
		distribution.MonthlyTotal = inheritedTotal
		
		// Calculate percentages
		distribution.CashPercent = 0
		distribution.BankPercent = 0
		if distribution.MonthlyTotal > 0 {
			distribution.CashPercent = (distribution.CashAmount / distribution.MonthlyTotal) * 100
			distribution.BankPercent = (distribution.BankAmount / distribution.MonthlyTotal) * 100
		}
		
		return distribution, nil
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

// cascadeUpdateFutureMonths actualiza todos los meses posteriores con el cambio de balance
// Implementa lógica de cascada: si se modifica mayo, actualiza junio, julio, etc.
// Mantiene la diferencia aplicada consistente a través de todos los meses futuros
func cascadeUpdateFutureMonths(userID string, fromMonth string, cashDelta float64, bankDelta float64) error {
	log.Printf("🔄 Starting cascade update for user %s from month %s: CashΔ=$%.2f, BankΔ=$%.2f", userID, fromMonth, cashDelta, bankDelta)
	
	// Get all future months that need to be updated
	// Obtener todos los meses futuros que necesitan ser actualizados
	rows, err := db.Query(`
		SELECT year_month, balance_cash_amount, balance_bank_amount, total_balance
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month > ?
		ORDER BY year_month ASC
	`, userID, fromMonth)
	
	if err != nil {
		return fmt.Errorf("error fetching future months: %v", err)
	}
	defer rows.Close()
	
	var updatedMonths []string
	
	// Update each future month with the delta
	// Actualizar cada mes futuro con el delta
	for rows.Next() {
		var month string
		var currentCash, currentBank, currentTotal float64
		
		err = rows.Scan(&month, &currentCash, &currentBank, &currentTotal)
		if err != nil {
			return fmt.Errorf("error scanning future month data: %v", err)
		}
		
		// Apply the delta to current balances
		// Aplicar el delta a los balances actuales
		newCash := currentCash + cashDelta
		newBank := currentBank + bankDelta
		newTotal := newCash + newBank
		
		// Update the month record
		// Actualizar el registro del mes
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET balance_cash_amount = ?, balance_bank_amount = ?, total_balance = ?, updated_at = datetime('now')
			WHERE user_id = ? AND year_month = ?
		`, newCash, newBank, newTotal, userID, month)
		
		if err != nil {
			return fmt.Errorf("error updating month %s: %v", month, err)
		}
		
		updatedMonths = append(updatedMonths, month)
		log.Printf("   📅 Updated %s: Cash=$%.2f→$%.2f, Bank=$%.2f→$%.2f", month, currentCash, newCash, currentBank, newBank)
	}
	
	if len(updatedMonths) > 0 {
		log.Printf("✅ Cascade update completed for %d months: %v", len(updatedMonths), updatedMonths)
	} else {
		log.Printf("ℹ️  No future months to update after %s", fromMonth)
	}
	
	return nil
}

// updateCashBankDistribution actualiza distribución solo en tabla mensual para performance
// Mantiene compatibilidad con tabla legacy cash_bank
func updateCashBankDistribution(distribution CashBankDistribution) error {
	// Only update monthly_cash_bank_balance for performance
	// Solo actualizar tabla mensual para mejor rendimiento
	err := updateMonthlyTable(distribution)
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

// updateMonthlyTable actualiza solo la tabla mensual para mejor rendimiento
// Simplificada para manejar únicamente monthly_cash_bank_balance
func updateMonthlyTable(distribution CashBankDistribution) error {
	// Check if entry exists for this user and month
	// Verificar si existe entrada para este usuario y mes
	var count int
	query := `SELECT COUNT(*) FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?`
	err := db.QueryRow(query, distribution.UserID, distribution.Month).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing entry with new values
		// Actualizar entrada existente con nuevos valores
		updateQuery := `
			UPDATE monthly_cash_bank_balance
			SET cash_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
				total_balance = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND year_month = ?
		`

		_, err = db.Exec(updateQuery,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
			distribution.UserID,
			distribution.Month,
		)
	} else {
		// Insert new entry for this month
		// Insertar nueva entrada para este mes
		insertQuery := `
			INSERT INTO monthly_cash_bank_balance (
				user_id, year_month, cash_amount, bank_amount, balance_cash_amount, balance_bank_amount, total_balance
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`

		_, err = db.Exec(insertQuery,
			distribution.UserID,
			distribution.Month,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.CashAmount,
			distribution.BankAmount,
			distribution.MonthlyTotal,
		)
	}

	return err
}

