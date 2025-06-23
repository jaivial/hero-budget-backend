#!/bin/bash

# Single Bill Update Test to check fixed logic
# Tests the /bills/update endpoint with one scenario to verify fixes

# Configuration
BASE_URL="http://localhost:8091"
USER_ID="test_single_fix"
DB_PATH="/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Function to check database coherence
check_database_coherence() {
    local bill_id=$1
    local scenario_name="$2"
    
    echo -e "\n${YELLOW}--- Database Coherence Check for $scenario_name ---${NC}"
    
    # Check bills table
    echo "Bills table:"
    sqlite3 "$DB_PATH" "SELECT id, name, amount, start_date, duration_months, payment_method, payment_day FROM bills WHERE id = $bill_id;"
    
    # Check monthly_cash_bank_balance table
    echo -e "\nMonthly Cash Bank Balance (showing income, expenses, bills, and final balances):"
    sqlite3 "$DB_PATH" "SELECT year_month, 
        printf('I: B=%.1f C=%.1f', COALESCE(income_bank_amount,0), COALESCE(income_cash_amount,0)) as Income,
        printf('E: B=%.1f C=%.1f', COALESCE(expense_bank_amount,0), COALESCE(expense_cash_amount,0)) as Expenses,
        printf('Bill: B=%.1f C=%.1f', COALESCE(bill_bank_amount,0), COALESCE(bill_cash_amount,0)) as Bills,
        printf('Final: B=%.1f C=%.1f', COALESCE(bank_amount,0), COALESCE(cash_amount,0)) as Final_Balance
        FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID' ORDER BY year_month LIMIT 8;"
    
    # Check bill_payments table
    echo -e "\nBill Payments:"
    sqlite3 "$DB_PATH" "SELECT bill_id, year_month, payment_date, amount, payment_method, paid FROM bill_payments WHERE bill_id = $bill_id ORDER BY year_month;"
    
    echo -e "${YELLOW}--- End Database Check ---${NC}\n"
}

# Function to create a test bill
create_test_bill() {
    local amount=$1
    local start_date=$2
    local duration=$3
    local payment_method=$4
    local payment_day=$5
    local category="$6"
    
    local response=$(curl -s -X POST "$BASE_URL/bills/add" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$USER_ID\",
            \"name\": \"Test Bill\",
            \"amount\": $amount,
            \"due_date\": \"$start_date\",
            \"category\": \"$category\",
            \"icon\": \"💡\",
            \"start_date\": \"$start_date\",
            \"payment_day\": $payment_day,
            \"duration_months\": $duration,
            \"regularity\": \"monthly\",
            \"payment_method\": \"$payment_method\",
            \"recurring\": true,
            \"paid\": false,
            \"overdue\": false,
            \"overdue_days\": 0
        }")
    
    echo "$response" | grep -o '"id":[0-9]*' | grep -o '[0-9]*'
}

# Function to update a bill
update_bill() {
    local bill_id=$1
    local amount=$2
    local start_date=$3
    local duration=$4
    local payment_method=$5
    local payment_day=$6
    local category="$7"
    
    curl -s -X POST "$BASE_URL/bills/update" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$USER_ID\",
            \"bill_id\": $bill_id,
            \"name\": \"Updated Test Bill\",
            \"amount\": $amount,
            \"due_date\": \"$start_date\",
            \"category\": \"$category\",
            \"icon\": \"💡\",
            \"start_date\": \"$start_date\",
            \"payment_day\": $payment_day,
            \"duration_months\": $duration,
            \"regularity\": \"monthly\",
            \"payment_method\": \"$payment_method\",
            \"recurring\": true
        }"
}

echo -e "${BLUE}Testing Single Bill Update Scenario (Fixed Logic)${NC}"
echo -e "${BLUE}User ID: $USER_ID${NC}"
echo -e "${BLUE}Database: $DB_PATH${NC}"

# Clean up any existing test data
sqlite3 "$DB_PATH" "DELETE FROM bills WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM bill_payments WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM expenses WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID';"

# Initialize monthly_cash_bank_balance with base data INCLUDING INCOME AMOUNTS
sqlite3 "$DB_PATH" "INSERT OR REPLACE INTO monthly_cash_bank_balance (user_id, year_month, bank_amount, cash_amount, previous_bank_amount, previous_cash_amount, balance_bank_amount, balance_cash_amount, bill_bank_amount, bill_cash_amount, expense_bank_amount, expense_cash_amount, income_bank_amount, income_cash_amount) VALUES 
('$USER_ID', '2024-06', 1000.0, 500.0, 0.0, 0.0, 1000.0, 500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-07', 1000.0, 500.0, 1000.0, 500.0, 2000.0, 1000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-08', 1000.0, 500.0, 2000.0, 1000.0, 3000.0, 1500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-09', 1000.0, 500.0, 3000.0, 1500.0, 4000.0, 2000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-10', 1000.0, 500.0, 4000.0, 2000.0, 5000.0, 2500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-11', 1000.0, 500.0, 5000.0, 2500.0, 6000.0, 3000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0);"

print_info "Initial database setup completed"

# Check initial state
echo -e "\n${BLUE}=== INITIAL STATE ===${NC}"
check_database_coherence "0" "Initial Setup"

# SCENARIO: Basic amount increase
echo -e "\n${BLUE}=== CREATING BILL ===${NC}"
bill_id=$(create_test_bill 100.0 "2024-07-01" 3 "bank" 15 "Utilities")
print_info "Created bill with ID: $bill_id"
check_database_coherence "$bill_id" "After Bill Creation"

echo -e "\n${BLUE}=== UPDATING BILL ===${NC}"
update_response=$(update_bill "$bill_id" 150.0 "2024-07-01" 3 "bank" 15 "Utilities")
print_info "Update response: $update_response"
check_database_coherence "$bill_id" "After Bill Update (100 -> 150)"

echo -e "\n${GREEN}=== Single scenario test completed ===${NC}"
echo -e "${BLUE}Check the database coherence results above to verify if the logic is now correct${NC}"