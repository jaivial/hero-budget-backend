#!/bin/bash

# =============================================================================
# SCRIPT DE VERIFICACIÓN VPS - SERVICIOS HERO BUDGET
# =============================================================================

# Configuración de colores
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
echo "   🖥️  VERIFICACIÓN DE SERVICIOS VPS - HERO BUDGET"
echo "   🌐 IP: $VPS_IP"
echo "   📊 Verificando 9 servicios críticos (incluye User Locale)"
echo "============================================================================="
echo -e "${NC}"

echo -e "${BLUE}🔍 Verificando conexión SSH...${NC}"
ssh -o ConnectTimeout=10 -o BatchMode=yes $VPS_USER@$VPS_IP 'echo "SSH conectado correctamente"' 2>/dev/null
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Conexión SSH exitosa${NC}"
else
    echo -e "${RED}❌ Error de conexión SSH${NC}"
    exit 1
fi

echo -e "\n${WHITE}=== 🏥 ESTADO GENERAL DEL SISTEMA ===${NC}\n"

echo -e "${CYAN}📊 Verificando systemctl status herobudget...${NC}"
ssh $VPS_USER@$VPS_IP 'systemctl status herobudget --no-pager -l' 2>/dev/null || echo -e "${YELLOW}⚠️  Servicio herobudget no encontrado o error${NC}"

echo -e "\n${CYAN}📊 Verificando procesos Hero Budget...${NC}"
ssh $VPS_USER@$VPS_IP 'ps aux | grep -E "(hero_budget|main\.go)" | grep -v grep' 2>/dev/null || echo -e "${YELLOW}⚠️  No se encontraron procesos hero_budget${NC}"

echo -e "\n${WHITE}=== 🔌 VERIFICACIÓN DE PUERTOS ESPECÍFICOS ===${NC}\n"

# Lista de servicios críticos que están fallando
CRITICAL_SERVICES=(
    "8082:signup_service"
    "8089:savings_service"
    "8091:bills_service"
    "8093:income_service"
    "8094:expense_service"
    "8085:fetch_dashboard_service"
    "8092:profile_service"
    "8098:budget_overview_service"
    "8099:user_locale_service"
)

echo -e "${CYAN}🔍 Verificando servicios críticos...${NC}"

for service in "${CRITICAL_SERVICES[@]}"; do
    port=$(echo $service | cut -d: -f1)
    name=$(echo $service | cut -d: -f2)
    
    echo -e "${BLUE}Probando puerto $port ($name)...${NC}"
    
    # Verificar si el puerto está en uso
    ssh $VPS_USER@$VPS_IP "netstat -tulpn | grep :$port" 2>/dev/null
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  ✅ Puerto $port activo${NC}"
        
        # Probar health check
        health_response=$(ssh $VPS_USER@$VPS_IP "curl -s -o /dev/null -w '%{http_code}' http://localhost:$port/health" 2>/dev/null)
        if [ "$health_response" = "200" ]; then
            echo -e "${GREEN}  ✅ Health check OK ($health_response)${NC}"
        else
            echo -e "${YELLOW}  ⚠️  Health check: $health_response${NC}"
        fi
    else
        echo -e "${RED}  ❌ Puerto $port NO activo${NC}"
    fi
    echo ""
done

echo -e "${WHITE}=== 🔧 VERIFICACIÓN DE NGINX ===${NC}\n"

echo -e "${CYAN}📋 Verificando configuración nginx activa...${NC}"
ssh $VPS_USER@$VPS_IP 'nginx -t' 2>&1 | head -10

echo -e "\n${CYAN}📋 Verificando archivo de configuración actual...${NC}"
ssh $VPS_USER@$VPS_IP 'ls -la /etc/nginx/sites-available/herobudget' 2>/dev/null
ssh $VPS_USER@$VPS_IP 'ls -la /etc/nginx/sites-enabled/herobudget' 2>/dev/null

