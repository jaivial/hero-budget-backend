package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

// DeleteBillRequest represents the request structure for deleting a bill
type DeleteBillRequest struct {
	UserID string `json:"user_id"`
	BillID int    `json:"bill_id"`
}

// BillData represents the bill data needed for monthly balance updates
type BillData struct {
	ID            int
	UserID        string
	Amount        float64
	PaymentMethod string
	StartDate     string
	Duration      int
}

// ExpenseMonth represents a month where the bill has associated expenses
type ExpenseMonth struct {
	YearMonth string
	Date      string
}

// handleDeleteBill handles the HTTP request to delete a bill
func handleDeleteBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest DeleteBillRequest
	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		sendErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := validateDeleteBillRequest(deleteRequest); err != nil {
		sendErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute deletion
	if err := deleteBill(deleteRequest); err != nil {
		if err.Error() == "bill not found" {
			sendErrorResponse(w, "Bill not found or you don't have permission to delete it", http.StatusNotFound)
			return
		}
		log.Printf("Error deleting bill: %v", err)
		sendErrorResponse(w, "Error deleting bill", http.StatusInternalServerError)
		return
	}

	sendSuccessResponse(w, "Bill deleted successfully", map[string]interface{}{
		"bill_id": deleteRequest.BillID,
		"user_id": deleteRequest.UserID,
		"status":  "deleted",
	})
}

// validateDeleteBillRequest validates the delete bill request
func validateDeleteBillRequest(request DeleteBillRequest) error {
	if request.UserID == "" {
		return NewValidationError("User ID is required")
	}
	if request.BillID <= 0 {
		return NewValidationError("Valid bill ID is required")
	}
	return nil
}

// deleteBill performs the actual deletion of a bill and related data
func deleteBill(request DeleteBillRequest) error {
	// Check if bill exists and belongs to the user
	if err := verifyBillOwnership(request.BillID, request.UserID); err != nil {
		return err
	}

	// Get bill data before deletion for balance updates
	billData, err := getBillDataBeforeDelete(request.BillID, request.UserID)
	if err != nil {
		return err
	}

	// Update monthly cash bank balance before deleting the bill
	if err := updateMonthlyBalanceForDeletedBill(db, *billData); err != nil {
		log.Printf("Error updating monthly balance for deleted bill: %v", err)
		return err
	}

	// Delete related bill_payments first
	if err := deleteBillPayments(request.BillID); err != nil {
		return err
	}

	// Delete the bill
	return deleteBillRecord(request.BillID, request.UserID)
}

// verifyBillOwnership checks if the bill exists and belongs to the user
func verifyBillOwnership(billID int, userID string) error {
	var existingBillID int
	checkQuery := "SELECT id FROM bills WHERE id = ? AND user_id = ?"
	err := db.QueryRow(checkQuery, billID, userID).Scan(&existingBillID)

	if err == sql.ErrNoRows {
		return NewNotFoundError("bill not found")
	}
	if err != nil {
		log.Printf("Error checking bill existence: %v", err)
		return err
	}
	return nil
}

// deleteBillPayments removes all payment records associated with a bill
func deleteBillPayments(billID int) error {
	deletePaymentsQuery := "DELETE FROM bill_payments WHERE bill_id = ?"
	_, err := db.Exec(deletePaymentsQuery, billID)
	if err != nil {
		log.Printf("Error deleting bill payments: %v", err)
		return err
	}
	return nil
}

// deleteBillRecord removes the bill record from the database
func deleteBillRecord(billID int, userID string) error {
	deleteBillQuery := "DELETE FROM bills WHERE id = ? AND user_id = ?"
	result, err := db.Exec(deleteBillQuery, billID, userID)
	if err != nil {
		log.Printf("Error deleting bill: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		return NewNotFoundError("bill not found or already deleted")
	}

	return nil
}

// Custom error types for better error handling
type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string {
	return e.Message
}

func NewNotFoundError(message string) NotFoundError {
	return NotFoundError{Message: message}
}

// updateMainBalanceColumnsForDeletedBill updates main balance columns incrementally when a bill is deleted
// Eliminating a bill IMPROVES the month's balance, so we ADD the amounts to main balance columns
func updateMainBalanceColumnsForDeletedBill(billData BillData, targetMonth string) error {
	log.Printf("Updating main balance columns for deleted bill - Bill: %d, Amount: %.2f, Method: %s, Month: %s",
		billData.ID, billData.Amount, billData.PaymentMethod, targetMonth)

	// Prepare the update query based on payment method
	var updateQuery string
	switch billData.PaymentMethod {
	case "bank":
		updateQuery = `
			UPDATE monthly_balance 
			SET bank_amount = bank_amount + ?,
				total_balance = bank_amount + cash_amount
			WHERE user_id = ? AND year_month = ?`
	case "cash":
		updateQuery = `
			UPDATE monthly_balance 
			SET cash_amount = cash_amount + ?,
				total_balance = bank_amount + cash_amount
			WHERE user_id = ? AND year_month = ?`
	default:
		return NewValidationError("Invalid payment method: " + billData.PaymentMethod)
	}

	// Execute the update
	result, err := db.Exec(updateQuery, billData.Amount, billData.UserID, targetMonth)
	if err != nil {
		log.Printf("Error updating main balance columns for deleted bill: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Error getting rows affected: %v", err)
		return err
	}

	if rowsAffected == 0 {
		log.Printf("Warning: No rows affected when updating balance for month %s", targetMonth)
	} else {
		log.Printf("Successfully updated main balance columns for month %s", targetMonth)
	}

	return nil
}
