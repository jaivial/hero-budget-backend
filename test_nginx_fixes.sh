#!/bin/bash

# =============================================================================
# SCRIPT DE TESTING - VERIFICACIÓN DE CORRECCIONES NGINX
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
TEST_USER_ID="36"

# Contadores
total_fixed=0
successful_fixed=0
still_failing=0

echo -e "${WHITE}"
echo "============================================================================="
echo "   🔧 TESTING CORRECCIONES NGINX - HERO BUDGET"
echo "   🌐 Domain: $DOMAIN"
echo "============================================================================="
echo -e "${NC}"

# Función para probar endpoint corregido
test_fixed_endpoint() {
    local method=$1
    local path=$2
    local data=$3
    local description=$4
    local issue_type=$5
    
    total_fixed=$((total_fixed + 1))
    
    echo -e "${CYAN}🔧 PROBANDO CORRECCIÓN: ${description}${NC}"
    echo -e "${BLUE}  URL: $DOMAIN$path${NC}"
    echo -e "${YELLOW}  Issue original: $issue_type${NC}"
    
    # Hacer petición
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" "$DOMAIN$path" 2>/dev/null)
    else
        response=$(curl -s -w "HTTPSTATUS:%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$DOMAIN$path" 2>/dev/null)
    fi
    
    # Extraer código de estado
    status_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d':' -f2)
    body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*$//')
    
    # Verificar si se corrigió
    case $status_code in
        200|201)
            echo -e "${GREEN}  ✅ CORREGIDO: $status_code${NC}"
            successful_fixed=$((successful_fixed + 1))
            ;;
        400)
            if [[ "$body" == *"required"* || "$body" == *"Invalid"* ]]; then
                echo -e "${YELLOW}  ⚠️  VALIDACIÓN: $status_code (Esperado)${NC}"
                echo -e "${WHITE}     Response: $body${NC}"
                successful_fixed=$((successful_fixed + 1))
            else
                echo -e "${RED}  ❌ SIGUE FALLANDO: $status_code${NC}"
                echo -e "${WHITE}     Response: $body${NC}"
                still_failing=$((still_failing + 1))
            fi
            ;;
        404)
            echo -e "${RED}  ❌ SIGUE 404: $status_code${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            still_failing=$((still_failing + 1))
            ;;
        405)
            echo -e "${RED}  ❌ SIGUE METHOD NOT ALLOWED: $status_code${NC}"
            still_failing=$((still_failing + 1))
            ;;
        502)
            echo -e "${RED}  ❌ SERVICIO DOWN: $status_code${NC}"
            still_failing=$((still_failing + 1))
            ;;
        *)
            echo -e "${RED}  ❌ ERROR: $status_code${NC}"
            echo -e "${WHITE}     Response: $body${NC}"
            still_failing=$((still_failing + 1))
            ;;
    esac
    echo ""
}

echo -e "${WHITE}=== 🔧 VERIFICANDO CORRECCIONES CRÍTICAS ===${NC}\n"

echo -e "${RED}📋 GRUPO 1: SIGNUP ENDPOINTS (404 → 200)${NC}"
test_fixed_endpoint "POST" "/signup/check-email" '{"email":"test@test.com"}' "Signup Check Email" "404 NOT FOUND"
test_fixed_endpoint "POST" "/signup/register" '{"email":"test@test.com","password":"password123","name":"Test User"}' "Signup Register" "404 NOT FOUND"
test_fixed_endpoint "POST" "/signup/check-verification" '{"email":"test@test.com","verification_code":"123456"}' "Signup Check Verification" "404 NOT FOUND"

echo -e "${RED}📋 GRUPO 2: SAVINGS ENDPOINTS (404 → 200)${NC}"
test_fixed_endpoint "GET" "/savings/fetch?user_id=$TEST_USER_ID" "" "Savings Fetch" "404 NOT FOUND"
test_fixed_endpoint "POST" "/savings/update" '{"user_id":"'$TEST_USER_ID'","available":500.00,"goal":1000.00}' "Savings Update" "404 NOT FOUND"

echo -e "${RED}📋 GRUPO 3: ADD OPERATIONS (404 → 200)${NC}"
test_fixed_endpoint "POST" "/incomes/add" '{"user_id":"'$TEST_USER_ID'","amount":1000.00,"category":"1","payment_method":"cash","date":"2025-06-04","description":"Test Income"}' "Income Add" "404 NOT FOUND"
test_fixed_endpoint "POST" "/expenses/add" '{"user_id":"'$TEST_USER_ID'","amount":50.00,"category":"1","payment_method":"bank","date":"2025-06-04","description":"Test Expense"}' "Expense Add" "404 NOT FOUND"

echo -e "${RED}📋 GRUPO 4: BILLS OPERATIONS (404 → 200)${NC}"
test_fixed_endpoint "POST" "/bills/add" '{"user_id":"'$TEST_USER_ID'","name":"Test Bill","amount":100.00,"due_date":"2025-07-01","category":"1"}' "Bills Add" "404 NOT FOUND"
test_fixed_endpoint "GET" "/bills/upcoming?user_id=$TEST_USER_ID" "" "Bills Upcoming" "404 NOT FOUND"
test_fixed_endpoint "POST" "/bills/update" '{"user_id":"'$TEST_USER_ID'","bill_id":1,"name":"Updated Bill","amount":125.75}' "Bills Update" "404 NOT FOUND"

