#!/bin/bash

# =============================================================================
# SCRIPT PARA MONITOREAR CAMBIOS EN LA TABLA USERS
# DETECTA QUÉ SERVICIO ESTÁ ELIMINANDO USUARIOS
# =============================================================================

VPS_HOST="178.16.130.178"
DB_PATH="/opt/hero_budget/backend/google_auth/users.db"
LOG_FILE="/tmp/users_table_monitor.log"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# Función para obtener estado actual de usuarios
get_users_state() {
    ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) as total FROM users; SELECT email, created_at FROM users ORDER BY created_at;'" 2>/dev/null
}

# Función para obtener servicios activos
get_active_services() {
    ssh root@$VPS_HOST "ps aux | grep -E '(8081|8082|8083|8084|8085|8086|8087|8088|8089|8090|8091|8092|8093|8094|8095|8096|8097|8098|8099)' | grep -v grep | wc -l" 2>/dev/null
}

# Función para crear snapshot del estado
create_snapshot() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local users_count=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
    local active_services=$(get_active_services)
    
    echo "[$timestamp] Users: $users_count | Services: $active_services" >> "$LOG_FILE"
    
    # Si hay cambios en el número de usuarios, registrar detalles
    if [ "$users_count" != "$LAST_USER_COUNT" ]; then
        echo "[$timestamp] ⚠️  CAMBIO DETECTADO: $LAST_USER_COUNT -> $users_count usuarios" >> "$LOG_FILE"
        ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT email, created_at FROM users ORDER BY created_at;'" >> "$LOG_FILE" 2>/dev/null
        echo "----------------------------------------" >> "$LOG_FILE"
    fi
    
    LAST_USER_COUNT=$users_count
}

# Función para monitorear en tiempo real
monitor_realtime() {
    echo -e "${WHITE}🔍 INICIANDO MONITOREO DE TABLA USERS EN VPS${NC}"
    echo -e "${CYAN}📊 Presiona Ctrl+C para detener${NC}"
    echo -e "${YELLOW}📋 Log: $LOG_FILE${NC}"
    echo ""
    
    # Estado inicial
    LAST_USER_COUNT=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
    echo -e "${GREEN}✅ Estado inicial: $LAST_USER_COUNT usuarios${NC}"
    
    # Crear log inicial
    echo "=== MONITOREO TABLA USERS INICIADO $(date) ===" > "$LOG_FILE"
    create_snapshot
    
    # Monitoreo continuo
    while true; do
        sleep 5
        create_snapshot
        
        # Mostrar estado actual
        local current_count=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
        local services_count=$(get_active_services)
        
        echo -e "${BLUE}$(date '+%H:%M:%S')${NC} | Usuarios: ${WHITE}$current_count${NC} | Servicios: ${WHITE}$services_count${NC}"
        
        # Alerta si hay cambios
        if [ "$current_count" != "$LAST_USER_COUNT" ]; then
            echo -e "${RED}🚨 ALERTA: Cambio detectado $LAST_USER_COUNT -> $current_count usuarios${NC}"
            echo -e "${YELLOW}📋 Revisar log: $LOG_FILE${NC}"
        fi
    done
}

# Función para mostrar estadísticas del log
show_stats() {
    if [ ! -f "$LOG_FILE" ]; then
        echo -e "${RED}❌ No existe log de monitoreo${NC}"
        return 1
    fi
    
    echo -e "${WHITE}📊 ESTADÍSTICAS DE MONITOREO${NC}"
    echo -e "${WHITE}============================================${NC}"
    
    local total_records=$(grep -c "Users:" "$LOG_FILE")
    local changes=$(grep -c "CAMBIO DETECTADO" "$LOG_FILE")
    
    echo -e "${BLUE}Total registros:${NC} $total_records"
    echo -e "${BLUE}Cambios detectados:${NC} $changes"
    
    if [ $changes -gt 0 ]; then
        echo -e "\n${YELLOW}🔍 CAMBIOS DETECTADOS:${NC}"
        grep "CAMBIO DETECTADO" "$LOG_FILE"
    fi
    
    echo -e "\n${CYAN}📋 Últimos 10 registros:${NC}"
    tail -n 10 "$LOG_FILE"
}

