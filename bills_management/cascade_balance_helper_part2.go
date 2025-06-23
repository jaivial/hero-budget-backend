package main

import (
	"database/sql"
	"fmt"
	"log"
)

// applyNewCascadeLogic aplica la nueva lógica de acumulación a una factura existente
// Útil para migrar facturas existentes al nuevo sistema de cascada
func applyNewCascadeLogic(db *sql.DB, userID string, billID int) error {
	// Obtener información de la factura
	var amount float64
	var startDate string
	var durationMonths int
	var paymentMethod string

	err := db.QueryRow(`
		SELECT amount, start_date, duration_months, payment_method 
		FROM bills 
		WHERE id = ? AND user_id = ?
	`, billID, userID).Scan(&amount, &startDate, &durationMonths, &paymentMethod)

	if err != nil {
		return fmt.Errorf("factura no encontrada: %v", err)
	}

	log.Printf("🔧 Aplicando nueva lógica de cascada para factura ID=%d, Amount=%.2f, StartDate=%s, Duration=%d",
		billID, amount, startDate, durationMonths)

	// FUNCIÓN ELIMINADA: updateCascadeBillBalance ya no está disponible
	// La lógica de cascada debe implementarse por separado si es necesaria
	log.Printf("⚠️ ADVERTENCIA: updateCascadeBillBalance ha sido eliminada - no se aplica lógica de cascada")
	return nil
}

// revertCascadeBillBalance revierte la acumulación en cascada al eliminar una factura
// CORREGIDO: Implementa reversión precisa del efecto cascada
func revertCascadeBillBalance(db *sql.DB, userID, startDate string, durationMonths int, amount float64, paymentMethod string) error {
	log.Printf("🔄 REVERT: Iniciando reversión cascada para factura de %.2f desde %s durante %d meses", amount, startDate, durationMonths)

	// CORREGIDO: Usar calculateMonthsFromDuration con meses consecutivos
	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		log.Printf("🔄 REVERT: Error calculando meses: %v", err)
		return err
	}

	log.Printf("🔄 REVERT: Meses a revertir: %v", months)

	// Para cada mes, revertir el impacto acumulado
	for i, month := range months {
		// POSITIVO porque revierte la substracción anterior
		accumulatedImpact := amount * float64(i+1)
		log.Printf("🔄 REVERT: Mes %s - impacto acumulado a revertir: +%.2f", month, accumulatedImpact)

		// Revertir este mes con el impacto acumulado
		err = revertCascadeBalanceForMonth(db, userID, month, amount, accumulatedImpact, paymentMethod)
		if err != nil {
			log.Printf("🔄 REVERT: Error revirtiendo mes %s: %v", month, err)
			return err
		}

		log.Printf("🔄 REVERT: Mes %s revertido exitosamente", month)
	}

	// Actualizar previous_amounts correctamente después de la reversión
	log.Printf("🔄 REVERT: Actualizando previous_amounts después de reversión...")
	err = updatePreviousAmountsCorrectlyAfterRevert(db, userID, months, paymentMethod)
	if err != nil {
		log.Printf("🔄 REVERT: Error actualizando previous_amounts: %v", err)
		return err
	}

	log.Printf("🔄 REVERT: Reversión cascada completada exitosamente")
	return nil
}

