#!/bin/bash

# Test de los 21 escenarios de actualización de facturas en monthly_cash_bank_balance
# Verificación completa de coherencia de balances

BASE_URL="http://localhost:8091"
USER_ID="test_user"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Función para imprimir headers
print_header() {
    echo -e "\n${BLUE}================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================================${NC}"
}

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
                ', balance_bank: ' || balance_bank_amount || ', balance_cash: ' || balance_cash_amount || 
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
    
    echo "Creando factura inicial: amount=$amount, start=$start_date, duration=$duration, method=$payment_method, day=$payment_day"
    
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
    echo "Bill ID creado: $bill_id"
    echo $bill_id
}

# Función para actualizar factura con endpoint cash-bank
update_bill_cash_bank() {
    local bill_id=$1
    local json_data=$2
    
    echo "Actualizando bill $bill_id con: $json_data"
    
    response=$(curl -s -X POST "$BASE_URL/bills/update-cash-bank" \
        -H "Content-Type: application/json" \
        -d "$json_data")
    
    echo "Respuesta: $response"
    
    if echo $response | jq -r '.success' | grep -q "true"; then
        echo -e "${GREEN}✅ Actualización exitosa${NC}"
        return 0
    else
        echo -e "${RED}❌ Error en actualización: $(echo $response | jq -r '.message')${NC}"
        return 1
    fi
}

# Función para limpiar base de datos
cleanup_bills() {
    echo "Limpiando facturas de test..."
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "DELETE FROM bill_payments WHERE bill_id IN (SELECT id FROM bills WHERE user_id='$USER_ID');
         DELETE FROM bills WHERE user_id='$USER_ID';"
}

# Función para resetear balances de test
reset_test_balances() {
    echo "Reseteando balances de test..."
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

print_header "INICIO DE TESTS DE ACTUALIZACIÓN DE FACTURAS"
echo "Usuario de prueba: $USER_ID"
echo "URL base: $BASE_URL"

# Preparación inicial
cleanup_bills
reset_test_balances

print_header "ESCENARIOS DE PRUEBA"

# TEST 1: Cambiar la cantidad solamente
print_test "1" "Cambiar la cantidad solamente (100 -> 150)"
bill_id=$(create_initial_bill 100.0 "2025-01" 3 "bank")
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":150.0}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 2: Cambiar la duración de meses solo
print_test "2" "Cambiar la duración de meses solo (3 -> 6 meses)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-03"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"duration_months\":6}"
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-06"
cleanup_bills
reset_test_balances

# TEST 3: Cambiar el mes de inicio solo
print_test "3" "Cambiar el mes de inicio solo (2025-01 -> 2025-02)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"start_date\":\"2025-02\"}"
check_balance "2025-01"
check_balance "2025-02"
check_balance "2025-04"
cleanup_bills
reset_test_balances

# TEST 4: Cambiar el día de pago solo
print_test "4" "Cambiar el día de pago solo (15 -> 25)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":25}"
cleanup_bills
reset_test_balances

# TEST 5: Cambiar el método de pago solo
print_test "5" "Cambiar el método de pago solo (bank -> cash)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_method\":\"cash\"}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 6: Cambiar cantidad + duración de meses
print_test "6" "Cambiar cantidad + duración de meses (100->120, 3->5 meses)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":120.0,\"duration_months\":5}"
check_balance "2025-01"
check_balance "2025-05"
cleanup_bills
reset_test_balances

# TEST 7: Cambiar cantidad + mes de inicio
print_test "7" "Cambiar cantidad + mes de inicio (100->110, 2025-01->2025-02)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":110.0,\"start_date\":\"2025-02\"}"
check_balance "2025-01"
check_balance "2025-02"
cleanup_bills
reset_test_balances

# TEST 8: Cambiar cantidad + día de pago
print_test "8" "Cambiar cantidad + día de pago (100->130, 15->20)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":130.0,\"payment_day\":20}"
cleanup_bills
reset_test_balances

# TEST 9: Cambiar cantidad + método de pago
print_test "9" "Cambiar cantidad + método de pago (100->140, bank->cash)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":140.0,\"payment_method\":\"cash\"}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 10: Cambiar cantidad + duración + mes de inicio
print_test "10" "Cambiar cantidad + duración + mes de inicio (100->160, 3->4, 2025-01->2025-03)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-03"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":160.0,\"duration_months\":4,\"start_date\":\"2025-03\"}"
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-06"
cleanup_bills
reset_test_balances

