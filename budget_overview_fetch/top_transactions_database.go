package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// fetchTopExpenses retrieves the top expenses for a specific month
func fetchTopExpenses(userID, month string, limit int) ([]TopTransactionItem, error) {
	// Query to get top expenses for the specified month
	query := `
		SELECT 
			id,
			amount,
			date,
			category,
			payment_method,
			COALESCE(description, '') as description
		FROM expenses 
		WHERE user_id = ? 
		  AND strftime('%Y-%m', date) = ?
		ORDER BY amount DESC 
		LIMIT ?
	`

	rows, err := db.Query(query, userID, month, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top expenses: %v", err)
	}
	defer rows.Close()

	var topExpenses []TopTransactionItem
	for rows.Next() {
		var expense TopTransactionItem
		var description string
		
		err := rows.Scan(
			&expense.ID,
			&expense.Amount,
			&expense.Date,
			&expense.Category,
			&expense.PaymentMethod,
			&description,
		)
		if err != nil {
			log.Printf("Error scanning expense row: %v", err)
			continue
		}

		// Set transaction details
		expense.Type = "expense"
		expense.Name = description
		if expense.Name == "" && expense.Category != "" {
			expense.Name = expense.Category
		}
		if expense.Name == "" {
			expense.Name = "Expense"
		}

		// Get category icon from categories table
		expense.Icon = getCategoryIcon(userID, expense.Category, "expense", "💸")

		topExpenses = append(topExpenses, expense)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expense rows: %v", err)
	}

	log.Printf("📊 Fetched %d top expenses for user %s, month %s", len(topExpenses), userID, month)
	return topExpenses, nil
}

// fetchTopIncomes retrieves the top incomes for a specific month
func fetchTopIncomes(userID, month string, limit int) ([]TopTransactionItem, error) {
	// Query to get top incomes for the specified month
	query := `
		SELECT 
			id,
			amount,
			date,
			category,
			payment_method,
			COALESCE(description, '') as description
		FROM incomes 
		WHERE user_id = ? 
		  AND strftime('%Y-%m', date) = ?
		ORDER BY amount DESC 
		LIMIT ?
	`

	rows, err := db.Query(query, userID, month, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top incomes: %v", err)
	}
	defer rows.Close()

	var topIncomes []TopTransactionItem
	for rows.Next() {
		var income TopTransactionItem
		var description string
		
		err := rows.Scan(
			&income.ID,
			&income.Amount,
			&income.Date,
			&income.Category,
			&income.PaymentMethod,
			&description,
		)
		if err != nil {
			log.Printf("Error scanning income row: %v", err)
			continue
		}

		// Set transaction details
		income.Type = "income"
		income.Name = description
		if income.Name == "" && income.Category != "" {
			income.Name = income.Category
		}
		if income.Name == "" {
			income.Name = "Income"
		}

		// Get category icon from categories table
		income.Icon = getCategoryIcon(userID, income.Category, "income", "💰")

		topIncomes = append(topIncomes, income)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating income rows: %v", err)
	}

	log.Printf("📊 Fetched %d top incomes for user %s, month %s", len(topIncomes), userID, month)
	return topIncomes, nil
}

// fetchTopBills retrieves the top unpaid bills for a specific month
func fetchTopBills(userID, month string, limit int) ([]TopTransactionItem, error) {
	// Query to get top unpaid bills for the specified month
	query := `
		SELECT 
			b.id,
			b.amount,
			b.due_date as date,
			COALESCE(b.category, '') as category,
			b.payment_method,
			COALESCE(b.name, '') as name,
			COALESCE(b.icon, '') as icon,
			COALESCE(bp.paid, 0) as is_paid
		FROM bills b
		LEFT JOIN bill_payments bp ON b.id = bp.bill_id 
			AND strftime('%Y-%m', bp.payment_date) = ?
		WHERE b.user_id = ? 
		  AND strftime('%Y-%m', b.due_date) = ?
		  AND COALESCE(bp.paid, 0) = 0
		ORDER BY b.amount DESC 
		LIMIT ?
	`

	rows, err := db.Query(query, month, userID, month, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query top bills: %v", err)
	}
	defer rows.Close()

	var topBills []TopTransactionItem
	for rows.Next() {
		var bill TopTransactionItem
		var isPaidInt int
		
		err := rows.Scan(
			&bill.ID,
			&bill.Amount,
			&bill.Date,
			&bill.Category,
			&bill.PaymentMethod,
			&bill.Name,
			&bill.Icon,
			&isPaidInt,
		)
		if err != nil {
			log.Printf("Error scanning bill row: %v", err)
			continue
		}

		// Set transaction details
		bill.Type = "bill"
		if bill.Name == "" && bill.Category != "" {
			bill.Name = bill.Category
		}
		if bill.Name == "" {
			bill.Name = "Bill"
		}

		// Convert int to bool pointer
		isPaid := isPaidInt == 1
		bill.IsPaid = &isPaid

		// Use bill's own icon or get from categories table
		if bill.Icon == "" {
			bill.Icon = getCategoryIcon(userID, bill.Category, "expense", "🧾")
		}

		topBills = append(topBills, bill)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bill rows: %v", err)
	}

	log.Printf("📊 Fetched %d top bills for user %s, month %s", len(topBills), userID, month)
	return topBills, nil
}

