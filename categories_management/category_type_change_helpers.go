package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// CheckCategoryTransactionsResult represents the result of checking category transactions
// Contains counts of linked transactions and their types for validation before type change
type CheckCategoryTransactionsResult struct {
	HasTransactions bool   `json:"has_transactions"` // True if category has any linked transactions
	Count           int    `json:"count"`            // Total count of transactions
	ExpenseCount    int    `json:"expense_count"`    // Count of expense transactions
	IncomeCount     int    `json:"income_count"`     // Count of income transactions
	TransactionType string `json:"transaction_type"` // "income", "expense", "both", or empty
}

// checkCategoryTransactions checks if a category has linked transactions
// Queries both expenses and incomes tables to count transactions with matching category_id
// Returns detailed breakdown of transaction counts and types for validation
//
// Purpose: Validate if category type change requires cascade re-calculation
// Used before displaying confirmation modal to user
//
// Algorithm:
// 1. Query expenses table for transactions with category_id
// 2. Query incomes table for transactions with category_id
// 3. Calculate totals and determine transaction type
// 4. Return structured result
//
// Parameters:
//   - db: Database connection
//   - categoryID: ID of category to check
//   - userID: User ID for permission validation
//
// Returns:
//   - CheckCategoryTransactionsResult with transaction counts
//   - error if database query fails
func checkCategoryTransactions(db *sql.DB, categoryID int, userID string) (CheckCategoryTransactionsResult, error) {
	operationID := fmt.Sprintf("check-category-tx-%d-%s", categoryID, userID)
	log.Printf("🔍 Checking transactions for category: ID=%d, User=%s, Operation=%s", categoryID, userID, operationID)

	result := CheckCategoryTransactionsResult{
		HasTransactions: false,
		Count:           0,
		ExpenseCount:    0,
		IncomeCount:     0,
		TransactionType: "",
	}

	// Step 1: Count expense transactions with this category_id
	var expenseCount int
	expenseQuery := `
		SELECT COUNT(*)
		FROM expenses
		WHERE category_id = ? AND user_id = ?
	`
	err := db.QueryRow(expenseQuery, categoryID, userID).Scan(&expenseCount)
	if err != nil {
		log.Printf("❌ Error counting expense transactions: %v", err)
		return result, fmt.Errorf("failed to count expense transactions: %v", err)
	}

	log.Printf("📊 Found %d expense transaction(s) for category ID %d", expenseCount, categoryID)

	// Step 2: Count income transactions with this category_id
	var incomeCount int
	incomeQuery := `
		SELECT COUNT(*)
		FROM incomes
		WHERE category_id = ? AND user_id = ?
	`
	err = db.QueryRow(incomeQuery, categoryID, userID).Scan(&incomeCount)
	if err != nil {
		log.Printf("❌ Error counting income transactions: %v", err)
		return result, fmt.Errorf("failed to count income transactions: %v", err)
	}

	log.Printf("📊 Found %d income transaction(s) for category ID %d", incomeCount, categoryID)

	// Step 3: Calculate totals and determine transaction type
	totalCount := expenseCount + incomeCount
	result.Count = totalCount
	result.ExpenseCount = expenseCount
	result.IncomeCount = incomeCount
	result.HasTransactions = totalCount > 0

	// Determine transaction type based on counts
	if expenseCount > 0 && incomeCount > 0 {
		result.TransactionType = "both"
		log.Printf("⚠️ Category has BOTH income and expense transactions (unusual case)")
	} else if expenseCount > 0 {
		result.TransactionType = "expense"
		log.Printf("💸 Category has expense transactions only")
	} else if incomeCount > 0 {
		result.TransactionType = "income"
		log.Printf("💰 Category has income transactions only")
	} else {
		result.TransactionType = ""
		log.Printf("✅ Category has no linked transactions")
	}

	log.Printf("✅ Transaction check complete: Total=%d, Expense=%d, Income=%d, Type=%s",
		result.Count, result.ExpenseCount, result.IncomeCount, result.TransactionType)

	return result, nil
}

