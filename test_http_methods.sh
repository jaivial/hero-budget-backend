#!/bin/bash

# =============================================================================
# SCRIPT PARA PROBAR MÉTODOS HTTP - HERO BUDGET
# =============================================================================

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
echo "   🧪 TESTING MÉTODOS HTTP - HERO BUDGET"
echo "   🌐 Domain: $DOMAIN"
echo "============================================================================="
echo -e "${NC}"

# Función para probar múltiples métodos HTTP
test_http_methods() {
    local url=$1
    local description=$2
    local data=${3:-"{}"}
    
    echo -e "${CYAN}🧪 TESTING: ${description}${NC}"
    echo -e "${BLUE}   URL: $url${NC}"
    
    # Probar GET
    echo -e "${YELLOW}   Probando GET...${NC}"
    response=$(curl -s -o /dev/null -w '%{http_code}' -X GET "$url" 2>/dev/null)
    echo -e "   GET: $response"
    
    # Probar POST
    echo -e "${YELLOW}   Probando POST...${NC}"
    response=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null)
    echo -e "   POST: $response"
    
    # Probar PUT
    echo -e "${YELLOW}   Probando PUT...${NC}"
    response=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null)
    echo -e "   PUT: $response"
    
    # Probar PATCH
    echo -e "${YELLOW}   Probando PATCH...${NC}"
    response=$(curl -s -o /dev/null -w '%{http_code}' -X PATCH "$url" -H "Content-Type: application/json" -d "$data" 2>/dev/null)
    echo -e "   PATCH: $response"
    
    echo ""
}

echo -e "${WHITE}=== 📋 TESTING SIGNUP ENDPOINTS ===${NC}\n"

test_http_methods "$DOMAIN/signup/check-email" "Signup Check Email" '{"email":"test@test.com"}'
test_http_methods "$DOMAIN/signup/register" "Signup Register" '{"email":"test@test.com","password":"test"}'
test_http_methods "$DOMAIN/signup/check-verification" "Signup Check Verification" '{"email":"test@test.com","code":"123456"}'

echo -e "${WHITE}=== 📋 TESTING SAVINGS ENDPOINTS ===${NC}\n"

test_http_methods "$DOMAIN/savings/fetch?user_id=36" "Savings Fetch" '{}'
test_http_methods "$DOMAIN/savings/update" "Savings Update" '{"user_id":36,"amount":1000}'

echo -e "${WHITE}=== 📋 TESTING ADD OPERATIONS ===${NC}\n"

test_http_methods "$DOMAIN/incomes/add" "Income Add" '{"user_id":36,"amount":1000,"description":"test"}'
test_http_methods "$DOMAIN/expenses/add" "Expense Add" '{"user_id":36,"amount":100,"description":"test"}'

echo -e "${WHITE}=== 📋 TESTING BILLS OPERATIONS ===${NC}\n"

test_http_methods "$DOMAIN/bills/add" "Bills Add" '{"user_id":36,"title":"test","amount":100}'
test_http_methods "$DOMAIN/bills/upcoming?user_id=36" "Bills Upcoming" '{}'
test_http_methods "$DOMAIN/bills/update" "Bills Update" '{"id":1,"title":"updated","amount":200}'

echo -e "${WHITE}=== 📊 RESUMEN ===${NC}\n"

echo -e "${GREEN}🎯 OBJETIVO: Identificar qué métodos HTTP funcionan${NC}"
echo -e "${WHITE}   Códigos esperados:${NC}"
echo -e "${GREEN}   200/201 = Funciona correctamente${NC}"
echo -e "${YELLOW}   400/422 = Ruta existe, validación fallida${NC}"
echo -e "${RED}   404 = Ruta no existe${NC}"
echo -e "${YELLOW}   405 = Método incorrecto${NC}" 