// fetchCategoryStats retrieves category statistics for a specific type and month
func fetchCategoryStats(userID, month, categoryType string, limit int) ([]CategoryStatsItem, error) {
	var categoryStats []CategoryStatsItem
	var totalAmount float64

	if categoryType == "income" {
		stats, total, err := fetchIncomeCategoryStats(userID, month, limit)
		if err != nil {
			return nil, err
		}
		categoryStats = stats
		totalAmount = total
	} else if categoryType == "expense" {
		stats, total, err := fetchExpenseCategoryStats(userID, month, limit)
		if err != nil {
			return nil, err
		}
		categoryStats = stats
		totalAmount = total
	} else {
		return nil, fmt.Errorf("invalid category type: %s", categoryType)
	}

	// Calculate percentages
	for i := range categoryStats {
		if totalAmount > 0 {
			categoryStats[i].Percentage = (categoryStats[i].TotalAmount / totalAmount) * 100
		} else {
			categoryStats[i].Percentage = 0
		}
	}

	log.Printf("📊 Fetched %d category stats for %s, user %s, month %s", len(categoryStats), categoryType, userID, month)
	return categoryStats, nil
}

// fetchIncomeCategoryStats retrieves income category statistics
func fetchIncomeCategoryStats(userID, month string, limit int) ([]CategoryStatsItem, float64, error) {
	query := `
		SELECT 
			category,
			SUM(amount) as total_amount,
			COUNT(*) as transaction_count
		FROM incomes 
		WHERE user_id = ? 
		  AND strftime('%Y-%m', date) = ?
		GROUP BY category
		ORDER BY total_amount DESC 
		LIMIT ?
	`

	rows, err := db.Query(query, userID, month, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query income category stats: %v", err)
	}
	defer rows.Close()

	var categoryStats []CategoryStatsItem
	var grandTotal float64

	for rows.Next() {
		var stat CategoryStatsItem
		
		err := rows.Scan(
			&stat.Category,
			&stat.TotalAmount,
			&stat.TransactionCount,
		)
		if err != nil {
			log.Printf("Error scanning income category stat row: %v", err)
			continue
		}

		// Set additional fields
		stat.Type = "income"
		stat.DisplayName = stat.Category
		if stat.DisplayName == "" {
			stat.DisplayName = "Sin categoría"
		}

		// Get category icon
		stat.Icon = getCategoryIcon(userID, stat.Category, "income", "💰")

		categoryStats = append(categoryStats, stat)
		grandTotal += stat.TotalAmount
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating income category stat rows: %v", err)
	}

	return categoryStats, grandTotal, nil
}