# TEST 11: Cambiar cantidad + duración + método de pago
print_test "11" "Cambiar cantidad + duración + método de pago (100->170, 3->6, bank->cash)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":170.0,\"duration_months\":6,\"payment_method\":\"cash\"}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 12: Cambiar cantidad + duración + día de pago
print_test "12" "Cambiar cantidad + duración + día de pago (100->180, 3->5, 15->28)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":180.0,\"duration_months\":5,\"payment_day\":28}"
cleanup_bills
reset_test_balances

# TEST 13: Cambiar cantidad + mes de inicio + día de pago
print_test "13" "Cambiar cantidad + mes de inicio + día de pago (100->190, 2025-01->2025-02, 15->10)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":190.0,\"start_date\":\"2025-02\",\"payment_day\":10}"
cleanup_bills
reset_test_balances

# TEST 14: Cambiar cantidad + mes de inicio + método de pago
print_test "14" "Cambiar cantidad + mes de inicio + método de pago (100->200, 2025-01->2025-02, bank->cash)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":200.0,\"start_date\":\"2025-02\",\"payment_method\":\"cash\"}"
check_balance "2025-01"
check_balance "2025-02"
cleanup_bills
reset_test_balances

# TEST 15: Cambiar cantidad + duración + mes de inicio + método de pago
print_test "15" "Cambiar cantidad + duración + mes de inicio + método de pago (100->250, 3->2, 2025-01->2025-03, bank->cash)"
create_initial_bill 100.0 "2025-01" 3 "bank"
bill_id=$?
check_balance "2025-01"
check_balance "2025-03"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":250.0,\"duration_months\":2,\"start_date\":\"2025-03\",\"payment_method\":\"cash\"}"
check_balance "2025-01"
check_balance "2025-03"
check_balance "2025-04"
cleanup_bills
reset_test_balances

# TEST 16: Cambiar todos los parámetros
print_test "16" "Cambiar cantidad + duración + mes de inicio + método de pago + día de pago (100->300, 3->4, 2025-01->2025-02, bank->cash, 15->25)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":300.0,\"duration_months\":4,\"start_date\":\"2025-02\",\"payment_method\":\"cash\",\"payment_day\":25}"
check_balance "2025-01"
check_balance "2025-02"
check_balance "2025-05"
cleanup_bills
reset_test_balances

# TEST 17: Cambiar día de pago + duración de meses
print_test "17" "Cambiar día de pago + duración de meses (15->5, 3->7)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":5,\"duration_months\":7}"
cleanup_bills
reset_test_balances

# TEST 18: Cambiar día de pago + mes de inicio
print_test "18" "Cambiar día de pago + mes de inicio (15->20, 2025-01->2025-03)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
check_balance "2025-01"
check_balance "2025-03"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":20,\"start_date\":\"2025-03\"}"
check_balance "2025-01"
check_balance "2025-03"
cleanup_bills
reset_test_balances

# TEST 19: Cambiar día de pago + duración + mes de inicio
print_test "19" "Cambiar día de pago + duración + mes de inicio (15->12, 3->5, 2025-01->2025-02)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":12,\"duration_months\":5,\"start_date\":\"2025-02\"}"
check_balance "2025-01"
check_balance "2025-02"
check_balance "2025-06"
cleanup_bills
reset_test_balances

# TEST 20: Cambiar día de pago + método de pago
print_test "20" "Cambiar día de pago + método de pago (15->8, bank->cash)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
check_balance "2025-01"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":8,\"payment_method\":\"cash\"}"
check_balance "2025-01"
cleanup_bills
reset_test_balances

# TEST 21: Cambiar día de pago + método de pago + duración + mes de inicio
print_test "21" "Cambiar día de pago + método de pago + duración + mes de inicio (15->3, bank->cash, 3->8, 2025-01->2025-02)"
create_initial_bill 100.0 "2025-01" 3 "bank" 15
bill_id=$?
check_balance "2025-01"
check_balance "2025-02"
update_bill_cash_bank $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":3,\"payment_method\":\"cash\",\"duration_months\":8,\"start_date\":\"2025-02\"}"
check_balance "2025-01"
check_balance "2025-02"
check_balance "2025-09"
cleanup_bills
reset_test_balances

print_header "TESTS COMPLETADOS"
echo -e "${GREEN}✅ Todos los tests de actualización han sido ejecutados${NC}"
echo -e "${BLUE}📊 Revisa los balances mostrados para verificar coherencia${NC}"

# Limpieza final
cleanup_bills
reset_test_balances

echo -e "\n${YELLOW}💡 Para verificar manualmente el estado final:${NC}"
echo "sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \"SELECT * FROM monthly_cash_bank_balance WHERE user_id='$USER_ID';\""