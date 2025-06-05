#!/bin/bash

# ============================================================================
# Script para verificar mapeo de puertos y servicios
# ============================================================================

echo "=========================================="
echo "🔍 VERIFICANDO MAPEO DE PUERTOS Y SERVICIOS"
echo "=========================================="

# Función para probar endpoint directo
test_direct_service() {
    local service_name="$1"
    local port="$2"
    local endpoint="$3"
    
    echo "🔍 Testing $service_name (puerto $port):"
    echo "   Endpoint: http://localhost:$port$endpoint"
    
    response=$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$port$endpoint")
    echo "   Respuesta: $response"
    
    if [ "$response" != "000" ]; then
        echo "   ✅ Servicio activo"
    else
        echo "   ❌ Servicio no responde"
    fi
    echo ""
}

echo "=== 🏥 VERIFICANDO HEALTH ENDPOINTS ==="
test_direct_service "Savings" "8089" "/health"
test_direct_service "Budget Overview" "8098" "/health"
test_direct_service "Main (Language)" "8083" "/health"

echo "=== 💰 VERIFICANDO MONEY FLOW ==="
test_direct_service "Money Flow" "8097" "/money-flow/data?user_id=36"
test_direct_service "Money Flow" "8097" "/data?user_id=36"
test_direct_service "Money Flow" "8097" "/"

echo "=== 🏦 VERIFICANDO CASH/BANK ==="
test_direct_service "Cash Bank" "8090" "/cash-bank/distribution?user_id=36"
test_direct_service "Cash Bank" "8090" "/distribution?user_id=36"
test_direct_service "Cash Bank" "8090" "/"

echo "=== 📊 VERIFICANDO DASHBOARD ==="
test_direct_service "Dashboard Data" "8087" "/dashboard/data?user_id=36"
test_direct_service "Dashboard Data" "8087" "/data?user_id=36"
test_direct_service "Dashboard Data" "8087" "/"

echo "=== 👤 VERIFICANDO PROFILE ==="
test_direct_service "Profile" "8092" "/user/info?user_id=36"
test_direct_service "Profile" "8092" "/info?user_id=36"
test_direct_service "Profile" "8092" "/"

echo "=== 📂 VERIFICANDO CATEGORÍAS ==="
test_direct_service "Categories" "8096" "/categories?user_id=36"
test_direct_service "Categories" "8096" "/?user_id=36"
test_direct_service "Categories" "8096" "/"

echo "=== 🌐 VERIFICANDO LANGUAGE ==="
test_direct_service "Language" "8083" "/language/get?user_id=36"
test_direct_service "Language" "8083" "/get?user_id=36"
test_direct_service "Language" "8083" "/"

echo "=========================================="
echo "📋 PUERTOS ACTUALMENTE ESCUCHANDO:"
echo "=========================================="
ss -tln | grep -E ':(808[1-9]|809[0-8])' | sort

echo ""
echo "=========================================="
echo "🔍 ANÁLISIS DE ROUTING NECESARIO:"
echo "=========================================="
echo "1. Verificar si servicios usan prefijo en sus rutas"
echo "2. Confirmar métodos HTTP correctos"
echo "3. Validar estructura de URLs esperadas"
echo "4. Corregir configuración nginx según hallazgos" 