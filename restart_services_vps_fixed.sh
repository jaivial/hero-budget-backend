#!/bin/bash

# =============================================================================
# SCRIPT PARA REINICIAR SERVICIOS CON NUEVOS ENDPOINTS IMPLEMENTADOS
# CONFIGURADO PARA VPS - RUTAS ABSOLUTAS
# =============================================================================

# Configuración de rutas del VPS
BASE_PATH="/opt/hero_budget/backend"

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
echo "   🔄 REINICIANDO SERVICIOS CON NUEVOS ENDPOINTS IMPLEMENTADOS - VPS"
echo "============================================================================="
echo -e "${NC}"

# Función para detener procesos existentes
stop_existing_services() {
    echo -e "${YELLOW}🛑 Deteniendo servicios existentes...${NC}"
    
    # Encontrar y matar procesos de Go en los puertos específicos
    ports=(8081 8082 8083 8084 8085 8086 8087 8088 8089 8090 8091 8092 8093 8094 8095 8096 8097 8098)
    
    for port in "${ports[@]}"; do
        PID=$(lsof -ti:$port 2>/dev/null)
        if [ ! -z "$PID" ]; then
            echo -e "${YELLOW}  Deteniendo servicio en puerto $port (PID: $PID)${NC}"
            kill -9 $PID 2>/dev/null
        fi
    done
    
    sleep 2
    echo -e "${GREEN}✅ Servicios existentes detenidos${NC}"
}

# Función para iniciar un servicio
start_service() {
    local service_name=$1
    local port=$2
    local service_path="${BASE_PATH}/${service_name}"
    
    echo -e "${CYAN}🚀 Iniciando $service_name en puerto $port...${NC}"
    
    # Verificar que el directorio existe
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Error: Directorio $service_path no encontrado${NC}"
        return 1
    fi
    
    # Cambiar al directorio del servicio
    cd "$service_path" || { echo -e "${RED}❌ Error: No se pudo acceder a $service_path${NC}"; return 1; }
    
    # Verificar que existe main.go
    if [ ! -f "main.go" ]; then
        echo -e "${RED}❌ Error: main.go no encontrado en $service_path${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Descargar dependencias primero
    echo -e "${YELLOW}    📦 Descargando dependencias para $service_name...${NC}"
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}    ❌ Error descargando dependencias para $service_name${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Compilar y ejecutar en background con CGO habilitado
    echo -e "${YELLOW}    🔨 Compilando y ejecutando $service_name...${NC}"
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run main.go > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado (PID: $pid)${NC}"
    echo -e "${WHITE}      Logs: /tmp/${service_name}.log${NC}"
    
    # Volver al directorio base
    cd "$BASE_PATH"
    
    # Esperar un poco para que el servicio se establezca
    sleep 1
}

# Función para verificar que un servicio esté respondiendo
verify_service() {
    local service_name=$1
    local port=$2
    local endpoint=$3
    
    echo -e "${BLUE}🔍 Verificando $service_name...${NC}"
    
    # Intentar conectar al endpoint
    local response=$(curl -s -w "%{http_code}" -o /dev/null "http://localhost:$port$endpoint" 2>/dev/null)
    
    if [ "$response" = "200" ] || [ "$response" = "404" ]; then
        echo -e "${GREEN}  ✅ $service_name está respondiendo (Status: $response)${NC}"
        return 0
    else
        echo -e "${RED}  ❌ $service_name no está respondiendo (Status: $response)${NC}"
        echo -e "${YELLOW}      Check logs: /tmp/${service_name,,}.log${NC}"
        return 1
    fi
}

# Verificar que estamos en el directorio correcto
if [ ! -d "$BASE_PATH" ]; then
    echo -e "${RED}❌ Error: El directorio base $BASE_PATH no existe${NC}"
    exit 1
fi

# Cambiar al directorio base
cd "$BASE_PATH" || { echo -e "${RED}❌ Error: No se pudo acceder a $BASE_PATH${NC}"; exit 1; }

echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"

# Detener servicios existentes
stop_existing_services

echo -e "\n${WHITE}=== INICIANDO SERVICIOS CON NUEVOS ENDPOINTS ===${NC}"

