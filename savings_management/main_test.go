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
	testDBPath := filepath.Join(os.TempDir(), "test_savings.db")

	testDB, err = sql.Open("sqlite3", testDBPath)
	if err != nil {
		panic("Failed to create test database: " + err.Error())
	}

	// Create the savings table
	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS savings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			available REAL NOT NULL,
			goal REAL NOT NULL,
			period TEXT NOT NULL DEFAULT 'monthly',
			percent REAL NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		panic("Failed to create test table: " + err.Error())
	}

	// Replace the global db with our test database
	db = testDB
}

func cleanupTestDB() {
	if testDB != nil {
		testDB.Close()
	}
}

func insertTestSavingsData(userID string, available, goal float64, period string) error {
	percent := (available / goal) * 100
	_, err := testDB.Exec(`
		INSERT INTO savings (user_id, available, goal, period, percent)
		VALUES (?, ?, ?, ?, ?)
	`, userID, available, goal, period, percent)
	return err
}

func clearTestData() {
	testDB.Exec("DELETE FROM savings")
}

func TestHandleDeleteSavings_Success(t *testing.T) {
	clearTestData()

	// Insert test data
	userID := "test_user_123"
	err := insertTestSavingsData(userID, 500.0, 1000.0, "monthly")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Create delete request
	deleteRequest := SavingsDeleteRequest{
		UserID: userID,
	}

	requestBody, _ := json.Marshal(deleteRequest)
	req := httptest.NewRequest("DELETE", "/savings/delete", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handleDeleteSavings(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	// Check response body
	var response ApiResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success to be true, got false")
	}

	if response.Message != "Savings goal deleted successfully" {
		t.Errorf("Expected message 'Savings goal deleted successfully', got '%s'", response.Message)
	}

	// Verify data was actually deleted from database
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM savings WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 records after deletion, got %d", count)
	}
}

func TestHandleDeleteSavings_UserNotFound(t *testing.T) {
	clearTestData()

	// Create delete request for non-existent user
	deleteRequest := SavingsDeleteRequest{
		UserID: "non_existent_user",
	}

	requestBody, _ := json.Marshal(deleteRequest)
	req := httptest.NewRequest("DELETE", "/savings/delete", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handleDeleteSavings(rr, req)

	// Check status code
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, rr.Code)
	}

	// Check response body
	var response ApiResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success to be false, got true")
	}

	if response.Message != "Error deleting savings data" {
		t.Errorf("Expected message 'Error deleting savings data', got '%s'", response.Message)
	}
}

func TestHandleDeleteSavings_InvalidMethod(t *testing.T) {
	// Create request with wrong method
	req := httptest.NewRequest("GET", "/savings/delete", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handleDeleteSavings(rr, req)

	// Check status code
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status code %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}

	// Check response body
	var response ApiResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success to be false, got true")
	}

	if response.Message != "Method not allowed" {
		t.Errorf("Expected message 'Method not allowed', got '%s'", response.Message)
	}
}

func TestHandleDeleteSavings_InvalidRequestBody(t *testing.T) {
	// Create request with invalid JSON
	req := httptest.NewRequest("DELETE", "/savings/delete", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handleDeleteSavings(rr, req)

	// Check status code
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Check response body
	var response ApiResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success to be false, got true")
	}

	if response.Message != "Invalid request body" {
		t.Errorf("Expected message 'Invalid request body', got '%s'", response.Message)
	}
}

func TestHandleDeleteSavings_MissingUserID(t *testing.T) {
	// Create delete request without user ID
	deleteRequest := SavingsDeleteRequest{
		UserID: "",
	}

	requestBody, _ := json.Marshal(deleteRequest)
	req := httptest.NewRequest("DELETE", "/savings/delete", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call the handler
	handleDeleteSavings(rr, req)

	// Check status code
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// Check response body
	var response ApiResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success to be false, got true")
	}

	if response.Message != "User ID is required" {
		t.Errorf("Expected message 'User ID is required', got '%s'", response.Message)
	}
}

func TestDeleteSavingsData_Success(t *testing.T) {
	clearTestData()

	// Insert test data
	userID := "test_user_456"
	err := insertTestSavingsData(userID, 300.0, 800.0, "weekly")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Call deleteSavingsData
	err = deleteSavingsData(userID)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify data was deleted
	var count int
	err = testDB.QueryRow("SELECT COUNT(*) FROM savings WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query database: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 records after deletion, got %d", count)
	}
}

func TestDeleteSavingsData_UserNotFound(t *testing.T) {
	clearTestData()

	// Try to delete non-existent user
	err := deleteSavingsData("non_existent_user")
	if err == nil {
		t.Errorf("Expected error for non-existent user, got nil")
	}

	expectedError := "no savings goal found for user non_existent_user"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}