echo -e "\n${CYAN}📋 Últimas líneas del log de nginx...${NC}"
ssh $VPS_USER@$VPS_IP 'tail -10 /var/log/nginx/herobudget_error.log' 2>/dev/null || echo -e "${YELLOW}⚠️  Log de nginx no encontrado${NC}"

echo -e "\n${WHITE}=== 🚀 TESTS LOCALES DE SERVICIOS ===${NC}\n"

echo -e "${CYAN}🔍 Probando endpoints localmente en VPS...${NC}"

# Tests específicos de los servicios que fallan
FAILED_ENDPOINTS=(
    "8082:/signup/check-email:POST"
    "8089:/savings/fetch:GET"
    "8091:/bills/add:POST"
    "8093:/incomes/add:POST"
    "8094:/expenses/add:POST"
)

for endpoint in "${FAILED_ENDPOINTS[@]}"; do
    port=$(echo $endpoint | cut -d: -f1)
    path=$(echo $endpoint | cut -d: -f2)
    method=$(echo $endpoint | cut -d: -f3)
    
    echo -e "${BLUE}Probando $method localhost:$port$path...${NC}"
    
    if [ "$method" = "GET" ]; then
        response=$(ssh $VPS_USER@$VPS_IP "curl -s -o /dev/null -w '%{http_code}' http://localhost:$port$path" 2>/dev/null)
    else
        response=$(ssh $VPS_USER@$VPS_IP "curl -s -o /dev/null -w '%{http_code}' -X $method -H 'Content-Type: application/json' -d '{}' http://localhost:$port$path" 2>/dev/null)
    fi
    
    case $response in
        200|201)
            echo -e "${GREEN}  ✅ Servicio responde: $response${NC}"
            ;;
        400|404|405)
            echo -e "${YELLOW}  ⚠️  Respuesta: $response (Servicio activo)${NC}"
            ;;
        000|502|503)
            echo -e "${RED}  ❌ Servicio NO responde: $response${NC}"
            ;;
        *)
            echo -e "${YELLOW}  ⚠️  Respuesta inesperada: $response${NC}"
            ;;
    esac
done

echo -e "\n${WHITE}=== 📊 RESUMEN Y RECOMENDACIONES ===${NC}\n"

echo -e "${GREEN}🚀 COMANDOS PARA CORRECCIÓN INMEDIATA:${NC}"

echo -e "\n${CYAN}1. APLICAR CONFIGURACIÓN NGINX CORREGIDA:${NC}"
echo -e "${WHITE}scp nginx-herobudget-https-fixed.conf $VPS_USER@$VPS_IP:/etc/nginx/sites-available/herobudget${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'nginx -t && systemctl reload nginx'${NC}"

echo -e "\n${CYAN}2. VERIFICAR/REINICIAR SERVICIOS HERO BUDGET:${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'systemctl status herobudget'${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'systemctl restart herobudget'${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'systemctl enable herobudget'${NC}"

echo -e "\n${CYAN}3. VERIFICAR LOGS EN TIEMPO REAL:${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'tail -f /var/log/nginx/herobudget_error.log'${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'journalctl -u herobudget -f'${NC}"

echo -e "\n${CYAN}4. TESTS INMEDIATOS POST-CORRECCIÓN:${NC}"
echo -e "${WHITE}bash test_nginx_fixes.sh${NC}"
echo -e "${WHITE}bash test_production_endpoints.sh${NC}"

echo -e "\n${CYAN}5. VERIFICAR NUEVO SERVICIO USER LOCALE:${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'curl http://localhost:8099/health'${NC}"
echo -e "${WHITE}ssh $VPS_USER@$VPS_IP 'curl http://localhost:8099/user_locale/get?user_id=2'${NC}"

echo -e "\n${GREEN}📋 EJECUTAR ESTOS COMANDOS EN SECUENCIA PARA SOLUCIONAR${NC}" 