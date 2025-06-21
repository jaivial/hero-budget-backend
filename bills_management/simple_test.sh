#!/bin/bash

echo "🧪 Testing Bills Update Scenarios"
echo "================================="

# Reset test data
echo "🧹 Resetting test data..."
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

# Create initial bill
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

BILL_ID=$(echo $RESPONSE | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
echo "Created bill with ID: $BILL_ID"

# Apply June income/expense manually
sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
UPDATE monthly_balance SET 
  cash_amount = cash_amount - 25, 
  bank_amount = bank_amount + 1100,
  total_balance = (cash_amount - 25) + (bank_amount + 1100)
WHERE user_id="13" AND year_month="2025-06";
'

echo "📊 BEFORE UPDATE:"
sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
SELECT "Month: " || year_month || 
       " | Cash: " || cash_amount || 
       " | Bank: " || bank_amount || 
       " | TotalPrev: " || total_previous_balance || 
       " | TotalBal: " || total_balance
FROM monthly_balance 
WHERE user_id="13" 
ORDER BY year_month;
'

echo ""
echo "🚀 TEST 1: Change amount only (100 → 50)"
RESPONSE=$(curl -s -X POST "http://localhost:8091/bills/update" -H "Content-Type: application/json" -d "{
  \"user_id\": \"13\",
  \"bill_id\": $BILL_ID,
  \"amount\": 50
}")

echo "📡 Response: $RESPONSE"

echo "📊 AFTER UPDATE:"
sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' '
SELECT "Month: " || year_month || 
       " | Cash: " || cash_amount || 
       " | Bank: " || bank_amount || 
       " | TotalPrev: " || total_previous_balance || 
       " | TotalBal: " || total_balance
FROM monthly_balance 
WHERE user_id="13" 
ORDER BY year_month;
'

echo ""
echo "✅ Test 1 completed"