// fetchExpenseCategoryStats retrieves expense category statistics (expenses + unpaid bills)
func fetchExpenseCategoryStats(userID, month string, limit int) ([]CategoryStatsItem, float64, error) {
	// First, get expense categories
	expenseQuery := `
		SELECT 
			category,
			SUM(amount) as total_amount,
			COUNT(*) as transaction_count
		FROM expenses 
		WHERE user_id = ? 
		  AND strftime('%Y-%m', date) = ?
		GROUP BY category
	`

	// Second, get unpaid bill categories for the month
	billQuery := `
		SELECT 
			b.category,
			SUM(b.amount) as total_amount,
			COUNT(*) as transaction_count
		FROM bills b
		LEFT JOIN bill_payments bp ON b.id = bp.bill_id 
			AND strftime('%Y-%m', bp.payment_date) = ?
		WHERE b.user_id = ? 
		  AND strftime('%Y-%m', b.due_date) = ?
		  AND COALESCE(bp.paid, 0) = 0
		GROUP BY b.category
	`

	// Execute expense query
	expenseRows, err := db.Query(expenseQuery, userID, month)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query expense category stats: %v", err)
	}
	defer expenseRows.Close()

	categoryMap := make(map[string]*CategoryStatsItem)

	// Process expenses
	for expenseRows.Next() {
		var category string
		var totalAmount float64
		var transactionCount int
		
		err := expenseRows.Scan(&category, &totalAmount, &transactionCount)
		if err != nil {
			log.Printf("Error scanning expense category stat row: %v", err)
			continue
		}

		// Normalize empty category
		if category == "" {
			category = "Sin categoría"
		}

		categoryMap[category] = &CategoryStatsItem{
			Category:         category,
			DisplayName:      category,
			TotalAmount:      totalAmount,
			TransactionCount: transactionCount,
			Type:             "expense",
		}
	}

	// Execute bill query
	billRows, err := db.Query(billQuery, month, userID, month)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query bill category stats: %v", err)
	}
	defer billRows.Close()

	// Process bills
	for billRows.Next() {
		var category string
		var totalAmount float64
		var transactionCount int
		
		err := billRows.Scan(&category, &totalAmount, &transactionCount)
		if err != nil {
			log.Printf("Error scanning bill category stat row: %v", err)
			continue
		}

		// Normalize empty category
		if category == "" {
			category = "Sin categoría"
		}

		if existingStat, exists := categoryMap[category]; exists {
			// Add to existing expense category
			existingStat.TotalAmount += totalAmount
			existingStat.TransactionCount += transactionCount
		} else {
			// Create new category for bills only
			categoryMap[category] = &CategoryStatsItem{
				Category:         category,
				DisplayName:      category,
				TotalAmount:      totalAmount,
				TransactionCount: transactionCount,
				Type:             "expense",
			}
		}
	}

	// Convert map to slice and add icons
	var categoryStats []CategoryStatsItem
	var grandTotal float64

	for _, stat := range categoryMap {
		// Get category icon
		stat.Icon = getCategoryIcon(userID, stat.Category, "expense", "💸")
		categoryStats = append(categoryStats, *stat)
		grandTotal += stat.TotalAmount
	}

	// Sort by total amount (descending) and limit results
	// Note: Go doesn't have built-in sorting for custom slices, so we'll implement simple sorting
	for i := 0; i < len(categoryStats)-1; i++ {
		for j := i + 1; j < len(categoryStats); j++ {
			if categoryStats[i].TotalAmount < categoryStats[j].TotalAmount {
				categoryStats[i], categoryStats[j] = categoryStats[j], categoryStats[i]
			}
		}
	}

	// Apply limit
	if len(categoryStats) > limit {
		categoryStats = categoryStats[:limit]
	}

	return categoryStats, grandTotal, nil
}

// getCategoryIcon retrieves the icon for a category from the categories table
func getCategoryIcon(userID, category, categoryType, fallback string) string {
	if category == "" {
		return fallback
	}

	query := `
		SELECT emoji 
		FROM categories 
		WHERE user_id = ? 
		  AND LOWER(name) = LOWER(?) 
		  AND type = ?
		LIMIT 1
	`

	var icon string
	err := db.QueryRow(query, userID, category, categoryType).Scan(&icon)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error fetching category icon: %v", err)
		}
		// Try case-insensitive partial match
		query = `
			SELECT emoji 
			FROM categories 
			WHERE user_id = ? 
			  AND (LOWER(name) LIKE '%' || LOWER(?) || '%' OR LOWER(?) LIKE '%' || LOWER(name) || '%')
			  AND type = ?
			ORDER BY 
				CASE 
					WHEN LOWER(name) = LOWER(?) THEN 1
					WHEN LOWER(name) LIKE LOWER(?) || '%' THEN 2
					WHEN LOWER(name) LIKE '%' || LOWER(?) || '%' THEN 3
					ELSE 4
				END
			LIMIT 1
		`
		
		err = db.QueryRow(query, userID, category, category, categoryType, category, category, category).Scan(&icon)
		if err != nil {
			return fallback
		}
	}

	if icon == "" {
		return fallback
	}

	return icon
}