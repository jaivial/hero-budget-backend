package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// Funciones de validación para Transaction Delete Service - Versión Final Compacta
// Validaciones comprensivas para eliminación segura de transacciones

// validateTransactionDeletionConsistency valida eliminación de transacción
func validateTransactionDeletionConsistency(deletion OfflineTransactionDeletion) error {
	if deletion.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if deletion.TransactionID == "" && deletion.TransactionLocalID == "" {
		return fmt.Errorf("transaction_id or transaction_local_id required")
	}
	if deletion.Action != "delete" {
		return fmt.Errorf("action must be 'delete'")
	}
	if deletion.OriginalAmount < 0 {
		return fmt.Errorf("original_amount cannot be negative")
	}
	if deletion.Version < 0 {
		return fmt.Errorf("version cannot be negative")
	}
	if len(deletion.DeletionReason) > 500 {
		return fmt.Errorf("deletion_reason too long")
	}
	return nil
}

// checkTransactionCanBeDeleted verifica si transacción puede eliminarse
func checkTransactionCanBeDeleted(transactionID, userID string) (exists bool, canDelete bool, err error) {
	var count int
	var transactionType string
	var amount float64
	var createdAt time.Time

	query := `SELECT COUNT(*), COALESCE(transaction_type, ''), COALESCE(amount, 0), COALESCE(created_at, datetime('now'))
			  FROM cash_bank_transactions WHERE id = ? AND user_id = ?`

	err = db.QueryRow(query, transactionID, userID).Scan(&count, &transactionType, &amount, &createdAt)
	if err != nil {
		return false, false, fmt.Errorf("error checking transaction: %v", err)
	}

	exists = count > 0
	if !exists {
		return false, false, nil
	}

	// Check protection
	if strings.HasPrefix(transactionID, "SYS_") || time.Since(createdAt) > 30*24*time.Hour || amount > 10000.0 {
		return true, false, nil
	}

	// Check dependencies
	var depCount int
	err = db.QueryRow("SELECT COUNT(*) FROM cash_bank_transactions WHERE user_id = ? AND related_transaction_id = ?",
		userID, transactionID).Scan(&depCount)
	if err != nil && err != sql.ErrNoRows {
		return true, false, err
	}
	if depCount > 0 {
		return true, false, nil
	}

	// Check business rules - don't delete last transaction
	var totalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM cash_bank_transactions WHERE user_id = ?", userID).Scan(&totalCount)
	if err != nil {
		return true, false, err
	}
	if totalCount <= 1 {
		return true, false, nil
	}

	return true, true, nil
}

// calculateBalanceImpactForDeletion calcula impacto en balance
func calculateBalanceImpactForDeletion(transactionID, userID string) (TransactionDeleteBalanceImpact, error) {
	impact := TransactionDeleteBalanceImpact{TransactionID: transactionID}

	var transactionType string
	var amount float64

	err := db.QueryRow("SELECT transaction_type, amount FROM cash_bank_transactions WHERE id = ? AND user_id = ?",
		transactionID, userID).Scan(&transactionType, &amount)
	if err != nil {
		return impact, fmt.Errorf("error fetching transaction: %v", err)
	}

	impact.TransactionType = transactionType
	impact.Amount = amount

	// Use current month for validation - TODO: consider transaction-specific month
	currentMonth := time.Now().Format("2006-01")
	distribution, err := fetchCashBankDistribution(userID, currentMonth)
	if err != nil {
		return impact, fmt.Errorf("error fetching distribution: %v", err)
	}

	impact.BalanceBefore = distribution.MonthlyTotal

	// Calculate impact
	switch transactionType {
	case "cash_update":
		impact.CashImpact = -amount
		impact.AdjustmentMade = -amount
	case "bank_update":
		impact.BankImpact = -amount
		impact.AdjustmentMade = -amount
	case "cash_to_bank":
		impact.CashImpact = amount
		impact.BankImpact = -amount
	case "bank_to_cash":
		impact.CashImpact = -amount
		impact.BankImpact = amount
	case "expense":
		impact.AdjustmentMade = amount
		if amount < 100.0 {
			impact.CashImpact = amount
		} else {
			impact.BankImpact = amount
		}
	case "income":
		impact.AdjustmentMade = -amount
		if amount < 200.0 {
			impact.CashImpact = -amount
		} else {
			impact.BankImpact = -amount
		}
	}

	impact.BalanceAfter = impact.BalanceBefore + impact.AdjustmentMade
	impact.RequiresRecalculation = (impact.CashImpact != 0 || impact.BankImpact != 0) && impact.BalanceAfter > 0

	return impact, nil
}

