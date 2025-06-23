#!/bin/bash

# 25 Bill Update Test Scenarios
# Tests the /bills/update endpoint with comprehensive scenarios
# Tests database coherence in monthly_cash_bank_balance, bills, bill_payments, expenses

# Configuration
BASE_URL="http://localhost:8091"
USER_ID="test_user_25_scenarios"
DB_PATH="/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TEST_NUM=1

print_test_header() {
    echo -e "\n${BLUE}=== SCENARIO $TEST_NUM: $1 ===${NC}"
    ((TEST_NUM++))
}

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
    echo -e "\nMonthly Cash Bank Balance (last 6 months):"
    sqlite3 "$DB_PATH" "SELECT year_month, bank_amount, cash_amount, previous_bank_amount, previous_cash_amount, balance_bank_amount, balance_cash_amount, bill_bank_amount, bill_cash_amount FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID' ORDER BY year_month DESC LIMIT 6;"
    
    # Check bill_payments table
    echo -e "\nBill Payments:"
    sqlite3 "$DB_PATH" "SELECT bill_id, year_month, payment_date, amount, payment_method, paid FROM bill_payments WHERE bill_id = $bill_id ORDER BY year_month;"
    
    # Check expenses table
    echo -e "\nExpenses (related to this bill):"
    sqlite3 "$DB_PATH" "SELECT id, amount, date, payment_method, category FROM expenses WHERE user_id = '$USER_ID' AND category IN (SELECT category FROM bills WHERE id = $bill_id) ORDER BY date DESC LIMIT 5;"
    
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

# Function to pay a bill
pay_bill() {
    local bill_id=$1
    local year_month=$2
    local payment_method=$3
    local payment_date=$4
    
    curl -s -X POST "$BASE_URL/bills/pay" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\": \"$USER_ID\",
            \"bill_id\": $bill_id,
            \"year_month\": \"$year_month\",
            \"payment_method\": \"$payment_method\",
            \"payment_date\": \"$payment_date\"
        }"
}

echo -e "${BLUE}Starting 25 Bill Update Test Scenarios${NC}"
echo -e "${BLUE}User ID: $USER_ID${NC}"
echo -e "${BLUE}Database: $DB_PATH${NC}"

# Clean up any existing test data
sqlite3 "$DB_PATH" "DELETE FROM bills WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM bill_payments WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM expenses WHERE user_id = '$USER_ID';"
sqlite3 "$DB_PATH" "DELETE FROM monthly_cash_bank_balance WHERE user_id = '$USER_ID';"

# Initialize monthly_cash_bank_balance with base data INCLUDING INCOME AMOUNTS
sqlite3 "$DB_PATH" "INSERT OR REPLACE INTO monthly_cash_bank_balance (user_id, year_month, bank_amount, cash_amount, previous_bank_amount, previous_cash_amount, balance_bank_amount, balance_cash_amount, bill_bank_amount, bill_cash_amount, expense_bank_amount, expense_cash_amount, income_bank_amount, income_cash_amount) VALUES 
('$USER_ID', '2024-01', 1000.0, 500.0, 0.0, 0.0, 1000.0, 500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-02', 1000.0, 500.0, 1000.0, 500.0, 2000.0, 1000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-03', 1000.0, 500.0, 2000.0, 1000.0, 3000.0, 1500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-04', 1000.0, 500.0, 3000.0, 1500.0, 4000.0, 2000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-05', 1000.0, 500.0, 4000.0, 2000.0, 5000.0, 2500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-06', 1000.0, 500.0, 5000.0, 2500.0, 6000.0, 3000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-07', 1000.0, 500.0, 6000.0, 3000.0, 7000.0, 3500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-08', 1000.0, 500.0, 7000.0, 3500.0, 8000.0, 4000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-09', 1000.0, 500.0, 8000.0, 4000.0, 9000.0, 4500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-10', 1000.0, 500.0, 9000.0, 4500.0, 10000.0, 5000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-11', 1000.0, 500.0, 10000.0, 5000.0, 11000.0, 5500.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0),
('$USER_ID', '2024-12', 1000.0, 500.0, 11000.0, 5500.0, 12000.0, 6000.0, 0.0, 0.0, 0.0, 0.0, 1000.0, 500.0);"

print_info "Initial database setup completed"

# SCENARIO 1: Basic amount increase
print_test_header "Basic amount increase (100 -> 150)"
bill_id=$(create_test_bill 100.0 "2024-06-01" 6 "bank" 15 "Utilities")
sleep 1
update_bill "$bill_id" 150.0 "2024-06-01" 6 "bank" 15 "Utilities"
check_database_coherence "$bill_id" "Amount increase"

# SCENARIO 2: Basic amount decrease
print_test_header "Basic amount decrease (200 -> 120)"
bill_id=$(create_test_bill 200.0 "2024-06-01" 6 "cash" 10 "Internet")
sleep 1
update_bill "$bill_id" 120.0 "2024-06-01" 6 "cash" 10 "Internet"
check_database_coherence "$bill_id" "Amount decrease"

