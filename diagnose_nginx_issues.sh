#!/bin/bash

# =============================================================================
# SCRIPT DE DIAGNÓSTICO - ANÁLISIS DE FALLOS NGINX HERO BUDGET
# =============================================================================

# Configuración de colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

DOMAIN="https://herobudget.jaimedigitalstudio.com"

echo -e "${WHITE}"
echo "============================================================================="
echo "   🔍 DIAGNÓSTICO DE PROBLEMAS NGINX - HERO BUDGET"
echo "   🌐 Domain: $DOMAIN"
echo "============================================================================="
echo -e "${NC}"

# Función para probar endpoint específico con análisis detallado
diagnose_endpoint() {
    local method=$1
    local path=$2
    local data=$3
    local description=$4
    local expected_service=$5
    
    echo -e "${CYAN}🔍 DIAGNOSTICANDO: ${description}${NC}"
    echo -e "${BLUE}  URL: $DOMAIN$path${NC}"
    echo -e "${YELLOW}  Servicio esperado: $expected_service${NC}"
    
    # Hacer petición con headers detallados
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTPSTATUS:%{http_code}\nTIME:%{time_total}" -H "Accept: application/json" "$DOMAIN$path" 2>/dev/null)
    else
        response=$(curl -s -w "HTTPSTATUS:%{http_code}\nTIME:%{time_total}" -X "$method" -H "Content-Type: application/json" -H "Accept: application/json" -d "$data" "$DOMAIN$path" 2>/dev/null)
    fi
    
    # Extraer información
    status_code=$(echo "$response" | grep "HTTPSTATUS:" | cut -d':' -f2)
    time_total=$(echo "$response" | grep "TIME:" | cut -d':' -f2)
    body=$(echo "$response" | sed '/HTTPSTATUS:/d' | sed '/TIME:/d')
    
    # Análisis del resultado
    case $status_code in
        200|201)
            echo -e "${GREEN}  ✅ SUCCESS: $status_code (${time_total}s)${NC}"
            ;;
        404)
            echo -e "${RED}  ❌ 404 NOT FOUND${NC}"
            echo -e "${YELLOW}     Issue: Ruta no configurada en nginx o servicio no responde${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            ;;
        400)
            echo -e "${YELLOW}  ⚠️  400 BAD REQUEST${NC}"
            echo -e "${YELLOW}     Issue: Problema con formato de datos enviados${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            ;;
        401)
            echo -e "${YELLOW}  🔐 401 UNAUTHORIZED${NC}"
            echo -e "${YELLOW}     Issue: Autenticación requerida o inválida${NC}"
            ;;
        405)
            echo -e "${RED}  ❌ 405 METHOD NOT ALLOWED${NC}"
            echo -e "${YELLOW}     Issue: Método HTTP no permitido en esta ruta${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            ;;
        502)
            echo -e "${RED}  ❌ 502 BAD GATEWAY${NC}"
            echo -e "${YELLOW}     Issue: Servicio $expected_service no está corriendo${NC}"
            ;;
        000)
            echo -e "${RED}  ❌ CONNECTION FAILED${NC}"
            echo -e "${YELLOW}     Issue: No se puede conectar al servidor${NC}"
            ;;
        *)
            echo -e "${RED}  ❌ ERROR: $status_code${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            ;;
    esac
    echo ""
}

echo -e "${WHITE}=== 🚨 ANÁLISIS DE ENDPOINTS FALLIDOS ===${NC}\n"

# 1. SIGNUP ENDPOINTS (todos dan 404)
echo -e "${RED}🔥 PROBLEMA 1: SIGNUP ENDPOINTS${NC}"
diagnose_endpoint "POST" "/signup/check-email" '{"email":"test@test.com"}' "Signup Check Email" "backend_signup (8082)"
diagnose_endpoint "POST" "/signup/register" '{"email":"test@test.com","password":"pass","name":"test"}' "Signup Register" "backend_signup (8082)"
diagnose_endpoint "POST" "/signup/check-verification" '{"email":"test@test.com","code":"123"}' "Signup Check Verification" "backend_signup (8082)"

# 2. SAVINGS ENDPOINTS (dan 404)
echo -e "${RED}🔥 PROBLEMA 2: SAVINGS ENDPOINTS${NC}"
diagnose_endpoint "GET" "/savings/fetch?user_id=36" "" "Savings Fetch" "backend_savings (8089)"
diagnose_endpoint "POST" "/savings/update" '{"user_id":"36","available":500,"goal":1000}' "Savings Update" "backend_savings (8089)"

# 3. INCOMES/EXPENSES ADD (dan 404)
echo -e "${RED}🔥 PROBLEMA 3: ADD OPERATIONS${NC}"
diagnose_endpoint "POST" "/incomes/add" '{"user_id":"36","amount":100,"category":"1"}' "Income Add" "backend_income (8093)"
diagnose_endpoint "POST" "/expenses/add" '{"user_id":"36","amount":50,"category":"1"}' "Expense Add" "backend_expense (8094)"

# 4. BILLS OPERATIONS (varios 404)
echo -e "${RED}🔥 PROBLEMA 4: BILLS OPERATIONS${NC}"
diagnose_endpoint "POST" "/bills/add" '{"user_id":"36","name":"Test","amount":100,"due_date":"2025-07-01"}' "Bills Add" "backend_bills (8091)"
diagnose_endpoint "GET" "/bills/upcoming?user_id=36" "" "Bills Upcoming" "backend_bills (8091)"
diagnose_endpoint "POST" "/bills/update" '{"user_id":"36","bill_id":1,"name":"Updated"}' "Bills Update" "backend_bills (8091)"

