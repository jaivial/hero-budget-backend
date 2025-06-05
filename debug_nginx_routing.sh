#!/bin/bash

# =============================================================================
# SCRIPT DEBUG ROUTING NGINX - HERO BUDGET
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
VPS_IP="178.16.130.178"
VPS_USER="root"

echo -e "${WHITE}"
echo "============================================================================="
echo "   🐛 DEBUG ROUTING NGINX - HERO BUDGET"
echo "   🌐 Domain: $DOMAIN"
echo "============================================================================="
echo -e "${NC}"

# Función para debug de endpoint específico
debug_endpoint() {
    local path=$1
    local expected_port=$2
    local description=$3
    
    echo -e "${CYAN}🐛 DEBUGGING: ${description}${NC}"
    echo -e "${BLUE}  URL: $DOMAIN$path${NC}"
    echo -e "${YELLOW}  Puerto esperado: $expected_port${NC}"
    
    # Probar en dominio con headers verbose
    echo -e "${WHITE}  📡 Respuesta del dominio:${NC}"
    response=$(curl -s -w "STATUS:%{http_code}|TIME:%{time_total}" -H "Accept: application/json" "$DOMAIN$path" 2>&1)
    echo "     $response"
    
    # Probar directamente en VPS localhost
    echo -e "${WHITE}  🖥️  Respuesta del VPS localhost:${NC}"
    vps_response=$(ssh $VPS_USER@$VPS_IP "curl -s -w 'STATUS:%{http_code}' http://localhost:$expected_port$path" 2>/dev/null)
    echo "     $vps_response"
    
    # Verificar configuración nginx para esta ruta
    echo -e "${WHITE}  📋 Configuración nginx para esta ruta:${NC}"
    ssh $VPS_USER@$VPS_IP "grep -A 2 -B 2 'location.*$(echo $path | cut -d'?' -f1)' /etc/nginx/sites-available/herobudget" 2>/dev/null || echo "     ❌ No se encontró configuración específica"
    
    echo ""
}

echo -e "${WHITE}=== 🐛 DEBUGGING ENDPOINTS ESPECÍFICOS ===${NC}\n"

# Debug endpoints críticos que fallan
debug_endpoint "/signup/check-email" "8082" "Signup Check Email"
debug_endpoint "/savings/fetch" "8089" "Savings Fetch"  
debug_endpoint "/incomes/add" "8093" "Income Add"
debug_endpoint "/expenses/add" "8094" "Expense Add"
debug_endpoint "/bills/add" "8091" "Bills Add"

echo -e "${WHITE}=== 🔍 ANÁLISIS DE CONFIGURACIÓN NGINX ===${NC}\n"

echo -e "${CYAN}📋 Verificando orden de locations en nginx...${NC}"
ssh $VPS_USER@$VPS_IP "grep -n 'location.*/' /etc/nginx/sites-available/herobudget | head -20"

echo -e "\n${CYAN}📋 Verificando locations específicos problemáticos...${NC}"

echo -e "${YELLOW}SIGNUP locations:${NC}"
ssh $VPS_USER@$VPS_IP "grep -n -A 2 'location.*signup' /etc/nginx/sites-available/herobudget"

echo -e "\n${YELLOW}SAVINGS locations:${NC}"
ssh $VPS_USER@$VPS_IP "grep -n -A 2 'location.*savings' /etc/nginx/sites-available/herobudget"

echo -e "\n${YELLOW}INCOMES locations:${NC}"
ssh $VPS_USER@$VPS_IP "grep -n -A 2 'location.*incomes' /etc/nginx/sites-available/herobudget"

echo -e "\n${WHITE}=== 🧪 TESTS EXPERIMENTALES ===${NC}\n"

echo -e "${CYAN}🧪 Probando ruta genérica de signup...${NC}"
curl -s -w "STATUS:%{http_code}" "$DOMAIN/signup/" | head -1

echo -e "\n${CYAN}🧪 Probando health checks que funcionan...${NC}"
curl -s -w "STATUS:%{http_code}" "$DOMAIN/health" | head -1
curl -s -w "STATUS:%{http_code}" "$DOMAIN/savings/health" | head -1

echo -e "\n${CYAN}🧪 Probando rutas que funcionan vs que fallan...${NC}"
echo -e "${GREEN}Funciona - Categories:${NC}"
curl -s -w "STATUS:%{http_code}" "$DOMAIN/categories" | head -1

echo -e "\n${RED}Falla - Signup Check Email:${NC}"
curl -s -w "STATUS:%{http_code}" "$DOMAIN/signup/check-email" | head -1

echo -e "\n${WHITE}=== 💡 HIPÓTESIS Y SOLUCIONES ===${NC}\n"

echo -e "${YELLOW}🔍 POSIBLES CAUSAS:${NC}"
echo -e "${WHITE}1. Orden incorrecto de locations en nginx (más específicas después de genéricas)${NC}"
echo -e "${WHITE}2. Conflicto entre location /signup/ y location /signup/check-email${NC}"
echo -e "${WHITE}3. Proxy_pass mal configurado para rutas específicas${NC}"
echo -e "${WHITE}4. Cache de nginx manteniendo configuración antigua${NC}"

echo -e "\n${YELLOW}🔧 SOLUCIONES A PROBAR:${NC}"
echo -e "${WHITE}1. Reordenar locations (específicas primero, genéricas después)${NC}"
echo -e "${WHITE}2. Usar ~ regex para rutas específicas${NC}"
echo -e "${WHITE}3. Limpiar cache de nginx: nginx -s reload${NC}"
echo -e "${WHITE}4. Verificar upstream backend activos${NC}"

echo -e "\n${GREEN}📋 COMANDO INMEDIATO PARA SOLUCIONAR:${NC}"
echo -e "${WHITE}   Reordenar configuración nginx con prioridad correcta${NC}" 