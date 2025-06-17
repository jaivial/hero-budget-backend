package main

import (
	"database/sql"
	"fmt"
	"log"
)

// updateCascadeBillBalance actualiza los balances en cascada para facturas con duración
// Esto garantiza que el impacto de las facturas se acumule mes a mes
func updateCascadeBillBalance(db *sql.DB, userID, startDate string, durationMonths int, amount float64, paymentMethod string) error {
	log.Printf("🔥 DEBUG: updateCascadeBillBalance iniciada - userID=%s, amount=%.2f, durationMonths=%d", userID, amount, durationMonths)

	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		log.Printf("🔥 DEBUG: Error calculando meses: %v", err)
		return err
	}

	log.Printf("🔥 DEBUG: Meses calculados: %v", months)

	log.Printf("🔄 Iniciando acumulación en cascada para factura de %.2f desde %s durante %d meses", amount, startDate, durationMonths)
	log.Printf("📅 Meses afectados: %v", months)

	// Para cada mes, calcular el impacto acumulado
	for i, month := range months {
		accumulatedImpact := -amount * float64(i+1)
		log.Printf("🔥 DEBUG: Mes %s - impacto acumulado: %.2f", month, accumulatedImpact)

		// Asegurar que el registro existe
		_, err = db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", userID, month)
		if err != nil {
			log.Printf("🔥 DEBUG: Error creando registro para mes %s: %v", month, err)
			return err
		}

		// Actualizar este mes con el impacto acumulado
		err = updateCascadeBalanceForMonth(db, userID, month, amount, accumulatedImpact, paymentMethod)
		if err != nil {
			log.Printf("🔥 DEBUG: Error actualizando mes %s: %v", month, err)
			return err
		}

		log.Printf("🔥 DEBUG: Mes %s actualizado exitosamente", month)
	}

	// Actualizar previous_amounts correctamente
	log.Printf("🔥 DEBUG: Actualizando previous_amounts...")
	err = updatePreviousAmountsCorrectly(db, userID, months, paymentMethod)
	if err != nil {
		log.Printf("🔥 DEBUG: Error actualizando previous_amounts: %v", err)
		return err
	}

	log.Printf("🔥 DEBUG: updateCascadeBillBalance completada exitosamente")
	return nil
}

// updateCascadeBalanceForMonth actualiza un mes específico con el impacto acumulado
func updateCascadeBalanceForMonth(db *sql.DB, userID, month string, monthlyAmount, accumulatedImpact float64, paymentMethod string) error {
	// Actualizar bill_amount para este mes específico (sumar al valor existente)
	updateBalanceColumns(db, userID, month, monthlyAmount, paymentMethod, "bill", 1)

	// CORRECCIÓN: Restar el impacto acumulado de los valores EXISTENTES en lugar de sobrescribir
	if paymentMethod == "bank" {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET bank_amount = bank_amount + ?, 
			    balance_bank_amount = balance_bank_amount + ?, 
			    total_balance = cash_amount + (bank_amount + ?)
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	} else {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET cash_amount = cash_amount + ?, 
			    balance_cash_amount = balance_cash_amount + ?, 
			    total_balance = (cash_amount + ?) + bank_amount
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	}
}

