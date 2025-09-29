package main

import (
	"database/sql"
	"fmt"
	"log"
)

// FUNCIÓN ELIMINADA: updateCascadeBillBalance
// Esta función fue eliminada porque ya no se utiliza en el código
// La función original implementaba lógica de acumulación en cascada para facturas con duración
// Si se necesita esta funcionalidad en el futuro, debe reimplementarse con la nueva arquitectura

// updateCascadeBalanceForMonth actualiza un mes específico con el impacto acumulado
// Aplica el efecto cascada calculado para un mes individual
func updateCascadeBalanceForMonth(db *sql.DB, userID, month string, monthlyAmount, accumulatedImpact float64, paymentMethod string) error {
	// Actualizar bill_amount para este mes específico (sumar al valor existente)
	updateBalanceColumns(db, userID, month, monthlyAmount, paymentMethod, "bill", 1)

	// CORRECCIÓN: Restar el impacto acumulado de los valores EXISTENTES en lugar de sobrescribir
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

// updatePreviousAmountsCorrectly actualiza previous_amounts correctamente después de procesar todos los meses
// CORREGIDO: Implementa actualización cascada según algoritmo corregido
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

// recalculateBalancesFromStartMonth recalcula balances desde un mes de inicio
// Útil para mantener consistencia tras cambios en facturas
func recalculateBalancesFromStartMonth(db *sql.DB, userID, startMonth string) error {
	log.Printf("🔄 Recalculando balances desde mes: %s", startMonth)

	// Obtener todos los meses desde el startMonth
	rows, err := db.Query("SELECT year_month FROM monthly_balance WHERE user_id = ? AND year_month >= ? ORDER BY year_month", userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months for recalculation: %v", err)
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			months = append(months, month)
		}
	}

	// Recalcular total_balance para cada mes
	for _, month := range months {
		_, err = db.Exec("UPDATE monthly_balance SET total_balance = cash_amount + bank_amount WHERE user_id = ? AND year_month = ?", userID, month)
		if err != nil {
			log.Printf("Error recalculando balance para mes %s: %v", month, err)
		}
	}

	return nil
}

// debugPrintMonthlyBalance imprime estado de monthly_balance para depuración
// Útil para verificar el estado de las tablas durante desarrollo
func debugPrintMonthlyBalance(db *sql.DB, userID, month string) {
	log.Printf("🔍 DEBUG: Estado de monthly_balance para user=%s, month=%s", userID, month)

	var cashAmount, bankAmount, billCash, billBank, expenseCash, expenseBank, balanceCash, balanceBank, totalBalance float64
	err := db.QueryRow(`
		SELECT 
			COALESCE(cash_amount, 0),
			COALESCE(bank_amount, 0),
			COALESCE(bills_amount, 0),
			COALESCE(bills_amount, 0),
			COALESCE(expense_amount, 0),
			COALESCE(expense_amount, 0),
			COALESCE(balance_cash_amount, 0),
			COALESCE(balance_bank_amount, 0),
			COALESCE(total_balance, 0)
		FROM monthly_balance 
		WHERE user_id = ? AND year_month = ?
	`, userID, month).Scan(
		&cashAmount, &bankAmount, &billCash, &billBank,
		&expenseCash, &expenseBank, &balanceCash, &balanceBank, &totalBalance)

	if err != nil {
		log.Printf("❌ No se encontró registro para user=%s, month=%s", userID, month)
		return
	}

	log.Printf("📊 cash_amount: %.2f, bank_amount: %.2f", cashAmount, bankAmount)
	log.Printf("📊 bill_cash: %.2f, bill_bank: %.2f", billCash, billBank)
	log.Printf("📊 expense_cash: %.2f, expense_bank: %.2f", expenseCash, expenseBank)
	log.Printf("📊 balance_cash: %.2f, balance_bank: %.2f", balanceCash, balanceBank)
	log.Printf("📊 total_balance: %.2f", totalBalance)
}
