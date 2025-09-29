package main

import (
	"database/sql"
	"fmt"
	"log"
)

// recalculateAllCumulativeBalances recalcula todos los balances de forma acumulativa
// NUEVA ARQUITECTURA: Los balances se arrastran mes a mes de forma acumulativa
func recalculateAllCumulativeBalances(db *sql.DB, userID, startMonth string) error {
	log.Printf("🔄 Recalculando balances acumulativos desde %s para user %s", startMonth, userID)

	// Obtener todos los meses desde startMonth en adelante, ordenados cronológicamente
	rows, err := db.Query(`
		SELECT year_month 
		FROM monthly_cash_bank_balance 
		WHERE user_id = ? AND year_month >= ? 
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		return fmt.Errorf("error fetching months: %v", err)
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if rows.Scan(&month) == nil {
			months = append(months, month)
		}
	}

	if len(months) == 0 {
		log.Printf("No hay meses para recalcular")
		return nil
	}

	// Recalcular cada mes de forma acumulativa
	for i, currentMonth := range months {
		log.Printf("📊 Procesando mes %s (%d/%d)", currentMonth, i+1, len(months))

		// Obtener datos del mes actual
		var incomeBank, incomeCash, expenseBank, expenseCash, billBank, billCash float64
		err := db.QueryRow(`
			SELECT 
				COALESCE(income_bank_amount, 0), COALESCE(income_cash_amount, 0),
				COALESCE(expense_bank_amount, 0), COALESCE(expense_cash_amount, 0),
				COALESCE(bill_bank_amount, 0), COALESCE(bill_cash_amount, 0)
			FROM monthly_cash_bank_balance 
			WHERE user_id = ? AND year_month = ?
		`, userID, currentMonth).Scan(&incomeBank, &incomeCash, &expenseBank, &expenseCash, &billBank, &billCash)

		if err != nil {
			log.Printf("Error obteniendo datos del mes %s: %v", currentMonth, err)
			continue
		}

		// Calcular previous_amounts (balance del mes anterior)
		var prevCash, prevBank float64
		if i > 0 {
			// Obtener balance final del mes anterior
			previousMonth := months[i-1]
			err := db.QueryRow(`
				SELECT COALESCE(cash_amount, 0), COALESCE(bank_amount, 0)
				FROM monthly_cash_bank_balance 
				WHERE user_id = ? AND year_month = ?
			`, userID, previousMonth).Scan(&prevCash, &prevBank)

			if err != nil {
				log.Printf("Error obteniendo balance del mes anterior %s: %v", previousMonth, err)
				// Usar 0 si no hay mes anterior
			}
		}
		// Para el primer mes, previous_amounts = 0

		// BALANCES ACUMULATIVOS CORREGIDOS:
		// Según la explicación del usuario:
		// - bill_bank_amount y bill_cash_amount se SUMAN (valores positivos cuando se añade/aumenta un bill)
		// - expense_bank_amount y expense_cash_amount también se SUMAN (valores positivos)
		// - cash_amount y bank_amount representan el balance final acumulativo
		//
		// LA INTERPRETACIÓN CORRECTA:
		// Los balances finales deben ser ACUMULATIVOS pero DISPONIBLES
		// Formula corregida (el balance debe DECRECER cuando se añaden bills/expenses):
		// cash_amount = previous_cash + income_cash - expense_cash - bill_cash
		// bank_amount = previous_bank + income_bank - expense_bank - bill_bank
		newCashAmount := prevCash + incomeCash - expenseCash - billCash
		newBankAmount := prevBank + incomeBank - expenseBank - billBank

		// BALANCES DISPONIBLES (después de comprometer dinero para facturas):
		// balance_cash_amount = cash_amount (ya considerado en el cálculo)
		// balance_bank_amount = bank_amount (ya considerado en el cálculo)
		balanceCashAmount := newCashAmount
		balanceBankAmount := newBankAmount

		// TOTALES:
		totalPreviousBalance := prevCash + prevBank
		totalBalance := newCashAmount + newBankAmount

		// Actualizar todos los valores calculados
		_, err = db.Exec(`
			UPDATE monthly_cash_bank_balance 
			SET 
				previous_cash_amount = ?,
				previous_bank_amount = ?,
				cash_amount = ?,
				bank_amount = ?,
				balance_cash_amount = ?,
				balance_bank_amount = ?,
				total_previous_balance = ?,
				total_balance = ?
			WHERE user_id = ? AND year_month = ?
		`, prevCash, prevBank, newCashAmount, newBankAmount,
			balanceCashAmount, balanceBankAmount, totalPreviousBalance, totalBalance,
			userID, currentMonth)

		if err != nil {
			log.Printf("Error actualizando balances para mes %s: %v", currentMonth, err)
			return err
		}

		log.Printf("✅ Mes %s: cash=%.2f, bank=%.2f, prev_cash=%.2f, prev_bank=%.2f, total=%.2f",
			currentMonth, newCashAmount, newBankAmount, prevCash, prevBank, totalBalance)
	}

	log.Printf("✅ Recálculo acumulativo completado para %d meses", len(months))
	return nil
}

// addBillToCashBankBalanceCumulative añade una factura usando lógica acumulativa
// NUEVA ARQUITECTURA: Reemplaza addBillToCashBankBalance con lógica acumulativa correcta
func addBillToCashBankBalanceCumulative(db *sql.DB, userID string, amount float64, startDate string, durationMonths int, paymentMethod string) error {
	log.Printf("🔄 Añadiendo bill acumulativo: user=%s, amount=%.2f, start=%s, duration=%d, method=%s",
		userID, amount, startDate, durationMonths, paymentMethod)

	// Parsear fecha de inicio
	startTime, err := parseFlexibleDateCashBank(startDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %v", err)
	}

	// Añadir bill_amount a cada mes del periodo
	for i := 0; i < durationMonths; i++ {
		monthDate := startTime.AddDate(0, i, 0)
		yearMonth := monthDate.Format("2006-01")

		// Asegurar que existe fila para el mes
		_, err := db.Exec("INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month) VALUES (?, ?)", userID, yearMonth)
		if err != nil {
			return fmt.Errorf("error creating monthly record for %s: %v", yearMonth, err)
		}

		// Sumar amount a bill_cash_amount o bill_bank_amount
		if paymentMethod == "cash" {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = COALESCE(bill_cash_amount, 0) + ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		} else {
			_, err = db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = COALESCE(bill_bank_amount, 0) + ? WHERE user_id = ? AND year_month = ?", amount, userID, yearMonth)
		}
		if err != nil {
			return fmt.Errorf("error updating bill amount for %s: %v", yearMonth, err)
		}
	}

	// CORREGIDO: Recalcular balances acumulativos desde el primer mes existente
	// para asegurar que todos los arrastres se actualicen correctamente
	var firstMonth string
	err = db.QueryRow(`
		SELECT MIN(year_month) 
		FROM monthly_cash_bank_balance 
		WHERE user_id = ?
	`, userID).Scan(&firstMonth)

	if err != nil {
		startMonth := startTime.Format("2006-01")
		return recalculateAllCumulativeBalances(db, userID, startMonth)
	}

	return recalculateAllCumulativeBalances(db, userID, firstMonth)
}

// revertBillFromCashBankBalanceCumulative revierte una factura usando lógica acumulativa
// NUEVA ARQUITECTURA: Reemplaza revertBillEffectsFromCashBank con lógica acumulativa correcta
func revertBillFromCashBankBalanceCumulative(db *sql.DB, billData BillData) error {
	log.Printf("🔄 Revirtiendo bill acumulativo: ID=%d, Amount=%.2f, Method=%s",
		billData.ID, billData.Amount, billData.PaymentMethod)

	// Calcular meses afectados por la factura
	months, err := calculateMonthsFromDurationCashBank(billData.StartDate, billData.Duration)
	if err != nil {
		return err
	}

	// Revertir bill_amount de cada mes de la factura
	for _, month := range months {
		// Restar amount de bill_cash_amount o bill_bank_amount
		if billData.PaymentMethod == "cash" {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_cash_amount = COALESCE(bill_cash_amount, 0) - ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		} else {
			db.Exec("UPDATE monthly_cash_bank_balance SET bill_bank_amount = COALESCE(bill_bank_amount, 0) - ? WHERE user_id = ? AND year_month = ?", billData.Amount, billData.UserID, month)
		}
	}

	// CORREGIDO: Recalcular balances acumulativos desde el primer mes existente
	var firstMonth string
	err = db.QueryRow(`
		SELECT MIN(year_month) 
		FROM monthly_cash_bank_balance 
		WHERE user_id = ?
	`, billData.UserID).Scan(&firstMonth)

	if err != nil {
		startTime, err := parseFlexibleDateCashBank(billData.StartDate)
		if err != nil {
			return err
		}
		startMonth := startTime.Format("2006-01")
		return recalculateAllCumulativeBalances(db, billData.UserID, startMonth)
	}

	return recalculateAllCumulativeBalances(db, billData.UserID, firstMonth)
}

// updateBillInCashBankBalanceCumulative actualiza una factura usando lógica acumulativa
// NUEVA ARQUITECTURA: Implementa update de facturas con lógica acumulativa
func updateBillInCashBankBalanceCumulative(db *sql.DB, oldBillData, newBillData BillData) error {
	log.Printf("🔄 Actualizando bill acumulativo: ID=%d", oldBillData.ID)

	// Revertir la factura anterior
	err := revertBillFromCashBankBalanceCumulative(db, oldBillData)
	if err != nil {
		return fmt.Errorf("error reverting old bill: %v", err)
	}

	// Aplicar la nueva factura
	err = addBillToCashBankBalanceCumulative(db, newBillData.UserID, newBillData.Amount, newBillData.StartDate, newBillData.Duration, newBillData.PaymentMethod)
	if err != nil {
		return fmt.Errorf("error applying new bill: %v", err)
	}

	log.Printf("✅ Bill actualizado correctamente con lógica acumulativa")
	return nil
}
