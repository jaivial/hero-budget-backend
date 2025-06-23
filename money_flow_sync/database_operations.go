package main

import ()

// updateBudgetData actualiza o inserta datos de presupuesto en la base de datos
func updateBudgetData(budget *BudgetData) error {
	// Check if a budget entry already exists for this user and period
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
		// Update existing budget entry with new calculated values
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
		// Insert new budget entry for user and period
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

// updateFinanceMetrics actualiza métricas financieras para reporting
func updateFinanceMetrics(userID, period string, income, expenses, bills float64) error {
	// Check if a finance metrics entry already exists for this user and period
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM finance_metrics 
		WHERE user_id = ? AND period = ?
	`, userID, period).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Update existing finance metrics entry
		_, err = db.Exec(`
			UPDATE finance_metrics
			SET income = ?,
				expenses = ?,
				bills = ?,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ? AND period = ?
		`,
			income,
			expenses,
			bills,
			userID,
			period,
		)
	} else {
		// Insert new finance metrics entry
		_, err = db.Exec(`
			INSERT INTO finance_metrics (
				user_id, period, income, expenses, bills
			) VALUES (?, ?, ?, ?, ?)
		`,
			userID,
			period,
			income,
			expenses,
			bills,
		)
	}

	return err
}