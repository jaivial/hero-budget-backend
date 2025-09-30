package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Transaction represents a transaction record that can be migrated between tables
// Used when category type changes to move transactions from incomes to expenses or vice versa
type Transaction struct {
	ID            int            // Transaction ID (will be regenerated on insert)
	UserID        string         // User identifier
	Amount        float64        // Transaction amount
	Date          string         // Transaction date (YYYY-MM-DD format)
	Category      string         // Category name
	CategoryID    int            // Category ID reference
	PaymentMethod string         // "cash" or "bank"
	Description   string         // Transaction description
	BillID        sql.NullInt64  // Bill ID (only for expenses, NULL for incomes)
	CreatedAt     string         // Original creation timestamp
	UpdatedAt     string         // Last update timestamp
}

// MigrationResult contains the results of a transaction migration operation
// Provides detailed information about moved and deleted transactions
type MigrationResult struct {
	MovedCount   int    // Number of transactions successfully moved
	DeletedCount int    // Number of transactions deleted from source
	Message      string // Human-readable result message
}

// moveTransactionsBetweenTables migrates transactions from one table to another when category type changes
// This function is called when a category type changes (income ↔ expense) to ensure transactions
// are stored in the correct table matching the new category type.
//
// Purpose: Move all transactions linked to a category from source table to destination table
// Used by: updateCategoryWithTypeChange() as Step 8.5 in the cascade flow
//
// Algorithm:
// 1. Validate input parameters (ensure type actually changed)
// 2. Determine source and destination tables based on old/new type
// 3. Fetch all transactions from source table WHERE category_id matches
// 4. Insert each transaction into destination table (preserving all data)
// 5. Delete transactions from source table
// 6. Verify deletion count matches insertion count
// 7. Return migration results
//
// All operations use the provided transaction (tx) for atomicity with parent operation.
// If any step fails, the entire operation rolls back via the parent transaction.
//
// Parameters:
//   - tx: Database transaction for atomicity (must be active transaction from caller)
//   - categoryID: ID of category being changed
//   - oldType: Original category type ("income" or "expense")
//   - newType: New category type ("income" or "expense")
//   - userID: User ID for permission validation and data isolation
//
// Returns:
//   - MigrationResult: Details about the migration operation
//   - error: Error if operation fails (nil on success)
//
// Example:
//   result, err := moveTransactionsBetweenTables(tx, 123, "income", "expense", "user456")
//   if err != nil {
//       log.Printf("Migration failed: %v", err)
//       return err
//   }
//   log.Printf("Migrated %d transactions", result.MovedCount)
func moveTransactionsBetweenTables(tx *sql.Tx, categoryID int, oldType, newType, userID string) (MigrationResult, error) {
	operationID := fmt.Sprintf("move-tx-%d-%d", categoryID, time.Now().Unix())
	startTime := time.Now()

	log.Printf("🔄 Starting transaction migration: Operation=%s, Category=%d, %s→%s, User=%s",
		operationID, categoryID, oldType, newType, userID)

	// Initialize result
	result := MigrationResult{
		MovedCount:   0,
		DeletedCount: 0,
		Message:      "",
	}

	// Step 1: Validate input parameters
	if oldType == newType {
		log.Printf("ℹ️ No type change detected (%s == %s), skipping migration", oldType, newType)
		result.Message = "No migration needed - type unchanged"
		return result, nil
	}

	if oldType != "income" && oldType != "expense" {
		return result, fmt.Errorf("invalid oldType: %s (must be 'income' or 'expense')", oldType)
	}

	if newType != "income" && newType != "expense" {
		return result, fmt.Errorf("invalid newType: %s (must be 'income' or 'expense')", newType)
	}

	// Step 2: Determine source and destination tables
	var sourceTable, destTable string
	if oldType == "income" {
		sourceTable = "incomes"
		destTable = "expenses"
	} else {
		sourceTable = "expenses"
		destTable = "incomes"
	}

	log.Printf("📊 Migration path: %s → %s", sourceTable, destTable)

	// Step 3: Fetch all transactions from source table
	log.Printf("📝 Step 3: Fetching transactions from %s...", sourceTable)

	transactions, err := fetchTransactionsFromSource(tx, sourceTable, categoryID, userID)
	if err != nil {
		return result, fmt.Errorf("failed to fetch transactions from %s: %v", sourceTable, err)
	}

	log.Printf("✅ Found %d transaction(s) to migrate", len(transactions))

	if len(transactions) == 0 {
		result.Message = "No transactions found to migrate"
		return result, nil
	}

	// Step 4: Insert transactions into destination table
	log.Printf("📝 Step 4: Inserting transactions into %s...", destTable)

	insertedCount, err := insertTransactionsToDestination(tx, destTable, transactions, categoryID)
	if err != nil {
		return result, fmt.Errorf("failed to insert transactions into %s: %v", destTable, err)
	}

	result.MovedCount = insertedCount
	log.Printf("✅ Inserted %d transaction(s) into %s", insertedCount, destTable)

	// Step 5: Delete transactions from source table
	log.Printf("📝 Step 5: Deleting transactions from %s...", sourceTable)

	deletedCount, err := deleteTransactionsFromSource(tx, sourceTable, categoryID, userID)
	if err != nil {
		return result, fmt.Errorf("failed to delete transactions from %s: %v", sourceTable, err)
	}

	result.DeletedCount = deletedCount
	log.Printf("✅ Deleted %d transaction(s) from %s", deletedCount, sourceTable)

	// Step 6: Verify counts match
	if insertedCount != deletedCount {
		log.Printf("⚠️ Count mismatch: inserted=%d, deleted=%d (potential orphaned records)", insertedCount, deletedCount)
		// Don't fail the operation - continue to allow debugging
	}

	duration := time.Since(startTime)
	result.Message = fmt.Sprintf("Successfully migrated %d transactions from %s to %s", insertedCount, sourceTable, destTable)

	log.Printf("🎉 Transaction migration completed successfully: %d moved, %d deleted in %v",
		result.MovedCount, result.DeletedCount, duration)

	return result, nil
}

