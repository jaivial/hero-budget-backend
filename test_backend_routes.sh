#!/bin/bash

# =============================================================================
# SCRIPT PARA PROBAR RUTAS REALES DE SERVICIOS BACKEND
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

VPS_IP="178.16.130.178"
VPS_USER="root"

echo -e "${WHITE}"
echo "============================================================================="
echo "   🔍 TESTING RUTAS REALES DE SERVICIOS BACKEND"
echo "============================================================================="
echo -e "${NC}"

# Función para probar múltiples variaciones de ruta
test_service_routes() {
    local port=$1
    local service_name=$2
    shift 2
    local test_paths=("$@")
    
    echo -e "${CYAN}🔍 PROBANDO SERVICIO: ${service_name} (Puerto $port)${NC}"
    
    for path in "${test_paths[@]}"; do
        echo -e "${BLUE}  Probando: localhost:$port$path${NC}"
        response=$(ssh $VPS_USER@$VPS_IP "curl -s -o /dev/null -w '%{http_code}' http://localhost:$port$path" 2>/dev/null)
        
        case $response in
            200|201)
                echo -e "${GREEN}    ✅ FUNCIONA: $response${NC}"
                ;;
            400|422)
                echo -e "${YELLOW}    ⚠️  RUTA EXISTE (validación): $response${NC}"
                ;;
            404)
                echo -e "${RED}    ❌ NO EXISTE: $response${NC}"
                ;;
            405)
                echo -e "${YELLOW}    ⚠️  MÉTODO INCORRECTO: $response${NC}"
                ;;
            *)
                echo -e "${YELLOW}    ⚠️  RESPUESTA: $response${NC}"
                ;;
        esac
    done
    echo ""
}

echo -e "${WHITE}=== 📋 TESTING SIGNUP SERVICE (8082) ===${NC}\n"
test_service_routes 8082 "signup_service" \
    "/health" \
    "/check-email" \
    "/signup/check-email" \
    "/register" \
    "/signup/register" \
    "/check-verification" \
    "/signup/check-verification" \
    "/"

echo -e "${WHITE}=== 📋 TESTING SAVINGS SERVICE (8089) ===${NC}\n"
test_service_routes 8089 "savings_service" \
    "/health" \
    "/fetch" \
    "/savings/fetch" \
    "/update" \
    "/savings/update" \
    "/"

echo -e "${WHITE}=== 📋 TESTING INCOME SERVICE (8093) ===${NC}\n"
test_service_routes 8093 "income_service" \
    "/health" \
    "/add" \
    "/incomes/add" \
    "/"

echo -e "${WHITE}=== 📋 TESTING EXPENSE SERVICE (8094) ===${NC}\n"
test_service_routes 8094 "expense_service" \
    "/health" \
    "/add" \
    "/expenses/add" \
    "/"

echo -e "${WHITE}=== 📋 TESTING BILLS SERVICE (8091) ===${NC}\n"
test_service_routes 8091 "bills_service" \
    "/health" \
    "/add" \
    "/bills/add" \
    "/upcoming" \
    "/bills/upcoming" \
    "/update" \
    "/bills/update" \
    "/"

echo -e "${WHITE}=== 📊 RESUMEN DE DISCOVERY ===${NC}\n"

echo -e "${GREEN}🔍 SIGUIENTE PASO: Analizar los resultados para entender${NC}"
echo -e "${WHITE}   qué rutas están realmente implementadas en cada servicio${NC}"
echo -e "${WHITE}   y ajustar nginx en consecuencia.${NC}"

echo -e "\n${CYAN}💡 POSIBLES SOLUCIONES:${NC}"
echo -e "${WHITE}1. Los servicios usan rutas sin prefijo (/add vs /incomes/add)${NC}"
echo -e "${WHITE}2. Los servicios usan rutas con prefijo diferente${NC}"
echo -e "${WHITE}3. Necesitamos ajustar proxy_pass en nginx${NC}"
echo -e "${WHITE}4. Los servicios están en subdirectorios diferentes${NC}" 