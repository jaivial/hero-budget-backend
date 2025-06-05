package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	// Setup test database
	setupTestDB()

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestDB()

	os.Exit(code)
}

func setupTestDB() {
	var err error

	// Create a temporary database file for testing
	testDBPath := filepath.Join(os.TempDir(), "test_users.db")

	testDB, err = sql.Open("sqlite3", testDBPath)
	if err != nil {
		panic("Failed to open test database: " + err.Error())
	}

	// Replace the global db with our test database
	db = testDB

	// Create tables
	createTablesIfNotExist()

	// Insert test data
	insertTestData()
}

func cleanupTestDB() {
	if testDB != nil {
		testDB.Close()
	}
}

func insertTestData() {
	// Insert test cash_bank data
	_, err := testDB.Exec(`
		INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "test_user_1", "2024-01", 500.0, 50.0, 500.0, 50.0, 1000.0)

	if err != nil {
		panic("Failed to insert test data: " + err.Error())
	}
}

func clearTestData() {
	testDB.Exec("DELETE FROM cash_bank WHERE user_id LIKE 'test_%'")
	testDB.Exec("DELETE FROM cash_bank_transactions WHERE user_id LIKE 'test_%'")
}

func TestHandleFetchDistribution(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		expectedStatus int
		expectData     bool
	}{
		{
			name:           "Valid user ID",
			userID:         "test_user_1",
			expectedStatus: http.StatusOK,
			expectData:     true,
		},
		{
			name:           "Missing user ID",
			userID:         "",
			expectedStatus: http.StatusBadRequest,
			expectData:     false,
		},
		{
			name:           "Non-existent user",
			userID:         "non_existent_user",
			expectedStatus: http.StatusOK,
			expectData:     true, // Should return default values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/cash-bank/distribution", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.userID != "" {
				q := req.URL.Query()
				q.Add("user_id", tt.userID)
				req.URL.RawQuery = q.Encode()
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleFetchDistribution)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			if tt.expectData && tt.expectedStatus == http.StatusOK {
				var response ApiResponse
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				if err != nil {
					t.Errorf("Failed to unmarshal response: %v", err)
				}

				if !response.Success {
					t.Errorf("Expected success response, got: %v", response.Success)
				}
			}
		})
	}
}

func TestHandleCashToBankTransfer(t *testing.T) {
	// Clear and setup fresh test data
	clearTestData()
	insertTestData()

	tests := []struct {
		name           string
		requestBody    TransferRequest
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name: "Valid transfer",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 100.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name: "Invalid amount - zero",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 0.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "Invalid amount - negative",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: -50.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "Insufficient cash",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 1000.0, // More than available cash (500)
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "Missing user ID",
			requestBody: TransferRequest{
				UserID: "",
				Amount: 100.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset test data for each test
			clearTestData()
			insertTestData()

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, err := http.NewRequest("POST", "/transfer/cash-to-bank", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleCashToBankTransfer)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			var response ApiResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			if err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			if response.Success != tt.expectSuccess {
				t.Errorf("Expected success: %v, got: %v", tt.expectSuccess, response.Success)
			}

			// If transfer was successful, verify the amounts were updated correctly
			if tt.expectSuccess && tt.expectedStatus == http.StatusOK {
				distribution, err := fetchCashBankDistribution(tt.requestBody.UserID)
				if err != nil {
					t.Errorf("Failed to fetch updated distribution: %v", err)
				}

				expectedCash := 500.0 - tt.requestBody.Amount
				expectedBank := 500.0 + tt.requestBody.Amount

				if distribution.CashAmount != expectedCash {
					t.Errorf("Expected cash amount: %v, got: %v", expectedCash, distribution.CashAmount)
				}

				if distribution.BankAmount != expectedBank {
					t.Errorf("Expected bank amount: %v, got: %v", expectedBank, distribution.BankAmount)
				}
			}
		})
	}
}

func TestHandleBankToCashTransfer(t *testing.T) {
	// Clear and setup fresh test data
	clearTestData()
	insertTestData()

	tests := []struct {
		name           string
		requestBody    TransferRequest
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name: "Valid transfer",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 100.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name: "Invalid amount - zero",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 0.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "Insufficient bank balance",
			requestBody: TransferRequest{
				UserID: "test_user_1",
				Amount: 1000.0, // More than available bank balance (500)
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset test data for each test
			clearTestData()
			insertTestData()

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, err := http.NewRequest("POST", "/transfer/bank-to-cash", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleBankToCashTransfer)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			var response ApiResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			if err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			if response.Success != tt.expectSuccess {
				t.Errorf("Expected success: %v, got: %v", tt.expectSuccess, response.Success)
			}

			// If transfer was successful, verify the amounts were updated correctly
			if tt.expectSuccess && tt.expectedStatus == http.StatusOK {
				distribution, err := fetchCashBankDistribution(tt.requestBody.UserID)
				if err != nil {
					t.Errorf("Failed to fetch updated distribution: %v", err)
				}

				expectedCash := 500.0 + tt.requestBody.Amount
				expectedBank := 500.0 - tt.requestBody.Amount

				if distribution.CashAmount != expectedCash {
					t.Errorf("Expected cash amount: %v, got: %v", expectedCash, distribution.CashAmount)
				}

				if distribution.BankAmount != expectedBank {
					t.Errorf("Expected bank amount: %v, got: %v", expectedBank, distribution.BankAmount)
				}
			}
		})
	}
}

func TestUpdateCashAmount(t *testing.T) {
	clearTestData()
	insertTestData()

	tests := []struct {
		name           string
		requestBody    UpdateAmountRequest
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name: "Valid cash update",
			requestBody: UpdateAmountRequest{
				UserID: "test_user_1",
				Amount: 750.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name: "Invalid amount - negative",
			requestBody: UpdateAmountRequest{
				UserID: "test_user_1",
				Amount: -100.0,
				Date:   time.Now().Format(time.RFC3339),
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTestData()
			insertTestData()

			jsonBody, _ := json.Marshal(tt.requestBody)
			req, err := http.NewRequest("POST", "/cash/update", bytes.NewBuffer(jsonBody))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handleUpdateCash)
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}

			var response ApiResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			if err != nil {
				t.Errorf("Failed to unmarshal response: %v", err)
			}

			if response.Success != tt.expectSuccess {
				t.Errorf("Expected success: %v, got: %v", tt.expectSuccess, response.Success)
			}
		})
	}
}

func TestTransactionHistory(t *testing.T) {
	clearTestData()

	// Test adding a transaction
	err := addTransaction("test_user_1", "cash_to_bank", 100.0, time.Now().Format(time.RFC3339))
	if err != nil {
		t.Errorf("Failed to add transaction: %v", err)
	}

	// Verify transaction was added
	var count int
	err = testDB.QueryRow(`
		SELECT COUNT(*) 
		FROM cash_bank_transactions 
		WHERE user_id = ? AND transaction_type = ? AND amount = ?
	`, "test_user_1", "cash_to_bank", 100.0).Scan(&count)

	if err != nil {
		t.Errorf("Failed to query transaction: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 transaction, got %d", count)
	}
}

func TestPercentageCalculation(t *testing.T) {
	clearTestData()

	// Insert test data with specific amounts
	_, err := testDB.Exec(`
		INSERT INTO cash_bank (user_id, month, cash_amount, cash_percent, bank_amount, bank_percent, monthly_total)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "test_user_2", "2024-01", 300.0, 30.0, 700.0, 70.0, 1000.0)

	if err != nil {
		t.Fatal("Failed to insert test data")
	}

	// Perform a transfer that should update percentages
	requestBody := TransferRequest{
		UserID: "test_user_2",
		Amount: 100.0, // Transfer 100 from cash to bank
		Date:   time.Now().Format(time.RFC3339),
	}

	jsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", "/transfer/cash-to-bank", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleCashToBankTransfer)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Transfer failed with status: %d", rr.Code)
	}

	// Verify percentages were recalculated correctly
	distribution, err := fetchCashBankDistribution("test_user_2")
	if err != nil {
		t.Fatal("Failed to fetch distribution")
	}

	expectedCashPercent := 20.0 // 200/1000 * 100
	expectedBankPercent := 80.0 // 800/1000 * 100

	if distribution.CashPercent != expectedCashPercent {
		t.Errorf("Expected cash percent: %v, got: %v", expectedCashPercent, distribution.CashPercent)
	}

	if distribution.BankPercent != expectedBankPercent {
		t.Errorf("Expected bank percent: %v, got: %v", expectedBankPercent, distribution.BankPercent)
	}
}

func TestCORSHeaders(t *testing.T) {
	req, err := http.NewRequest("OPTIONS", "/cash-bank/distribution", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := corsMiddleware(handleFetchDistribution)
	handler.ServeHTTP(rr, req)

	// Check CORS headers
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected Access-Control-Allow-Origin: *, got: %s", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	if rr.Header().Get("Access-Control-Allow-Methods") != "GET, POST, OPTIONS" {
		t.Errorf("Expected Access-Control-Allow-Methods: GET, POST, OPTIONS, got: %s", rr.Header().Get("Access-Control-Allow-Methods"))
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS request, got: %d", rr.Code)
	}
}
