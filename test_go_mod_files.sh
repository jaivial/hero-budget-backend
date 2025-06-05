#!/bin/bash

# Script para probar que todos los archivos go.mod funcionen correctamente

echo "🔧 Probando archivos go.mod creados..."

# Lista de servicios a probar
services=(
    "cash_bank_management"
    "profile_management" 
    "signup"
    "signin"
    "reset_password"
    "money_flow_sync"
    "categories_management"
    "income_management"
    "expense_management"
    "bills_management"
    "dashboard_data"
    "savings_management"
    "budget_management"
    "fetch_dashboard"
    "transaction_delete_service"
    "language_cookie"
)

success_count=0
total_count=${#services[@]}

for service in "${services[@]}"; do
    echo "📦 Probando $service..."
    
    if [ -d "$service" ]; then
        cd "$service" || continue
        
        # Verificar go.mod existe
        if [ ! -f "go.mod" ]; then
            echo "❌ $service: go.mod no encontrado"
            cd ..
            continue
        fi
        
        # Probar go mod tidy
        if go mod tidy &>/dev/null; then
            echo "✅ $service: go mod tidy exitoso"
            success_count=$((success_count + 1))
        else
            echo "❌ $service: go mod tidy falló"
        fi
        
        cd ..
    else
        echo "❌ $service: directorio no encontrado"
    fi
done

echo ""
echo "📊 RESULTADOS:"
echo "  Exitosos: $success_count/$total_count"
echo "  Fallidos: $((total_count - success_count))/$total_count"

if [ $success_count -eq $total_count ]; then
    echo "🎉 ¡Todos los servicios tienen go.mod funcionando!"
    exit 0
else
    echo "⚠️  Algunos servicios tienen problemas con go.mod"
    exit 1
fi 