# 5. USER/PROFILE ISSUES (400 errors)
echo -e "${RED}🔥 PROBLEMA 5: USER/PROFILE VALIDATION${NC}"
diagnose_endpoint "GET" "/user/info?user_id=36" "" "User Info" "backend_fetch_dashboard (8085)"
diagnose_endpoint "POST" "/user/update" '{"id":"36","name":"Test","email":"test@test.com"}' "User Update" "backend_fetch_dashboard (8085)"
diagnose_endpoint "POST" "/profile/update" '{"user_id":"36","name":"Test","email":"test@test.com"}' "Profile Update" "backend_profile (8092)"

# 6. METHOD NOT ALLOWED
echo -e "${RED}🔥 PROBLEMA 6: METHOD ISSUES${NC}"
diagnose_endpoint "GET" "/budget-overview?user_id=36" "" "Budget Overview GET" "backend_budget_overview (8098)"
diagnose_endpoint "POST" "/budget-overview" '{"user_id":"36"}' "Budget Overview POST" "backend_budget_overview (8098)"

# 7. TRANSACTION HISTORY (404)
echo -e "${RED}🔥 PROBLEMA 7: MISSING ENDPOINTS${NC}"
diagnose_endpoint "GET" "/transactions/history?user_id=36" "" "Transaction History" "backend_main (8083)"

echo -e "${WHITE}=== 🔧 ANÁLISIS DE CONFIGURACIÓN NGINX ===${NC}\n"

echo -e "${BLUE}🔍 Verificando configuración actual de nginx...${NC}"

# Verificar si el archivo existe
if [ ! -f "nginx-herobudget-https.conf" ]; then
    echo -e "${RED}❌ Archivo nginx-herobudget-https.conf no encontrado${NC}"
else
    echo -e "${GREEN}✅ Archivo nginx encontrado${NC}"
    
    # Buscar configuraciones específicas problemáticas
    echo -e "\n${YELLOW}📋 Configuraciones de routing encontradas:${NC}"
    
    echo -e "${CYAN}SIGNUP routes:${NC}"
    grep -n "location.*signup" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No signup routes found${NC}"
    
    echo -e "${CYAN}SAVINGS routes:${NC}"
    grep -n "location.*savings" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No savings routes found${NC}"
    
    echo -e "${CYAN}INCOMES routes:${NC}"
    grep -n "location.*incomes" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No incomes routes found${NC}"
    
    echo -e "${CYAN}BILLS routes:${NC}"
    grep -n "location.*bills" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No bills routes found${NC}"
    
    echo -e "${CYAN}USER routes:${NC}"
    grep -n "location.*user" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No user routes found${NC}"
    
    echo -e "${CYAN}BUDGET-OVERVIEW routes:${NC}"
    grep -n "location.*budget-overview" nginx-herobudget-https.conf || echo -e "${RED}  ❌ No budget-overview routes found${NC}"
fi

echo -e "\n${WHITE}=== 🚀 COMANDOS DE REPARACIÓN SUGERIDOS ===${NC}\n"

echo -e "${GREEN}1. VERIFICAR SERVICIOS EN VPS:${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'ps aux | grep -E \"8082|8089|8091|8093|8094\"'${NC}"

echo -e "\n${GREEN}2. VERIFICAR LOGS DE NGINX:${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'tail -f /var/log/nginx/herobudget_error.log'${NC}"

echo -e "\n${GREEN}3. PROBAR SERVICIOS LOCALMENTE:${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'curl http://localhost:8082/health'${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'curl http://localhost:8089/health'${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'curl http://localhost:8091/health'${NC}"

echo -e "\n${GREEN}4. VERIFICAR CONFIGURACIÓN NGINX ACTIVA:${NC}"
echo -e "${WHITE}ssh root@178.16.130.178 'nginx -t && nginx -s reload'${NC}"

echo -e "\n${WHITE}=== 📊 RESUMEN DE PROBLEMAS IDENTIFICADOS ===${NC}\n"

echo -e "${RED}🔥 PROBLEMAS CRÍTICOS:${NC}"
echo -e "${WHITE}1. Routes /signup/* configuradas incorrectamente o servicio 8082 down${NC}"
echo -e "${WHITE}2. Routes /savings/* no funcionan correctamente${NC}"
echo -e "${WHITE}3. Routes /*add endpoints fallan (incomes, expenses, bills)${NC}"
echo -e "${WHITE}4. Validation errors en user/profile endpoints${NC}"
echo -e "${WHITE}5. Method not allowed en budget-overview${NC}"

echo -e "\n${YELLOW}⚡ SOLUCIONES RECOMENDADAS:${NC}"
echo -e "${WHITE}1. Revisar que todos los microservicios estén corriendo${NC}"
echo -e "${WHITE}2. Corregir nginx routing para endpoints específicos${NC}"
echo -e "${WHITE}3. Validar payloads de testing contra APIs reales${NC}"
echo -e "${WHITE}4. Verificar conectividad entre nginx y servicios${NC}"

echo -e "\n${GREEN}🎯 SIGUIENTE PASO: Ejecutar correcciones automáticas${NC}" 