// updatePreviousAmountsCorrectly actualiza previous_amounts correctamente después de procesar todos los meses
func updatePreviousAmountsCorrectly(db *sql.DB, userID string, months []string, paymentMethod string) error {
	log.Printf("🔄 Actualizando previous_amounts correctamente para meses: %v", months)

	// Para cada mes (excepto el primero), actualizar previous_amounts con el valor del mes anterior
	for i := 1; i < len(months); i++ {
		currentMonth := months[i]
		previousMonth := months[i-1]

		// Obtener el bank_amount/cash_amount del mes anterior
		var previousAmount float64
		var query string
		if paymentMethod == "bank" {
			query = "SELECT bank_amount FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?"
		} else {
			query = "SELECT cash_amount FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?"
		}

		err := db.QueryRow(query, userID, previousMonth).Scan(&previousAmount)
		if err != nil {
			log.Printf("⚠️ Error obteniendo balance del mes anterior %s: %v", previousMonth, err)
			continue
		}

		// Actualizar previous_amount del mes actual
		if paymentMethod == "bank" {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_bank_amount = ?,
				    total_previous_balance = previous_cash_amount + ?
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		} else {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_cash_amount = ?,
				    total_previous_balance = ? + previous_bank_amount
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		}

		if err != nil {
			return fmt.Errorf("error actualizando previous_amount para %s: %v", currentMonth, err)
		}

		log.Printf("✅ %s: previous_amount = %.2f (del mes anterior %s)", currentMonth, previousAmount, previousMonth)
	}

	// Actualizar también meses posteriores a la factura (si existen)
	lastMonth := months[len(months)-1]
	return updateSubsequentMonthsPreviousAmounts(db, userID, lastMonth, paymentMethod)
}

// updateSubsequentMonthsPreviousAmounts actualiza los previous_amounts de meses posteriores a la factura
func updateSubsequentMonthsPreviousAmounts(db *sql.DB, userID, lastMonth, paymentMethod string) error {
	// Obtener meses posteriores al último mes de la factura
	rows, err := db.Query(`
		SELECT year_month 
		FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month > ? 
		ORDER BY year_month
	`, userID, lastMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	var subsequentMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			subsequentMonths = append(subsequentMonths, month)
		}
	}

	// Para cada mes posterior, calcular su previous_amount basado en el mes inmediatamente anterior
	for i, month := range subsequentMonths {
		var previousMonth string
		if i == 0 {
			previousMonth = lastMonth // El primer mes posterior usa el último mes de la factura
		} else {
			previousMonth = subsequentMonths[i-1] // Los demás usan el mes anterior en la secuencia
		}

		// Obtener el total_balance del mes anterior
		var previousAmount float64
		err := db.QueryRow(
			"SELECT total_balance FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?",
			userID, previousMonth).Scan(&previousAmount)
		if err != nil {
			continue
		}

		// Actualizar el total_previous_balance del mes actual
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, previousAmount, userID, month)
		if err != nil {
			return err
		}

		// Recalcular total_balance para el mes (sumando el previous_balance)
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET total_balance = (cash_amount + bank_amount) + ?
			WHERE user_id = ? AND year_month = ?
		`, previousAmount, userID, month)
		if err != nil {
			return err
		}
	}

	return nil
}

// applyNewCascadeLogic aplica la nueva lógica de acumulación a una factura existente
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

	// Aplicar la nueva lógica de acumulación en cascada
	return updateCascadeBillBalance(db, userID, startDate, durationMonths, amount, paymentMethod)
}

// revertCascadeBillBalance revierte la acumulación en cascada al eliminar una factura
func revertCascadeBillBalance(db *sql.DB, userID, startDate string, durationMonths int, amount float64, paymentMethod string) error {
	log.Printf("🔄 REVERT: Iniciando reversión cascada para factura de %.2f desde %s durante %d meses", amount, startDate, durationMonths)

	months, err := calculateMonthsFromDuration(startDate, durationMonths)
	if err != nil {
		log.Printf("🔄 REVERT: Error calculando meses: %v", err)
		return err
	}

	log.Printf("🔄 REVERT: Meses a revertir: %v", months)

	// Para cada mes, revertir el impacto acumulado
	for i, month := range months {
		accumulatedImpact := amount * float64(i+1) // POSITIVO porque revierte la substracción
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
func revertCascadeBalanceForMonth(db *sql.DB, userID, month string, monthlyAmount, accumulatedImpact float64, paymentMethod string) error {
	// Restar bill_amount para este mes específico
	updateBalanceColumns(db, userID, month, monthlyAmount, paymentMethod, "bill", -1)

	// REVERSIÓN: Sumar el impacto acumulado a los valores existentes
	if paymentMethod == "bank" {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET bank_amount = bank_amount + ?, 
			    balance_bank_amount = balance_bank_amount + ?, 
			    total_balance = cash_amount + (bank_amount + ?)
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	} else {
		_, err := db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET cash_amount = cash_amount + ?, 
			    balance_cash_amount = balance_cash_amount + ?, 
			    total_balance = (cash_amount + ?) + bank_amount
			WHERE user_id = ? AND year_month = ?
		`, accumulatedImpact, accumulatedImpact, accumulatedImpact, userID, month)
		return err
	}
}

