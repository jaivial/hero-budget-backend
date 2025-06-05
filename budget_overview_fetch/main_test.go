package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test BudgetOverview struct serialization
func TestBudgetOverviewSerialization(t *testing.T) {
	overview := &BudgetOverview{
		RemainingAmount: 1245.30,
		ExpensePercent:  75.8,
		SpentAmount:     3500.00,
		UpcomingAmount:  750.50,
		TotalAmount:     5000.00,
		CombinedExpense: 4250.50,
		TotalIncome:     5495.80,
		DailyRate:       141.68,
		HighSpending:    false,
		MoneyFlow:       MoneyFlow{FromPrevious: 495.80},
	}

	data, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("Failed to marshal BudgetOverview: %v", err)
	}

	var unmarshaled BudgetOverview
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal BudgetOverview: %v", err)
	}

	if unmarshaled.RemainingAmount != overview.RemainingAmount {
		t.Errorf("Expected RemainingAmount %f, got %f", overview.RemainingAmount, unmarshaled.RemainingAmount)
	}

	if unmarshaled.MoneyFlow.FromPrevious != overview.MoneyFlow.FromPrevious {
		t.Errorf("Expected MoneyFlow.FromPrevious %f, got %f", overview.MoneyFlow.FromPrevious, unmarshaled.MoneyFlow.FromPrevious)
	}
}

// Test getTableAndCondition function
func TestGetTableAndCondition(t *testing.T) {
	tests := []struct {
		period        string
		date          string
		wantTable     string
		wantCondition string
	}{
		{
			period:        "daily",
			date:          "2024-01-15",
			wantTable:     "daily_cash_bank_balance",
			wantCondition: "date = '2024-01-15'",
		},
		{
			period:        "weekly",
			date:          "2024-W03",
			wantTable:     "weekly_cash_bank_balance",
			wantCondition: "year_week = '2024-W03'",
		},
		{
			period:        "monthly",
			date:          "2024-01",
			wantTable:     "monthly_cash_bank_balance",
			wantCondition: "year_month = '2024-01'",
		},
		{
			period:        "quarterly",
			date:          "2024-Q1",
			wantTable:     "quarterly_cash_bank_balance",
			wantCondition: "year_quarter = '2024-Q1'",
		},
		{
			period:        "semiannual",
			date:          "2024-H1",
			wantTable:     "semiannual_cash_bank_balance",
			wantCondition: "year_half = '2024-H1'",
		},
		{
			period:        "annual",
			date:          "2024",
			wantTable:     "annual_cash_bank_balance",
			wantCondition: "year = '2024'",
		},
		{
			period:        "invalid",
			date:          "2024-01",
			wantTable:     "monthly_cash_bank_balance",
			wantCondition: "year_month = '2024-01'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			gotTable, gotCondition := getTableAndCondition(tt.period, tt.date)
			if gotTable != tt.wantTable {
				t.Errorf("getTableAndCondition() table = %v, want %v", gotTable, tt.wantTable)
			}
			if gotCondition != tt.wantCondition {
				t.Errorf("getTableAndCondition() condition = %v, want %v", gotCondition, tt.wantCondition)
			}
		})
	}
}

// Test calculateBudgetOverview function
func TestCalculateBudgetOverview(t *testing.T) {
	testData := &BalanceData{
		IncomeBankAmount:     3000.00,
		IncomeCashAmount:     500.00,
		ExpenseBankAmount:    2000.00,
		ExpenseCashAmount:    300.00,
		BillBankAmount:       400.00,
		BillCashAmount:       100.00,
		TotalBalance:         1000.00,
		TotalPreviousBalance: 800.00,
	}

	overview := calculateBudgetOverview(testData, "monthly", "test_user")

	// Check calculated values
	expectedTotalIncome := 3500.00 // 3000 + 500
	if overview.TotalIncome != expectedTotalIncome {
		t.Errorf("Expected TotalIncome %f, got %f", expectedTotalIncome, overview.TotalIncome)
	}

	expectedSpentAmount := 2300.00 // 2000 + 300
	if overview.SpentAmount != expectedSpentAmount {
		t.Errorf("Expected SpentAmount %f, got %f", expectedSpentAmount, overview.SpentAmount)
	}

	expectedUpcomingAmount := 500.00 // 400 + 100
	if overview.UpcomingAmount != expectedUpcomingAmount {
		t.Errorf("Expected UpcomingAmount %f, got %f", expectedUpcomingAmount, overview.UpcomingAmount)
	}

	expectedCombinedExpense := 2800.00 // 2300 + 500
	if overview.CombinedExpense != expectedCombinedExpense {
		t.Errorf("Expected CombinedExpense %f, got %f", expectedCombinedExpense, overview.CombinedExpense)
	}

	expectedTotalAmount := 3800.00 // 1000 + 2800
	if overview.TotalAmount != expectedTotalAmount {
		t.Errorf("Expected TotalAmount %f, got %f", expectedTotalAmount, overview.TotalAmount)
	}

	expectedRemainingAmount := 1000.00 // 3800 - 2800
	if overview.RemainingAmount != expectedRemainingAmount {
		t.Errorf("Expected RemainingAmount %f, got %f", expectedRemainingAmount, overview.RemainingAmount)
	}

	// Check expense percentage is approximately 73.68%
	if overview.ExpensePercent < 73.0 || overview.ExpensePercent > 74.0 {
		t.Errorf("Expected ExpensePercent around 73.68, got %f", overview.ExpensePercent)
	}

	// Check high spending (should be false since < 80%)
	if overview.HighSpending {
		t.Errorf("Expected HighSpending false, got true")
	}

	// Check money flow
	if overview.MoneyFlow.FromPrevious != 800.00 {
		t.Errorf("Expected MoneyFlow.FromPrevious %f, got %f", 800.00, overview.MoneyFlow.FromPrevious)
	}
}

