#!/bin/bash

# =============================================================================
# SCRIPT PARA REINICIAR SERVICIOS CON NUEVOS ENDPOINTS IMPLEMENTADOS
# CONFIGURADO PARA VPS - ESTRUCTURA MODULAR ADAPTADA
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

# Configuración de servicios y puertos para VPS
ALL_SERVICES=(
    "apple_auth:8100"
    "google_auth:8081"
    "signup:8082"
    "language_cookie:8083"
    "signin:8084"
    "fetch_dashboard:8085"
    "reset_password:8086"
    "dashboard_data:8087"
    "budget_management:8088"
    "savings_management:8089"
    "cash_bank_management:8090"
    "bills_management:8091"
    "profile_management:8092"
    "income_management:8093"
    "expense_management:8094"
    "transaction_delete_service:8095"
    "categories_management:8096"
    "money_flow_sync:8097"
    "budget_overview_fetch:8098"
    "user_locale:8099"
)

# Servicios críticos (se inician primero)
CRITICAL_SERVICES=(
    "google_auth:8081"
    "signin:8084"
    "fetch_dashboard:8085"
    "cash_bank_management:8090"
)

# Función para verificar puerto en uso
is_port_in_use() {
    local port=$1
    lsof -ti:$port >/dev/null 2>&1
}

# Función para detener procesos existentes
stop_all_services() {
    echo -e "${YELLOW}🛑 Deteniendo servicios existentes...${NC}"
    
    for service_info in "${ALL_SERVICES[@]}"; do
        local port=$(echo $service_info | cut -d':' -f2)
        local pid=$(lsof -ti:$port 2>/dev/null)
        if [ ! -z "$pid" ]; then
            echo -e "${YELLOW}  Deteniendo servicio en puerto $port (PID: $pid)${NC}"
            kill -9 $pid 2>/dev/null
        fi
    done
    
    sleep 2
    echo -e "${GREEN}✅ Servicios existentes detenidos${NC}"
}

# Función para verificar dependencias del sistema
check_system_dependencies() {
    echo -e "${YELLOW}🔍 Verificando dependencias del sistema...${NC}"
    
    if ! dpkg -l | grep -q libsqlite3-dev; then
        echo -e "${YELLOW}📦 Instalando libsqlite3-dev...${NC}"
        apt-get update && apt-get install -y libsqlite3-dev
    fi
    
    if ! dpkg -l | grep -q build-essential; then
        echo -e "${YELLOW}📦 Instalando build-essential...${NC}"
        apt-get install -y build-essential
    fi
    
    echo -e "${GREEN}✅ Dependencias verificadas${NC}"
}

# Función para iniciar un servicio
start_service() {
    local service_name=$1
    local port=$2
    local service_path="${BASE_PATH}/${service_name}"
    
    echo -e "${CYAN}🚀 Iniciando $service_name en puerto $port...${NC}"
    
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Error: Directorio $service_path no encontrado${NC}"
        return 1
    fi
    
    cd "$service_path" || return 1
    
    if [ ! -f "main.go" ]; then
        echo -e "${RED}❌ Error: main.go no encontrado en $service_path${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Inicializar go.mod si no existe
    if [ ! -f "go.mod" ]; then
        /usr/local/go/bin/go mod init $service_name >> "/tmp/${service_name}.log" 2>&1
    fi
    
    # Descargar dependencias
    /usr/local/go/bin/go mod tidy >> "/tmp/${service_name}.log" 2>&1
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    # Verificar compilación
    if ! /usr/local/go/bin/go build -o "/tmp/test_${service_name}" . >> "/tmp/${service_name}.log" 2>&1; then
        echo -e "${RED}    ❌ Error de compilación para $service_name${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    rm -f "/tmp/test_${service_name}"
    
    # Ejecutar en background
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run . > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado (PID: $pid)${NC}"
    cd "$BASE_PATH"
    sleep 1
}

# Función para verificar servicio
verify_service() {
    local service_name=$1
    local port=$2
    
    if is_port_in_use "$port"; then
        local pid=$(lsof -ti:$port 2>/dev/null)
        echo -e "${GREEN}  ✅ $service_name está activo (Puerto: $port, PID: $pid)${NC}"
        return 0
    else
        echo -e "${RED}  ❌ $service_name no está activo en puerto $port${NC}"
        return 1
    fi
}

