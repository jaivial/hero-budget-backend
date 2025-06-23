package main

import (
	"fmt"
	"time"
)

// recalculateSemiannualBalances recalcula los balances semestrales
func recalculateSemiannualBalances(userID string, date time.Time) error {
	year := date.Year()
	semester := 1
	if date.Month() > 6 {
		semester = 2
	}
	
	// Calculate semester date range
	var semesterStart, semesterEnd time.Time
	if semester == 1 {
		semesterStart = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		semesterEnd = time.Date(year, 6, 30, 0, 0, 0, 0, time.UTC)
	} else {
		semesterStart = time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC)
		semesterEnd = time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	
	semesterStartStr := semesterStart.Format("2006-01-02")
	semesterEndStr := semesterEnd.Format("2006-01-02")
	
	// Calculate semiannual totals
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`
	
	var cashIncome, bankIncome, totalIncome float64
	err := db.QueryRow(query, userID, semesterStartStr, semesterEndStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update semiannual balance
	return updateSemiannualBalance(userID, totalIncome, 0, 0, cashIncome, bankIncome, date)
}

// recalculateAnnualBalances recalcula los balances anuales
func recalculateAnnualBalances(userID string, date time.Time) error {
	year := date.Year()
	yearStr := fmt.Sprintf("%d", year)
	
	// Calculate annual totals
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN amount ELSE 0 END), 0) as cash_income,
			COALESCE(SUM(CASE WHEN payment_method = 'bank' THEN amount ELSE 0 END), 0) as bank_income,
			COALESCE(SUM(amount), 0) as total_income
		FROM incomes 
		WHERE user_id = ? AND substr(date, 1, 4) = ?
	`
	
	var cashIncome, bankIncome, totalIncome float64
	err := db.QueryRow(query, userID, yearStr).Scan(&cashIncome, &bankIncome, &totalIncome)
	if err != nil {
		return err
	}

	// Update annual balance
	return updateAnnualBalance(userID, totalIncome, 0, 0, cashIncome, bankIncome, date)
}

// invalidateIncomeAnalytics invalida la caché de analytics para un usuario
func invalidateIncomeAnalytics(userID string) {
	// This is a placeholder function for invalidating income analytics cache
	// Implementation depends on the specific caching strategy used
	if cacheManager != nil {
		err := cacheManager.InvalidateIncomeCache(userID, "analytics")
		if err != nil {
			// Log error but don't fail the operation
		}
	}
}