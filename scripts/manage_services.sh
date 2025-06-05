#!/bin/bash
# Script de gestión de servicios Hero Budget Backend
# Versión: 2.0
# Uso: ./manage_services.sh {start|stop|restart|status|logs|health|backup|rollback|deploy}

set -e

# Configuración
VPS_HOST="178.16.130.178"
VPS_USER="root"
VPS_PATH="/opt/hero_budget/backend"
BACKUP_PATH="/opt/hero_budget/backups"
SERVICE_NAME="herobudget"
NGINX_SERVICE="nginx"
POSTGRES_SERVICE="postgresql"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Funciones de logging
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_header() {
    echo -e "${PURPLE}${1}${NC}"
    echo "=================================================="
}

# Verificar conexión SSH
check_ssh() {
    log_info "Verificando conexión SSH al VPS..."
    if ssh -o ConnectTimeout=10 $VPS_USER@$VPS_HOST "echo 'SSH OK'" >/dev/null 2>&1; then
        log_success "Conexión SSH establecida"
        return 0
    else
        log_error "No se puede conectar al VPS $VPS_HOST"
        return 1
    fi
}

# Función start - Iniciar servicios
start_services() {
    log_header "🚀 INICIANDO SERVICIOS HERO BUDGET"
    
    if ! check_ssh; then
        exit 1
    fi
    
    log_info "Iniciando servicios en el VPS..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        # Iniciar PostgreSQL primero
        echo "🗄️ Iniciando PostgreSQL..."
        systemctl start postgresql
        sleep 2
        
        # Iniciar servicio Hero Budget
        echo "⚡ Iniciando Hero Budget Backend..."
        systemctl start herobudget
        sleep 3
        
        # Iniciar/recargar Nginx
        echo "🌐 Iniciando Nginx..."
        systemctl start nginx
        systemctl reload nginx
        
        # Verificar estados
        echo ""
        echo "📊 Estado de servicios:"
        
        if systemctl is-active --quiet postgresql; then
            echo "✅ PostgreSQL: ACTIVO"
        else
            echo "❌ PostgreSQL: INACTIVO"
        fi
        
        if systemctl is-active --quiet herobudget; then
            echo "✅ Hero Budget: ACTIVO"
        else
            echo "❌ Hero Budget: INACTIVO"
        fi
        
        if systemctl is-active --quiet nginx; then
            echo "✅ Nginx: ACTIVO"
        else
            echo "❌ Nginx: INACTIVO"
        fi
EOF
    
    log_success "Comando de inicio ejecutado"
    
    # Verificar endpoints después de iniciar
    log_info "Esperando 5 segundos para verificar servicios..."
    sleep 5
    health_check
}

# Función stop - Parar servicios
stop_services() {
    log_header "⏹️ PARANDO SERVICIOS HERO BUDGET"
    
    if ! check_ssh; then
        exit 1
    fi
    
    log_warning "Parando servicios en el VPS..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        echo "⏹️ Parando Hero Budget Backend..."
        systemctl stop herobudget || true
        
        echo "🌐 Manteniendo Nginx activo (otros sitios pueden usarlo)..."
        # No paramos nginx por si hay otros sitios
        
        echo ""
        echo "📊 Estado final de servicios:"
        
        if systemctl is-active --quiet herobudget; then
            echo "⚠️ Hero Budget: AÚN ACTIVO"
        else
            echo "✅ Hero Budget: PARADO"
        fi
        
        if systemctl is-active --quiet nginx; then
            echo "ℹ️ Nginx: ACTIVO (mantenido intencionalmente)"
        else
            echo "⚠️ Nginx: PARADO"
        fi
        
        if systemctl is-active --quiet postgresql; then
            echo "ℹ️ PostgreSQL: ACTIVO (mantenido para otros servicios)"
        else
            echo "⚠️ PostgreSQL: PARADO"
        fi
EOF
    
    log_success "Servicios Hero Budget parados"
}

# Función restart - Reiniciar servicios
restart_services() {
    log_header "🔄 REINICIANDO SERVICIOS HERO BUDGET"
    
    if ! check_ssh; then
        exit 1
    fi
    
    log_info "Reiniciando servicios en el VPS..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        echo "🔄 Reiniciando Hero Budget Backend..."
        systemctl restart herobudget
        sleep 3
        
        echo "🌐 Recargando configuración Nginx..."
        systemctl reload nginx
        
        echo ""
        echo "📊 Estado después del reinicio:"
        
        if systemctl is-active --quiet herobudget; then
            echo "✅ Hero Budget: REINICIADO Y ACTIVO"
        else
            echo "❌ Hero Budget: ERROR AL REINICIAR"
        fi
        
        if systemctl is-active --quiet nginx; then
            echo "✅ Nginx: CONFIGURACIÓN RECARGADA"
        else
            echo "❌ Nginx: PROBLEMA AL RECARGAR"
        fi
EOF
    
    log_success "Reinicio completado"
    
    # Verificar que todo funciona después del reinicio
    log_info "Esperando 5 segundos para verificar servicios..."
    sleep 5
    health_check
}

