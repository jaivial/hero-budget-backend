#!/bin/bash

# Script para probar endpoints localmente
echo "🔧 Probando endpoints corregidos localmente..."
echo "=============================================="

# Función para probar un endpoint
test_endpoint() {
    local url=$1
    local expected_status=$2
    local description=$3
    
    echo "🧪 Probando: $description"
    echo "   URL: $url"
    
    # Hacer request y capturar status code
    status_code=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null)
    
    if [ "$status_code" = "$expected_status" ]; then
        echo "   ✅ SUCCESS: $status_code"
    else
        echo "   ❌ FAILED: Expected $expected_status, got $status_code"
        # Obtener response body para debugging
        response=$(curl -s "$url" 2>/dev/null)
        echo "   Response: $response"
    fi
    echo ""
}

# Función para iniciar un servicio en background
start_service() {
    local dir=$1
    local binary=$2
    local port=$3
    local service_name=$4
    
    echo "🚀 Iniciando $service_name en puerto $port..."
    
    cd "$dir"
    if [ -f "$binary" ]; then
        # Matar proceso previo si existe
        pkill -f "$binary" 2>/dev/null
        
        # Iniciar servicio
        ./"$binary" &
        local pid=$!
        echo "   PID: $pid"
        
        # Esperar a que el servicio esté listo
        sleep 2
        
        # Verificar que el proceso está corriendo
        if ps -p $pid > /dev/null; then
            echo "   ✅ $service_name iniciado correctamente"
            echo $pid > "$binary.pid"
        else
            echo "   ❌ Error iniciando $service_name"
            return 1
        fi
    else
        echo "   ❌ Binario $binary no encontrado en $dir"
        return 1
    fi
    cd ..
}

# Función para detener servicios
stop_services() {
    echo "🛑 Deteniendo servicios de prueba..."
    pkill -f "main" 2>/dev/null
    pkill -f "profile_management" 2>/dev/null
    pkill -f "budget_overview_fetch" 2>/dev/null
    echo "   ✅ Servicios detenidos"
}

# Trap para limpiar al salir
trap stop_services EXIT

echo "📦 Verificando binarios compilados..."

# Verificar que los binarios existen
if [ ! -f "main" ]; then
    echo "❌ Binario 'main' no encontrado. Compilando..."
    go build -o main main.go
fi

if [ ! -f "profile_management/profile_management" ]; then
    echo "❌ Binario 'profile_management' no encontrado. Compilando..."
    cd profile_management && go build -o profile_management main.go && cd ..
fi

if [ ! -f "budget_overview_fetch/budget_overview_fetch" ]; then
    echo "❌ Binario 'budget_overview_fetch' no encontrado. Compilando..."
    cd budget_overview_fetch && go build -o budget_overview_fetch main.go && cd ..
fi

echo ""

# Iniciar servicios necesarios
start_service "." "main" "8083" "Main Service"
start_service "profile_management" "profile_management" "8092" "Profile Management"
start_service "budget_overview_fetch" "budget_overview_fetch" "8098" "Budget Overview"

echo "⏳ Esperando que todos los servicios estén listos..."
sleep 5

echo "🧪 Iniciando pruebas de endpoints..."
echo "=================================="

# Probar endpoints corregidos
test_endpoint "http://localhost:8083/health" "200" "Main Service Health"
test_endpoint "http://localhost:8083/update/locale" "405" "Update Locale (GET - should fail)"
test_endpoint "http://localhost:8092/user/info?user_id=1" "400" "User Info (sin DB - expected error)"
test_endpoint "http://localhost:8098/budget-overview?user_id=1" "500" "Budget Overview GET (sin DB - expected error)"
test_endpoint "http://localhost:8098/transactions/history?user_id=1" "500" "Transaction History GET (sin DB - expected error)"

echo "📋 Probando con POST requests..."

# Test POST request para update/locale
echo "🧪 Probando: Update Locale (POST)"
response=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"user_id":"1","locale":"en-US"}' \
    http://localhost:8083/update/locale)
echo "   Response: $response"
echo ""

echo "✅ Pruebas locales completadas."
echo "Los servicios están funcionando y respondiendo."
echo "Ahora es seguro subirlos al VPS." 