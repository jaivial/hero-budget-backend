#!/bin/bash

# Gap Months Analysis Test - Tests bill behavior with gap months between existing records
# This test demonstrates how bills are handled when there are gap months without any records

# Configuration
BASE_URL="http://localhost:8091"
USER_ID="test_gap_months"
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

# Function to check database coherence with extended months
check_database_coherence_extended() {
    local bill_id=$1
    local scenario_name="$2"
    
    echo -e "\n${YELLOW}--- Extended Database Coherence Check for $scenario_name ---${NC}"
    
    # Check bills table
    echo "Bills table:"
    sqlite3 "$DB_PATH" "SELECT id, name, amount, start_date, duration_months, payment_method, payment_day FROM bills WHERE id = $bill_id;"
    
    # Check monthly_cash_bank_balance table with extended range
    echo -e "\nMonthly Cash Bank Balance (showing all relevant months):"
    sqlite3 "$DB_PATH" "SELECT year_month, 
        printf('I: B=%.1f C=%.1f', COALESCE(income_bank_amount,0), COALESCE(income_cash_amount,0)) as Income,
        printf('E: B=%.1f C=%.1f', COALESCE(expense_bank_amount,0), COALESCE(expense_cash_amount,0)) as Expenses,
        printf('Bill: B=%.1f C=%.1f', COALESCE(bill_bank_amount,0), COALESCE(bill_cash_amount,0)) as Bills,
        printf('Prev: B=%.1f C=%.1f', COALESCE(previous_bank_amount,0), COALESCE(previous_cash_amount,0)) as Previous,
        printf('Final: B=%.1f C=%.1f', COALESCE(bank_amount,0), COALESCE(cash_amount,0)) as Final_Balance,
        printf('Total: %.1f', COALESCE(total_balance,0)) as Total
        FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID' ORDER BY year_month;"
    
    # Check bill_payments table
    echo -e "\nBill Payments:"
    sqlite3 "$DB_PATH" "SELECT bill_id, year_month, payment_date, amount, payment_method, paid FROM bill_payments WHERE bill_id = $bill_id ORDER BY year_month;"
    
    echo -e "${YELLOW}--- End Extended Database Check ---${NC}\n"
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
            \"name\": \"Gap Test Bill\",
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

echo -e "${BLUE}Testing Gap Months Behavior with Bills${NC}"
echo -e "${BLUE}User ID: $USER_ID${NC}"
echo -e "${BLUE}Database: $DB_PATH${NC}"

# Clean up any existing test data
sqlite3 "$DB_PATH" "DELETE FROM bills WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM bill_payments WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM expenses WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID';"

# GAP MONTHS SCENARIO: Create initial data with GAPS between months
# We'll create records for 2024-06, 2024-08, and 2024-11 (leaving gaps in 2024-07, 2024-09, 2024-10)
print_info "Setting up scenario with GAP MONTHS (2024-06, 2024-08, 2024-11)"
sqlite3 "$DB_PATH" "INSERT OR REPLACE INTO monthly_cash_bank_balance (user_id, year_month, bank_amount, cash_amount, previous_bank_amount, previous_cash_amount, balance_bank_amount, balance_cash_amount, bill_bank_amount, bill_cash_amount, expense_bank_amount, expense_cash_amount, income_bank_amount, income_cash_amount, total_balance, total_previous_balance) VALUES 
('$USER_ID', '2024-06', 1000.0, 500.0, 0.0, 0.0, 1000.0, 500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0, 1500.0, 0.0),
('$USER_ID', '2024-08', 1200.0, 600.0, 1000.0, 500.0, 1200.0, 600.0, 0.0, 0.0, 0.0, 0.0, 1200.0, 600.0, 1800.0, 1500.0),
('$USER_ID', '2024-11', 1500.0, 750.0, 1200.0, 600.0, 1500.0, 750.0, 0.0, 0.0, 0.0, 0.0, 1500.0, 750.0, 2250.0, 1800.0);"

print_info "Initial database setup with GAP MONTHS completed"

# Check initial state
echo -e "\n${BLUE}=== INITIAL STATE (GAP MONTHS) ===${NC}"
check_database_coherence_extended "0" "Initial Setup with Gap Months"

# SCENARIO 1: Create a bill that spans ACROSS the gap months
echo -e "\n${BLUE}=== CREATING BILL SPANNING GAP MONTHS ===${NC}"
print_info "Creating bill from 2024-07 for 4 months (covers gap months 2024-07, 2024-09, 2024-10)"
bill_id=$(create_test_bill 125.5 "2024-07-01" 4 "bank" 15 "Utilities")
print_info "Created bill with ID: $bill_id spanning gap months"
check_database_coherence_extended "$bill_id" "After Bill Creation (Spanning Gap Months)"

echo -e "\n${GREEN}=== ANALYSIS COMPLETE ===${NC}"
echo -e "${BLUE}Key Points to Analyze:${NC}"
echo -e "${YELLOW}1. Did the system create records for missing months (2024-07, 2024-09, 2024-10)?${NC}"
echo -e "${YELLOW}2. Are bill_bank_amounts properly registered in ALL bill months?${NC}"
echo -e "${YELLOW}3. How are previous_amounts calculated for gap months?${NC}"
echo -e "${YELLOW}4. Are final balances (bank_amount, cash_amount) propagated correctly through gaps?${NC}"
echo -e "${YELLOW}5. Is the cascade/cumulative logic working properly across gap months?${NC}"

print_info "This test demonstrates the current gap month handling behavior"