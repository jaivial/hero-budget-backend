#!/bin/bash

# =============================================================================
# SCRIPT PARA REINICIAR SERVICIOS SIN PÉRDIDA DE DATOS - VPS
# VERSIÓN MEJORADA: PROTEGE DATOS EXISTENTES
# =============================================================================

# Configuración de rutas del VPS
BASE_PATH="/opt/hero_budget/backend"
DB_PATH="/opt/hero_budget/backend/google_auth/users.db"

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
    "apple-auth:8100"
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

# Función para crear backup de la base de datos
create_database_backup() {
    echo -e "${CYAN}🔒 Creando backup de la base de datos antes del reinicio...${NC}"
    
    if [ -f "$DB_PATH" ]; then
        local backup_name="users_backup_$(date +%Y%m%d_%H%M%S).db"
        local backup_path="/opt/hero_budget/backend/google_auth/$backup_name"
        
        cp "$DB_PATH" "$backup_path"
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✅ Backup creado: $backup_path${NC}"
            echo -e "${WHITE}📁 Tamaño del backup: $(du -h "$backup_path" | cut -f1)${NC}"
            
            # Mantener solo los últimos 5 backups
            cd "/opt/hero_budget/backend/google_auth/"
            ls -t users_backup_*.db 2>/dev/null | tail -n +6 | xargs rm -f 2>/dev/null
            local backup_count=$(ls users_backup_*.db 2>/dev/null | wc -l)
            echo -e "${BLUE}📊 Backups mantenidos: $backup_count/5${NC}"
        else
            echo -e "${RED}❌ Error creando backup${NC}"
            return 1
        fi
    else
        echo -e "${YELLOW}⚠️  Base de datos no encontrada en: $DB_PATH${NC}"
        echo -e "${YELLOW}💡 Se creará nueva base de datos si es necesario${NC}"
    fi
}

# Función para verificar integridad de la base de datos
verify_database_integrity() {
    echo -e "${CYAN}🔍 Verificando integridad de la base de datos...${NC}"
    
    if [ ! -f "$DB_PATH" ]; then
        echo -e "${YELLOW}⚠️  Base de datos no existe, se creará durante la inicialización${NC}"
        return 0
    fi
    
    # Verificar que la base de datos no esté corrupta
    local integrity_check=$(sqlite3 "$DB_PATH" "PRAGMA integrity_check;" 2>/dev/null)
    if [ "$integrity_check" = "ok" ]; then
        echo -e "${GREEN}✅ Base de datos íntegra${NC}"
        
        # Mostrar estadísticas de datos existentes
        local user_count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "0")
        local income_count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM incomes;" 2>/dev/null || echo "0")
        local expense_count=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM expenses;" 2>/dev/null || echo "0")
        
        echo -e "${BLUE}📊 Datos existentes:${NC}"
        echo -e "${WHITE}  • Usuarios: $user_count${NC}"
        echo -e "${WHITE}  • Ingresos: $income_count${NC}"
        echo -e "${WHITE}  • Gastos: $expense_count${NC}"
        
        return 0
    else
        echo -e "${RED}❌ Base de datos corrupta o inaccesible${NC}"
        echo -e "${YELLOW}💡 Se utilizará el backup más reciente${NC}"
        return 1
    fi
}

