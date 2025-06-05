#!/bin/bash

# Script para compilar todos los servicios de Hero Budget Backend
# Autor: Hero Budget Development Team
# Fecha: $(date)

echo "🚀 Compilando todos los servicios de Hero Budget Backend..."
echo "========================================================"

# Función para compilar un servicio
compile_service() {
    local service_dir=$1
    local service_name=$2
    
    echo "📦 Compilando $service_name..."
    
    if [ -d "$service_dir" ]; then
        cd "$service_dir"
        
        # Verificar si existe go.mod
        if [ -f "go.mod" ]; then
            echo "   - Ejecutando go mod tidy..."
            go mod tidy
        fi
        
        # Compilar el servicio
        echo "   - Compilando binario..."
        go build -o "$service_name" main.go
        
        if [ $? -eq 0 ]; then
            echo "   ✅ $service_name compilado exitosamente"
        else
            echo "   ❌ Error compilando $service_name"
            return 1
        fi
        
        cd ..
    else
        echo "   ⚠️  Directorio $service_dir no encontrado"
        return 1
    fi
}

# Lista de servicios a compilar
services=(
    ".:main"
    "google_auth:google_auth"
    "signup:signup"
    "signin:signin"
    "savings_management:savings_management"
    "reset_password:reset_password"
    "profile_management:profile_management"
    "money_flow_sync:money_flow_sync"
    "income_management:income_management"
    "fetch_dashboard:fetch_dashboard"
    "expense_management:expense_management"
    "dashboard_data:dashboard_data"
    "cash_bank_management:cash_bank_management"
    "categories_management:categories_management"
    "bills_management:bills_management"
    "budget_overview_fetch:budget_overview_fetch"
    "budget_management:budget_management"
    "recurring_bills_management:recurring_bills_management"
)

# Contador de éxitos y fallos
success_count=0
error_count=0

# Compilar cada servicio
for service in "${services[@]}"; do
    IFS=':' read -r dir name <<< "$service"
    
    if compile_service "$dir" "$name"; then
        ((success_count++))
    else
        ((error_count++))
    fi
    echo ""
done

# Resumen final
echo "========================================================"
echo "🏁 Compilación completada:"
echo "   ✅ Exitosos: $success_count"
echo "   ❌ Errores: $error_count"

if [ $error_count -eq 0 ]; then
    echo "   🎉 ¡Todos los servicios compilados exitosamente!"
    exit 0
else
    echo "   ⚠️  Algunos servicios tuvieron errores"
    exit 1
fi 