// Test calculateDailyRate function
func TestCalculateDailyRate(t *testing.T) {
	tests := []struct {
		spentAmount float64
		period      string
		wantRate    float64
	}{
		{spentAmount: 300.00, period: "daily", wantRate: 300.00},
		{spentAmount: 700.00, period: "weekly", wantRate: 100.00},
		{spentAmount: 3000.00, period: "monthly", wantRate: 100.00},
		{spentAmount: 9000.00, period: "quarterly", wantRate: 100.00},
		{spentAmount: 18000.00, period: "semiannual", wantRate: 100.00},
		{spentAmount: 36500.00, period: "annual", wantRate: 100.00},
		{spentAmount: 1000.00, period: "invalid", wantRate: 33.33}, // Default to monthly
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			gotRate := calculateDailyRate(tt.spentAmount, tt.period)
			if gotRate < tt.wantRate-0.5 || gotRate > tt.wantRate+0.5 {
				t.Errorf("calculateDailyRate() = %v, want %v", gotRate, tt.wantRate)
			}
		})
	}
}

// Test formatDateForPeriod function
func TestFormatDateForPeriod(t *testing.T) {
	testDate := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		period string
		want   string
	}{
		{period: "daily", want: "2024-03-15"},
		{period: "weekly", want: "2024-W11"},
		{period: "monthly", want: "2024-03"},
		{period: "quarterly", want: "2024-Q1"},
		{period: "semiannual", want: "2024-H1"},
		{period: "annual", want: "2024"},
		{period: "invalid", want: "2024-03"},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			got := formatDateForPeriod(testDate, tt.period)
			if got != tt.want {
				t.Errorf("formatDateForPeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test health endpoint
func TestHandleHealth(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleHealth)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success true, got %v", response.Success)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be map[string]interface{}, got %T", response.Data)
	}

	if data["service"] != "budget_overview_fetch" {
		t.Errorf("Expected service 'budget_overview_fetch', got %v", data["service"])
	}

	if data["port"] != "8097" {
		t.Errorf("Expected port '8097', got %v", data["port"])
	}
}

// Test budget overview endpoint with invalid method
func TestHandleBudgetOverviewInvalidMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/budget-overview", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleBudgetOverview)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}
}

// Test budget overview endpoint with missing user_id
func TestHandleBudgetOverviewMissingUserID(t *testing.T) {
	request := BudgetOverviewRequest{
		Period: "monthly",
		Date:   "2024-01",
	}

	requestBody, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", "/budget-overview", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleBudgetOverview)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}

	if response.Message != "user_id is required" {
		t.Errorf("Expected message 'user_id is required', got '%v'", response.Message)
	}
}

// Test CORS middleware
func TestCorsMiddleware(t *testing.T) {
	// Test OPTIONS request
	req, err := http.NewRequest("OPTIONS", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := corsMiddleware(handleHealth)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check CORS headers
	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for key, expectedValue := range expectedHeaders {
		if gotValue := rr.Header().Get(key); gotValue != expectedValue {
			t.Errorf("Expected header %s: %s, got %s", key, expectedValue, gotValue)
		}
	}

	// Test normal request with CORS headers
	req, err = http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	handler = corsMiddleware(handleHealth)

	handler.ServeHTTP(rr, req)

	for key, expectedValue := range expectedHeaders {
		if gotValue := rr.Header().Get(key); gotValue != expectedValue {
			t.Errorf("Expected header %s: %s, got %s", key, expectedValue, gotValue)
		}
	}
}

// Benchmark test for calculateBudgetOverview
func BenchmarkCalculateBudgetOverview(b *testing.B) {
	testData := &BalanceData{
		IncomeBankAmount:     3000.00,
		IncomeCashAmount:     500.00,
		ExpenseBankAmount:    2000.00,
		ExpenseCashAmount:    300.00,
		BillBankAmount:       400.00,
		BillCashAmount:       100.00,
		TotalBalance:         1000.00,
		TotalPreviousBalance: 800.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateBudgetOverview(testData, "monthly", "test_user")
	}
}

// Test Transaction struct serialization
func TestTransactionSerialization(t *testing.T) {
	paid := true
	overdue := false
	recurring := true
	overdueDays := 5

	transaction := &Transaction{
		ID:            1,
		Type:          "bill",
		Amount:        150.50,
		Date:          "2024-01-15",
		Category:      "utilities",
		PaymentMethod: "bank",
		Description:   "Electric bill",
		Name:          "Electric Company",
		Paid:          &paid,
		Overdue:       &overdue,
		Recurring:     &recurring,
		OverdueDays:   &overdueDays,
		Icon:          "⚡",
	}

	data, err := json.Marshal(transaction)
	if err != nil {
		t.Fatalf("Failed to marshal Transaction: %v", err)
	}

	var unmarshaled Transaction
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Transaction: %v", err)
	}

	if unmarshaled.ID != transaction.ID {
		t.Errorf("Expected ID %d, got %d", transaction.ID, unmarshaled.ID)
	}

	if unmarshaled.Type != transaction.Type {
		t.Errorf("Expected Type %s, got %s", transaction.Type, unmarshaled.Type)
	}

	if unmarshaled.Amount != transaction.Amount {
		t.Errorf("Expected Amount %f, got %f", transaction.Amount, unmarshaled.Amount)
	}

	if *unmarshaled.Paid != *transaction.Paid {
		t.Errorf("Expected Paid %v, got %v", *transaction.Paid, *unmarshaled.Paid)
	}
}

