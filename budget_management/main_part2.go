package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Funciones de base de datos y utilidades para gestión de presupuestos
// Contiene toda la lógica de persistencia y operaciones con datos

// fetchBudgetData obtiene los datos de presupuesto desde la base de datos
// Busca el presupuesto más reciente para el usuario y período especificados
func fetchBudgetData(userID, period string) (BudgetData, error) {
	var budget BudgetData

	// Query para obtener datos de presupuesto más recientes
	err := db.QueryRow(`
		SELECT user_id, period, date, total_amount, remaining_amount, spent_amount, 
		       upcoming_amount, from_previous, percent, COALESCE(total_income, 0)
		FROM budget
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&budget.UserID,
		&budget.Period,
		&budget.Date,
		&budget.TotalAmount,
		&budget.RemainingAmount,
		&budget.SpentAmount,
		&budget.UpcomingAmount,
		&budget.FromPrevious,
		&budget.Percent,
		&budget.TotalIncome,
	)

	// Handle case when no budget data exists
	if err == sql.ErrNoRows {
		// Initialize default budget values
		budget.UserID = userID
		budget.Period = period
		budget.Date = time.Now().Format("2006-01-02")
		budget.TotalAmount = 0
		budget.RemainingAmount = 0
		budget.SpentAmount = 0
		budget.UpcomingAmount = 0
		budget.FromPrevious = 0
		budget.Percent = 0
		budget.TotalIncome = 0

		// Check for previous period data to inherit
		previousPeriod, previousAmount := getPreviousPeriodData(userID, period)
		if previousAmount > 0 {
			budget.FromPrevious = previousAmount
			budget.TotalAmount = previousAmount
			budget.RemainingAmount = previousAmount

			// Log inheritance for debugging
			log.Printf("Inheriting %f from previous period %s for user %s in period %s",
				previousAmount, previousPeriod, userID, period)
		}

		return budget, nil
	} else if err != nil {
		return budget, err
	}

	return budget, nil
}

// getPreviousPeriodData obtiene datos del período anterior para herencia
// Calcula automáticamente el período anterior basado en el actual
func getPreviousPeriodData(userID, currentPeriod string) (string, float64) {
	var previousPeriod string
	var queryDateCondition string

	now := time.Now()

	// Determine previous period based on current period type
	switch currentPeriod {
	case "daily":
		previousPeriod = "daily"
		previousDate := now.AddDate(0, 0, -1).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date = '%s'", previousDate)
		
	case "weekly":
		previousPeriod = "weekly"
		previousWeekStart := now.AddDate(0, 0, -7).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousWeekStart)
		
	case "monthly":
		previousPeriod = "monthly"
		previousMonthStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousMonthStart)
		
	case "quarterly":
		previousPeriod = "quarterly"
		currentQuarter := (int(now.Month())-1)/3 + 1
		previousQuarter := currentQuarter - 1
		var year int
		if previousQuarter <= 0 {
			previousQuarter = 4
			year = now.Year() - 1
		} else {
			year = now.Year()
		}
		previousQuarterStart := time.Date(year, time.Month((previousQuarter-1)*3+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousQuarterStart)
		
	case "semiannual":
		previousPeriod = "semiannual"
		currentHalf := (int(now.Month())-1)/6 + 1
		previousHalf := currentHalf - 1
		var year int
		if previousHalf <= 0 {
			previousHalf = 2
			year = now.Year() - 1
		} else {
			year = now.Year()
		}
		previousHalfStart := time.Date(year, time.Month((previousHalf-1)*6+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousHalfStart)
		
	case "annual":
		previousPeriod = "annual"
		previousYearStart := time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		queryDateCondition = fmt.Sprintf("AND date <= '%s' ORDER BY date DESC", previousYearStart)
		
	default:
		// Return zero values for unrecognized periods
		return "", 0
	}

	// Query to get the most recent budget entry for the previous period
	query := fmt.Sprintf(`
		SELECT remaining_amount FROM budget 
		WHERE user_id = ? AND period = ? %s
		LIMIT 1
	`, queryDateCondition)

	var remainingAmount float64
	err := db.QueryRow(query, userID, previousPeriod).Scan(&remainingAmount)

	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error getting previous period data: %v", err)
		}
		return "", 0
	}

	return previousPeriod, remainingAmount
}

// updateBudgetData actualiza o inserta datos de presupuesto en la base de datos
// Utiliza lógica de upsert para manejar tanto inserciones como actualizaciones
func updateBudgetData(budget BudgetData) error {
	// Check if budget entry already exists for this user and period
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM budget 
		WHERE user_id = ? AND period = ?
	`, budget.UserID, budget.Period).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing budget entry
		_, err = db.Exec(`
			UPDATE budget
			SET total_amount = ?,
				remaining_amount = ?,
				spent_amount = ?,
				upcoming_amount = ?,
				from_previous = ?,
				percent = ?,
				total_income = ?,
				date = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND period = ?
		`,
			budget.TotalAmount,
			budget.RemainingAmount,
			budget.SpentAmount,
			budget.UpcomingAmount,
			budget.FromPrevious,
			budget.Percent,
			budget.TotalIncome,
			budget.Date,
			budget.UserID,
			budget.Period,
		)
	} else {
		// Insert new budget entry
		_, err = db.Exec(`
			INSERT INTO budget (
				user_id, period, date, total_amount, remaining_amount, 
				spent_amount, upcoming_amount, from_previous, percent, total_income
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			budget.UserID,
			budget.Period,
			budget.Date,
			budget.TotalAmount,
			budget.RemainingAmount,
			budget.SpentAmount,
			budget.UpcomingAmount,
			budget.FromPrevious,
			budget.Percent,
			budget.TotalIncome,
		)
	}

	return err
}

// sendSuccessResponse envía una respuesta HTTP exitosa con datos
// Utiliza el formato estándar ApiResponse para consistencia
func sendSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// sendErrorResponse envía una respuesta HTTP de error
// Utiliza el formato estándar ApiResponse para consistencia
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ApiResponse{
		Success: false,
		Message: message,
	})
}