# Función para restaurar backup si es necesario
restore_backup_if_needed() {
    if ! verify_database_integrity; then
        echo -e "${YELLOW}🔄 Restaurando desde backup...${NC}"
        
        local latest_backup=$(ls -t /opt/hero_budget/backend/google_auth/users_backup_*.db 2>/dev/null | head -n 1)
        if [ ! -z "$latest_backup" ]; then
            cp "$latest_backup" "$DB_PATH"
            if verify_database_integrity; then
                echo -e "${GREEN}✅ Base de datos restaurada desde backup${NC}"
            else
                echo -e "${RED}❌ Error restaurando backup${NC}"
                return 1
            fi
        else
            echo -e "${YELLOW}⚠️  No hay backups disponibles${NC}"
        fi
    fi
}

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
            kill -15 $pid 2>/dev/null  # SIGTERM primero
            sleep 2
            if kill -0 $pid 2>/dev/null; then
                kill -9 $pid 2>/dev/null  # SIGKILL si no responde
            fi
        fi
    done
    
    sleep 3
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
    
    # Verificar si existe main.go o archivos main_part*.go (para servicios divididos)
    if [ ! -f "main.go" ] && [ ! -f "main_part1.go" ]; then
        echo -e "${RED}❌ Error: main.go o main_part1.go no encontrado en $service_path${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Limpiar archivos conflictivos específicos para este servicio
    cd "$BASE_PATH"
    clean_conflicting_files "$service_name"
    cd "$service_path"
    
    # Inicializar go.mod si no existe
    if [ ! -f "go.mod" ]; then
        /usr/local/go/bin/go mod init $service_name >> "/tmp/${service_name}.log" 2>&1
    fi
    
    # Descargar dependencias
    /usr/local/go/bin/go mod tidy >> "/tmp/${service_name}.log" 2>&1
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    # Verificar compilación con manejo mejorado de errores
    if ! /usr/local/go/bin/go build -buildvcs=false -o "/tmp/test_${service_name}" . >> "/tmp/${service_name}.log" 2>&1; then
        echo -e "${RED}    ❌ Error de compilación para $service_name${NC}"
        echo -e "${YELLOW}    📋 Error de compilación:${NC}"
        tail -5 "/tmp/${service_name}.log" | sed 's/^/    /'
        cd "$BASE_PATH"
        return 1
    fi
    rm -f "/tmp/test_${service_name}"
    
    # Ejecutar en background
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run -buildvcs=false . > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado (PID: $pid)${NC}"
    cd "$BASE_PATH"
    sleep 1
}

# Función para limpiar archivos conflictivos específicos de un servicio
clean_conflicting_files() {
    local service_name=$1
    
    case "$service_name" in
        "bills_management")
            if [ -f "bills_management/main.go" ]; then
                echo -e "${YELLOW}  Limpiando bills_management/main.go conflictivo${NC}"
                rm -f "bills_management/main.go"
            fi
            ;;
    esac
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

# Función para actualizar dependencias de todos los servicios
update_all_dependencies() {
    echo -e "\n${CYAN}🔧 Actualizando dependencias de todos los servicios...${NC}"
    
    local services=(
        "apple-auth" "bills_management" "budget_management" "budget_overview_fetch"
        "cash_bank_management" "categories_management" "dashboard_data" "expense_management"
        "fetch_dashboard" "google_auth" "income_management" "money_flow_sync"
        "profile_management" "reset_password" "signin" "signup" "user_locale"
        "savings_management" "language_cookie" "transaction_delete_service"
    )
    
    for service in "${services[@]}"; do
        if [ -d "$service" ]; then
            echo -e "${BLUE}  🔧 Actualizando dependencias: $service${NC}"
            cd "$service"
            /usr/local/go/bin/go mod tidy > /dev/null 2>&1
            /usr/local/go/bin/go mod download > /dev/null 2>&1
            cd "$BASE_PATH"
        fi
    done
    
    echo -e "${GREEN}✅ Dependencias actualizadas${NC}"
}