// handleCheckCategoryTransactions HTTP handler for checking category transactions
// Endpoint: GET /categories/check-transactions?category_id=123&user_id=abc
// Returns JSON response with transaction counts and validation info
//
// Purpose: API endpoint for frontend to validate category before type change
// Used by EditCategoryScreen to determine if confirmation modal should be shown
//
// Query Parameters:
//   - category_id: Required, category ID to check
//   - user_id: Required, user ID for permission validation
//
// Response Format:
// {
//   "success": true,
//   "message": "Transaction check completed",
//   "data": {
//     "has_transactions": true,
//     "count": 15,
//     "expense_count": 15,
//     "income_count": 0,
//     "transaction_type": "expense"
//   }
// }
func handleCheckCategoryTransactions(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only GET allowed
	if r.Method != "GET" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract and validate query parameters
	categoryIDStr := r.URL.Query().Get("category_id")
	userID := r.URL.Query().Get("user_id")

	if categoryIDStr == "" {
		sendErrorResponse(w, "category_id is required", http.StatusBadRequest)
		return
	}

	if userID == "" {
		sendErrorResponse(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Parse category ID to integer
	categoryID, err := strconv.Atoi(categoryIDStr)
	if err != nil {
		sendErrorResponse(w, "Invalid category_id format", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Check category transactions request: CategoryID=%d, UserID=%s", categoryID, userID)

	// Verify category exists and belongs to user
	var existingCategoryUserID string
	err = db.QueryRow("SELECT user_id FROM categories WHERE id = ?", categoryID).Scan(&existingCategoryUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			sendErrorResponse(w, "Category not found", http.StatusNotFound)
		} else {
			log.Printf("❌ Error checking category existence: %v", err)
			sendErrorResponse(w, "Error checking category", http.StatusInternalServerError)
		}
		return
	}

	// Verify user owns this category
	if existingCategoryUserID != userID {
		sendErrorResponse(w, "Unauthorized: Category does not belong to user", http.StatusForbidden)
		return
	}

	// Check category transactions
	result, err := checkCategoryTransactions(db, categoryID, userID)
	if err != nil {
		log.Printf("❌ Error checking category transactions: %v", err)
		sendErrorResponse(w, "Error checking category transactions", http.StatusInternalServerError)
		return
	}

	// Return success response with transaction check result
	sendSuccessResponse(w, "Transaction check completed", result)
}

// CategoryTransaction represents a single transaction linked to a category
// Contains all necessary information for cascade re-calculation operations
type CategoryTransaction struct {
	ID            int     `json:"id"`             // Transaction ID
	Amount        float64 `json:"amount"`         // Transaction amount
	Date          string  `json:"date"`           // Transaction date (YYYY-MM-DD format)
	YearMonth     string  `json:"year_month"`     // Year-month extracted from date (YYYY-MM)
	PaymentMethod string  `json:"payment_method"` // "cash" or "bank"
	Type          string  `json:"type"`           // "income" or "expense"
}

// getTransactionsForCategory fetches all transactions linked to a category
// Queries both expenses and incomes tables, combines results, and sorts chronologically
// Extracts year-month from transaction dates for monthly balance calculations
//
// Purpose: Get complete list of transactions for cascade re-calculation
// Used when category type changes to recalculate monthly balance impacts
//
// Algorithm:
// 1. Query expenses table WHERE category_id matches
// 2. Query incomes table WHERE category_id matches
// 3. Combine both result sets
// 4. Sort by date chronologically (oldest first)
// 5. Extract year_month (YYYY-MM) from each transaction date
// 6. Return sorted transaction array
//
// Parameters:
//   - db: Database connection (or transaction)
//   - categoryID: ID of category to fetch transactions for
//   - userID: User ID for permission validation
//
// Returns:
//   - []CategoryTransaction: Array of transactions sorted by date
//   - error if database query fails
func getTransactionsForCategory(db *sql.DB, categoryID int, userID string) ([]CategoryTransaction, error) {
	operationID := fmt.Sprintf("get-category-tx-%d-%s", categoryID, userID)
	startTime := time.Now()

	log.Printf("📥 Fetching transactions for category: ID=%d, User=%s, Operation=%s", categoryID, userID, operationID)

	var transactions []CategoryTransaction

	// Step 1: Fetch expense transactions
	expenseQuery := `
		SELECT id, amount, date, payment_method
		FROM expenses
		WHERE category_id = ? AND user_id = ?
		ORDER BY date ASC
	`

	expenseRows, err := db.Query(expenseQuery, categoryID, userID)
	if err != nil {
		log.Printf("❌ Error querying expense transactions: %v", err)
		return nil, fmt.Errorf("failed to query expense transactions: %v", err)
	}
	defer expenseRows.Close()

	expenseCount := 0
	for expenseRows.Next() {
		var tx CategoryTransaction
		err := expenseRows.Scan(&tx.ID, &tx.Amount, &tx.Date, &tx.PaymentMethod)
		if err != nil {
			log.Printf("❌ Error scanning expense row: %v", err)
			continue
		}

		tx.Type = "expense"

		// Extract year-month from date (YYYY-MM-DD -> YYYY-MM)
		if len(tx.Date) >= 7 {
			tx.YearMonth = tx.Date[0:7] // Extract "YYYY-MM"
		} else {
			log.Printf("⚠️ Invalid date format for expense ID %d: %s", tx.ID, tx.Date)
			continue
		}

		transactions = append(transactions, tx)
		expenseCount++
	}

	log.Printf("📊 Found %d expense transaction(s)", expenseCount)

	// Step 2: Fetch income transactions
	incomeQuery := `
		SELECT id, amount, date, payment_method
		FROM incomes
		WHERE category_id = ? AND user_id = ?
		ORDER BY date ASC
	`

	incomeRows, err := db.Query(incomeQuery, categoryID, userID)
	if err != nil {
		log.Printf("❌ Error querying income transactions: %v", err)
		return nil, fmt.Errorf("failed to query income transactions: %v", err)
	}
	defer incomeRows.Close()

	incomeCount := 0
	for incomeRows.Next() {
		var tx CategoryTransaction
		err := incomeRows.Scan(&tx.ID, &tx.Amount, &tx.Date, &tx.PaymentMethod)
		if err != nil {
			log.Printf("❌ Error scanning income row: %v", err)
			continue
		}

		tx.Type = "income"

		// Extract year-month from date (YYYY-MM-DD -> YYYY-MM)
		if len(tx.Date) >= 7 {
			tx.YearMonth = tx.Date[0:7] // Extract "YYYY-MM"
		} else {
			log.Printf("⚠️ Invalid date format for income ID %d: %s", tx.ID, tx.Date)
			continue
		}

		transactions = append(transactions, tx)
		incomeCount++
	}

	log.Printf("📊 Found %d income transaction(s)", incomeCount)

	// Step 3: Sort combined transactions by date chronologically
	// Using custom sort to ensure chronological order
	sortTransactionsByDate(transactions)

	duration := time.Since(startTime)
	log.Printf("✅ Transaction fetch complete: Total=%d, Expense=%d, Income=%d, Duration=%v",
		len(transactions), expenseCount, incomeCount, duration)

	// Log first and last transaction for debugging
	if len(transactions) > 0 {
		log.Printf("📅 First transaction: Date=%s, YearMonth=%s, Type=%s, Amount=%.2f",
			transactions[0].Date, transactions[0].YearMonth, transactions[0].Type, transactions[0].Amount)
		log.Printf("📅 Last transaction: Date=%s, YearMonth=%s, Type=%s, Amount=%.2f",
			transactions[len(transactions)-1].Date, transactions[len(transactions)-1].YearMonth,
			transactions[len(transactions)-1].Type, transactions[len(transactions)-1].Amount)
	}

	return transactions, nil
}

// sortTransactionsByDate sorts transactions chronologically by date (oldest first)
// Uses simple bubble sort for small transaction arrays (efficient for typical use case)
func sortTransactionsByDate(transactions []CategoryTransaction) {
	n := len(transactions)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			// Compare dates as strings (YYYY-MM-DD format naturally sorts correctly)
			if strings.Compare(transactions[j].Date, transactions[j+1].Date) > 0 {
				// Swap
				transactions[j], transactions[j+1] = transactions[j+1], transactions[j]
			}
		}
	}
}

// MonthlyTransactionAmount represents aggregated transaction amounts for a single month
// Used to calculate how much to adjust monthly balance columns during type change
type MonthlyTransactionAmount struct {
	YearMonth  string  `json:"year_month"`   // Month in YYYY-MM format
	CashAmount float64 `json:"cash_amount"`  // Total cash transactions for this month
	BankAmount float64 `json:"bank_amount"`  // Total bank transactions for this month
	TotalAmount float64 `json:"total_amount"` // Total all transactions for this month
	TransactionCount int `json:"transaction_count"` // Number of transactions in this month
}

// calculateMonthlyTransactionAmounts groups transactions by month and calculates totals
// Used to determine how much to adjust income/expense columns when category type changes
//
// Purpose: Calculate monthly adjustments needed for type change cascade
// Unlike frontend ×2 multiplier approach, backend directly updates source columns
// and relies on cumulative recalculation to propagate changes
//
// Backend Simplification:
// Instead of frontend's complex ×2 reversal logic, we:
// 1. Group transactions by year_month
// 2. Calculate total amount per month (separated by cash/bank)
// 3. Move amounts from old type columns to new type columns
// 4. Call recalculateAllCumulativeBalances() to propagate
//
// Algorithm:
// 1. Create map indexed by year_month
// 2. For each transaction:
//    - Add amount to appropriate payment method (cash or bank)
//    - Track transaction count
// 3. Calculate totals for each month
// 4. Return sorted array of monthly amounts
//
// Example:
// Input: 3 transactions in 2025-01 (2 cash, 1 bank)
// Output: MonthlyTransactionAmount{
//   YearMonth: "2025-01",
//   CashAmount: 150.00,
//   BankAmount: 100.00,
//   TotalAmount: 250.00,
//   TransactionCount: 3
// }
//
// Parameters:
//   - transactions: Array of transactions (already sorted by date)
//
// Returns:
//   - []MonthlyTransactionAmount: Monthly amounts sorted chronologically
func calculateMonthlyTransactionAmounts(transactions []CategoryTransaction) []MonthlyTransactionAmount {
	operationID := fmt.Sprintf("calc-monthly-amounts-%d", time.Now().UnixNano())
	log.Printf("📊 Calculating monthly transaction amounts: Operation=%s, Transactions=%d", operationID, len(transactions))

	// Step 1: Group transactions by year_month using map
	monthlyMap := make(map[string]*MonthlyTransactionAmount)

	for _, tx := range transactions {
		// Get or create monthly amount entry
		if monthlyMap[tx.YearMonth] == nil {
			monthlyMap[tx.YearMonth] = &MonthlyTransactionAmount{
				YearMonth:        tx.YearMonth,
				CashAmount:       0,
				BankAmount:       0,
				TotalAmount:      0,
				TransactionCount: 0,
			}
		}

		monthlyAmount := monthlyMap[tx.YearMonth]

		// Add to appropriate payment method total
		if tx.PaymentMethod == "cash" {
			monthlyAmount.CashAmount += tx.Amount
		} else if tx.PaymentMethod == "bank" {
			monthlyAmount.BankAmount += tx.Amount
		} else {
			log.Printf("⚠️ Unknown payment method '%s' for transaction ID %d, treating as bank", tx.PaymentMethod, tx.ID)
			monthlyAmount.BankAmount += tx.Amount
		}

		// Update totals
		monthlyAmount.TotalAmount += tx.Amount
		monthlyAmount.TransactionCount++
	}

	log.Printf("📊 Grouped into %d month(s)", len(monthlyMap))

	// Step 2: Convert map to sorted array
	var monthlyAmounts []MonthlyTransactionAmount

	for _, amount := range monthlyMap {
		monthlyAmounts = append(monthlyAmounts, *amount)
	}

	// Step 3: Sort by year_month chronologically
	sortMonthlyAmountsByYearMonth(monthlyAmounts)

	// Step 4: Log summary
	for i, amount := range monthlyAmounts {
		log.Printf("📅 Month %d: %s - Cash=%.2f, Bank=%.2f, Total=%.2f, Count=%d",
			i+1, amount.YearMonth, amount.CashAmount, amount.BankAmount, amount.TotalAmount, amount.TransactionCount)
	}

	log.Printf("✅ Monthly amounts calculated for %d month(s)", len(monthlyAmounts))

	return monthlyAmounts
}

// sortMonthlyAmountsByYearMonth sorts monthly amounts chronologically
// Uses bubble sort for small arrays (typically < 50 months)
func sortMonthlyAmountsByYearMonth(amounts []MonthlyTransactionAmount) {
	n := len(amounts)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			// Compare year_month strings (YYYY-MM format sorts correctly)
			if strings.Compare(amounts[j].YearMonth, amounts[j+1].YearMonth) > 0 {
				// Swap
				amounts[j], amounts[j+1] = amounts[j+1], amounts[j]
			}
		}
	}
}

