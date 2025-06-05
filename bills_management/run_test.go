package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	log.Println("========= TESTING BILL PAYMENT FIXES =========")

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// 1. Create a test bill
	testBill := Bill{
		UserID:        "test_user",
		Name:          "Test Bill for Fixes",
		Amount:        50.00,
		DueDate:       time.Now().Format("2006-01-02"),
		Paid:          false,
		Overdue:       false,
		OverdueDays:   0,
		Recurring:     false,
		Category:      "Test",
		Icon:          "test",
		PaymentMethod: "bank",
	}

	// Insert the test bill
	billID, err := addBill(testBill)
	if err != nil {
		log.Fatalf("Failed to add test bill: %v", err)
	}
	log.Printf("Created test bill with ID: %d", billID)

	// 2. Pay the bill
	testBill.ID = billID
	paymentMethod := "cash"
	description := "Test payment for fixes"

	// Create payment request
	expenseReq := BillToExpenseRequest{
		UserID:        testBill.UserID,
		Amount:        testBill.Amount,
		Date:          time.Now().Format("2006-01-02"),
		Category:      testBill.Category,
		PaymentMethod: paymentMethod,
		Description:   description,
	}

	// Mark the bill as paid in the database
	_, err = db.Exec(`
		UPDATE bills
		SET paid = 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, testBill.ID, testBill.UserID)
	if err != nil {
		log.Fatalf("Error marking bill as paid: %v", err)
	}
	log.Printf("Marked bill %d as paid", testBill.ID)

	// 3. Test the createExpenseFromBill function
	if err := createExpenseFromBill(expenseReq); err != nil {
		log.Printf("Error creating expense from bill: %v", err)
	} else {
		log.Printf("Successfully created expense from bill")
	}

	// 4. Verify the bill was marked as paid correctly
	var paid bool
	err = db.QueryRow(`
		SELECT paid FROM bills WHERE id = ? AND user_id = ?
	`, testBill.ID, testBill.UserID).Scan(&paid)
	if err != nil {
		log.Fatalf("Error fetching bill paid status: %v", err)
	}
	log.Printf("Bill %d paid status in database: %t", testBill.ID, paid)

	// 5. Verify the expense was created
	var expenseCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM expenses 
		WHERE user_id = ? AND amount = ? AND category = ? AND payment_method = ?
	`, testBill.UserID, testBill.Amount, testBill.Category, paymentMethod).Scan(&expenseCount)
	if err != nil {
		log.Printf("Error checking for expense: %v", err)
	} else {
		log.Printf("Found %d expenses matching the bill payment", expenseCount)
	}

	// 6. Test balance updates
	var monthlyBalanceCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM monthly_balance 
		WHERE user_id = ? AND bills_amount > 0
	`, testBill.UserID).Scan(&monthlyBalanceCount)
	if err != nil {
		log.Printf("Error checking monthly_balance: %v", err)
	} else {
		log.Printf("Found %d monthly_balance records with bills_amount > 0", monthlyBalanceCount)
	}

	log.Println("========= TEST COMPLETED =========")
}