# Función para iniciar servicio con flags específicos
start_service_with_flags() {
    local service_name=$1
    local port=$2
    local flags=$3
    local service_path="${BASE_PATH}/${service_name}"
    
    echo -e "${CYAN}🚀 Iniciando $service_name en puerto $port con flags: $flags...${NC}"
    
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Error: Directorio $service_path no encontrado${NC}"
        return 1
    fi
    
    cd "$service_path" || return 1
    
    # Verificar si existe main.go o archivos main_part*.go (para servicios divididos)
    if [ ! -f "main.go" ] && [ ! -f "main_part1.go" ]; then
        echo -e "${RED}❌ Error: main.go o main_part1.go no encontrado en $service_path${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Limpiar archivos conflictivos específicos para este servicio
    cd "$BASE_PATH"
    clean_conflicting_files "$service_name"
    cd "$service_path"
    
    # Inicializar go.mod si no existe
    if [ ! -f "go.mod" ]; then
        /usr/local/go/bin/go mod init $service_name >> "/tmp/${service_name}.log" 2>&1
    fi
    
    # Descargar dependencias
    /usr/local/go/bin/go mod tidy >> "/tmp/${service_name}.log" 2>&1
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    # Verificar compilación con manejo mejorado de errores
    if ! /usr/local/go/bin/go build -buildvcs=false -o "/tmp/test_${service_name}" . >> "/tmp/${service_name}.log" 2>&1; then
        echo -e "${RED}    ❌ Error de compilación para $service_name${NC}"
        echo -e "${YELLOW}    📋 Error de compilación:${NC}"
        tail -5 "/tmp/${service_name}.log" | sed 's/^/    /'
        cd "$BASE_PATH"
        return 1
    fi
    rm -f "/tmp/test_${service_name}"
    
    # Ejecutar en background con flags
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run -buildvcs=false . $flags > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado con $flags (PID: $pid)${NC}"
    
    # Verificar que el servicio responde
    sleep 2
    if verify_service "$service_name" "$port"; then
        echo -e "${GREEN}    ✅ $service_name responde correctamente en puerto $port${NC}"
    else
        echo -e "${YELLOW}    ⚠️  $service_name puede estar iniciando...${NC}"
    fi
    
    cd "$BASE_PATH"
}