// revertCascadeBalanceForMonth revierte el impacto acumulado de un mes específico
// Deshace los cambios aplicados por updateCascadeBalanceForMonth
func revertCascadeBalanceForMonth(db *sql.DB, userID, month string, monthlyAmount, accumulatedImpact float64, paymentMethod string) error {
	// Restar bill_amount para este mes específico
	updateBalanceColumns(db, userID, month, monthlyAmount, paymentMethod, "bill", -1)

	// REVERSIÓN: Sumar el impacto acumulado a los valores existentes
	if paymentMethod == "bank" {
		_, err := db.Exec(`
			UPDATE monthly_balance 
			SET bank_amount = bank_amount + ?, 
			    balance_bank_amount = balance_bank_amount + ?, 
			    total_balance = cash_amount + (bank_amount + ?)
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	} else {
		_, err := db.Exec(`
			UPDATE monthly_balance 
			SET cash_amount = cash_amount + ?, 
			    balance_cash_amount = balance_cash_amount + ?, 
			    total_balance = (cash_amount + ?) + bank_amount
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	}
}

// updatePreviousAmountsCorrectlyAfterRevert actualiza previous_amounts después de revertir una factura
// Recalcula los valores de previous_amounts tras la eliminación de una factura
func updatePreviousAmountsCorrectlyAfterRevert(db *sql.DB, userID string, months []string, paymentMethod string) error {
	log.Printf("🔄 REVERT: Actualizando previous_amounts después de reversión para meses: %v", months)

	// Para cada mes (excepto el primero), recalcular previous_amounts con el valor del mes anterior
	for i := 1; i < len(months); i++ {
		currentMonth := months[i]
		previousMonth := months[i-1]

		// Obtener el bank_amount/cash_amount actualizado del mes anterior
		var previousAmount float64
		var query string
		if paymentMethod == "bank" {
			query = "SELECT bank_amount FROM monthly_balance WHERE user_id = ? AND year_month = ?"
		} else {
			query = "SELECT cash_amount FROM monthly_balance WHERE user_id = ? AND year_month = ?"
		}

		err := db.QueryRow(query, userID, previousMonth).Scan(&previousAmount)
		if err != nil {
			log.Printf("Error obteniendo amount del mes anterior %s: %v", previousMonth, err)
			continue
		}

		// Actualizar previous_amounts en el mes actual
		if paymentMethod == "bank" {
			_, err = db.Exec(`
				UPDATE monthly_balance 
				SET previous_bank_amount = ?, 
				    total_previous_balance = ? 
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		} else {
			_, err = db.Exec(`
				UPDATE monthly_balance 
				SET previous_cash_amount = ?, 
				    total_previous_balance = ? 
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		}

		if err != nil {
			log.Printf("Error actualizando previous_amounts para mes %s: %v", currentMonth, err)
		}
	}

	return nil
}

// cleanupOrphanedMonthlyRecords limpia registros huérfanos en monthly_balance
// Útil para mantener la base de datos limpia tras operaciones complejas
func cleanupOrphanedMonthlyRecords(db *sql.DB, userID string) error {
	log.Printf("🧹 Limpiando registros huérfanos para user_id: %s", userID)
	
	// Eliminar registros con todos los valores en 0
	_, err := db.Exec(`
		DELETE FROM monthly_balance 
		WHERE user_id = ? 
		  AND cash_amount = 0 
		  AND bank_amount = 0 
		  AND bills_amount = 0 
		  AND bills_amount = 0 
		  AND expense_amount = 0 
		  AND expense_amount = 0 
		  AND balance_cash_amount = 0 
		  AND balance_bank_amount = 0 
		  AND total_balance = 0
	`, userID)
	
	if err != nil {
		log.Printf("Error limpiando registros huérfanos: %v", err)
		return err
	}
	
	log.Printf("✅ Limpieza de registros huérfanos completada")
	return nil
}

// validateMonthlyBalanceConsistency valida la consistencia de monthly_balance
// Verifica que los cálculos sean coherentes y detecta posibles errores
func validateMonthlyBalanceConsistency(db *sql.DB, userID string) error {
	log.Printf("🔍 Validando consistencia de monthly_balance para user_id: %s", userID)
	
	// Obtener todos los registros del usuario
	rows, err := db.Query(`
		SELECT year_month, cash_amount, bank_amount, total_balance
		FROM monthly_balance 
		WHERE user_id = ? 
		ORDER BY year_month
	`, userID)
	
	if err != nil {
		return fmt.Errorf("error fetching records for validation: %v", err)
	}
	defer rows.Close()
	
	var inconsistencies []string
	for rows.Next() {
		var month string
		var cashAmount, bankAmount, totalBalance float64
		
		if rows.Scan(&month, &cashAmount, &bankAmount, &totalBalance) == nil {
			// Verificar que total_balance = cash_amount + bank_amount
			expectedTotal := cashAmount + bankAmount
			if fmt.Sprintf("%.2f", expectedTotal) != fmt.Sprintf("%.2f", totalBalance) {
				inconsistency := fmt.Sprintf("Mes %s: total_balance=%.2f, esperado=%.2f", month, totalBalance, expectedTotal)
				inconsistencies = append(inconsistencies, inconsistency)
			}
		}
	}
	
	if len(inconsistencies) > 0 {
		log.Printf("⚠️ Se encontraron %d inconsistencias:", len(inconsistencies))
		for _, inconsistency := range inconsistencies {
			log.Printf("  - %s", inconsistency)
		}
		return fmt.Errorf("found %d balance inconsistencies", len(inconsistencies))
	}
	
	log.Printf("✅ Validación de consistencia completada sin errores")
	return nil
}