// updatePreviousAmountsCorrectlyAfterRevert actualiza previous_amounts después de revertir una factura
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
			query = "SELECT bank_amount FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?"
		} else {
			query = "SELECT cash_amount FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?"
		}

		err := db.QueryRow(query, userID, previousMonth).Scan(&previousAmount)
		if err != nil {
			log.Printf("⚠️ REVERT: Error obteniendo balance del mes anterior %s: %v", previousMonth, err)
			continue
		}

		// Actualizar previous_amount del mes actual
		if paymentMethod == "bank" {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_bank_amount = ?,
				    total_previous_balance = previous_cash_amount + ?
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		} else {
			_, err = db.Exec(`
				UPDATE monthly_cash_bank_balance 
				SET previous_cash_amount = ?,
				    total_previous_balance = ? + previous_bank_amount
				WHERE user_id = ? AND year_month = ?
			`, previousAmount, previousAmount, userID, currentMonth)
		}

		if err != nil {
			return fmt.Errorf("error actualizando previous_amount para %s: %v", currentMonth, err)
		}

		log.Printf("✅ REVERT: %s: previous_amount = %.2f (del mes anterior %s)", currentMonth, previousAmount, previousMonth)
	}

	// Actualizar también meses posteriores a la factura (si existen)
	lastMonth := months[len(months)-1]
	return updateSubsequentMonthsPreviousAmountsAfterRevert(db, userID, lastMonth, paymentMethod)
}

// updateSubsequentMonthsPreviousAmountsAfterRevert actualiza los previous_amounts de meses posteriores después de revertir
func updateSubsequentMonthsPreviousAmountsAfterRevert(db *sql.DB, userID, lastMonth, paymentMethod string) error {
	// Obtener meses posteriores al último mes de la factura
	rows, err := db.Query(`
		SELECT year_month 
		FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month > ? 
		ORDER BY year_month
	`, userID, lastMonth)
	if err != nil {
		return err
	}
	defer rows.Close()

	var subsequentMonths []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			subsequentMonths = append(subsequentMonths, month)
		}
	}

	// Para cada mes posterior, recalcular su previous_amount basado en el mes inmediatamente anterior
	for i, month := range subsequentMonths {
		var previousMonth string
		if i == 0 {
			previousMonth = lastMonth // El primer mes posterior usa el último mes de la factura
		} else {
			previousMonth = subsequentMonths[i-1] // Los demás usan el mes anterior en la secuencia
		}

		// Obtener el total_balance actualizado del mes anterior
		var previousAmount float64
		err := db.QueryRow(
			"SELECT total_balance FROM monthly_cash_bank_balance WHERE user_id = ? AND year_month = ?",
			userID, previousMonth).Scan(&previousAmount)
		if err != nil {
			continue
		}

		// Actualizar el total_previous_balance del mes actual
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET total_previous_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, previousAmount, userID, month)
		if err != nil {
			return err
		}

		// Recalcular total_balance para el mes (sumando el previous_balance actualizado)
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET total_balance = (cash_amount + bank_amount) + ?
			WHERE user_id = ? AND year_month = ?
		`, previousAmount, userID, month)
		if err != nil {
			return err
		}
	}

	return nil
}