# Función para reiniciar servicios con flags específicos  
restart_all_services_with_flags() {
    local flags=$1
    echo -e "\n${WHITE}=== REINICIANDO SERVICIOS CON FLAGS: $flags ===${NC}"
    
    # Verificar directorio base
    if [ ! -d "$BASE_PATH" ]; then
        echo -e "${RED}❌ Error: El directorio base $BASE_PATH no existe${NC}"
        exit 1
    fi
    
    cd "$BASE_PATH" || exit 1
    echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"
    
    # PROTECCIÓN DE DATOS: Solo para modo producción usar backup automático
    if [ "$flags" = "--produccion" ]; then
        echo -e "${CYAN}🔒 Modo producción: creando backup de seguridad...${NC}"
        create_database_backup
        restore_backup_if_needed
    else
        echo -e "${BLUE}🔧 Modo desarrollo: usando base de datos local${NC}"
    fi
    
    # Verificar dependencias y detener servicios
    check_system_dependencies
    stop_all_services
    
    # Actualizar dependencias
    update_all_dependencies
    
    # Iniciar servicios críticos primero
    echo -e "\n${CYAN}📋 INICIANDO SERVICIOS CRÍTICOS CON FLAGS: $flags${NC}"
    for service_info in "${CRITICAL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        start_service_with_flags "$service_name" "$port" "$flags"
    done
    
    # Iniciar resto de servicios
    echo -e "\n${CYAN}📋 INICIANDO RESTO DE SERVICIOS CON FLAGS: $flags${NC}"
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        
        if [[ ! " ${CRITICAL_SERVICES[@]} " =~ " ${service_info} " ]]; then
            start_service_with_flags "$service_name" "$port" "$flags"
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
    echo "   ✅ SERVICIOS INICIADOS CON FLAGS: $flags"
    echo "   📊 Servicios activos: $active_count/${#ALL_SERVICES[@]}"
    if [ "$flags" = "--produccion" ]; then
        echo "   🏭 Modo: PRODUCCIÓN - Base de datos: /opt/hero_budget/database/hero_budget.db"
    else
        echo "   🔧 Modo: DESARROLLO - Base de datos: local users.db"
    fi
    echo "============================================================================="
    echo -e "${NC}"
    
    if [ $active_count -eq ${#ALL_SERVICES[@]} ]; then
        echo -e "${GREEN}🎉 TODOS LOS SERVICIOS ESTÁN FUNCIONANDO CON $flags${NC}"
    else
        echo -e "${YELLOW}⚠️  $active_count/${#ALL_SERVICES[@]} servicios funcionando${NC}"
        echo -e "${YELLOW}💡 Revisar logs en: /tmp/[servicio].log${NC}"
    fi
}

# Función principal de reinicio
restart_all_services() {
    echo -e "\n${WHITE}=== REINICIANDO SERVICIOS SIN PÉRDIDA DE DATOS - VPS ===${NC}"
    
    # Verificar directorio base
    if [ ! -d "$BASE_PATH" ]; then
        echo -e "${RED}❌ Error: El directorio base $BASE_PATH no existe${NC}"
        exit 1
    fi
    
    cd "$BASE_PATH" || exit 1
    echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"
    
    # PROTECCIÓN DE DATOS: Crear backup y verificar integridad
    create_database_backup
    restore_backup_if_needed
    
    # Verificar dependencias y detener servicios
    check_system_dependencies
    stop_all_services
    
    # Actualizar dependencias
    update_all_dependencies
    
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
    
    # Verificar integridad de datos después del reinicio
    echo -e "${CYAN}🔍 Verificando que los datos se mantuvieron...${NC}"
    verify_database_integrity
    
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
    echo "   ✅ SERVICIOS REINICIADOS SIN PÉRDIDA DE DATOS - VPS"
    echo "   📊 Servicios activos: $active_count/${#ALL_SERVICES[@]}"
    echo "   🔒 Datos protegidos: Backup creado y integridad verificada"
    echo "============================================================================="
    echo -e "${NC}"
    
    if [ $active_count -eq ${#ALL_SERVICES[@]} ]; then
        echo -e "${GREEN}🎉 TODOS LOS SERVICIOS ESTÁN FUNCIONANDO PERFECTAMENTE${NC}"
        echo -e "${GREEN}🔒 TUS DATOS ESTÁN SEGUROS${NC}"
    else
        echo -e "${YELLOW}⚠️  $active_count/${#ALL_SERVICES[@]} servicios funcionando${NC}"
        echo -e "${YELLOW}💡 Revisar logs en: /tmp/[servicio].log${NC}"
    fi
}

# Función para mostrar ayuda
show_help() {
    echo -e "\n${WHITE}🔧 GESTIÓN DE MICROSERVICIOS GO - VPS (VERSIÓN SEGURA)${NC}"
    echo -e "${CYAN}Uso: $0 [OPCIÓN]${NC}"
    echo -e "\n${YELLOW}Opciones:${NC}"
    echo -e "  ${GREEN}-a, --all${NC}         Reiniciar TODOS los servicios con protección de datos"
    echo -e "  ${GREEN}--produccion${NC}       Reiniciar servicios en modo PRODUCCIÓN"
    echo -e "  ${GREEN}--dev${NC}             Reiniciar servicios en modo DESARROLLO" 
    echo -e "  ${GREEN}-s, --status${NC}      Mostrar estado actual de todos los servicios"
    echo -e "  ${GREEN}-h, --help${NC}        Mostrar esta ayuda"
    echo -e "\n${YELLOW}Características de seguridad:${NC}"
    echo -e "  ${GREEN}🔒 Backup automático${NC} antes de reiniciar servicios"
    echo -e "  ${GREEN}🔍 Verificación de integridad${NC} de la base de datos"
    echo -e "  ${GREEN}📊 Estadísticas de datos${NC} preservados"
    echo -e "  ${GREEN}🔄 Restauración automática${NC} desde backup si es necesario"
}

# Función principal
main() {
    echo -e "${WHITE}"
    echo "============================================================================="
    echo "   🔄 GESTIÓN DE MICROSERVICIOS GO - VPS (VERSIÓN SEGURA)"
    echo "   🔒 CON PROTECCIÓN DE DATOS"
    echo "============================================================================="
    echo -e "${NC}"
    
    case "$1" in
        "--all"|"-a"|"")
            restart_all_services
            ;;
        "--produccion")
            echo -e "${GREEN}🏭 Iniciando servicios en modo PRODUCCIÓN...${NC}"
            restart_all_services_with_flags "--produccion"
            ;;
        "--dev")
            echo -e "${GREEN}🔧 Iniciando servicios en modo DESARROLLO...${NC}"
            restart_all_services_with_flags "--dev"
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