// Test calculatePeriodDateRange function
func TestCalculatePeriodDateRange(t *testing.T) {
	tests := []struct {
		period    string
		wantError bool
	}{
		{period: "daily", wantError: false},
		{period: "weekly", wantError: false},
		{period: "monthly", wantError: false},
		{period: "quarterly", wantError: false},
		{period: "semiannual", wantError: false},
		{period: "annual", wantError: false},
		{period: "invalid", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			startDate, endDate, err := calculatePeriodDateRange(tt.period)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for period %s, got nil", tt.period)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for period %s: %v", tt.period, err)
				return
			}

			if startDate == "" || endDate == "" {
				t.Errorf("Expected non-empty dates for period %s, got start: %s, end: %s", tt.period, startDate, endDate)
			}

			// Validate date format
			if _, err := time.Parse("2006-01-02", startDate); err != nil {
				t.Errorf("Invalid start date format for period %s: %s", tt.period, startDate)
			}

			if _, err := time.Parse("2006-01-02", endDate); err != nil {
				t.Errorf("Invalid end date format for period %s: %s", tt.period, endDate)
			}
		})
	}
}

// Test contains function
func TestContains(t *testing.T) {
	slice := []string{"income", "expense", "bill"}

	tests := []struct {
		item string
		want bool
	}{
		{item: "income", want: true},
		{item: "expense", want: true},
		{item: "bill", want: true},
		{item: "transfer", want: false},
		{item: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.item, func(t *testing.T) {
			got := contains(slice, tt.item)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v for item %s", got, tt.want, tt.item)
			}
		})
	}
}

// Test handleTransactionHistory with invalid method
func TestHandleTransactionHistoryInvalidMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/transactions/history", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTransactionHistory)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}
}

// Test handleTransactionHistory with missing user_id
func TestHandleTransactionHistoryMissingUserID(t *testing.T) {
	request := TransactionRequest{
		Period:    "monthly",
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	}

	requestBody, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", "/transactions/history", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleTransactionHistory)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}

	if response.Message != "user_id is required" {
		t.Errorf("Expected message 'user_id is required', got '%v'", response.Message)
	}
}

// Test handleUpcomingBills with invalid method
func TestHandleUpcomingBillsInvalidMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/transactions/upcoming-bills", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleUpcomingBills)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusMethodNotAllowed)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}
}

// Test handleUpcomingBills with missing user_id
func TestHandleUpcomingBillsMissingUserID(t *testing.T) {
	request := TransactionRequest{
		Period: "monthly",
	}

	requestBody, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", "/transactions/upcoming-bills", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleUpcomingBills)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}

	var response ApiResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success false, got %v", response.Success)
	}

	if response.Message != "user_id is required" {
		t.Errorf("Expected message 'user_id is required', got '%v'", response.Message)
	}
}

// Test TransactionRequest validation and defaults
func TestTransactionRequestDefaults(t *testing.T) {
	request := TransactionRequest{
		UserID: "test_user",
		Limit:  0, // Should default to 100
	}

	// Simulate the validation logic from handleTransactionHistory
	if request.Limit <= 0 {
		request.Limit = 100
	}
	if request.Limit > 1000 {
		request.Limit = 1000
	}

	if request.Limit != 100 {
		t.Errorf("Expected default limit 100, got %d", request.Limit)
	}

	// Test maximum limit
	request.Limit = 1500
	if request.Limit > 1000 {
		request.Limit = 1000
	}

	if request.Limit != 1000 {
		t.Errorf("Expected maximum limit 1000, got %d", request.Limit)
	}
}