# SCENARIO 3: Payment method change (bank to cash)
print_test_header "Payment method change (bank to cash)"
bill_id=$(create_test_bill 80.0 "2024-05-01" 4 "bank" 5 "Phone")
sleep 1
update_bill "$bill_id" 80.0 "2024-05-01" 4 "cash" 5 "Phone"
check_database_coherence "$bill_id" "Payment method change"

# SCENARIO 4: Payment method change (cash to bank)
print_test_header "Payment method change (cash to bank)"
bill_id=$(create_test_bill 90.0 "2024-05-01" 4 "cash" 20 "Subscription")
sleep 1
update_bill "$bill_id" 90.0 "2024-05-01" 4 "bank" 20 "Subscription"
check_database_coherence "$bill_id" "Payment method change"

# SCENARIO 5: Duration extension
print_test_header "Duration extension (3 months -> 8 months)"
bill_id=$(create_test_bill 60.0 "2024-07-01" 3 "bank" 1 "Gym")
sleep 1
update_bill "$bill_id" 60.0 "2024-07-01" 8 "bank" 1 "Gym"
check_database_coherence "$bill_id" "Duration extension"

# SCENARIO 6: Duration reduction
print_test_header "Duration reduction (12 months -> 6 months)"
bill_id=$(create_test_bill 110.0 "2024-04-01" 12 "cash" 25 "Insurance")
sleep 1
update_bill "$bill_id" 110.0 "2024-04-01" 6 "cash" 25 "Insurance"
check_database_coherence "$bill_id" "Duration reduction"

# SCENARIO 7: Start date moved forward
print_test_header "Start date moved forward (2024-05-01 -> 2024-07-01)"
bill_id=$(create_test_bill 75.0 "2024-05-01" 6 "bank" 12 "Cable TV")
sleep 1
update_bill "$bill_id" 75.0 "2024-07-01" 6 "bank" 12 "Cable TV"
check_database_coherence "$bill_id" "Start date forward"

# SCENARIO 8: Start date moved backward
print_test_header "Start date moved backward (2024-08-01 -> 2024-06-01)"
bill_id=$(create_test_bill 95.0 "2024-08-01" 5 "cash" 30 "Streaming")
sleep 1
update_bill "$bill_id" 95.0 "2024-06-01" 5 "cash" 30 "Streaming"
check_database_coherence "$bill_id" "Start date backward"

# SCENARIO 9: Payment day change
print_test_header "Payment day change (15 -> 28)"
bill_id=$(create_test_bill 55.0 "2024-06-01" 4 "bank" 15 "Software")
sleep 1
update_bill "$bill_id" 55.0 "2024-06-01" 4 "bank" 28 "Software"
check_database_coherence "$bill_id" "Payment day change"

# SCENARIO 10: Multiple changes (amount + payment method + duration)
print_test_header "Multiple changes (amount + payment method + duration)"
bill_id=$(create_test_bill 120.0 "2024-05-01" 6 "cash" 10 "Rent")
sleep 1
update_bill "$bill_id" 180.0 "2024-05-01" 10 "bank" 10 "Rent"
check_database_coherence "$bill_id" "Multiple changes"

# SCENARIO 11: Update with some payments already made
print_test_header "Update with some payments already made"
bill_id=$(create_test_bill 100.0 "2024-04-01" 8 "bank" 15 "Electric")
sleep 1
pay_bill "$bill_id" "2024-04" "bank" "2024-04-15"
pay_bill "$bill_id" "2024-05" "bank" "2024-05-15"
sleep 1
update_bill "$bill_id" 130.0 "2024-04-01" 8 "bank" 15 "Electric"
check_database_coherence "$bill_id" "Update with payments made"

# SCENARIO 12: Update changing payment method with existing payments
print_test_header "Update changing payment method with existing payments"
bill_id=$(create_test_bill 85.0 "2024-05-01" 6 "cash" 20 "Water")
sleep 1
pay_bill "$bill_id" "2024-05" "cash" "2024-05-20"
sleep 1
update_bill "$bill_id" 85.0 "2024-05-01" 6 "bank" 20 "Water"
check_database_coherence "$bill_id" "Payment method change with existing payments"

# SCENARIO 13: Extreme amount increase
print_test_header "Extreme amount increase (50 -> 500)"
bill_id=$(create_test_bill 50.0 "2024-06-01" 4 "bank" 5 "Gas")
sleep 1
update_bill "$bill_id" 500.0 "2024-06-01" 4 "bank" 5 "Gas"
check_database_coherence "$bill_id" "Extreme amount increase"

# SCENARIO 14: Extreme amount decrease
print_test_header "Extreme amount decrease (300 -> 30)"
bill_id=$(create_test_bill 300.0 "2024-07-01" 5 "cash" 1 "Consulting")
sleep 1
update_bill "$bill_id" 30.0 "2024-07-01" 5 "cash" 1 "Consulting"
check_database_coherence "$bill_id" "Extreme amount decrease"