// executeTransactionDeletion ejecuta eliminación física
func executeTransactionDeletion(transactionID, userID, deletionReason string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback()

	// Get details for audit
	var transactionType string
	var amount float64
	var originalDate string

	err = tx.QueryRow("SELECT transaction_type, amount, date FROM cash_bank_transactions WHERE id = ? AND user_id = ?",
		transactionID, userID).Scan(&transactionType, &amount, &originalDate)
	if err != nil {
		return fmt.Errorf("error fetching transaction: %v", err)
	}

	// Create audit record
	auditID := fmt.Sprintf("AUDIT_%d", time.Now().Unix())
	auditQuery := `INSERT INTO transaction_deletion_log (id, user_id, transaction_id, transaction_type, 
					original_amount, original_date, deletion_reason, deleted_at, deleted_by, balance_adjusted, can_be_restored) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = tx.Exec(auditQuery, auditID, userID, transactionID, transactionType,
		amount, originalDate, deletionReason, time.Now(), fmt.Sprintf("sync_%s", userID[:8]), true, true)
	if err != nil {
		return fmt.Errorf("error creating audit record: %v", err)
	}

	// Delete transaction
	result, err := tx.Exec("DELETE FROM cash_bank_transactions WHERE id = ? AND user_id = ?", transactionID, userID)
	if err != nil {
		return fmt.Errorf("error deleting transaction: %v", err)
	}

	if rowsAffected, _ := result.RowsAffected(); rowsAffected != 1 {
		return fmt.Errorf("unexpected rows affected: %d", rowsAffected)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing: %v", err)
	}

	log.Printf("Deleted transaction %s for user %s", transactionID, userID)
	return nil
}

// resolveTransactionDeleteConflict resuelve conflictos
func resolveTransactionDeleteConflict(request SyncTransactionDeleteConflictRequest) (*SyncTransactionDeleteResult, error) {
	result := &SyncTransactionDeleteResult{
		LocalID:       request.LocalID,
		ServerID:      request.ServerID,
		TransactionID: request.TransactionID,
		Action:        "delete",
		Status:        "success",
		SyncTimestamp: time.Now().Format(time.RFC3339),
	}

	switch request.Resolution {
	case "proceed":
		err := executeTransactionDeletion(request.TransactionID, request.UserID, "Conflict resolved: proceed")
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("Error proceeding: %v", err)
			return result, err
		}
	case "cancel":
		result.Status = "cancelled"
		result.Error = "Deletion cancelled"
	case "force_delete":
		err := executeTransactionDeletion(request.TransactionID, request.UserID, "Forced deletion")
		if err != nil {
			result.Status = "error"
			result.Error = fmt.Sprintf("Error forcing: %v", err)
			return result, err
		}
	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("Unknown resolution: %s", request.Resolution)
		return result, fmt.Errorf("unknown resolution: %s", request.Resolution)
	}

	return result, nil
}

// Helper functions
func validateDateFormat(dateStr string) error {
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		_, err = time.Parse(time.RFC3339, dateStr)
	}
	return err
}

func validateTimestampFormat(timestampStr string) error {
	_, err := time.Parse(time.RFC3339, timestampStr)
	return err
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func isValidTransactionType(transactionType string, validTypes []string) bool {
	return contains(validTypes, transactionType)
}
