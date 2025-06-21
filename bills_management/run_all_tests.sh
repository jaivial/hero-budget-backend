#!/bin/bash

run_test() {
    local test_num=$1
    local description="$2"
    local update_json="$3"
    
    echo ""
    echo "==================== TEST $test_num ===================="
    echo "📋 $description"
    
    # Reset data
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    DELETE FROM bills WHERE user_id="13";
    DELETE FROM incomes WHERE user_id="13";
    DELETE FROM expenses WHERE user_id="13";
    DELETE FROM monthly_balance WHERE user_id="13";
    DELETE FROM bill_payments WHERE bill_id IN (SELECT id FROM bills WHERE user_id="13");
    
    INSERT INTO incomes (user_id, amount, date, category, payment_method, description) VALUES 
    ("13", 1000, "2025-06-15", "Trabajo", "bank", "Income 1000 EUR bank"),
    ("13", 100, "2025-06-20", "Trabajo", "bank", "Income 100 EUR bank");
    INSERT INTO expenses (user_id, amount, date, category, payment_method, description) VALUES 
    ("13", 25, "2025-06-10", "Comida", "cash", "Expense 25 EUR cash");
    '
    
    # Create bill
    RESPONSE=$(curl -s -X POST "http://localhost:8091/bills/add" -H "Content-Type: application/json" -d '{
      "user_id": "13", "name": "Test Bill", "amount": 100, "start_date": "2025-04-01",
      "payment_day": 15, "duration_months": 4, "regularity": "monthly",
      "category": "Test", "icon": "📝", "payment_method": "bank"
    }')
    
    BILL_ID=$(echo $RESPONSE | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
    
    # Apply June data
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
    UPDATE monthly_balance SET 
      cash_amount = cash_amount - 25, bank_amount = bank_amount + 1100,
      total_balance = (cash_amount - 25) + (bank_amount + 1100)
    WHERE user_id="13" AND year_month="2025-06";
    '
    
    # Execute update
    UPDATE_CMD=$(echo "$update_json" | sed "s/BILL_ID/$BILL_ID/g")
    RESPONSE=$(curl -s -X POST "http://localhost:8091/bills/update" -H "Content-Type: application/json" -d "$UPDATE_CMD")
    
    if echo "$RESPONSE" | grep -q '"success":true'; then
        echo "✅ PASSED"
        sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
        SELECT "  " || year_month || ": TotalBal=" || total_balance || ", TotalPrev=" || total_previous_balance
        FROM monthly_balance WHERE user_id="13" ORDER BY year_month;
        '
    else
        echo "❌ FAILED: $RESPONSE"
    fi
}

echo "🧪 Bills Management - Comprehensive Test Suite"
echo "=============================================="

# Basic single-field changes
run_test 1 "Change amount only (100→50)" '{"user_id":"13","bill_id":BILL_ID,"amount":50}'
run_test 2 "Change duration only (4→6 months)" '{"user_id":"13","bill_id":BILL_ID,"duration_months":6}'
run_test 3 "Change start month only (April→May)" '{"user_id":"13","bill_id":BILL_ID,"start_date":"2025-05-01"}'
run_test 4 "Change payment day only (15→20)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":20}'
run_test 5 "Change payment method only (bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"payment_method":"cash"}'

# Dual field combinations
run_test 6 "Change amount + duration (100→30, 4→2)" '{"user_id":"13","bill_id":BILL_ID,"amount":30,"duration_months":2}'
run_test 7 "Change amount + start month (100→75, Apr→Jun)" '{"user_id":"13","bill_id":BILL_ID,"amount":75,"start_date":"2025-06-01"}'
run_test 8 "Change amount + payment day (100→40, 15→25)" '{"user_id":"13","bill_id":BILL_ID,"amount":40,"payment_day":25}'
run_test 9 "Change amount + payment method (100→80, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"amount":80,"payment_method":"cash"}'

# Triple field combinations
run_test 10 "Change amount + duration + start month (100→60, 4→3, Apr→May)" '{"user_id":"13","bill_id":BILL_ID,"amount":60,"duration_months":3,"start_date":"2025-05-01"}'
run_test 11 "Change amount + duration + method (100→45, 4→5, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"amount":45,"duration_months":5,"payment_method":"cash"}'
run_test 12 "Change amount + duration + payment day (100→35, 4→2, 15→10)" '{"user_id":"13","bill_id":BILL_ID,"amount":35,"duration_months":2,"payment_day":10}'
run_test 13 "Change amount + start month + payment day (100→90, Apr→Jul, 15→5)" '{"user_id":"13","bill_id":BILL_ID,"amount":90,"start_date":"2025-07-01","payment_day":5}'
run_test 14 "Change amount + start month + method (100→65, Apr→May, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"amount":65,"start_date":"2025-05-01","payment_method":"cash"}'

# Quadruple and quintuple combinations
run_test 15 "Change amount + duration + start + method (100→55, 4→3, Apr→Jun, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"amount":55,"duration_months":3,"start_date":"2025-06-01","payment_method":"cash"}'
run_test 16 "Change ALL fields (100→25, 4→2, Apr→Aug, 15→1, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"amount":25,"duration_months":2,"start_date":"2025-08-01","payment_day":1,"payment_method":"cash"}'

# Complex non-amount changes
run_test 17 "Change payment day + duration (15→30, 4→6)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":30,"duration_months":6}'
run_test 18 "Change payment day + start month (15→7, Apr→Mar)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":7,"start_date":"2025-03-01"}'
run_test 19 "Change payment day + duration + start (15→12, 4→8, Apr→Feb)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":12,"duration_months":8,"start_date":"2025-02-01"}'
run_test 20 "Change payment day + method (15→28, bank→cash)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":28,"payment_method":"cash"}'
run_test 21 "Change payment day + method + duration + start (15→3, bank→cash, 4→1, Apr→Sep)" '{"user_id":"13","bill_id":BILL_ID,"payment_day":3,"payment_method":"cash","duration_months":1,"start_date":"2025-09-01"}'

echo ""
echo "🎉 All 21 test scenarios completed!"
echo "Check results above for each test case."