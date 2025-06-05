#!/bin/bash
# Script de Despliegue Automático - Hero Budget Backend
# Versión: 2.0
# Uso: ./deploy_backend.sh [opciones]

set -e

# Configuración
VPS_HOST="178.16.130.178"
VPS_USER="root"
VPS_PATH="/opt/hero_budget/backend"
BACKUP_PATH="/opt/hero_budget/backups"
SERVICE_NAME="herobudget"
REPO_URL="https://github.com/jaivial/herobudget-backend.git"
DEFAULT_BRANCH="main"

# Variables de estado
FORCE_DEPLOY=false
SKIP_BACKUP=false
TARGET_BRANCH="$DEFAULT_BRANCH"

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Función de ayuda
show_help() {
    cat << EOF
🚀 Script de Despliegue Hero Budget Backend

Uso: $0 [opciones]

Opciones:
  --force              Forzar despliegue sin confirmaciones
  --no-backup          Saltar creación de backup
  --branch=BRANCH      Especificar branch (default: main)
  --help               Mostrar esta ayuda

EOF
}

# Procesar argumentos
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --force)
                FORCE_DEPLOY=true
                shift
                ;;
            --no-backup)
                SKIP_BACKUP=true
                shift
                ;;
            --branch=*)
                TARGET_BRANCH="${1#*=}"
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Opción desconocida: $1"
                exit 1
                ;;
        esac
    done
}

# Verificar SSH
check_ssh() {
    log_info "Verificando conexión SSH..."
    if ssh -o ConnectTimeout=10 $VPS_USER@$VPS_HOST "echo 'SSH OK'" >/dev/null 2>&1; then
        log_success "SSH conectado"
        return 0
    else
        log_error "No se puede conectar al VPS"
        return 1
    fi
}

# Crear backup
create_backup() {
    if [ "$SKIP_BACKUP" = true ]; then
        log_warning "Saltando backup"
        return 0
    fi
    
    log_info "Creando backup..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        TIMESTAMP=$(date +%Y%m%d_%H%M%S)
        mkdir -p /opt/hero_budget/backups
        
        if [ -d "/opt/hero_budget/backend" ]; then
            cd /opt/hero_budget
            tar -czf backups/backup_${TIMESTAMP}.tar.gz backend/
            echo "Backup creado: backup_${TIMESTAMP}.tar.gz"
        fi
EOF
    
    log_success "Backup completado"
}

# Desplegar código
deploy_code() {
    log_info "Desplegando código..."
    
    ssh $VPS_USER@$VPS_HOST << EOF
        cd /opt/hero_budget
        
        # Clonar o actualizar repositorio
        if [ ! -d "backend/.git" ]; then
            echo "Clonando repositorio..."
            rm -rf backend
            git clone $REPO_URL backend
        else
            echo "Actualizando repositorio..."
            cd backend
            git fetch origin
            git reset --hard origin/$TARGET_BRANCH
            cd ..
        fi
        
        cd backend
        
        # Compilar si hay Go
        if [ -f "go.mod" ]; then
            echo "Instalando dependencias Go..."
            go mod download
            go mod tidy
            
            echo "Compilando aplicación..."
            if [ -f "main.go" ]; then
                go build -o main .
            fi
            
            # Compilar microservicios
            find . -name "main.go" -not -path "./main.go" | while read main_file; do
                dir=\$(dirname "\$main_file")
                service_name=\$(basename "\$dir")
                echo "Compilando \$service_name..."
                cd "\$dir"
                go build -o "\$service_name" .
                cd - > /dev/null
            done
        fi
EOF
    
    log_success "Código desplegado"
}

# Reiniciar servicios
restart_services() {
    log_info "Reiniciando servicios..."
    
    ssh $VPS_USER@$VPS_HOST << EOF
        # Parar servicio
        if systemctl is-active --quiet $SERVICE_NAME; then
            systemctl stop $SERVICE_NAME
        fi
        
        sleep 2
        
        # Iniciar servicio
        systemctl start $SERVICE_NAME
        sleep 3
        
        # Verificar
        if systemctl is-active --quiet $SERVICE_NAME; then
            echo "✅ Servicio iniciado"
        else
            echo "❌ Error iniciando servicio"
            journalctl -u $SERVICE_NAME --no-pager -n 5
            exit 1
        fi
        
        # Recargar nginx
        systemctl reload nginx
EOF
    
    log_success "Servicios reiniciados"
}

# Verificar despliegue
verify_deployment() {
    log_info "Verificando despliegue..."
    
    sleep 5
    
    # Verificar endpoints
    if curl -s --max-time 10 https://herobudget.jaimedigitalstudio.com/ >/dev/null; then
        log_success "✅ Sitio accesible"
    else
        log_warning "⚠️ Sitio no accesible aún"
    fi
    
    log_success "Verificación completada"
}

# Función principal
main() {
    echo "🚀 Iniciando despliegue Hero Budget Backend"
    echo "==========================================="
    echo "Branch: $TARGET_BRANCH"
    echo "Backup: $([ "$SKIP_BACKUP" = true ] && echo "NO" || echo "SÍ")"
    echo ""
    
    if [ "$FORCE_DEPLOY" = false ]; then
        read -p "¿Continuar? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 0
        fi
    fi
    
    check_ssh || exit 1
    create_backup || exit 1
    deploy_code || exit 1
    restart_services || exit 1
    verify_deployment
    
    echo ""
    log_success "🎉 ¡Despliegue completado!"
    echo ""
    echo "URLs para verificar:"
    echo "• https://herobudget.jaimedigitalstudio.com/"
    echo "• https://herobudget.jaimedigitalstudio.com/auth/google"
    echo ""
}

# Ejecutar
parse_arguments "$@"
main 