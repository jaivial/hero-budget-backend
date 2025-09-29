package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// Database operations for expense management with comprehensive balance tracking

func fetchExpensesFromDatabase(userID string) ([]Expense, error) {
	// SQL query to fetch all expenses for a user, ordered by most recent, including category_id
	query := `
		SELECT id, user_id, amount, date, category, category_id, payment_method, description, created_at, updated_at
		FROM expenses
		WHERE user_id = ?
		ORDER BY date DESC, id DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse rows into expense objects
	expenses := []Expense{}
	for rows.Next() {
		var expense Expense
		err := rows.Scan(
			&expense.ID,
			&expense.UserID,
			&expense.Amount,
			&expense.Date,
			&expense.Category,
			&expense.CategoryID,
			&expense.PaymentMethod,
			&expense.Description,
			&expense.CreatedAt,
			&expense.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}

	return expenses, nil
}

func getExpenseByID(expenseID int, userID string) (*Expense, error) {
	// SQL query to fetch a specific expense by ID and user ID, including category_id
	query := `
		SELECT id, user_id, amount, date, category, category_id, payment_method, description, created_at, updated_at
		FROM expenses
		WHERE id = ? AND user_id = ?
	`

	row := db.QueryRow(query, expenseID, userID)

	var expense Expense
	err := row.Scan(
		&expense.ID,
		&expense.UserID,
		&expense.Amount,
		&expense.Date,
		&expense.Category,
		&expense.CategoryID,
		&expense.PaymentMethod,
		&expense.Description,
		&expense.CreatedAt,
		&expense.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &expense, nil
}

func addExpenseToDatabase(expense AddExpenseRequest) (int, error) {
	// SQL query to insert a new expense including category_id
	query := `
		INSERT INTO expenses (user_id, amount, date, category, category_id, payment_method, description)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(
		query,
		expense.UserID,
		expense.Amount,
		expense.Date,
		expense.Category,
		expense.CategoryID,
		expense.PaymentMethod,
		expense.Description,
	)
	if err != nil {
		return 0, err
	}

	// Get the ID of the inserted expense
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

func updateExpenseInDatabase(updateRequest UpdateExpenseRequest) error {
	// Build dynamic update query based on provided fields
	setParts := []string{}
	args := []interface{}{}

	if updateRequest.Amount > 0 {
		setParts = append(setParts, "amount = ?")
		args = append(args, updateRequest.Amount)
	}
	if updateRequest.Date != "" {
		setParts = append(setParts, "date = ?")
		args = append(args, updateRequest.Date)
	}
	if updateRequest.Category != "" {
		setParts = append(setParts, "category = ?")
		args = append(args, updateRequest.Category)
	}
	if updateRequest.CategoryID != nil {
		setParts = append(setParts, "category_id = ?")
		args = append(args, updateRequest.CategoryID)
	}
	if updateRequest.PaymentMethod != "" {
		setParts = append(setParts, "payment_method = ?")
		args = append(args, updateRequest.PaymentMethod)
	}
	if updateRequest.Description != "" {
		setParts = append(setParts, "description = ?")
		args = append(args, updateRequest.Description)
	}

	if len(setParts) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// Add updated_at timestamp
	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")

	// Add WHERE clause arguments
	args = append(args, updateRequest.ExpenseID, updateRequest.UserID)

	query := fmt.Sprintf(`
		UPDATE expenses
		SET %s
		WHERE id = ? AND user_id = ?
	`, strings.Join(setParts, ", "))

	_, err := db.Exec(query, args...)
	return err
}

func deleteExpenseFromDatabase(expenseID int, userID string) error {
	// SQL query to delete an expense
	query := `
		DELETE FROM expenses
		WHERE id = ? AND user_id = ?
	`

	_, err := db.Exec(query, expenseID, userID)
	if err != nil {
		return err
	}

	return nil
}

func reverseExpenseEffect(userID, date string, amount float64, paymentMethod string) error {
	// Update all balance tables to reverse the expense effect
	err := updateAllBalanceTablesExpense(userID, date, -amount, paymentMethod)
	if err != nil {
		log.Printf("Error reversing expense effect in balance tables: %v", err)
		return err
	}

	return nil
}

func updateAllBalanceTablesExpense(userID, dateStr string, amount float64, paymentMethod string) error {
	// Parse the expense date
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return fmt.Errorf("error parsing date: %v", err)
	}

	// Calculate cash and bank amounts based on payment method
	var cashAmount, bankAmount float64
	if paymentMethod == "cash" {
		cashAmount = amount
		bankAmount = 0
	} else {
		cashAmount = 0
		bankAmount = amount
	}

	// Update daily balance
	if err := updateDailyBalance(userID, 0, amount, 0, cashAmount, bankAmount, date); err != nil {
		log.Printf("Error updating daily balance: %v", err)
		return err
	}

	// Balance updates completed successfully
	log.Printf("Note: All balance updates for expense reversal completed - amount: %v, method: %v", amount, paymentMethod)

	return nil
}