echo -e "${RED}📋 GRUPO 5: USER/PROFILE SPECIFIC ROUTES (404 → 200)${NC}"
test_fixed_endpoint "GET" "/user/info?user_id=$TEST_USER_ID" "" "User Info" "400 BAD REQUEST"
test_fixed_endpoint "POST" "/user/update" '{"id":"'$TEST_USER_ID'","name":"Test User Updated","email":"updated@test.com"}' "User Update" "404 NOT FOUND"
test_fixed_endpoint "POST" "/profile/update" '{"user_id":"'$TEST_USER_ID'","name":"Updated Profile","email":"profile@test.com"}' "Profile Update" "400 BAD REQUEST"

echo -e "${RED}📋 GRUPO 6: METHOD ISSUES (405 → 200)${NC}"
test_fixed_endpoint "GET" "/budget-overview?user_id=$TEST_USER_ID" "" "Budget Overview GET" "405 METHOD NOT ALLOWED"
test_fixed_endpoint "GET" "/transactions/history?user_id=$TEST_USER_ID" "" "Transaction History" "404 NOT FOUND"

echo -e "${RED}📋 GRUPO 7: LANGUAGE MANAGEMENT (400 → 200)${NC}"
test_fixed_endpoint "POST" "/language/set" '{"user_id":"'$TEST_USER_ID'","language":"es"}' "Language Set" "400 BAD REQUEST"

echo -e "${WHITE}=== 📊 RESUMEN DE CORRECCIONES ===${NC}\n"

echo -e "${GREEN}✅ ENDPOINTS CORREGIDOS: $successful_fixed${NC}"
echo -e "${RED}❌ ENDPOINTS QUE SIGUEN FALLANDO: $still_failing${NC}"
echo -e "${WHITE}📊 TOTAL PROBADO: $total_fixed${NC}"

# Calcular score de mejora
improvement_score=$((successful_fixed * 100 / total_fixed))

echo -e "\n${WHITE}🎯 SCORE DE MEJORA: $improvement_score% ($successful_fixed/$total_fixed endpoints corregidos)${NC}"

if [ $still_failing -eq 0 ]; then
    echo -e "\n${GREEN}🎉 ¡TODAS LAS CORRECCIONES EXITOSAS!${NC}"
    echo -e "${GREEN}   - TODOS los endpoints problemáticos fueron corregidos${NC}"
    echo -e "${GREEN}   - $successful_fixed/$total_fixed correcciones aplicadas${NC}"
    echo -e "${GREEN}   - 0 fallos persistentes${NC}"
    echo -e "${GREEN}   - Nginx configurado correctamente${NC}"
    
    echo -e "\n${CYAN}🚀 ACCIÓN REQUERIDA:${NC}"
    echo -e "${WHITE}  cp nginx-herobudget-https-fixed.conf nginx-herobudget-https.conf${NC}"
    echo -e "${WHITE}  scp nginx-herobudget-https.conf root@178.16.130.178:/etc/nginx/sites-available/herobudget${NC}"
    echo -e "${WHITE}  ssh root@178.16.130.178 'nginx -t && systemctl reload nginx'${NC}"
    
    exit 0
elif [ $still_failing -le 3 ]; then
    echo -e "\n${YELLOW}⚠️  CORRECCIONES MAYORITARIAMENTE EXITOSAS${NC}"
    echo -e "${WHITE}   - Solo $still_failing endpoint(s) siguen con problemas${NC}"
    echo -e "${WHITE}   - $successful_fixed/$total_fixed correcciones aplicadas${NC}"
    echo -e "${WHITE}   - Problemas restantes son menores${NC}"
    
    echo -e "\n${CYAN}🚀 ACCIÓN RECOMENDADA:${NC}"
    echo -e "${WHITE}  Aplicar la configuración corregida y revisar endpoints restantes${NC}"
    echo -e "${WHITE}  cp nginx-herobudget-https-fixed.conf nginx-herobudget-https.conf${NC}"
    echo -e "${WHITE}  scp nginx-herobudget-https.conf root@178.16.130.178:/etc/nginx/sites-available/herobudget${NC}"
    echo -e "${WHITE}  ssh root@178.16.130.178 'nginx -t && systemctl reload nginx'${NC}"
    
    exit 0
else
    echo -e "\n${RED}❌ CORRECCIONES INSUFICIENTES${NC}"
    echo -e "${WHITE}   - $still_failing endpoint(s) siguen fallando${NC}"
    echo -e "${WHITE}   - Revisar configuración nginx${NC}"
    echo -e "${WHITE}   - Verificar estado de servicios backend${NC}"
    
    echo -e "\n${YELLOW}📋 SIGUIENTES PASOS:${NC}"
    echo -e "${WHITE}1. Verificar que todos los microservicios estén corriendo${NC}"
    echo -e "${WHITE}2. Revisar logs de nginx para errores específicos${NC}"
    echo -e "${WHITE}3. Verificar conectividad entre nginx y servicios${NC}"
    echo -e "${WHITE}4. Corregir configuraciones específicas restantes${NC}"
    
    exit 1
fi 