# SCENARIO 15: Update with end date in the past
print_test_header "Update with end date in the past"
bill_id=$(create_test_bill 70.0 "2024-01-01" 12 "bank" 10 "OldBill")
sleep 1
update_bill "$bill_id" 70.0 "2024-01-01" 3 "bank" 10 "OldBill"
check_database_coherence "$bill_id" "End date in past"

# SCENARIO 16: Update extending far into future
print_test_header "Update extending far into future"
bill_id=$(create_test_bill 40.0 "2024-08-01" 3 "cash" 15 "Future")
sleep 1
update_bill "$bill_id" 40.0 "2024-08-01" 24 "cash" 15 "Future"
check_database_coherence "$bill_id" "Extended into future"

# SCENARIO 17: Update with payment day at month end
print_test_header "Update with payment day at month end"
bill_id=$(create_test_bill 65.0 "2024-06-01" 4 "bank" 31 "MonthEnd")
sleep 1
update_bill "$bill_id" 65.0 "2024-06-01" 4 "bank" 28 "MonthEnd"
check_database_coherence "$bill_id" "Payment day month end"

# SCENARIO 18: Update with all payments made, then extend duration
print_test_header "Update with all payments made, then extend duration"
bill_id=$(create_test_bill 90.0 "2024-06-01" 3 "cash" 10 "AllPaid")
sleep 1
pay_bill "$bill_id" "2024-06" "cash" "2024-06-10"
pay_bill "$bill_id" "2024-07" "cash" "2024-07-10"
pay_bill "$bill_id" "2024-08" "cash" "2024-08-10"
sleep 1
update_bill "$bill_id" 90.0 "2024-06-01" 6 "cash" 10 "AllPaid"
check_database_coherence "$bill_id" "All paid then extended"

# SCENARIO 19: Update changing start date and payment method
print_test_header "Update changing start date and payment method"
bill_id=$(create_test_bill 105.0 "2024-05-01" 6 "bank" 15 "Complex")
sleep 1
update_bill "$bill_id" 105.0 "2024-07-01" 6 "cash" 15 "Complex"
check_database_coherence "$bill_id" "Start date and payment method change"

# SCENARIO 20: Update with fractional amounts
print_test_header "Update with fractional amounts"
bill_id=$(create_test_bill 99.99 "2024-06-01" 4 "bank" 20 "Fractional")
sleep 1
update_bill "$bill_id" 149.50 "2024-06-01" 4 "bank" 20 "Fractional"
check_database_coherence "$bill_id" "Fractional amounts"

# SCENARIO 21: Update reducing to 1 month duration
print_test_header "Update reducing to 1 month duration"
bill_id=$(create_test_bill 200.0 "2024-08-01" 8 "cash" 5 "OneMonth")
sleep 1
update_bill "$bill_id" 200.0 "2024-08-01" 1 "cash" 5 "OneMonth"
check_database_coherence "$bill_id" "One month duration"

# SCENARIO 22: Update with mid-month start date
print_test_header "Update with mid-month start date"
bill_id=$(create_test_bill 80.0 "2024-06-15" 5 "bank" 15 "MidMonth")
sleep 1
update_bill "$bill_id" 80.0 "2024-06-15" 5 "cash" 15 "MidMonth"
check_database_coherence "$bill_id" "Mid-month start"

# SCENARIO 23: Update with category change
print_test_header "Update with category change"
bill_id=$(create_test_bill 60.0 "2024-05-01" 6 "bank" 10 "OldCategory")
sleep 1
update_bill "$bill_id" 60.0 "2024-05-01" 6 "bank" 10 "NewCategory"
check_database_coherence "$bill_id" "Category change"

# SCENARIO 24: Sequential updates
print_test_header "Sequential updates (multiple changes)"
bill_id=$(create_test_bill 100.0 "2024-06-01" 6 "bank" 15 "Sequential")
sleep 1
update_bill "$bill_id" 120.0 "2024-06-01" 6 "bank" 15 "Sequential"
sleep 1
update_bill "$bill_id" 120.0 "2024-06-01" 8 "bank" 15 "Sequential"
sleep 1
update_bill "$bill_id" 120.0 "2024-06-01" 8 "cash" 15 "Sequential"
check_database_coherence "$bill_id" "Sequential updates"

# SCENARIO 25: Complex scenario with partial payments and multiple changes
print_test_header "Complex scenario with partial payments and multiple changes"
bill_id=$(create_test_bill 150.0 "2024-04-01" 10 "bank" 25 "Complex")
sleep 1
pay_bill "$bill_id" "2024-04" "bank" "2024-04-25"
pay_bill "$bill_id" "2024-06" "bank" "2024-06-25"
sleep 1
update_bill "$bill_id" 200.0 "2024-04-01" 12 "cash" 10 "Complex"
check_database_coherence "$bill_id" "Complex final scenario"

echo -e "\n${GREEN}=== All 25 scenarios completed ===${NC}"
echo -e "${BLUE}Check the database coherence results above to identify any logic errors${NC}"