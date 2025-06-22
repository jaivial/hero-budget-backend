#!/bin/bash

# Test completo de los 21 escenarios de actualización
BASE_URL="http://localhost:8091"
USER_ID="test_user"

echo "=== EJECUTANDO LOS 21 ESCENARIOS DE ACTUALIZACIÓN ==="

# Función para limpiar y resetear
cleanup_and_reset() {
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "DELETE FROM bill_payments WHERE bill_id IN (SELECT id FROM bills WHERE user_id='$USER_ID');
         DELETE FROM bills WHERE user_id='$USER_ID';" 2>/dev/null
    sqlite3 '/Users/usuario/Documents/PROYECTOS/herobudgetflutter/hero_budget/backend/google_auth/users.db' \
        "UPDATE monthly_cash_bank_balance SET 
            bill_bank_amount = 0, bill_cash_amount = 0,
            expense_bank_amount = 0, expense_cash_amount = 0,
            bank_amount = 1000, cash_amount = 500,
            balance_bank_amount = 1000, balance_cash_amount = 500,
            previous_bank_amount = 0, previous_cash_amount = 0,
            total_previous_balance = 0, total_balance = 1500
         WHERE user_id='$USER_ID';" 2>/dev/null
}

# Función para crear bill
create_bill() {
    local amount=$1
    local start_date=$2
    local duration=$3
    local payment_method=$4
    local payment_day=${5:-15}
    
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
        }" 2>/dev/null)
    
    echo $response | jq -r '.data.id' 2>/dev/null
}

# Función para actualizar bill
update_bill() {
    local bill_id=$1
    local json_data=$2
    
    response=$(curl -s -X POST "$BASE_URL/bills/update-cash-bank" \
        -H "Content-Type: application/json" \
        -d "$json_data" 2>/dev/null)
    
    if echo $response | jq -r '.success' 2>/dev/null | grep -q "true"; then
        echo "✅"
    else
        echo "❌"
    fi
}

cleanup_and_reset

echo "Ejecutando tests..."
echo "==================="

# TEST 1: Cambiar cantidad solamente
echo -n "[1] Cantidad 100->150: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":150.0}")
echo "$result"
cleanup_and_reset

# TEST 2: Cambiar duración solamente  
echo -n "[2] Duración 3->6 meses: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"duration_months\":6}")
echo "$result"
cleanup_and_reset

# TEST 3: Cambiar mes de inicio solamente
echo -n "[3] Mes inicio 2025-01->2025-02: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"start_date\":\"2025-02\"}")
echo "$result"
cleanup_and_reset

# TEST 4: Cambiar día de pago solamente
echo -n "[4] Día pago 15->25: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":25}")
echo "$result"
cleanup_and_reset

# TEST 5: Cambiar método de pago solamente
echo -n "[5] Método bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 6: Cambiar cantidad + duración
echo -n "[6] Cantidad+duración 100->120, 3->5: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":120.0,\"duration_months\":5}")
echo "$result"
cleanup_and_reset

# TEST 7: Cambiar cantidad + mes de inicio
echo -n "[7] Cantidad+inicio 100->110, 2025-01->2025-02: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":110.0,\"start_date\":\"2025-02\"}")
echo "$result"
cleanup_and_reset

# TEST 8: Cambiar cantidad + día de pago
echo -n "[8] Cantidad+día 100->130, 15->20: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":130.0,\"payment_day\":20}")
echo "$result"
cleanup_and_reset

# TEST 9: Cambiar cantidad + método de pago
echo -n "[9] Cantidad+método 100->140, bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":140.0,\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 10: Cambiar cantidad + duración + mes de inicio
echo -n "[10] Cantidad+duración+inicio 100->160, 3->4, 2025-01->2025-03: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":160.0,\"duration_months\":4,\"start_date\":\"2025-03\"}")
echo "$result"
cleanup_and_reset

# TEST 11: Cambiar cantidad + duración + método
echo -n "[11] Cantidad+duración+método 100->170, 3->6, bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":170.0,\"duration_months\":6,\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 12: Cambiar cantidad + duración + día
echo -n "[12] Cantidad+duración+día 100->180, 3->5, 15->28: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":180.0,\"duration_months\":5,\"payment_day\":28}")
echo "$result"
cleanup_and_reset

# TEST 13: Cambiar cantidad + inicio + día
echo -n "[13] Cantidad+inicio+día 100->190, 2025-01->2025-02, 15->10: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":190.0,\"start_date\":\"2025-02\",\"payment_day\":10}")
echo "$result"
cleanup_and_reset

# TEST 14: Cambiar cantidad + inicio + método
echo -n "[14] Cantidad+inicio+método 100->200, 2025-01->2025-02, bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":200.0,\"start_date\":\"2025-02\",\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 15: Cambiar cantidad + duración + inicio + método
echo -n "[15] Cantidad+duración+inicio+método 100->250, 3->2, 2025-01->2025-03, bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank")
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":250.0,\"duration_months\":2,\"start_date\":\"2025-03\",\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 16: Cambiar todos los parámetros
echo -n "[16] Todos los parámetros 100->300, 3->4, 2025-01->2025-02, bank->cash, 15->25: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"amount\":300.0,\"duration_months\":4,\"start_date\":\"2025-02\",\"payment_method\":\"cash\",\"payment_day\":25}")
echo "$result"
cleanup_and_reset

# TEST 17: Cambiar día + duración
echo -n "[17] Día+duración 15->5, 3->7: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":5,\"duration_months\":7}")
echo "$result"
cleanup_and_reset

# TEST 18: Cambiar día + inicio
echo -n "[18] Día+inicio 15->20, 2025-01->2025-03: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":20,\"start_date\":\"2025-03\"}")
echo "$result"
cleanup_and_reset

# TEST 19: Cambiar día + duración + inicio
echo -n "[19] Día+duración+inicio 15->12, 3->5, 2025-01->2025-02: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":12,\"duration_months\":5,\"start_date\":\"2025-02\"}")
echo "$result"
cleanup_and_reset

# TEST 20: Cambiar día + método
echo -n "[20] Día+método 15->8, bank->cash: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":8,\"payment_method\":\"cash\"}")
echo "$result"
cleanup_and_reset

# TEST 21: Cambiar día + método + duración + inicio
echo -n "[21] Día+método+duración+inicio 15->3, bank->cash, 3->8, 2025-01->2025-02: "
bill_id=$(create_bill 100.0 "2025-01" 3 "bank" 15)
result=$(update_bill $bill_id "{\"user_id\":\"$USER_ID\",\"bill_id\":$bill_id,\"payment_day\":3,\"payment_method\":\"cash\",\"duration_months\":8,\"start_date\":\"2025-02\"}")
echo "$result"
cleanup_and_reset

echo "==================="
echo "Tests completados"

# Contar resultados
total_tests=21
success_count=$(grep -c "✅" <<< "$(cat /tmp/test_results 2>/dev/null || echo '')")
echo "Resumen: $success_count/$total_tests tests exitosos"

cleanup_and_reset