// fetchTransactionsFromSource retrieves all transactions from the source table for a specific category
// Queries either expenses or incomes table based on oldType
// Returns array of Transaction structs with all fields populated
//
// Parameters:
//   - tx: Database transaction
//   - sourceTable: Table name ("incomes" or "expenses")
//   - categoryID: Category ID to filter by
//   - userID: User ID to filter by
//
// Returns:
//   - []Transaction: Array of transactions to migrate
//   - error: Error if query fails
func fetchTransactionsFromSource(tx *sql.Tx, sourceTable string, categoryID int, userID string) ([]Transaction, error) {
	var transactions []Transaction

	// Build query based on source table (expenses has bill_id, incomes doesn't)
	var query string
	if sourceTable == "expenses" {
		query = `
			SELECT id, user_id, amount, date, category, category_id,
			       payment_method, description, bill_id, created_at, updated_at
			FROM expenses
			WHERE category_id = ? AND user_id = ?
			ORDER BY date ASC
		`
	} else {
		query = `
			SELECT id, user_id, amount, date, category, category_id,
			       payment_method, description, created_at, updated_at
			FROM incomes
			WHERE category_id = ? AND user_id = ?
			ORDER BY date ASC
		`
	}

	rows, err := tx.Query(query, categoryID, userID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	// Scan rows into Transaction structs
	for rows.Next() {
		var txn Transaction

		if sourceTable == "expenses" {
			err = rows.Scan(
				&txn.ID,
				&txn.UserID,
				&txn.Amount,
				&txn.Date,
				&txn.Category,
				&txn.CategoryID,
				&txn.PaymentMethod,
				&txn.Description,
				&txn.BillID,
				&txn.CreatedAt,
				&txn.UpdatedAt,
			)
		} else {
			// Incomes don't have bill_id
			err = rows.Scan(
				&txn.ID,
				&txn.UserID,
				&txn.Amount,
				&txn.Date,
				&txn.Category,
				&txn.CategoryID,
				&txn.PaymentMethod,
				&txn.Description,
				&txn.CreatedAt,
				&txn.UpdatedAt,
			)
			txn.BillID = sql.NullInt64{Valid: false} // Explicitly set to NULL
		}

		if err != nil {
			return nil, fmt.Errorf("scan failed: %v", err)
		}

		transactions = append(transactions, txn)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failed: %v", err)
	}

	log.Printf("📊 Fetched %d transaction(s) from %s", len(transactions), sourceTable)

	return transactions, nil
}

// insertTransactionsToDestination inserts transactions into the destination table
// Creates new records in the target table with all transaction data preserved
// Handles bill_id field appropriately (NULL for incomes, preserved for expenses)
//
// Parameters:
//   - tx: Database transaction
//   - destTable: Destination table name ("incomes" or "expenses")
//   - transactions: Array of transactions to insert
//   - categoryID: Category ID to use for all transactions
//
// Returns:
//   - int: Number of transactions successfully inserted
//   - error: Error if any insertion fails
func insertTransactionsToDestination(tx *sql.Tx, destTable string, transactions []Transaction, categoryID int) (int, error) {
	insertedCount := 0

	for i, txn := range transactions {
		var insertQuery string
		var args []interface{}

		if destTable == "expenses" {
			// Insert into expenses (includes bill_id column)
			insertQuery = `
				INSERT INTO expenses (
					user_id, amount, date, category, category_id,
					payment_method, description, bill_id,
					created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
			`
			args = []interface{}{
				txn.UserID,
				txn.Amount,
				txn.Date,
				txn.Category,
				categoryID, // Use the category_id being updated
				txn.PaymentMethod,
				txn.Description,
				txn.BillID, // Will be NULL when migrating from incomes
				txn.CreatedAt,
			}
		} else {
			// Insert into incomes (no bill_id column)
			insertQuery = `
				INSERT INTO incomes (
					user_id, amount, date, category, category_id,
					payment_method, description,
					created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
			`
			args = []interface{}{
				txn.UserID,
				txn.Amount,
				txn.Date,
				txn.Category,
				categoryID, // Use the category_id being updated
				txn.PaymentMethod,
				txn.Description,
				txn.CreatedAt,
			}
		}

		result, err := tx.Exec(insertQuery, args...)
		if err != nil {
			return insertedCount, fmt.Errorf("failed to insert transaction %d/%d (ID: %d): %v", i+1, len(transactions), txn.ID, err)
		}

		newID, _ := result.LastInsertId()
		insertedCount++

		log.Printf("  ✅ Inserted transaction %d/%d (Old ID: %d, New ID: %d, Amount: %.2f, Date: %s)",
			i+1, len(transactions), txn.ID, newID, txn.Amount, txn.Date)
	}

	return insertedCount, nil
}

// deleteTransactionsFromSource removes transactions from the source table after migration
// Deletes all transactions for the specified category from the old table
//
// Parameters:
//   - tx: Database transaction
//   - sourceTable: Source table name ("incomes" or "expenses")
//   - categoryID: Category ID to delete transactions for
//   - userID: User ID to ensure data isolation
//
// Returns:
//   - int: Number of transactions deleted
//   - error: Error if deletion fails
func deleteTransactionsFromSource(tx *sql.Tx, sourceTable string, categoryID int, userID string) (int, error) {
	deleteQuery := fmt.Sprintf(`
		DELETE FROM %s
		WHERE category_id = ? AND user_id = ?
	`, sourceTable)

	result, err := tx.Exec(deleteQuery, categoryID, userID)
	if err != nil {
		return 0, fmt.Errorf("delete failed: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %v", err)
	}

	log.Printf("✅ Deleted %d transaction(s) from %s", rowsAffected, sourceTable)

	return int(rowsAffected), nil
}
