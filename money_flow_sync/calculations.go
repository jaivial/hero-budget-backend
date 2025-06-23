package main

import (
	"database/sql"
	"log"
	"time"
)

// getDateRangeForPeriod calcula fechas de inicio y fin para diferentes períodos
func getDateRangeForPeriod(period string) (string, string) {
	now := time.Now()

	switch period {
	case "daily":
		// Current day range for daily calculations
		return now.Format("2006-01-02"), now.Format("2006-01-02")
	case "weekly":
		// Start of the week (Monday) to end of week (Sunday)
		startDate := now.AddDate(0, 0, -int(now.Weekday())+1)
		if now.Weekday() == 0 { // If today is Sunday
			startDate = now.AddDate(0, 0, -6) // Go back to previous Monday
		}
		endDate := startDate.AddDate(0, 0, 6)
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "monthly":
		// Start of current month to end of current month
		startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month())-1)/3 + 1
		startDate := time.Date(now.Year(), time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), time.Month(quarter*3+1), 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "semiannual":
		halfYear := (int(now.Month())-1)/6 + 1
		startDate := time.Date(now.Year(), time.Month((halfYear-1)*6+1), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), time.Month(halfYear*6+1), 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	case "annual":
		startDate := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	default:
		// Default to monthly if period is not recognized
		startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location())
		return startDate.Format("2006-01-02"), endDate.Format("2006-01-02")
	}
}

// getPreviousPeriodData obtiene datos del período anterior para carry-over
func getPreviousPeriodData(userID, currentPeriod string) (string, float64) {
	// Para el cálculo del flujo de dinero, necesitamos el previous_amount del MES ACTUAL
	// no del mes anterior. Esto es porque previous_amount ya contiene el balance heredado.

	now := time.Now()

	switch currentPeriod {
	case "monthly":
		// Obtener el año-mes actual para consulta
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts for user %s, month %s", userID, currentYearMonth)

		// Consultar la tabla monthly_cash_bank_balance para obtener los previous_amounts del mes actual
		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			// Si no hay registro, devolver 0 (no hay balance heredado)
			return "monthly", 0
		}

		log.Printf("📊 Found previous amounts for %s: total_previous=%.2f", currentYearMonth, totalPrevious)
		return "monthly", totalPrevious

	case "daily":
		// Para períodos diarios, también buscamos en monthly_cash_bank_balance
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts (daily) for user %s, month %s", userID, currentYearMonth)

		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			return "daily", 0
		}

		return "daily", totalPrevious

	case "weekly":
		// Para períodos semanales, también buscamos en monthly_cash_bank_balance del mes actual
		currentYearMonth := now.Format("2006-01")

		log.Printf("🔍 DEBUG: Looking for previous_amounts (weekly) for user %s, month %s", userID, currentYearMonth)

		query := `
			SELECT 
				COALESCE(previous_cash_amount, 0) + COALESCE(previous_bank_amount, 0) as total_previous
			FROM monthly_cash_bank_balance
			WHERE user_id = ? AND year_month = ?
		`

		var totalPrevious float64
		err := db.QueryRow(query, userID, currentYearMonth).Scan(&totalPrevious)

		if err != nil {
			if err == sql.ErrNoRows {
				log.Printf("🔍 DEBUG: No record found in monthly_cash_bank_balance for user %s, month %s", userID, currentYearMonth)
			} else {
				log.Printf("🔍 DEBUG: Error getting previous amounts from monthly_cash_bank_balance: %v", err)
			}
			return "weekly", 0
		}

		return "weekly", totalPrevious

	default:
		// Para otros períodos, mantener la lógica original como fallback
		log.Printf("⚠️ Using fallback logic for period: %s", currentPeriod)
		return "", 0
	}
}

// getTotalIncomeForPeriod obtiene ingresos totales para el período especificado
func getTotalIncomeForPeriod(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM incomes
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var totalIncome float64
	err := db.QueryRow(query, userID, startDate, endDate).Scan(&totalIncome)
	if err != nil {
		return 0, err
	}

	return totalIncome, nil
}

// getSpentAmountForPeriod obtiene gastos totales para el período especificado
func getSpentAmountForPeriod(userID, startDate, endDate string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`

	var spentAmount float64
	err := db.QueryRow(query, userID, startDate, endDate).Scan(&spentAmount)
	if err != nil {
		return 0, err
	}

	return spentAmount, nil
}