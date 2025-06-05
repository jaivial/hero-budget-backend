#!/bin/bash

# =============================================================================
# VERIFICACIÓN RÁPIDA DEL ESTADO DE SERVICIOS - VPS
# =============================================================================

# Configuración de colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

echo -e "${WHITE}"
echo "============================================================================="
echo "   📊 ESTADO ACTUAL DE SERVICIOS GO - VPS"
echo "============================================================================="
echo -e "${NC}"

# Lista de servicios y sus puertos
declare -A SERVICES=(
    ["google_auth"]="8081"
    ["signup"]="8082"
    ["language_cookie"]="8083"
    ["signin"]="8084"
    ["fetch_dashboard"]="8085"
    ["reset_password"]="8086"
    ["dashboard_data"]="8087"
    ["budget_management"]="8088"
    ["savings_management"]="8089"
    ["cash_bank_management"]="8090"
    ["bills_management"]="8091"
    ["profile_management"]="8092"
    ["income_management"]="8093"
    ["expense_management"]="8094"
    ["transaction_delete_service"]="8095"
    ["categories_management"]="8096"
    ["money_flow_sync"]="8097"
    ["budget_overview_fetch"]="8098"
)

# Contadores
total_services=0
running_services=0
failed_services=0

echo -e "${CYAN}🔍 VERIFICANDO SERVICIOS:${NC}"

for service in "${!SERVICES[@]}"; do
    total_services=$((total_services + 1))
    port="${SERVICES[$service]}"
    
    # Verificar si el puerto está en uso
    pid=$(lsof -ti:$port 2>/dev/null)
    
    if [ ! -z "$pid" ]; then
        # Servicio está corriendo
        echo -e "${GREEN}  ✅ $service (puerto $port, PID: $pid)${NC}"
        running_services=$((running_services + 1))
    else
        # Servicio no está corriendo
        echo -e "${RED}  ❌ $service (puerto $port - NO ACTIVO)${NC}"
        failed_services=$((failed_services + 1))
        
        # Verificar si hay logs de error recientes
        log_file="/tmp/${service}.log"
        if [ -f "$log_file" ]; then
            # Buscar errores en las últimas líneas
            if tail -5 "$log_file" 2>/dev/null | grep -qi "error\|fatal\|panic"; then
                echo -e "${YELLOW}    ⚠️  Errores detectados en log${NC}"
            fi
        fi
    fi
done

echo -e "\n${WHITE}=== RESUMEN ===${NC}"
echo -e "${GREEN}✅ Servicios activos: $running_services/$total_services${NC}"
echo -e "${RED}❌ Servicios fallidos: $failed_services/$total_services${NC}"

# Calcular porcentaje
if [ $total_services -gt 0 ]; then
    percentage=$((running_services * 100 / total_services))
    echo -e "${CYAN}📊 Porcentaje de éxito: $percentage%${NC}"
fi

# Mostrar puertos activos
echo -e "\n${WHITE}=== PUERTOS ACTIVOS (808x/809x) ===${NC}"
active_ports=$(lsof -i -P -n 2>/dev/null | grep -E ':(808[0-9]|809[0-9])' | grep LISTEN)

if [ -z "$active_ports" ]; then
    echo -e "${RED}❌ No hay servicios escuchando en puertos 808x/809x${NC}"
else
    echo "$active_ports" | while IFS= read -r line; do
        echo -e "${WHITE}  $line${NC}"
    done
fi

# Recomendaciones basadas en el estado
echo -e "\n${CYAN}💡 RECOMENDACIONES:${NC}"

if [ $running_services -eq $total_services ]; then
    echo -e "${GREEN}🎉 ¡Todos los servicios están funcionando correctamente!${NC}"
elif [ $running_services -eq 0 ]; then
    echo -e "${RED}🚨 Ningún servicio está activo. Ejecutar:${NC}"
    echo -e "${WHITE}  ./restart_services_vps.sh${NC}"
else
    echo -e "${YELLOW}⚠️  Algunos servicios están fallando. Para diagnóstico detallado:${NC}"
    echo -e "${WHITE}  ./diagnose_vps_services.sh${NC}"
    echo -e "${WHITE}  Para reiniciar todos los servicios:${NC}"
    echo -e "${WHITE}  ./restart_services_vps.sh${NC}"
fi

echo "" 