# Función para mostrar estado de servicios
show_services_status() {
    echo -e "\n${WHITE}📊 ESTADO ACTUAL DE LOS SERVICIOS - VPS${NC}"
    echo -e "${WHITE}=============================================================================${NC}"
    
    local active_count=0
    for ((i=0; i<${#ALL_SERVICES[@]}; i++)); do
        local service_info="${ALL_SERVICES[$i]}"
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        local status_text="🔴 INACTIVO"
        local status_color="${RED}"
        
        if is_port_in_use "$port"; then
            status_text="🟢 ACTIVO"
            status_color="${GREEN}"
            ((active_count++))
        fi
        
        printf "${BLUE}%2d)${NC} %-35s ${YELLOW}:%s${NC} ${status_color}%s${NC}\n" \
            $((i+1)) "$service_name" "$port" "$status_text"
    done
    
    echo -e "\n${WHITE}📈 RESUMEN: ${GREEN}$active_count${NC}/${BLUE}${#ALL_SERVICES[@]}${NC} servicios activos${NC}"
}

# Función para mostrar ayuda
show_help() {
    echo -e "\n${WHITE}🔧 GESTIÓN DE MICROSERVICIOS GO - VPS${NC}"
    echo -e "${CYAN}Uso: $0 [OPCIÓN]${NC}"
    echo -e "\n${YELLOW}Opciones:${NC}"
    echo -e "  ${GREEN}-a, --all${NC}     Reiniciar TODOS los servicios automáticamente"
    echo -e "  ${GREEN}-s, --status${NC}  Mostrar estado actual de todos los servicios"
    echo -e "  ${GREEN}-h, --help${NC}    Mostrar esta ayuda"
    echo -e "\n${YELLOW}Ejemplos:${NC}"
    echo -e "  $0                 # Reiniciar todos los servicios"
    echo -e "  $0 --status        # Ver estado de servicios"
}

# Función principal de reinicio
restart_all_services() {
    echo -e "\n${WHITE}=== REINICIANDO TODOS LOS SERVICIOS - VPS ===${NC}"
    
    # Verificar directorio base
    if [ ! -d "$BASE_PATH" ]; then
        echo -e "${RED}❌ Error: El directorio base $BASE_PATH no existe${NC}"
        exit 1
    fi
    
    cd "$BASE_PATH" || exit 1
    echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"
    
    # Verificar dependencias y detener servicios
    check_system_dependencies
    stop_all_services
    
    # Iniciar servicios críticos primero
    echo -e "\n${CYAN}📋 INICIANDO SERVICIOS CRÍTICOS:${NC}"
    for service_info in "${CRITICAL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        start_service "$service_name" "$port"
    done
    
    # Iniciar resto de servicios
    echo -e "\n${CYAN}📋 INICIANDO RESTO DE SERVICIOS:${NC}"
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        
        if [[ ! " ${CRITICAL_SERVICES[@]} " =~ " ${service_info} " ]]; then
            start_service "$service_name" "$port"
        fi
    done
    
    # Verificación final
    echo -e "\n${WHITE}=== VERIFICACIÓN FINAL ===${NC}"
    echo -e "${YELLOW}⏳ Esperando 5 segundos para inicialización...${NC}"
    sleep 5
    
    local active_count=0
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        
        if verify_service "$service_name" "$port"; then
            ((active_count++))
        fi
    done
    
    # Mostrar resumen final
    echo -e "\n${WHITE}"
    echo "============================================================================="
    echo "   ✅ SERVICIOS REINICIADOS - VPS"
    echo "   📊 Servicios activos: $active_count/${#ALL_SERVICES[@]}"
    echo "============================================================================="
    echo -e "${NC}"
    
    if [ $active_count -eq ${#ALL_SERVICES[@]} ]; then
        echo -e "${GREEN}🎉 TODOS LOS SERVICIOS ESTÁN FUNCIONANDO${NC}"
    else
        echo -e "${YELLOW}⚠️  $active_count/${#ALL_SERVICES[@]} servicios funcionando${NC}"
        echo -e "${YELLOW}💡 Revisar logs en: /tmp/[servicio].log${NC}"
    fi
    
    # Mostrar URLs de servicios
    echo -e "\n${CYAN}📋 ENDPOINTS PRINCIPALES:${NC}"
    echo -e "${WHITE}  • Cash Update: http://localhost:8090/cash-bank/cash/update${NC}"
    echo -e "${WHITE}  • Bank Update: http://localhost:8090/cash-bank/bank/update${NC}"
    echo -e "${WHITE}  • Profile Update: http://localhost:8092/update/locale${NC}"
    echo -e "${WHITE}  • Money Flow: http://localhost:8097/money-flow/data${NC}"
    echo -e "${WHITE}  • User Locale: http://localhost:8099/user_locale/get${NC}"
}

# Función principal
main() {
    echo -e "${WHITE}"
    echo "============================================================================="
    echo "   🔄 GESTIÓN DE MICROSERVICIOS GO - VPS"
    echo "============================================================================="
    echo -e "${NC}"
    
    # Verificar argumentos de línea de comandos
    case "$1" in
        "--all"|"-a"|"")
            restart_all_services
            ;;
        "--status"|"-s")
            show_services_status
            ;;
        "--help"|"-h")
            show_help
            ;;
        *)
            echo -e "${RED}❌ Opción desconocida: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Ejecutar función principal con argumentos
main "$@" 