# Iniciar servicios prioritarios (los que hemos modificado)
echo -e "\n${CYAN}📋 SERVICIOS PRIORITARIOS (CON NUEVOS ENDPOINTS):${NC}"

start_service "cash_bank_management" 8090
start_service "profile_management" 8092
start_service "fetch_dashboard" 8085
start_service "money_flow_sync" 8097
start_service "savings_management" 8089

# Iniciar servicios críticos adicionales
echo -e "\n${CYAN}📋 SERVICIOS CRÍTICOS ADICIONALES:${NC}"

start_service "categories_management" 8096
start_service "income_management" 8093
start_service "expense_management" 8094
start_service "budget_overview_fetch" 8098
start_service "bills_management" 8091

# Iniciar servicios de autenticación
echo -e "\n${CYAN}📋 SERVICIOS DE AUTENTICACIÓN:${NC}"

start_service "signin" 8084
start_service "signup" 8082
start_service "google_auth" 8081
start_service "reset_password" 8086

# Iniciar servicios complementarios
echo -e "\n${CYAN}📋 SERVICIOS COMPLEMENTARIOS:${NC}"

start_service "language_cookie" 8083
start_service "dashboard_data" 8087
start_service "budget_management" 8088
start_service "transaction_delete_service" 8095

echo -e "\n${WHITE}=== VERIFICANDO SERVICIOS ===${NC}"

# Esperar a que todos los servicios se inicialicen
echo -e "${YELLOW}⏳ Esperando 5 segundos para que los servicios se inicialicen...${NC}"
sleep 5

# Verificar servicios prioritarios
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS PRIORITARIOS:${NC}"

verify_service "Cash Bank Management" 8090 "/cash-bank/distribution?user_id=1"
verify_service "Profile Management" 8092 "/health"
verify_service "Fetch Dashboard" 8085 "/health"
verify_service "Money Flow Sync" 8097 "/money-flow/data?user_id=1"
verify_service "Savings Management" 8089 "/health"

# Verificar servicios críticos
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS CRÍTICOS:${NC}"

verify_service "Categories Management" 8096 "/categories?user_id=1"
verify_service "Income Management" 8093 "/incomes?user_id=1"
verify_service "Expense Management" 8094 "/expenses?user_id=1"
verify_service "Budget Overview" 8098 "/health"
verify_service "Bills Management" 8091 "/bills?user_id=1"

# Verificar servicios de autenticación
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS DE AUTENTICACIÓN:${NC}"

verify_service "Google Auth" 8081 "/health"
verify_service "Signin" 8084 "/health"
verify_service "Signup" 8082 "/health" 
verify_service "Reset Password" 8086 "/health"

echo -e "\n${WHITE}"
echo "============================================================================="
echo "   ✅ SERVICIOS REINICIADOS CON NUEVOS ENDPOINTS - VPS"
echo "============================================================================="
echo -e "${NC}"

echo -e "${GREEN}🎉 NUEVOS ENDPOINTS IMPLEMENTADOS:${NC}"
echo -e "${WHITE}  • Cash Update: http://localhost:8090/cash-bank/cash/update${NC}"
echo -e "${WHITE}  • Bank Update: http://localhost:8090/cash-bank/bank/update${NC}"
echo -e "${WHITE}  • Locale Update: http://localhost:8092/update/locale${NC}"
echo -e "${WHITE}  • User Update: http://localhost:8085/user/update${NC}"
echo -e "${WHITE}  • Money Flow Data: http://localhost:8097/money-flow/data${NC}"
echo -e "${WHITE}  • Savings Health: http://localhost:8089/health${NC}"
echo -e "${WHITE}  • Dashboard Health: http://localhost:8085/health${NC}"

echo -e "\n${CYAN}📋 PARA VERIFICAR LOS NUEVOS ENDPOINTS:${NC}"
echo -e "${WHITE}  cd ${BASE_PATH}${NC}"
echo -e "${WHITE}  ./tests/endpoints/test_endpoints_final_solution.sh${NC}"

echo -e "\n${CYAN}📋 PARA VER LOGS DE UN SERVICIO:${NC}"
echo -e "${WHITE}  tail -f /tmp/[nombre_servicio].log${NC}"

echo -e "\n${GREEN}🎯 OBJETIVO: Pasar de 17/25 a 25/25 endpoints funcionando${NC}"

echo "" 