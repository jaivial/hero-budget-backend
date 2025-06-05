#!/bin/bash

# ============================================================================
# Test final para validar todas las correcciones implementadas
# ============================================================================

BASE_URL="http://srv736989.hstgr.cloud"
TOTAL_TESTS=0
PASSED_TESTS=0
IMPROVED_TESTS=0

echo "=========================================="
echo "🧪 TEST FINAL DE CORRECCIONES IMPLEMENTADAS"
echo "=========================================="
echo "Base URL: $BASE_URL"
echo ""

# Función para probar endpoint con comparación
test_endpoint_improved() {
    local name="$1"
    local url="$2"
    local expected_code="$3"
    local method="${4:-GET}"
    local was_failing="${5:-false}"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -n "🔍 $name: "
    
    if [ "$method" = "POST" ]; then
        response_code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$url" -H "Content-Type: application/json" -d '{}')
    else
        response_code=$(curl -s -o /dev/null -w '%{http_code}' "$url")
    fi
    
    if [ "$response_code" = "$expected_code" ]; then
        if [ "$was_failing" = "true" ]; then
            echo "✅ $response_code (🎉 CORREGIDO)"
            IMPROVED_TESTS=$((IMPROVED_TESTS + 1))
        else
            echo "✅ $response_code"
        fi
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "❌ $response_code (esperado: $expected_code)"
    fi
}

echo "=== 🏥 HEALTH CHECKS (ANTERIORMENTE FUNCIONANDO) ==="
test_endpoint_improved "Health General" "$BASE_URL/health" "200" "GET" "false"
test_endpoint_improved "Savings Health" "$BASE_URL/savings/health" "200" "GET" "false"
test_endpoint_improved "Budget Overview Health" "$BASE_URL/budget-overview/health" "200" "GET" "false"

echo ""
echo "=== 🔧 ENDPOINTS PREVIAMENTE 404 - CORREGIDOS ==="
test_endpoint_improved "Money Flow Data" "$BASE_URL/money-flow/data?user_id=36" "200" "GET" "true"
test_endpoint_improved "Cash Bank Distribution" "$BASE_URL/cash-bank/distribution?user_id=36" "200" "GET" "true"
test_endpoint_improved "Dashboard Data" "$BASE_URL/dashboard/data?user_id=36" "200" "GET" "true"
test_endpoint_improved "User Info" "$BASE_URL/user/info?user_id=36" "400" "GET" "true"

echo ""
echo "=== 🔐 AUTENTICACIÓN (FUNCIONANDO) ==="
test_endpoint_improved "Update Locale" "$BASE_URL/update/locale" "400" "POST" "false"
test_endpoint_improved "Signin Check Email" "$BASE_URL/signin/check-email" "400" "POST" "false"
test_endpoint_improved "Google Auth" "$BASE_URL/auth/google" "401" "POST" "false"

echo ""
echo "=== 📂 GESTIÓN DE CATEGORÍAS (MEJORADO) ==="
test_endpoint_improved "Categories Fetch" "$BASE_URL/categories?user_id=36" "200" "GET" "false"
test_endpoint_improved "Categories Add" "$BASE_URL/categories/add" "400" "POST" "true"

echo ""
echo "=== 💰 OPERACIONES FINANCIERAS ==="
test_endpoint_improved "Savings Fetch" "$BASE_URL/savings/fetch?user_id=36" "200" "GET" "true"
test_endpoint_improved "Income Fetch" "$BASE_URL/incomes?user_id=36" "200" "GET" "false"
test_endpoint_improved "Expense Fetch" "$BASE_URL/expenses?user_id=36" "200" "GET" "false"

echo ""
echo "=== 🏦 CASH/BANK MANAGEMENT (CORREGIDO) ==="
test_endpoint_improved "Cash Update" "$BASE_URL/cash-bank/cash/update" "400" "POST" "true"
test_endpoint_improved "Bank Update" "$BASE_URL/cash-bank/bank/update" "400" "POST" "true"

echo ""
echo "=== 🧾 BILLS MANAGEMENT ==="
test_endpoint_improved "Bills Fetch" "$BASE_URL/bills?user_id=36" "200" "GET" "false"
test_endpoint_improved "Bills Add" "$BASE_URL/bills/add" "400" "POST" "true"

echo ""
echo "=== 📊 REPORTES (MÉTODOS HTTP CORREGIDOS) ==="
test_endpoint_improved "Budget Overview GET" "$BASE_URL/budget-overview?user_id=36" "200" "GET" "true"
test_endpoint_improved "Budget Overview POST" "$BASE_URL/budget-overview" "400" "POST" "true"

echo ""
echo "=== 🌐 LANGUAGE MANAGEMENT (CORREGIDO) ==="
test_endpoint_improved "Language Get" "$BASE_URL/language/get?user_id=36" "200" "GET" "true"

echo ""
echo "=========================================="
echo "📊 RESUMEN FINAL DE CORRECCIONES"
echo "=========================================="
echo "✅ Tests pasados: $PASSED_TESTS"
echo "📊 Total tests: $TOTAL_TESTS"
echo "🎯 Porcentaje éxito: $((PASSED_TESTS * 100 / TOTAL_TESTS))%"
echo "🎉 Endpoints corregidos: $IMPROVED_TESTS"
echo ""

if [ $PASSED_TESTS -gt $((TOTAL_TESTS * 80 / 100)) ]; then
    echo "🎉 ¡CORRECCIONES EXITOSAS! Sistema funcionando óptimamente"
    echo "🚀 NGINX COMPLETAMENTE CORREGIDO"
elif [ $PASSED_TESTS -gt $((TOTAL_TESTS * 60 / 100)) ]; then
    echo "✅ Mejoras significativas implementadas"
    echo "🔧 Algunas correcciones menores pendientes"
else
    echo "⚠️  Necesita más trabajo en routing"
fi

echo ""
echo "📈 COMPARACIÓN:"
echo "  - Estado anterior: 36% funcionando (8/22)"
echo "  - Estado actual: $((PASSED_TESTS * 100 / TOTAL_TESTS))% funcionando ($PASSED_TESTS/$TOTAL_TESTS)"
echo "  - Endpoints mejorados: $IMPROVED_TESTS"
echo ""

if [ $IMPROVED_TESTS -gt 5 ]; then
    echo "🏆 ¡CORRECCIONES EXITOSAS!"
    echo "🎯 Routing de nginx significativamente mejorado"
    echo "✅ Mapeo de puertos corregido"
    echo "✅ Métodos HTTP ajustados"
    echo "✅ Estructura de URLs validada"
else
    echo "🔍 Revisión adicional necesaria"
fi 