func fetchDailyAnalytics(userID, dateStr string) (interface{}, error) {
	// Query daily balance data for analytics
	query := `
		SELECT date, income_amount, expense_amount, bills_amount, 
		       cash_amount, bank_amount, balance, previous_balance
		FROM daily_balance
		WHERE user_id = ? AND date = ?
	`

	var analytics struct {
		Date            string  `json:"date"`
		IncomeAmount    float64 `json:"income_amount"`
		ExpenseAmount   float64 `json:"expense_amount"`
		BillsAmount     float64 `json:"bills_amount"`
		CashAmount      float64 `json:"cash_amount"`
		BankAmount      float64 `json:"bank_amount"`
		Balance         float64 `json:"balance"`
		PreviousBalance float64 `json:"previous_balance"`
	}

	err := db.QueryRow(query, userID, dateStr).Scan(
		&analytics.Date,
		&analytics.IncomeAmount,
		&analytics.ExpenseAmount,
		&analytics.BillsAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.Balance,
		&analytics.PreviousBalance,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.Date = dateStr
		return analytics, nil
	}

	return analytics, err
}

func fetchWeeklyAnalytics(userID, weekStr string) (interface{}, error) {
	// Query weekly balance data for analytics
	query := `
		SELECT year_week, start_date, end_date, income_amount, expense_amount, bills_amount,
		       cash_amount, bank_amount, balance, previous_balance
		FROM weekly_balance
		WHERE user_id = ? AND year_week = ?
	`

	var analytics struct {
		YearWeek        string  `json:"year_week"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		IncomeAmount    float64 `json:"income_amount"`
		ExpenseAmount   float64 `json:"expense_amount"`
		BillsAmount     float64 `json:"bills_amount"`
		CashAmount      float64 `json:"cash_amount"`
		BankAmount      float64 `json:"bank_amount"`
		Balance         float64 `json:"balance"`
		PreviousBalance float64 `json:"previous_balance"`
	}

	err := db.QueryRow(query, userID, weekStr).Scan(
		&analytics.YearWeek,
		&analytics.StartDate,
		&analytics.EndDate,
		&analytics.IncomeAmount,
		&analytics.ExpenseAmount,
		&analytics.BillsAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.Balance,
		&analytics.PreviousBalance,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.YearWeek = weekStr
		return analytics, nil
	}

	return analytics, err
}

func fetchMonthlyAnalytics(userID, monthStr string) (interface{}, error) {
	// Query monthly balance data for analytics from monthly_cash_bank_balance table
	query := `
		SELECT year_month, income_cash_amount, income_bank_amount,
		       expense_cash_amount, expense_bank_amount, bill_cash_amount, bill_bank_amount,
		       cash_amount, bank_amount, balance_cash_amount, balance_bank_amount,
		       previous_cash_amount, previous_bank_amount
		FROM monthly_cash_bank_balance
		WHERE user_id = ? AND year_month = ?
	`

	var analytics struct {
		YearMonth          string  `json:"year_month"`
		IncomeCashAmount   float64 `json:"income_cash_amount"`
		IncomeBankAmount   float64 `json:"income_bank_amount"`
		ExpenseCashAmount  float64 `json:"expense_cash_amount"`
		ExpenseBankAmount  float64 `json:"expense_bank_amount"`
		BillCashAmount     float64 `json:"bill_cash_amount"`
		BillBankAmount     float64 `json:"bill_bank_amount"`
		CashAmount         float64 `json:"cash_amount"`
		BankAmount         float64 `json:"bank_amount"`
		BalanceCashAmount  float64 `json:"balance_cash_amount"`
		BalanceBankAmount  float64 `json:"balance_bank_amount"`
		PreviousCashAmount float64 `json:"previous_cash_amount"`
		PreviousBankAmount float64 `json:"previous_bank_amount"`
	}

	err := db.QueryRow(query, userID, monthStr).Scan(
		&analytics.YearMonth,
		&analytics.IncomeCashAmount,
		&analytics.IncomeBankAmount,
		&analytics.ExpenseCashAmount,
		&analytics.ExpenseBankAmount,
		&analytics.BillCashAmount,
		&analytics.BillBankAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.BalanceCashAmount,
		&analytics.BalanceBankAmount,
		&analytics.PreviousCashAmount,
		&analytics.PreviousBankAmount,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.YearMonth = monthStr
		return analytics, nil
	}

	return analytics, err
}

func fetchQuarterlyAnalytics(userID, quarterStr string) (interface{}, error) {
	// Query quarterly balance data for analytics
	query := `
		SELECT year_quarter, income_amount, expense_amount, bills_amount,
		       cash_amount, bank_amount, balance, previous_balance
		FROM quarterly_balance
		WHERE user_id = ? AND year_quarter = ?
	`

	var analytics struct {
		YearQuarter     string  `json:"year_quarter"`
		IncomeAmount    float64 `json:"income_amount"`
		ExpenseAmount   float64 `json:"expense_amount"`
		BillsAmount     float64 `json:"bills_amount"`
		CashAmount      float64 `json:"cash_amount"`
		BankAmount      float64 `json:"bank_amount"`
		Balance         float64 `json:"balance"`
		PreviousBalance float64 `json:"previous_balance"`
	}

	err := db.QueryRow(query, userID, quarterStr).Scan(
		&analytics.YearQuarter,
		&analytics.IncomeAmount,
		&analytics.ExpenseAmount,
		&analytics.BillsAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.Balance,
		&analytics.PreviousBalance,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.YearQuarter = quarterStr
		return analytics, nil
	}

	return analytics, err
}

func fetchSemiannualAnalytics(userID, halfStr string) (interface{}, error) {
	// Query semiannual balance data for analytics
	query := `
		SELECT year_half, income_amount, expense_amount, bills_amount,
		       cash_amount, bank_amount, balance, previous_balance
		FROM semiannual_balance
		WHERE user_id = ? AND year_half = ?
	`

	var analytics struct {
		YearHalf        string  `json:"year_half"`
		IncomeAmount    float64 `json:"income_amount"`
		ExpenseAmount   float64 `json:"expense_amount"`
		BillsAmount     float64 `json:"bills_amount"`
		CashAmount      float64 `json:"cash_amount"`
		BankAmount      float64 `json:"bank_amount"`
		Balance         float64 `json:"balance"`
		PreviousBalance float64 `json:"previous_balance"`
	}

	err := db.QueryRow(query, userID, halfStr).Scan(
		&analytics.YearHalf,
		&analytics.IncomeAmount,
		&analytics.ExpenseAmount,
		&analytics.BillsAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.Balance,
		&analytics.PreviousBalance,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.YearHalf = halfStr
		return analytics, nil
	}

	return analytics, err
}

func fetchAnnualAnalytics(userID, yearStr string) (interface{}, error) {
	// Query annual balance data for analytics
	query := `
		SELECT year, income_amount, expense_amount, bills_amount,
		       cash_amount, bank_amount, balance, previous_balance
		FROM annual_balance
		WHERE user_id = ? AND year = ?
	`

	var analytics struct {
		Year            string  `json:"year"`
		IncomeAmount    float64 `json:"income_amount"`
		ExpenseAmount   float64 `json:"expense_amount"`
		BillsAmount     float64 `json:"bills_amount"`
		CashAmount      float64 `json:"cash_amount"`
		BankAmount      float64 `json:"bank_amount"`
		Balance         float64 `json:"balance"`
		PreviousBalance float64 `json:"previous_balance"`
	}

	err := db.QueryRow(query, userID, yearStr).Scan(
		&analytics.Year,
		&analytics.IncomeAmount,
		&analytics.ExpenseAmount,
		&analytics.BillsAmount,
		&analytics.CashAmount,
		&analytics.BankAmount,
		&analytics.Balance,
		&analytics.PreviousBalance,
	)

	if err == sql.ErrNoRows {
		// Return default analytics if no data found
		analytics.Year = yearStr
		return analytics, nil
	}

	return analytics, err
}

func fetchUserBalance(userID string) (interface{}, error) {
	// Query user balance from balances table
	query := `
		SELECT user_id, cash_balance, bank_balance, created_at, updated_at
		FROM balances
		WHERE user_id = ?
	`

	var balance struct {
		UserID      string  `json:"user_id"`
		CashBalance float64 `json:"cash_balance"`
		BankBalance float64 `json:"bank_balance"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
	}

	err := db.QueryRow(query, userID).Scan(
		&balance.UserID,
		&balance.CashBalance,
		&balance.BankBalance,
		&balance.CreatedAt,
		&balance.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default balance if no data found
		balance.UserID = userID
		balance.CashBalance = 0
		balance.BankBalance = 0
		return balance, nil
	}

	return balance, err
}

func updateUserCashBalance(userID string, amount float64) error {
	// Check if user exists in balances table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM balances WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Insert new balance record
		_, err = db.Exec(`
			INSERT INTO balances (user_id, cash_balance, bank_balance)
			VALUES (?, ?, 0)
		`, userID, amount)
	} else {
		// Update existing balance
		_, err = db.Exec(`
			UPDATE balances
			SET cash_balance = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?
		`, amount, userID)
	}

	return err
}

func updateUserBankBalance(userID string, amount float64) error {
	// Check if user exists in balances table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM balances WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Insert new balance record
		_, err = db.Exec(`
			INSERT INTO balances (user_id, cash_balance, bank_balance)
			VALUES (?, 0, ?)
		`, userID, amount)
	} else {
		// Update existing balance
		_, err = db.Exec(`
			UPDATE balances
			SET bank_balance = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?
		`, amount, userID)
	}

	return err
}
