#!/bin/bash

# Test rápido de los primeros 5 escenarios
BASE_URL="http://localhost:8091"
USER_ID="test_user"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Función para imprimir test
print_test() {
    echo -e "\n${YELLOW}TEST $1: $2${NC}"
    echo "-----------------------------------------------"
}

# Función para verificar balance
check_balance() {
    local month=$1
    echo -e "${GREEN}Balance para $month:${NC}"
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "SELECT 'bill_bank: ' || bill_bank_amount || ', bill_cash: ' || bill_cash_amount || 
                ', bank: ' || bank_amount || ', cash: ' || cash_amount || 
                ', total_balance: ' || total_balance 
         FROM monthly_cash_bank_balance 
         WHERE user_id='$USER_ID' AND year_month='$month';"
}

# Función para crear factura inicial
create_initial_bill() {
    local amount=$1
    local start_date=$2
    local duration=$3
    local payment_method=$4
    local payment_day=${5:-15}
    
    echo "Creando factura: amount=$amount, start=$start_date, duration=$duration, method=$payment_method"
    
    response=$(curl -s -X POST "$BASE_URL/bills/add-cash-bank" \
        -H "Content-Type: application/json" \
        -d "{
            \"user_id\":\"$USER_ID\",
            \"name\":\"Test Bill\",
            \"amount\":$amount,
            \"start_date\":\"$start_date\",
            \"payment_day\":$payment_day,
            \"duration_months\":$duration,
            \"payment_method\":\"$payment_method\"
        }")
    
    bill_id=$(echo $response | jq -r '.data.id')
    echo "Bill ID: $bill_id"
    echo "$bill_id"
}

# Función para actualizar factura
update_bill_cash_bank() {
    local bill_id=$1
    local json_data=$2
    
    echo "Actualizando bill $bill_id"
    
    response=$(curl -s -X POST "$BASE_URL/bills/update-cash-bank" \
        -H "Content-Type: application/json" \
        -d "$json_data")
    
    if echo $response | jq -r '.success' | grep -q "true"; then
        echo -e "${GREEN}✅ Actualización exitosa${NC}"
        return 0
    else
        echo -e "${RED}❌ Error: $(echo $response | jq -r '.message')${NC}"
        return 1
    fi
}

# Función para limpiar base de datos
cleanup_bills() {
    echo "Limpiando..."
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "DELETE FROM bill_payments WHERE bill_id IN (SELECT id FROM bills WHERE user_id='$USER_ID');
         DELETE FROM bills WHERE user_id='$USER_ID';"
}

# Función para resetear balances
reset_test_balances() {
    echo "Reseteando balances..."
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "UPDATE monthly_cash_bank_balance SET 
            bill_bank_amount = 0, bill_cash_amount = 0,
            expense_bank_amount = 0, expense_cash_amount = 0,
            bank_amount = 1000, cash_amount = 500,
            balance_bank_amount = 1000, balance_cash_amount = 500,
            previous_bank_amount = 0, previous_cash_amount = 0,
            total_previous_balance = 0, total_balance = 1500
         WHERE user_id='$USER_ID';"
}

echo -e "${BLUE}=== TESTS RÁPIDOS DE ACTUALIZACIÓN ===${NC}"

# Preparación inicial
cleanup_bills
reset_test_balances

# TEST 1: Cambiar la cantidad solamente
print_test "1" "Cambiar la cantidad solamente (100 -> 150)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":150.0}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 2: Cambiar duración de meses
print_test "2" "Cambiar duración (3 -> 6 meses)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-04"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"duration_months\":6}"
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-06"
cleanup_bills
reset_test_balances

# TEST 3: Cambiar mes de inicio
print_test "3" "Cambiar mes de inicio (2025-01 -> 2025-02)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"start_date\":\"2025-02\"}"
check_balance "2025-01"
check_balance "2025-02"
cleanup_bills
reset_test_balances

# TEST 4: Cambiar método de pago
print_test "4" "Cambiar método de pago (bank -> cash)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_method\":\"cash\"}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 5: Cambiar cantidad + duración
print_test "5" "Cambiar cantidad + duración (100->120, 3->5 meses)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-05"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":120.0,\"duration_months\":5}"
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-05"
cleanup_bills
reset_test_balances

echo -e "\n${GREEN}✅ Tests rápidos completados${NC}"

# Limpieza final
cleanup_bills
reset_test_balances