# Función para realizar test de pérdida de datos
test_data_loss() {
    echo -e "${WHITE}🧪 EJECUTANDO TEST DE PÉRDIDA DE DATOS${NC}"
    echo -e "${WHITE}===============================================${NC}"
    
    # 1. Registrar usuario de prueba
    echo -e "${CYAN}1. Registrando usuario de prueba...${NC}"
    local test_email="monitor_test_$(date +%s)@test.com"
    
    curl -s -X POST "http://$VPS_HOST:8082/signup" \
         -H "Content-Type: application/json" \
         -d "{\"email\":\"$test_email\",\"password\":\"test123\",\"name\":\"Monitor Test\"}" \
         > /tmp/signup_response.json
    
    sleep 2
    local users_after_signup=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
    echo -e "${GREEN}✅ Usuarios después del registro: $users_after_signup${NC}"
    
    # 2. Reiniciar servicios uno por uno
    local services_to_test=("signup" "google_auth" "signin" "reset_password" "dashboard_data")
    
    for service in "${services_to_test[@]}"; do
        echo -e "${CYAN}2. Probando reinicio de $service...${NC}"
        
        # Matar el servicio
        ssh root@$VPS_HOST "pkill -f $service"
        sleep 1
        
        # Reiniciar el servicio
        ssh root@$VPS_HOST "cd /opt/hero_budget/backend/$service && nohup /usr/local/go/bin/go run *.go > /tmp/${service}_test.log 2>&1 &"
        sleep 3
        
        # Verificar usuarios
        local users_after_restart=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
        
        if [ "$users_after_restart" -lt "$users_after_signup" ]; then
            echo -e "${RED}🚨 ¡CULPABLE ENCONTRADO! El servicio $service eliminó usuarios${NC}"
            echo -e "${RED}   Usuarios antes: $users_after_signup, después: $users_after_restart${NC}"
            
            # Mostrar logs del servicio
            echo -e "${YELLOW}📋 Últimos logs de $service:${NC}"
            ssh root@$VPS_HOST "tail -n 20 /tmp/${service}_test.log"
            
            return 1
        else
            echo -e "${GREEN}✅ $service: OK ($users_after_restart usuarios)${NC}"
        fi
    done
    
    echo -e "${GREEN}🎉 Ningún servicio individual causó pérdida de datos${NC}"
    
    # 3. Probar script completo
    echo -e "${CYAN}3. Probando script completo de reinicio...${NC}"
    
    ssh root@$VPS_HOST "cd /opt/hero_budget/backend && timeout 60 ./restart_services_vps.sh --all > /tmp/full_restart_test.log 2>&1"
    sleep 5
    
    local users_after_full_restart=$(ssh root@$VPS_HOST "sqlite3 $DB_PATH 'SELECT COUNT(*) FROM users;'" 2>/dev/null)
    
    if [ "$users_after_full_restart" -lt "$users_after_signup" ]; then
        echo -e "${RED}🚨 El script completo de reinicio eliminó usuarios${NC}"
        echo -e "${RED}   Usuarios antes: $users_after_signup, después: $users_after_full_restart${NC}"
    else
        echo -e "${GREEN}✅ Script completo: OK ($users_after_full_restart usuarios)${NC}"
    fi
}

# Función para mostrar ayuda
show_help() {
    echo -e "${WHITE}🔍 MONITOR DE TABLA USERS - VPS${NC}"
    echo -e "${CYAN}Uso: $0 [OPCIÓN]${NC}"
    echo -e "\n${YELLOW}Opciones:${NC}"
    echo -e "  ${GREEN}-m, --monitor${NC}   Monitorear cambios en tiempo real"
    echo -e "  ${GREEN}-s, --stats${NC}    Mostrar estadísticas del log"
    echo -e "  ${GREEN}-t, --test${NC}     Ejecutar test de pérdida de datos"
    echo -e "  ${GREEN}-h, --help${NC}     Mostrar esta ayuda"
    echo -e "\n${YELLOW}Ejemplos:${NC}"
    echo -e "  $0 --monitor        # Monitorear en tiempo real"
    echo -e "  $0 --test          # Probar qué servicio elimina datos"
    echo -e "  $0 --stats         # Ver estadísticas de monitoreo"
}

# Función principal
main() {
    case "$1" in
        "--monitor"|"-m"|"")
            monitor_realtime
            ;;
        "--stats"|"-s")
            show_stats
            ;;
        "--test"|"-t")
            test_data_loss
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

# Ejecutar función principal
main "$@"