#!/bin/bash

# =============================================================================
# SCRIPT DE DIAGNÓSTICO PARA SERVICIOS FALLIDOS EN VPS
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
echo "   🔍 DIAGNÓSTICO DE SERVICIOS FALLIDOS - VPS"
echo "   📊 Analizando 19 servicios (incluye User Locale)"
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
    ["user_locale"]="8099"
)

# Función para verificar si un puerto está en uso
check_port() {
    local port=$1
    local pid=$(lsof -ti:$port 2>/dev/null)
    if [ ! -z "$pid" ]; then
        echo "✅ Puerto $port está en uso (PID: $pid)"
        return 0
    else
        echo "❌ Puerto $port no está en uso"
        return 1
    fi
}

# Función para revisar logs de un servicio
check_service_logs() {
    local service_name=$1
    local log_file="/tmp/${service_name}.log"
    
    echo -e "\n${CYAN}📋 REVISANDO $service_name:${NC}"
    
    # Verificar si existe el directorio del servicio
    if [ ! -d "$service_name" ]; then
        echo -e "${RED}❌ Directorio $service_name no existe${NC}"
        return 1
    fi
    
    # Verificar si existe main.go
    if [ ! -f "$service_name/main.go" ]; then
        echo -e "${RED}❌ main.go no encontrado en $service_name${NC}"
        return 1
    fi
    
    # Verificar puerto
    local port="${SERVICES[$service_name]}"
    check_port "$port"
    
    # Revisar logs si existen
    if [ -f "$log_file" ]; then
        echo -e "${YELLOW}📄 Últimas líneas del log:${NC}"
        tail -10 "$log_file" | while IFS= read -r line; do
            if [[ "$line" =~ error|Error|ERROR|fatal|Fatal|FATAL|panic ]]; then
                echo -e "${RED}  $line${NC}"
            elif [[ "$line" =~ warning|Warning|WARN ]]; then
                echo -e "${YELLOW}  $line${NC}"
            else
                echo -e "${WHITE}  $line${NC}"
            fi
        done
    else
        echo -e "${RED}❌ Log file no encontrado: $log_file${NC}"
    fi
    
    # Intentar compilar para ver errores
    echo -e "${BLUE}🔨 Probando compilación...${NC}"
    cd "$service_name" 2>/dev/null || { echo -e "${RED}❌ No se puede acceder al directorio${NC}"; return 1; }
    
    local compile_output
    compile_output=$(go build -o /tmp/test_${service_name} main.go 2>&1)
    local compile_status=$?
    
    if [ $compile_status -eq 0 ]; then
        echo -e "${GREEN}✅ Compilación exitosa${NC}"
        rm -f "/tmp/test_${service_name}"
    else
        echo -e "${RED}❌ Error de compilación:${NC}"
        echo "$compile_output" | while IFS= read -r line; do
            echo -e "${RED}  $line${NC}"
        done
    fi
    
    cd .. 2>/dev/null
}

# Función para verificar dependencias globales
check_dependencies() {
    echo -e "\n${WHITE}=== VERIFICANDO DEPENDENCIAS GLOBALES ===${NC}"
    
    # Verificar Go
    if command -v go &> /dev/null; then
        local go_version=$(go version)
        echo -e "${GREEN}✅ Go instalado: $go_version${NC}"
    else
        echo -e "${RED}❌ Go no está instalado${NC}"
    fi
    
    # Verificar CGO
    if go env CGO_ENABLED | grep -q "1"; then
        echo -e "${GREEN}✅ CGO habilitado${NC}"
    else
        echo -e "${YELLOW}⚠️  CGO deshabilitado${NC}"
    fi
    
    # Verificar SQLite3
    if [ -f "/usr/lib/x86_64-linux-gnu/libsqlite3.so" ] || [ -f "/usr/local/lib/libsqlite3.so" ]; then
        echo -e "${GREEN}✅ SQLite3 library encontrada${NC}"
    else
        echo -e "${RED}❌ SQLite3 library no encontrada${NC}"
        echo -e "${YELLOW}💡 Instalar con: apt-get install libsqlite3-dev${NC}"
    fi
    
    # Verificar base de datos
    if [ -f "google_auth/users.db" ]; then
        echo -e "${GREEN}✅ Base de datos encontrada${NC}"
    else
        echo -e "${YELLOW}⚠️  Base de datos no encontrada en google_auth/users.db${NC}"
    fi
}

# Función para mostrar resumen de puertos activos
show_active_ports() {
    echo -e "\n${WHITE}=== PUERTOS ACTIVOS ===${NC}"
    local active_ports=$(lsof -i -P -n | grep -E ':(808[0-9]|809[0-9])' | grep LISTEN)
    
    if [ -z "$active_ports" ]; then
        echo -e "${RED}❌ No hay servicios escuchando en puertos 808x o 809x${NC}"
    else
        echo -e "${GREEN}✅ Servicios activos:${NC}"
        echo "$active_ports" | while IFS= read -r line; do
            echo -e "${WHITE}  $line${NC}"
        done
    fi
}

# Ejecutar diagnóstico
echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"

check_dependencies
show_active_ports

echo -e "\n${WHITE}=== DIAGNÓSTICO INDIVIDUAL DE SERVICIOS ===${NC}"

# Revisar servicios que deberían estar funcionando pero no están
for service in "${!SERVICES[@]}"; do
    check_service_logs "$service"
done

echo -e "\n${WHITE}"
echo "============================================================================="
echo "   🎯 DIAGNÓSTICO COMPLETADO"
echo "============================================================================="
echo -e "${NC}"

echo -e "${CYAN}💡 ACCIONES RECOMENDADAS:${NC}"
echo -e "${WHITE}1. Si hay errores de SQLite3: apt-get install libsqlite3-dev${NC}"
echo -e "${WHITE}2. Si hay errores de Go modules: go mod tidy en cada servicio${NC}"
echo -e "${WHITE}3. Si hay errores de permisos: chown -R root:root /opt/hero_budget${NC}"
echo -e "${WHITE}4. Si hay errores de compilación: revisar sintaxis Go${NC}"

echo "" 