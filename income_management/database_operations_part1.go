package main

import (
	"database/sql"
)

// addIncome inserta un nuevo ingreso en la base de datos
func addIncome(income Income) (int, error) {
	// Insert income into the database
	query := `
		INSERT INTO incomes (
			user_id, amount, date, category, payment_method, description
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		income.UserID,
		income.Amount,
		income.Date,
		income.Category,
		income.PaymentMethod,
		income.Description,
	)

	if err != nil {
		return 0, err
	}

	// Get the last inserted ID
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

// updateIncome actualiza un ingreso existente en la base de datos
func updateIncome(income Income) error {
	// Update income in the database
	query := `
		UPDATE incomes
		SET amount = ?, date = ?, category = ?, payment_method = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(
		query,
		income.Amount,
		income.Date,
		income.Category,
		income.PaymentMethod,
		income.Description,
		income.ID,
		income.UserID,
	)

	return err
}

// deleteIncome elimina un ingreso de la base de datos
func deleteIncome(incomeID int, userID string) error {
	// Delete income from the database
	query := `
		DELETE FROM incomes
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(query, incomeID, userID)
	return err
}

// getIncomeByID obtiene un ingreso específico por ID y userID
func getIncomeByID(userID string, incomeID int) (Income, error) {
	// Query to get a specific income
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE id = ? AND user_id = ?
	`

	var income Income
	err := db.QueryRow(query, incomeID, userID).Scan(
		&income.ID,
		&income.UserID,
		&income.Amount,
		&income.Date,
		&income.Category,
		&income.PaymentMethod,
		&income.Description,
		&income.CreatedAt,
		&income.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return income, err // Return empty income and error
	} else if err != nil {
		return income, err
	}

	return income, nil
}

// getIncomes obtiene ingresos con filtros opcionales
func getIncomes(userID, category, startDate, endDate, paymentMethod string) ([]Income, error) {
	// Base query
	query := `
		SELECT id, user_id, amount, date, category, payment_method, description, created_at, updated_at
		FROM incomes
		WHERE user_id = ?
	`
	
	// Build parameters slice
	params := []interface{}{userID}
	
	// Add filters dynamically
	if category != "" {
		query += " AND category = ?"
		params = append(params, category)
	}
	
	if startDate != "" {
		query += " AND date >= ?"
		params = append(params, startDate)
	}
	
	if endDate != "" {
		query += " AND date <= ?"
		params = append(params, endDate)
	}
	
	if paymentMethod != "" {
		query += " AND payment_method = ?"
		params = append(params, paymentMethod)
	}
	
	query += " ORDER BY date DESC"

	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incomes []Income

	for rows.Next() {
		var income Income
		if err := rows.Scan(
			&income.ID,
			&income.UserID,
			&income.Amount,
			&income.Date,
			&income.Category,
			&income.PaymentMethod,
			&income.Description,
			&income.CreatedAt,
			&income.UpdatedAt,
		); err != nil {
			return nil, err
		}

		incomes = append(incomes, income)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return incomes, nil
}