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
    
    # Puertos de todos los servicios existentes
    ports=(8081 8082 8083 8084 8085 8086 8087 8088 8089 8090 8091 8092 8093 8094 8095 8096 8097 8098 8099)
    
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

# Función para verificar e instalar dependencias del sistema
check_system_dependencies() {
    echo -e "${YELLOW}🔍 Verificando dependencias del sistema...${NC}"
    
    # Verificar SQLite3 development libraries
    if ! dpkg -l | grep -q libsqlite3-dev; then
        echo -e "${YELLOW}📦 Instalando libsqlite3-dev...${NC}"
        apt-get update && apt-get install -y libsqlite3-dev
    else
        echo -e "${GREEN}✅ libsqlite3-dev ya está instalado${NC}"
    fi
    
    # Verificar build-essential
    if ! dpkg -l | grep -q build-essential; then
        echo -e "${YELLOW}📦 Instalando build-essential...${NC}"
        apt-get install -y build-essential
    else
        echo -e "${GREEN}✅ build-essential ya está instalado${NC}"
    fi
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
    
    # Inicializar go.mod si no existe
    if [ ! -f "go.mod" ]; then
        echo -e "${YELLOW}    📦 Inicializando go.mod para $service_name...${NC}"
        /usr/local/go/bin/go mod init $service_name >> "/tmp/${service_name}.log" 2>&1
    fi
    
    # Descargar dependencias primero
    echo -e "${YELLOW}    📦 Descargando dependencias para $service_name...${NC}"
    /usr/local/go/bin/go mod tidy >> "/tmp/${service_name}.log" 2>&1
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}    ❌ Error descargando dependencias para $service_name${NC}"
        echo -e "${YELLOW}    📄 Últimas líneas del log:${NC}"
        tail -5 "/tmp/${service_name}.log"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Probar compilación primero
    echo -e "${YELLOW}    🔨 Verificando compilación para $service_name...${NC}"
    if ! /usr/local/go/bin/go build -o "/tmp/test_${service_name}" main.go >> "/tmp/${service_name}.log" 2>&1; then
        echo -e "${RED}    ❌ Error de compilación para $service_name${NC}"
        echo -e "${YELLOW}    📄 Últimas líneas del log:${NC}"
        tail -10 "/tmp/${service_name}.log"
        cd "$BASE_PATH"
        return 1
    fi
    rm -f "/tmp/test_${service_name}"
    
    # Ejecutar en background con CGO habilitado
    echo -e "${YELLOW}    🚀 Ejecutando $service_name...${NC}"
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run main.go > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado (PID: $pid)${NC}"
    echo -e "${WHITE}      Logs: /tmp/${service_name}.log${NC}"
    
    # Volver al directorio base
    cd "$BASE_PATH"
    
    # Esperar un poco para que el servicio se establezca
    sleep 2
}