// applyTypeChangeToMonthlyBalance updates monthly_cash_bank_balance columns for type change
// Moves amounts from old type columns to new type columns for each affected month
//
// Purpose: Core operation for category type change cascade
// This replaces frontend's complex ×2 reversal + re-application logic
//
// Backend Approach (Simpler):
// For income → expense:
//   - Subtract amounts from income_bank_amount and income_cash_amount
//   - Add amounts to expense_bank_amount and expense_cash_amount
//
// For expense → income:
//   - Subtract amounts from expense_bank_amount and expense_cash_amount
//   - Add amounts to income_bank_amount and income_cash_amount
//
// Then call recalculateAllCumulativeBalances() to propagate changes forward
//
// Algorithm:
// 1. For each month with transactions:
//    - Ensure month record exists in monthly_cash_bank_balance
//    - Update income/expense columns based on type change direction
// 2. Return list of affected months for cumulative recalculation
//
// Parameters:
//   - db: Database connection (or transaction)
//   - userID: User ID for permission validation
//   - monthlyAmounts: Monthly transaction amounts to move between columns
//   - oldType: Original category type ("income" or "expense")
//   - newType: New category type ("income" or "expense")
//
// Returns:
//   - firstAffectedMonth: First month that needs recalculation (for cascade)
//   - error if database update fails
func applyTypeChangeToMonthlyBalance(db *sql.DB, userID string, monthlyAmounts []MonthlyTransactionAmount, oldType, newType string) (string, error) {
	operationID := fmt.Sprintf("apply-type-change-%s", time.Now().Format("20060102-150405"))
	startTime := time.Now()

	log.Printf("🔄 Applying type change to monthly balance: Operation=%s, Months=%d, %s→%s",
		operationID, len(monthlyAmounts), oldType, newType)

	if len(monthlyAmounts) == 0 {
		log.Printf("ℹ️ No monthly amounts to apply")
		return "", nil
	}

	// Track first affected month for cumulative recalculation
	firstAffectedMonth := monthlyAmounts[0].YearMonth

	// Process each month
	for i, amount := range monthlyAmounts {
		log.Printf("📝 Processing month %d/%d: %s (Cash=%.2f, Bank=%.2f)",
			i+1, len(monthlyAmounts), amount.YearMonth, amount.CashAmount, amount.BankAmount)

		// Step 1: Ensure month record exists
		_, err := db.Exec(`
			INSERT OR IGNORE INTO monthly_cash_bank_balance (user_id, year_month)
			VALUES (?, ?)
		`, userID, amount.YearMonth)

		if err != nil {
			log.Printf("❌ Error ensuring month record exists for %s: %v", amount.YearMonth, err)
			return "", fmt.Errorf("failed to ensure month record for %s: %v", amount.YearMonth, err)
		}

		// Step 2: Update income/expense columns based on type change direction
		var updateQuery string
		var queryParams []interface{}

		if oldType == "expense" && newType == "income" {
			// Was expense, now income
			// Subtract from expense columns, add to income columns
			updateQuery = `
				UPDATE monthly_cash_bank_balance
				SET expense_bank_amount = COALESCE(expense_bank_amount, 0) - ?,
				    expense_cash_amount = COALESCE(expense_cash_amount, 0) - ?,
				    income_bank_amount = COALESCE(income_bank_amount, 0) + ?,
				    income_cash_amount = COALESCE(income_cash_amount, 0) + ?,
				    updated_at = datetime('now')
				WHERE user_id = ? AND year_month = ?
			`
			queryParams = []interface{}{
				amount.BankAmount, amount.CashAmount, // Subtract from expense
				amount.BankAmount, amount.CashAmount, // Add to income
				userID, amount.YearMonth,
			}

			log.Printf("💸→💰 Expense→Income: Subtracting from expense columns, adding to income columns")

		} else if oldType == "income" && newType == "expense" {
			// Was income, now expense
			// Subtract from income columns, add to expense columns
			updateQuery = `
				UPDATE monthly_cash_bank_balance
				SET income_bank_amount = COALESCE(income_bank_amount, 0) - ?,
				    income_cash_amount = COALESCE(income_cash_amount, 0) - ?,
				    expense_bank_amount = COALESCE(expense_bank_amount, 0) + ?,
				    expense_cash_amount = COALESCE(expense_cash_amount, 0) + ?,
				    updated_at = datetime('now')
				WHERE user_id = ? AND year_month = ?
			`
			queryParams = []interface{}{
				amount.BankAmount, amount.CashAmount, // Subtract from income
				amount.BankAmount, amount.CashAmount, // Add to expense
				userID, amount.YearMonth,
			}

			log.Printf("💰→💸 Income→Expense: Subtracting from income columns, adding to expense columns")

		} else {
			// Type hasn't actually changed (shouldn't happen, but handle gracefully)
			log.Printf("⚠️ Type unchanged (%s→%s), skipping update", oldType, newType)
			continue
		}

		// Execute update
		result, err := db.Exec(updateQuery, queryParams...)
		if err != nil {
			log.Printf("❌ Error updating monthly balance for %s: %v", amount.YearMonth, err)
			return "", fmt.Errorf("failed to update monthly balance for %s: %v", amount.YearMonth, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("✅ Month %s updated successfully", amount.YearMonth)
		} else {
			log.Printf("⚠️ Month %s update affected 0 rows", amount.YearMonth)
		}
	}

	duration := time.Since(startTime)
	log.Printf("✅ Type change applied to %d month(s) in %v", len(monthlyAmounts), duration)
	log.Printf("📍 First affected month: %s (will trigger cumulative recalculation)", firstAffectedMonth)

	return firstAffectedMonth, nil
}

// recalculateAllCumulativeBalances recalculates all balances cumulatively from a start month
// This function implements cumulative cascade propagation for monthly balances
//
// Purpose: Apply cascade effect after type change to monthly_cash_bank_balance
// Replaces frontend's explicit reversal steps (Steps 6-8) with automatic recalculation
//
// Architecture: Cumulative Forward Propagation
// Each month builds on the previous month's final balance:
// - previous_cash_amount = previous month's cash_amount
// - previous_bank_amount = previous month's bank_amount
// - cash_amount = previous_cash + income_cash - expense_cash - bill_cash
// - bank_amount = previous_bank + income_bank - expense_bank - bill_bank
// - total_balance = cash_amount + bank_amount
//
// This automatically handles:
// 1. Balance column cascade (equivalent to frontend applyCascadeReversal)
// 2. 1-month delay for previous balance (equivalent to frontend applyDelayedPreviousBalance)
// 3. Income/expense column propagation (equivalent to frontend applyIncomeExpenseReversal)
//
// Algorithm:
// 1. Query all months from startMonth forward, sorted chronologically
// 2. For each month:
//    a. Get income/expense/bill amounts from database
//    b. Get previous month's final balance (if not first month)
//    c. Calculate new balances using cumulative formula
//    d. Update all balance columns
// 3. Continue until all months processed
//
// Parameters:
//   - db: Database connection (or transaction)
//   - userID: User ID for permission validation
//   - startMonth: First month to recalculate (YYYY-MM format)
//
// Returns:
//   - error if database operation fails
func recalculateAllCumulativeBalances(db *sql.DB, userID, startMonth string) error {
	operationID := fmt.Sprintf("recalc-cumulative-%s-%s", userID, startMonth)
	startTime := time.Now()

	log.Printf("🔄 Recalculating cumulative balances from %s for user %s (Operation: %s)", startMonth, userID, operationID)

	// Step 1: Get all months from startMonth forward, ordered chronologically
	rows, err := db.Query(`
		SELECT year_month
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month >= ?
		ORDER BY year_month
	`, userID, startMonth)
	if err != nil {
		log.Printf("❌ Error fetching months for recalculation: %v", err)
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
		log.Printf("ℹ️ No months to recalculate")
		return nil
	}

	log.Printf("📊 Processing %d month(s) for cumulative recalculation", len(months))

	// Step 2: Process each month cumulatively
	for i, currentMonth := range months {
		log.Printf("📊 Processing month %s (%d/%d)", currentMonth, i+1, len(months))

		// Step 2a: Get income/expense/bill amounts for current month
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
			log.Printf("❌ Error getting data for month %s: %v", currentMonth, err)
			continue
		}

		// Step 2b: Calculate previous_amounts from previous month's balance
		var prevCash, prevBank float64
		if i > 0 {
			// Get final balance from previous month
			previousMonth := months[i-1]
			err := db.QueryRow(`
				SELECT COALESCE(cash_amount, 0), COALESCE(bank_amount, 0)
				FROM monthly_cash_bank_balance
				WHERE user_id = ? AND year_month = ?
			`, userID, previousMonth).Scan(&prevCash, &prevBank)

			if err != nil {
				log.Printf("⚠️ Error getting previous month balance for %s: %v", previousMonth, err)
				// Use 0 if previous month not found
				prevCash = 0
				prevBank = 0
			}
		}
		// For first month, previous_amounts = 0 (no previous month)

		// Step 2c: Calculate new balances using cumulative formula
		// Cumulative Balance Formula:
		// cash_amount = previous_cash + income_cash - expense_cash - bill_cash
		// bank_amount = previous_bank + income_bank - expense_bank - bill_bank
		//
		// This formula ensures:
		// - Each month builds on previous month (cumulative)
		// - Income increases balance (positive)
		// - Expenses decrease balance (negative)
		// - Bills decrease balance (negative)
		newCashAmount := prevCash + incomeCash - expenseCash - billCash
		newBankAmount := prevBank + incomeBank - expenseBank - billBank

		// Balance amounts (available after bills) = final amounts
		balanceCashAmount := newCashAmount
		balanceBankAmount := newBankAmount

		// Calculate totals
		totalPreviousBalance := prevCash + prevBank
		totalBalance := newCashAmount + newBankAmount

		// Step 2d: Update all calculated balance columns
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
				total_balance = ?,
				updated_at = datetime('now')
			WHERE user_id = ? AND year_month = ?
		`, prevCash, prevBank, newCashAmount, newBankAmount,
			balanceCashAmount, balanceBankAmount, totalPreviousBalance, totalBalance,
			userID, currentMonth)

		if err != nil {
			log.Printf("❌ Error updating balances for month %s: %v", currentMonth, err)
			return fmt.Errorf("error updating balances for month %s: %v", currentMonth, err)
		}

		log.Printf("✅ Month %s: cash=%.2f, bank=%.2f, prev_cash=%.2f, prev_bank=%.2f, total=%.2f",
			currentMonth, newCashAmount, newBankAmount, prevCash, prevBank, totalBalance)
	}

	duration := time.Since(startTime)
	log.Printf("✅ Cumulative recalculation completed for %d month(s) in %v", len(months), duration)

	return nil
}

// UpdateCategoryWithTypeChangeRequest represents request to update category with type change
// Contains all necessary information for type change operation and cascade recalculation
type UpdateCategoryWithTypeChangeRequest struct {
	UserID     string `json:"user_id"`      // User ID (required)
	CategoryID int    `json:"category_id"`  // Category ID to update (required)
	OldType    string `json:"old_type"`     // Current category type (required)
	NewType    string `json:"new_type"`     // New category type (required)
	Name       string `json:"name"`         // New category name (optional)
	Emoji      string `json:"emoji"`        // New category emoji (optional)

	// Sync operation parameters
	OperationID string `json:"operation_id,omitempty"` // Sync operation ID
	DeviceID    string `json:"device_id,omitempty"`    // Device ID for sync
	Timestamp   int64  `json:"timestamp,omitempty"`    // Client timestamp
}

// UpdateCategoryWithTypeChangeResponse represents response from type change operation
// Contains detailed information about the operation results
type UpdateCategoryWithTypeChangeResponse struct {
	CategoryID           int    `json:"category_id"`
	TypeChanged          bool   `json:"type_changed"`
	OldType              string `json:"old_type"`
	NewType              string `json:"new_type"`
	TransactionsAffected int    `json:"transactions_affected"`
	MonthsAffected       int    `json:"months_affected"`
	OperationID          string `json:"operation_id"`
	Duration             string `json:"duration"`
}

// updateCategoryWithTypeChange orchestrates category type change with cascade recalculation
// Main entry point for category type change operations
//
// Purpose: Coordinate all operations needed when category type changes:
// 1. Validate request and permissions
// 2. Fetch all transactions for category
// 3. Calculate monthly amounts
// 4. Update income/expense columns
// 5. Recalculate cumulative balances
// 6. Move transactions between tables (incomes ↔ expenses)
// 7. Update category type
// 8. Record sync operation
//
// This function wraps all operations in a database transaction for atomicity
// If any step fails, all changes are rolled back
//
// Algorithm:
// 1. Begin database transaction
// 2. Validate category exists and belongs to user
// 3. Check if type actually changed
// 4. If type changed:
//    a. Get all transactions for category
//    b. Calculate monthly transaction amounts
//    c. Apply type change to monthly balance columns
//    d. Trigger cumulative recalculation
//    e. Move transactions between incomes/expenses tables
// 5. Update category type and other fields
// 6. Commit transaction
// 7. Record sync operation (outside transaction)
// 8. Return success response
//
// Parameters:
//   - request: Update request with category info and type change details
//
// Returns:
//   - ApiResponse with operation results
func updateCategoryWithTypeChange(request UpdateCategoryWithTypeChangeRequest) ApiResponse {
	operationID := fmt.Sprintf("update-type-change-%d-%s", request.CategoryID, time.Now().Format("20060102-150405"))
	startTime := time.Now()

	log.Printf("🔄 Starting category type change orchestration: Operation=%s, Category=%d, %s→%s",
		operationID, request.CategoryID, request.OldType, request.NewType)

	// Step 1: Validate request
	if request.UserID == "" {
		log.Printf("❌ Missing user_id in request")
		return ApiResponse{
			Success: false,
			Message: "user_id is required",
		}
	}

	if request.CategoryID <= 0 {
		log.Printf("❌ Invalid category_id: %d", request.CategoryID)
		return ApiResponse{
			Success: false,
			Message: "Valid category_id is required",
		}
	}

	if request.OldType == "" || request.NewType == "" {
		log.Printf("❌ Missing old_type or new_type")
		return ApiResponse{
			Success: false,
			Message: "old_type and new_type are required",
		}
	}

	// Validate type values
	if (request.OldType != "income" && request.OldType != "expense") ||
		(request.NewType != "income" && request.NewType != "expense") {
		log.Printf("❌ Invalid type values: old=%s, new=%s", request.OldType, request.NewType)
		return ApiResponse{
			Success: false,
			Message: "Type must be 'income' or 'expense'",
		}
	}

	// Step 2: Begin database transaction
	tx, err := db.Begin()
	if err != nil {
		log.Printf("❌ Failed to begin transaction: %v", err)
		return ApiResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to begin transaction: %v", err),
		}
	}
	defer tx.Rollback() // Rollback if not committed

	// Step 3: Verify category exists and belongs to user
	var existingType, existingUserID string
	err = tx.QueryRow(`
		SELECT type, user_id
		FROM categories
		WHERE id = ?
	`, request.CategoryID).Scan(&existingType, &existingUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("❌ Category not found: %d", request.CategoryID)
			return ApiResponse{
				Success: false,
				Message: "Category not found",
			}
		}
		log.Printf("❌ Error fetching category: %v", err)
		return ApiResponse{
			Success: false,
			Message: fmt.Sprintf("Error fetching category: %v", err),
		}
	}

	// Verify user owns this category
	if existingUserID != request.UserID {
		log.Printf("❌ Unauthorized: Category %d does not belong to user %s", request.CategoryID, request.UserID)
		return ApiResponse{
			Success: false,
			Message: "Unauthorized: Category does not belong to user",
		}
	}

	// Step 4: Check if type actually changed
	hasTypeChanged := existingType != request.NewType
	var transactionsAffected int
	var monthsAffected int

	if !hasTypeChanged {
		log.Printf("💡 Type unchanged (%s), using simple update flow", existingType)

		// Simple update without cascade
		updateQuery := "UPDATE categories SET updated_at = datetime('now')"
		var params []interface{}

		if request.Name != "" {
			updateQuery += ", name = ?"
			params = append(params, request.Name)
		}
		if request.Emoji != "" {
			updateQuery += ", emoji = ?"
			params = append(params, request.Emoji)
		}

		updateQuery += " WHERE id = ? AND user_id = ?"
		params = append(params, request.CategoryID, request.UserID)

		_, err = tx.Exec(updateQuery, params...)
		if err != nil {
			log.Printf("❌ Error updating category: %v", err)
			return ApiResponse{
				Success: false,
				Message: fmt.Sprintf("Error updating category: %v", err),
			}
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			log.Printf("❌ Failed to commit transaction: %v", err)
			return ApiResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to commit: %v", err),
			}
		}

		duration := time.Since(startTime)
		log.Printf("✅ Category updated successfully (no type change) in %v", duration)

		return ApiResponse{
			Success: true,
			Message: "Category updated successfully",
			Data: UpdateCategoryWithTypeChangeResponse{
				CategoryID:           request.CategoryID,
				TypeChanged:          false,
				OldType:              existingType,
				NewType:              existingType,
				TransactionsAffected: 0,
				MonthsAffected:       0,
				OperationID:          operationID,
				Duration:             duration.String(),
			},
		}
	}

	// TYPE HAS CHANGED - Execute cascade flow
	log.Printf("⚠️ Category type changed: %s → %s, initiating cascade recalculation", existingType, request.NewType)

	// Step 5: Get all transactions for category
	log.Printf("📝 Step 5: Fetching transactions for category...")
	transactions, err := getTransactionsForCategory(db, request.CategoryID, request.UserID)
	if err != nil {
		log.Printf("❌ Error fetching transactions: %v", err)
		return ApiResponse{
			Success: false,
			Message: fmt.Sprintf("Error fetching transactions: %v", err),
		}
	}

	transactionsAffected = len(transactions)
	log.Printf("✅ Found %d transaction(s) for category", transactionsAffected)

	if transactionsAffected > 0 {
		// Step 6: Calculate monthly amounts
		log.Printf("📝 Step 6: Calculating monthly transaction amounts...")
		monthlyAmounts := calculateMonthlyTransactionAmounts(transactions)
		monthsAffected = len(monthlyAmounts)
		log.Printf("✅ Calculated amounts for %d month(s)", monthsAffected)

		// Step 7: Apply type change to monthly balance
		log.Printf("📝 Step 7: Applying type change to monthly balance...")
		firstMonth, err := applyTypeChangeToMonthlyBalance(db, request.UserID, monthlyAmounts, existingType, request.NewType)
		if err != nil {
			log.Printf("❌ Error applying type change: %v", err)
			return ApiResponse{
				Success: false,
				Message: fmt.Sprintf("Error applying type change: %v", err),
			}
		}
		log.Printf("✅ Type change applied to monthly balance")

		// Step 8: Recalculate cumulative balances
		log.Printf("📝 Step 8: Recalculating cumulative balances from %s...", firstMonth)
		err = recalculateAllCumulativeBalances(db, request.UserID, firstMonth)
		if err != nil {
			log.Printf("❌ Error recalculating balances: %v", err)
			return ApiResponse{
				Success: false,
				Message: fmt.Sprintf("Error recalculating balances: %v", err),
			}
		}
		log.Printf("✅ Cumulative balances recalculated")

		// Step 8.5: Move transactions between tables (incomes ↔ expenses)
		log.Printf("📝 Step 8.5: Moving transactions between tables...")
		migrationResult, err := moveTransactionsBetweenTables(tx, request.CategoryID, existingType, request.NewType, request.UserID)
		if err != nil {
			log.Printf("❌ Error migrating transactions: %v", err)
			return ApiResponse{
				Success: false,
				Message: fmt.Sprintf("Error migrating transactions: %v", err),
			}
		}
		log.Printf("✅ Transactions migrated successfully: %d moved, %d deleted", migrationResult.MovedCount, migrationResult.DeletedCount)
	} else {
		log.Printf("ℹ️ No transactions found, skipping cascade calculations and migration")
	}

	// Step 9: Update category type in database
	log.Printf("📝 Step 9: Updating category type in database...")
	updateQuery := "UPDATE categories SET type = ?, updated_at = datetime('now')"
	var params []interface{}
	params = append(params, request.NewType)

	if request.Name != "" {
		updateQuery += ", name = ?"
		params = append(params, request.Name)
	}
	if request.Emoji != "" {
		updateQuery += ", emoji = ?"
		params = append(params, request.Emoji)
	}

	updateQuery += " WHERE id = ? AND user_id = ?"
	params = append(params, request.CategoryID, request.UserID)

	result, err := tx.Exec(updateQuery, params...)
	if err != nil {
		log.Printf("❌ Error updating category: %v", err)
		return ApiResponse{
			Success: false,
			Message: fmt.Sprintf("Error updating category: %v", err),
		}
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Printf("❌ No rows updated")
		return ApiResponse{
			Success: false,
			Message: "No rows updated - category not found",
		}
	}

	log.Printf("✅ Category type updated in database")

	// Step 10: Commit transaction
	log.Printf("📝 Step 10: Committing transaction...")
	if err := tx.Commit(); err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return ApiResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to commit: %v", err),
		}
	}
	log.Printf("✅ Transaction committed successfully")

	duration := time.Since(startTime)

	// Step 11: Record sync operation (outside transaction)
	log.Printf("📝 Step 11: Recording sync operation...")

	// Fetch updated category for sync data
	var updatedCategory Category
	err = db.QueryRow(`
		SELECT id, user_id, name, type, emoji, created_at, updated_at
		FROM categories
		WHERE id = ?
	`, request.CategoryID).Scan(
		&updatedCategory.ID,
		&updatedCategory.UserID,
		&updatedCategory.Name,
		&updatedCategory.Type,
		&updatedCategory.Emoji,
		&updatedCategory.CreatedAt,
		&updatedCategory.UpdatedAt,
	)

	if err == nil {
		syncData := map[string]interface{}{
			"id":         updatedCategory.ID,
			"user_id":    updatedCategory.UserID,
			"name":       updatedCategory.Name,
			"type":       updatedCategory.Type,
			"emoji":      updatedCategory.Emoji,
			"created_at": updatedCategory.CreatedAt,
			"updated_at": updatedCategory.UpdatedAt,
			"old_type":   existingType,
			"transactions_affected": transactionsAffected,
			"months_affected": monthsAffected,
		}

		err = addSyncOperation(
			request.UserID,
			request.OperationID,
			"update_with_type_change",
			"categories",
			strconv.Itoa(request.CategoryID),
			syncData,
			request.DeviceID,
			request.Timestamp,
		)

		if err != nil {
			log.Printf("⚠️ Failed to record sync operation: %v", err)
			// Don't fail the main operation
		} else {
			log.Printf("✅ Sync operation recorded")
		}
	}

	log.Printf("🎉 Category type change completed successfully in %v", duration)

	return ApiResponse{
		Success: true,
		Message: "Category type updated successfully with cascade recalculations",
		Data: UpdateCategoryWithTypeChangeResponse{
			CategoryID:           request.CategoryID,
			TypeChanged:          true,
			OldType:              existingType,
			NewType:              request.NewType,
			TransactionsAffected: transactionsAffected,
			MonthsAffected:       monthsAffected,
			OperationID:          operationID,
			Duration:             duration.String(),
		},
	}
}

// handleUpdateCategoryWithTypeChange HTTP handler for category type change endpoint
// Endpoint: POST /categories/update-with-type-change
// Accepts JSON request body with type change details
//
// Purpose: API endpoint for category type change with cascade recalculation
// Used by frontend when user confirms type change in modal
//
// Request Body:
// {
//   "user_id": "abc123",
//   "category_id": 5,
//   "old_type": "expense",
//   "new_type": "income",
//   "name": "Salary",        // optional
//   "emoji": "💰",           // optional
//   "operation_id": "...",   // optional for sync
//   "device_id": "...",      // optional for sync
//   "timestamp": 1234567890  // optional for sync
// }
//
// Response Format:
// {
//   "success": true,
//   "message": "Category type updated successfully with cascade recalculations",
//   "data": {
//     "category_id": 5,
//     "type_changed": true,
//     "old_type": "expense",
//     "new_type": "income",
//     "transactions_affected": 15,
//     "months_affected": 3,
//     "operation_id": "update-type-change-5-20250930",
//     "duration": "45ms"
//   }
// }
func handleUpdateCategoryWithTypeChange(w http.ResponseWriter, r *http.Request) {
	// Validate HTTP method - only POST allowed
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var request UpdateCategoryWithTypeChangeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		log.Printf("❌ Invalid request body: %v", err)
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("📥 Update category with type change request: CategoryID=%d, User=%s, %s→%s",
		request.CategoryID, request.UserID, request.OldType, request.NewType)

	// Execute orchestrator
	response := updateCategoryWithTypeChange(request)

	// Send response
	if response.Success {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	} else {
		sendErrorResponse(w, response.Message, http.StatusInternalServerError)
	}
}