# Función status - Mostrar estado detallado
show_status() {
    log_header "📊 ESTADO DETALLADO DE SERVICIOS"
    
    if ! check_ssh; then
        exit 1
    fi
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        echo "🔍 Estado detallado de servicios Hero Budget:"
        echo ""
        
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "📦 Hero Budget Backend Service:"
        systemctl status herobudget --no-pager -l | head -20
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🌐 Nginx Service:"
        systemctl status nginx --no-pager -l | head -10
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🗄️ PostgreSQL Service:"
        systemctl status postgresql --no-pager -l | head -10
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "💾 Uso de disco en /opt/hero_budget:"
        du -sh /opt/hero_budget/* 2>/dev/null || echo "Directorio no encontrado"
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🔗 Puertos en uso:"
        netstat -tlnp | grep -E ":(80|443|8080|5432)" | head -10
        
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "🕐 Últimos eventos del sistema:"
        journalctl --no-pager -n 5 --since "10 minutes ago"
EOF
}

# Función logs - Mostrar logs en tiempo real
show_logs() {
    log_header "📋 LOGS EN TIEMPO REAL - HERO BUDGET"
    
    if ! check_ssh; then
        exit 1
    fi
    
    log_info "Mostrando logs en tiempo real (Ctrl+C para salir)..."
    log_warning "Conectando a logs del VPS..."
    
    # Logs en tiempo real
    ssh $VPS_USER@$VPS_HOST "journalctl -u $SERVICE_NAME -f --no-pager"
}

# Función health - Verificación de salud
health_check() {
    log_header "🏥 VERIFICACIÓN DE SALUD"
    
    log_info "Verificando salud de servicios..."
    
    # Verificar servicios en VPS
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        echo "🔍 Verificando servicios internos..."
        
        # Verificar Hero Budget
        if systemctl is-active --quiet herobudget; then
            echo "✅ Hero Budget Backend: ACTIVO"
        else
            echo "❌ Hero Budget Backend: INACTIVO"
        fi
        
        # Verificar PostgreSQL
        if systemctl is-active --quiet postgresql; then
            echo "✅ PostgreSQL: ACTIVO"
            # Test conexión a DB
            if sudo -u postgres psql -d herobudget -c "SELECT 1;" >/dev/null 2>&1; then
                echo "✅ Conexión a base de datos: OK"
            else
                echo "⚠️ Conexión a base de datos: PROBLEMA"
            fi
        else
            echo "❌ PostgreSQL: INACTIVO"
        fi
        
        # Verificar Nginx
        if systemctl is-active --quiet nginx; then
            echo "✅ Nginx: ACTIVO"
        else
            echo "❌ Nginx: INACTIVO"
        fi
        
        # Verificar puertos
        echo ""
        echo "🔗 Verificando puertos:"
        if netstat -tln | grep -q ":80 "; then
            echo "✅ Puerto 80 (HTTP): ABIERTO"
        else
            echo "⚠️ Puerto 80 (HTTP): CERRADO"
        fi
        
        if netstat -tln | grep -q ":443 "; then
            echo "✅ Puerto 443 (HTTPS): ABIERTO"
        else
            echo "⚠️ Puerto 443 (HTTPS): CERRADO"
        fi
EOF
    
    # Verificar endpoints externos
    log_info "Verificando endpoints externos..."
    
    # Test endpoint principal
    if curl -s --max-time 10 https://herobudget.jaimedigitalstudio.com/ >/dev/null 2>&1; then
        log_success "✅ Sitio principal: ACCESIBLE"
    else
        log_warning "⚠️ Sitio principal: NO ACCESIBLE"
    fi
    
    # Test endpoint auth
    if curl -s --max-time 10 https://herobudget.jaimedigitalstudio.com/auth/google >/dev/null 2>&1; then
        log_success "✅ Endpoint /auth/google: ACCESIBLE"
    else
        log_warning "⚠️ Endpoint /auth/google: NO ACCESIBLE"
    fi
    
    log_info "Verificación de salud completada"
}

# Función backup - Crear backup manual
create_backup() {
    log_header "💾 CREANDO BACKUP MANUAL"
    
    if ! check_ssh; then
        exit 1
    fi
    
    log_info "Creando backup del backend y base de datos..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        TIMESTAMP=$(date +%Y%m%d_%H%M%S)
        
        echo "📦 Creando backup del código backend..."
        cd /opt/hero_budget
        tar -czf backups/manual_backend_backup_${TIMESTAMP}.tar.gz backend/
        
        echo "🗄️ Creando backup de la base de datos..."
        sudo -u postgres pg_dump herobudget > backups/manual_database_backup_${TIMESTAMP}.sql
        
        echo ""
        echo "✅ Backups creados:"
        ls -lh backups/*_${TIMESTAMP}.*
        
        echo ""
        echo "📊 Espacio usado en backups:"
        du -sh backups/
EOF
    
    log_success "Backup manual completado"
}

# Función main - Menú principal
main() {
    case "$1" in
        start)
            start_services
            ;;
        stop)
            stop_services
            ;;
        restart)
            restart_services
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        health)
            health_check
            ;;
        backup)
            create_backup
            ;;
        *)
            echo ""
            log_header "🛠️ HERO BUDGET - GESTIÓN DE SERVICIOS"
            echo ""
            echo "Uso: $0 {comando}"
            echo ""
            echo "Comandos disponibles:"
            echo "  start     - Iniciar servicios Hero Budget"
            echo "  stop      - Parar servicios Hero Budget"
            echo "  restart   - Reiniciar servicios Hero Budget"
            echo "  status    - Mostrar estado detallado"
            echo "  logs      - Ver logs en tiempo real"
            echo "  health    - Verificación de salud completa"
            echo "  backup    - Crear backup manual"
            echo ""
            echo "Ejemplos:"
            echo "  $0 start"
            echo "  $0 health"
            echo "  $0 logs"
            echo ""
            exit 1
            ;;
    esac
}

# Ejecutar función principal
main "$@" 