#!/bin/bash

# Helper function to reset test data
reset_test_data() {
    echo "🧹 Resetting test data..."
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    DELETE FROM bills WHERE user_id="13";
    DELETE FROM incomes WHERE user_id="13";
    DELETE FROM expenses WHERE user_id="13";
    DELETE FROM monthly_balance WHERE user_id="13";
    DELETE FROM bill_payments WHERE bill_id IN (SELECT id FROM bills WHERE user_id="13");
    '
    
    # Add base income and expense data
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    INSERT INTO incomes (user_id, amount, date, category, payment_method, description) VALUES 
    ("13", 1000, "2025-06-15", "Trabajo", "bank", "Income 1000 EUR bank"),
    ("13", 100, "2025-06-20", "Trabajo", "bank", "Income 100 EUR bank");
    INSERT INTO expenses (user_id, amount, date, category, payment_method, description) VALUES 
    ("13", 25, "2025-06-10", "Comida", "cash", "Expense 25 EUR cash");
    '
}

# Helper function to create initial bill
create_initial_bill() {
    echo "📝 Creating initial bill..."
    RESPONSE=$(curl -s -X POST "http://localhost:8091/bills/add" -H "Content-Type: application/json" -d '{
      "user_id": "13",
      "name": "Test Bill",
      "amount": 100,
      "start_date": "2025-04-01",
      "payment_day": 15,
      "duration_months": 4,
      "regularity": "monthly",
      "category": "Test",
      "icon": "📝",
      "payment_method": "bank"
    }')
    
    BILL_ID=$(echo $RESPONSE | grep -o '"id":[0-9]*' | cut -d':' -f2)
    echo "Created bill with ID: $BILL_ID"
    
    # Apply June income/expense manually
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    UPDATE monthly_balance SET 
      cash_amount = cash_amount - 25, 
      bank_amount = bank_amount + 1100,
      total_balance = (cash_amount - 25) + (bank_amount + 1100)
    WHERE user_id="13" AND year_month="2025-06";
    '
    
    echo $BILL_ID
}

# Helper function to check balance state
check_balance_state() {
    echo "📊 Current balance state:"
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    SELECT "Month: " || year_month || 
           " | Cash: " || cash_amount || 
           " | Bank: " || bank_amount || 
           " | PrevCash: " || previous_cash_amount || 
           " | PrevBank: " || previous_bank_amount || 
           " | TotalPrev: " || total_previous_balance || 
           " | TotalBal: " || total_balance
    FROM monthly_balance 
    WHERE user_id="13" 
    ORDER BY year_month;
    '
}

# Helper function to run test scenario
run_test_scenario() {
    local test_num=$1
    local description="$2"
    local update_json="$3"
    
    echo ""
    echo "==================== TEST $test_num ===================="
    echo "📋 Description: $description"
    echo "🔧 Update JSON: $update_json"
    
    # Reset and create initial state
    reset_test_data
    BILL_ID=$(create_initial_bill)
    
    echo "📊 BEFORE UPDATE:"
    check_balance_state
    
    # Apply update with dynamic bill_id
    UPDATE_WITH_ID=$(echo "$update_json" | sed "s/BILL_ID/$BILL_ID/g")
    echo "🚀 Executing update..."
    RESPONSE=$(curl -s -X POST "http://localhost:8091/bills/update" -H "Content-Type: application/json" -d "$UPDATE_WITH_ID")
    
    echo "📡 Update response: $RESPONSE"
    
    echo "📊 AFTER UPDATE:"
    check_balance_state
    
    # Validate response
    if echo "$RESPONSE" | grep -q '"success":true'; then
        echo "✅ Test $test_num PASSED"
    else
        echo "❌ Test $test_num FAILED"
        echo "Error details: $RESPONSE"
    fi
    
    echo "================================================="
    sleep 1
}

echo "🧪 Starting Bills Management Test Suite (21 scenarios)"
echo "======================================================="

# Test 1: Change amount only
run_test_scenario 1 "Change amount only (100 → 50)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 50}'

# Test 2: Change duration only
run_test_scenario 2 "Change duration only (4 → 6 months)" '{"user_id": "13", "bill_id": BILL_ID, "duration_months": 6}'

# Test 3: Change start month only
run_test_scenario 3 "Change start month only (April → May)" '{"user_id": "13", "bill_id": BILL_ID, "start_date": "2025-05-01"}'

# Test 4: Change payment day only
run_test_scenario 4 "Change payment day only (15 → 20)" '{"user_id": "13", "bill_id": BILL_ID, "payment_day": 20}'

# Test 5: Change payment method only
run_test_scenario 5 "Change payment method only (bank → cash)" '{"user_id": "13", "bill_id": BILL_ID, "payment_method": "cash"}'

# Test 6: Change amount + duration
run_test_scenario 6 "Change amount + duration (100→30, 4→2 months)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 30, "duration_months": 2}'

# Test 7: Change amount + start month
run_test_scenario 7 "Change amount + start month (100→75, April→June)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 75, "start_date": "2025-06-01"}'

# Test 8: Change amount + payment day
run_test_scenario 8 "Change amount + payment day (100→40, 15→25)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 40, "payment_day": 25}'

# Test 9: Change amount + payment method
run_test_scenario 9 "Change amount + payment method (100→80, bank→cash)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 80, "payment_method": "cash"}'

# Test 10: Change amount + duration + start month
run_test_scenario 10 "Change amount + duration + start month (100→60, 4→3, April→May)" '{"user_id": "13", "bill_id": BILL_ID, "amount": 60, "duration_months": 3, "start_date": "2025-05-01"}'

echo "🎉 Test suite completed! Check individual results above."