package main

import (
	"database/sql"
	"log"
	"time"
)

// fetchDashboardData obtiene y agrega todos los datos del dashboard
func fetchDashboardData(userID, period string) (DashboardData, error) {
	var dashboardData DashboardData
	dashboardData.Period = period

	// Get current date for dashboard data timestamp
	now := time.Now()
	dashboardData.Date = now.Format("2006-01-02")

	// Get budget overview with financial calculations
	budgetOverview, err := fetchBudgetOverview(userID, period)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.BudgetOverview = budgetOverview

	// Get savings overview with goal tracking
	savingsOverview, err := fetchSavingsOverview(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.SavingsOverview = savingsOverview

	// Get cash bank distribution for liquidity analysis
	cashBank, err := fetchCashBankDistribution(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.CashDistribution = cashBank

	// Get finance metrics for period overview
	financeMetrics, err := fetchFinanceMetrics(userID, period)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.FinanceMetrics = financeMetrics

	// Get upcoming bills for payment planning
	upcomingBills, err := fetchUpcomingBills(userID)
	if err != nil {
		return dashboardData, err
	}
	dashboardData.UpcomingBills = upcomingBills

	return dashboardData, nil
}

// fetchBudgetOverview obtiene datos del presupuesto con cálculos financieros
func fetchBudgetOverview(userID, period string) (BudgetOverview, error) {
	var budgetOverview BudgetOverview

	// Query budget data from database for the specified period
	err := db.QueryRow(`
		SELECT total_amount, remaining_amount, spent_amount, upcoming_amount, from_previous, percent
		FROM budget
		WHERE user_id = ? AND period = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, period).Scan(
		&budgetOverview.TotalAmount,
		&budgetOverview.RemainingAmount,
		&budgetOverview.SpentAmount,
		&budgetOverview.UpcomingAmount,
		&budgetOverview.MoneyFlow.FromPrevious,
		&budgetOverview.MoneyFlow.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no budget data found
		budgetOverview.TotalAmount = 0
		budgetOverview.RemainingAmount = 0
		budgetOverview.SpentAmount = 0
		budgetOverview.UpcomingAmount = 0
		budgetOverview.MoneyFlow.FromPrevious = 0
		budgetOverview.MoneyFlow.Percent = 0
	} else if err != nil {
		return budgetOverview, err
	}

	// Calculate the total income for the specified period
	var startDate, endDate string
	now := time.Now()

	// Determine date range based on period type
	switch period {
	case "daily":
		startDate = now.Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	case "weekly":
		// Start of the week (Monday) to end of week (Sunday)
		startDate = now.AddDate(0, 0, -int(now.Weekday())+1).Format("2006-01-02")
		endDate = now.AddDate(0, 0, 7-int(now.Weekday())).Format("2006-01-02")
	case "monthly":
		// Start of current month to end of current month
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "quarterly":
		quarter := (int(now.Month())-1)/3 + 1
		startDate = time.Date(now.Year(), time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), time.Month(quarter*3+1), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "semiannual":
		halfYear := (int(now.Month())-1)/6 + 1
		startDate = time.Date(now.Year(), time.Month((halfYear-1)*6+1), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), time.Month(halfYear*6+1), 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	case "annual":
		startDate = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), 12, 31, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	default:
		// Default to monthly if period is not recognized
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	}

	// Get total income for the calculated period
	var totalIncome float64
	err = db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM incomes
		WHERE user_id = ? AND date BETWEEN ? AND ?
	`, userID, startDate, endDate).Scan(&totalIncome)

	if err != nil {
		log.Printf("Error fetching total income: %v", err)
		totalIncome = 0 // Default to 0 if there's an error
	}

	budgetOverview.TotalIncome = totalIncome

	// Calculate combined expense and expense percentage
	budgetOverview.CombinedExpense = budgetOverview.SpentAmount + budgetOverview.UpcomingAmount
	if budgetOverview.TotalAmount > 0 {
		budgetOverview.ExpensePercent = (budgetOverview.CombinedExpense / budgetOverview.TotalAmount) * 100
	}

	// Calculate daily spending rate based on period
	daysInPeriod := 30 // Default for monthly
	switch period {
	case "daily":
		daysInPeriod = 1
	case "weekly":
		daysInPeriod = 7
	case "monthly":
		// Calculate actual days in the current month
		year, month, _ := time.Now().Date()
		lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Now().Location())
		daysInPeriod = lastDay.Day()
	case "quarterly":
		daysInPeriod = 90
	case "semiannual":
		daysInPeriod = 180
	case "annual":
		daysInPeriod = 365
	}

	if daysInPeriod > 0 {
		budgetOverview.DailyRate = budgetOverview.CombinedExpense / float64(daysInPeriod)
	}

	// Determine high spending warning based on period progress
	currentDay := time.Now().Day()
	if period == "monthly" && currentDay <= 10 && budgetOverview.ExpensePercent > 50 {
		budgetOverview.HighSpending = true
	}

	return budgetOverview, nil
}

// fetchSavingsOverview obtiene datos de ahorros con cálculos de objetivos
func fetchSavingsOverview(userID string) (SavingsOverview, error) {
	var savingsOverview SavingsOverview

	// Query savings data from database for user
	err := db.QueryRow(`
		SELECT available, goal, period, percent
		FROM savings
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(
		&savingsOverview.Available,
		&savingsOverview.Goal,
		&savingsOverview.Period,
		&savingsOverview.Percent,
	)

	if err == sql.ErrNoRows {
		// Return default values if no savings data found
		savingsOverview.Available = 0
		savingsOverview.Goal = 0
		savingsOverview.Period = "monthly" // Default period
		savingsOverview.Percent = 0
		return savingsOverview, nil
	} else if err != nil {
		return savingsOverview, err
	}

	// Calculate how much more needs to be saved to reach goal
	savingsOverview.NeedToSave = savingsOverview.Goal - savingsOverview.Available
	if savingsOverview.NeedToSave < 0 {
		savingsOverview.NeedToSave = 0
	}

	// Calculate daily target assuming goal needs to be achieved within a month (30 days)
	savingsOverview.DailyTarget = savingsOverview.NeedToSave / 30

	return savingsOverview, nil
}