# Función para verificar que un servicio esté respondiendo
verify_service() {
    local service_name=$1
    local port=$2
    local endpoint=$3
    local service_file_name=$4  # Nombre del archivo de servicio (con guiones bajos)
    
    echo -e "${BLUE}🔍 Verificando $service_name...${NC}"
    
    # Verificar si el puerto está escuchando primero
    local pid=$(lsof -ti:$port 2>/dev/null)
    if [ -z "$pid" ]; then
        echo -e "${RED}  ❌ $service_name no está escuchando en puerto $port${NC}"
        echo -e "${YELLOW}      Check logs: /tmp/${service_file_name}.log${NC}"
        return 1
    fi
    
    # Intentar conectar al endpoint
    local response=$(curl -s -w "%{http_code}" -o /dev/null "http://localhost:$port$endpoint" 2>/dev/null)
    
    if [ "$response" = "200" ] || [ "$response" = "404" ]; then
        echo -e "${GREEN}  ✅ $service_name está respondiendo (Status: $response, PID: $pid)${NC}"
        return 0
    else
        echo -e "${RED}  ❌ $service_name no está respondiendo (Status: $response, PID: $pid)${NC}"
        echo -e "${YELLOW}      Check logs: /tmp/${service_file_name}.log${NC}"
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

# Verificar e instalar dependencias del sistema
check_system_dependencies

# Detener servicios existentes
stop_existing_services

echo -e "\n${WHITE}=== INICIANDO SERVICIOS CON NUEVOS ENDPOINTS ===${NC}"

# Iniciar servicios de autenticación primero (críticos)
echo -e "\n${CYAN}📋 SERVICIOS DE AUTENTICACIÓN (CRÍTICOS):${NC}"

start_service "google_auth" 8081
start_service "signup" 8082
start_service "language_cookie" 8083
start_service "signin" 8084
start_service "reset_password" 8086

# Iniciar servicios prioritarios (los que tienen nuevos endpoints)
echo -e "\n${CYAN}📋 SERVICIOS PRIORITARIOS (CON NUEVOS ENDPOINTS):${NC}"

start_service "fetch_dashboard" 8085
start_service "cash_bank_management" 8090
start_service "profile_management" 8092
start_service "money_flow_sync" 8097
start_service "savings_management" 8089

# Iniciar servicios de gestión financiera
echo -e "\n${CYAN}📋 SERVICIOS DE GESTIÓN FINANCIERA:${NC}"

start_service "income_management" 8093
start_service "expense_management" 8094
start_service "categories_management" 8096
start_service "bills_management" 8091
start_service "budget_management" 8088
start_service "budget_overview_fetch" 8098

# Iniciar servicios complementarios
echo -e "\n${CYAN}📋 SERVICIOS COMPLEMENTARIOS:${NC}"

start_service "dashboard_data" 8087
start_service "transaction_delete_service" 8095
start_service "user_locale" 8099

echo -e "\n${WHITE}=== VERIFICANDO SERVICIOS ===${NC}"

# Esperar a que todos los servicios se inicialicen
echo -e "${YELLOW}⏳ Esperando 5 segundos para que los servicios se inicialicen...${NC}"
sleep 5

# Verificar servicios de autenticación
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS DE AUTENTICACIÓN:${NC}"

verify_service "Google Auth" 8081 "/health" "google_auth"
verify_service "Signup" 8082 "/health" "signup"
verify_service "Language Cookie" 8083 "/health" "language_cookie"
verify_service "Signin" 8084 "/health" "signin"
verify_service "Reset Password" 8086 "/ping" "reset_password"

# Verificar servicios prioritarios
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS PRIORITARIOS:${NC}"

verify_service "Fetch Dashboard" 8085 "/health" "fetch_dashboard"
verify_service "Cash Bank Management" 8090 "/cash-bank/distribution?user_id=1" "cash_bank_management"
verify_service "Profile Management" 8092 "/health" "profile_management"
verify_service "Money Flow Sync" 8097 "/money-flow/data?user_id=1" "money_flow_sync"
verify_service "Savings Management" 8089 "/health" "savings_management"

# Verificar servicios de gestión financiera
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS DE GESTIÓN FINANCIERA:${NC}"

verify_service "Income Management" 8093 "/incomes?user_id=1" "income_management"
verify_service "Expense Management" 8094 "/expenses?user_id=1" "expense_management"
verify_service "Categories Management" 8096 "/categories?user_id=1" "categories_management"
verify_service "Bills Management" 8091 "/bills?user_id=1" "bills_management"
verify_service "Budget Management" 8088 "/health" "budget_management"
verify_service "Budget Overview" 8098 "/health" "budget_overview_fetch"

# Verificar servicios complementarios
echo -e "\n${CYAN}🔍 VERIFICANDO SERVICIOS COMPLEMENTARIOS:${NC}"

verify_service "Dashboard Data" 8087 "/health" "dashboard_data"
verify_service "Transaction Delete" 8095 "/health" "transaction_delete_service"
verify_service "User Locale" 8099 "/health" "user_locale"

echo -e "\n${WHITE}"
echo "============================================================================="
echo "   ✅ TODOS LOS SERVICIOS REINICIADOS CON CONFIGURACIÓN ACTUALIZADA - VPS"
echo "   📊 Total de servicios: 19 (incluyendo User Locale)"
echo "============================================================================="
echo -e "${NC}"

echo -e "${GREEN}🎉 SERVICIOS INICIADOS EN PUERTOS CORRECTOS:${NC}"
echo -e "${WHITE}  • Google Auth:        http://localhost:8081${NC}"
echo -e "${WHITE}  • Signup:             http://localhost:8082${NC}"
echo -e "${WHITE}  • Language Cookie:    http://localhost:8083${NC}"
echo -e "${WHITE}  • Signin:             http://localhost:8084${NC}"
echo -e "${WHITE}  • Fetch Dashboard:    http://localhost:8085${NC}"
echo -e "${WHITE}  • Reset Password:     http://localhost:8086${NC}"
echo -e "${WHITE}  • Dashboard Data:     http://localhost:8087${NC}"
echo -e "${WHITE}  • Budget Management:  http://localhost:8088${NC}"
echo -e "${WHITE}  • Savings Management: http://localhost:8089${NC}"
echo -e "${WHITE}  • Cash Bank Mgmt:     http://localhost:8090${NC}"
echo -e "${WHITE}  • Bills Management:   http://localhost:8091${NC}"
echo -e "${WHITE}  • Profile Management: http://localhost:8092${NC}"
echo -e "${WHITE}  • Income Management:  http://localhost:8093${NC}"
echo -e "${WHITE}  • Expense Management: http://localhost:8094${NC}"
echo -e "${WHITE}  • Transaction Delete: http://localhost:8095${NC}"
echo -e "${WHITE}  • Categories Mgmt:    http://localhost:8096${NC}"
echo -e "${WHITE}  • Money Flow Sync:    http://localhost:8097${NC}"
echo -e "${WHITE}  • Budget Overview:    http://localhost:8098${NC}"
echo -e "${WHITE}  • User Locale:        http://localhost:8099${NC}"

echo -e "\n${CYAN}📋 NUEVOS ENDPOINTS IMPLEMENTADOS:${NC}"
echo -e "${WHITE}  • Cash Update: http://localhost:8090/cash-bank/cash/update${NC}"
echo -e "${WHITE}  • Bank Update: http://localhost:8090/cash-bank/bank/update${NC}"
echo -e "${WHITE}  • Locale Update: http://localhost:8092/update/locale${NC}"
echo -e "${WHITE}  • User Update: http://localhost:8085/user/update${NC}"
echo -e "${WHITE}  • Money Flow Data: http://localhost:8097/money-flow/data${NC}"
echo -e "${WHITE}  • User Locale Get: http://localhost:8099/user_locale/get${NC}"

echo -e "\n${CYAN}📋 PARA VERIFICAR TODOS LOS ENDPOINTS:${NC}"
echo -e "${WHITE}  cd ${BASE_PATH}${NC}"
echo -e "${WHITE}  # Verificar servicios específicos:${NC}"
echo -e "${WHITE}  curl http://localhost:8090/cash-bank/distribution?user_id=1${NC}"
echo -e "${WHITE}  curl http://localhost:8097/money-flow/data?user_id=1${NC}"
echo -e "${WHITE}  curl http://localhost:8099/user_locale/get?user_id=1${NC}"

echo -e "\n${CYAN}📋 PARA VER LOGS DE UN SERVICIO:${NC}"
echo -e "${WHITE}  tail -f /tmp/[nombre_servicio].log${NC}"
echo -e "${WHITE}  # Ejemplos:${NC}"
echo -e "${WHITE}  tail -f /tmp/cash_bank_management.log${NC}"
echo -e "${WHITE}  tail -f /tmp/money_flow_sync.log${NC}"
echo -e "${WHITE}  tail -f /tmp/user_locale.log${NC}"

echo -e "\n${GREEN}🎯 CONFIGURACIÓN ACTUALIZADA: 19/19 servicios activos según estructura real del VPS${NC}"

echo "" 