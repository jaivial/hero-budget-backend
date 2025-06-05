#!/bin/bash

# =============================================================================
# SCRIPT DE DEPLOYMENT AUTOMÁTICO - HERO BUDGET BACKEND
# Ejecutado por el webhook server cuando se recibe un push de GitHub
# =============================================================================

# Configuración
REPO_DIR="/opt/hero_budget"
BACKEND_DIR="/opt/hero_budget/backend"
LOG_FILE="/opt/hero_budget/webhook/deployment.log"
WEBHOOK_LOG_FILE="/opt/hero_budget/webhook/deployment.log"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# Función para logging con timestamp
log_message() {
    local message="$1"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${timestamp} - ${message}" >> "$LOG_FILE"
    echo -e "${message}"
}

# Función para manejar errores
handle_error() {
    local error_message="$1"
    log_message "${RED}❌ ERROR: ${error_message}${NC}"
    exit 1
}

# Función para verificar que estamos en el directorio correcto
verify_directory() {
    if [ ! -d "$REPO_DIR" ]; then
        handle_error "Directorio del repositorio no encontrado: $REPO_DIR"
    fi
    
    if [ ! -d "$BACKEND_DIR" ]; then
        handle_error "Directorio backend no encontrado: $BACKEND_DIR"
    fi
    
    if [ ! -f "$BACKEND_DIR/restart_services.sh" ]; then
        handle_error "Script restart_services.sh no encontrado en $BACKEND_DIR"
    fi
}

# Función para hacer backup del estado actual
create_backup() {
    log_message "${BLUE}📦 Creando backup del estado actual...${NC}"
    
    cd "$REPO_DIR" || handle_error "No se pudo acceder al directorio $REPO_DIR"
    
    # Crear directorio de backups si no existe
    mkdir -p backups
    
    # Crear backup con timestamp
    local backup_name="backup_$(date '+%Y%m%d_%H%M%S')"
    local current_commit=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
    
    # Guardar información del commit actual
    echo "Backup creado: $(date)" > "backups/${backup_name}_info.txt"
    echo "Commit: $current_commit" >> "backups/${backup_name}_info.txt"
    echo "Branch: $(git branch --show-current 2>/dev/null || echo "unknown")" >> "backups/${backup_name}_info.txt"
    
    log_message "${GREEN}✅ Backup creado: ${backup_name}${NC}"
}

# Función para actualizar el código
update_code() {
    log_message "${CYAN}🔄 Actualizando código desde GitHub...${NC}"
    
    cd "$REPO_DIR" || handle_error "No se pudo acceder al directorio $REPO_DIR"
    
    # Verificar que estamos en un repositorio git
    if [ ! -d ".git" ]; then
        handle_error "No es un repositorio Git válido"
    fi
    
    # Guardar cambios locales si los hay
    if ! git diff-index --quiet HEAD --; then
        log_message "${YELLOW}⚠️  Hay cambios locales, guardándolos...${NC}"
        git stash push -m "Auto-stash before deployment $(date)" || {
            log_message "${YELLOW}⚠️  No se pudieron guardar los cambios locales${NC}"
        }
    fi
    
    # Hacer fetch de los últimos cambios
    log_message "${BLUE}📡 Obteniendo últimos cambios...${NC}"
    git fetch origin || handle_error "Error en git fetch"
    
    # Hacer pull --rebase
    log_message "${BLUE}⬇️  Aplicando cambios con rebase...${NC}"
    git pull --rebase origin main || handle_error "Error en git pull --rebase"
    
    # Verificar el estado después del pull
    local new_commit=$(git rev-parse HEAD)
    log_message "${GREEN}✅ Código actualizado a commit: ${new_commit:0:8}${NC}"
    
    # Mostrar los últimos commits
    log_message "${BLUE}📝 Últimos commits:${NC}"
    git log --oneline -3 >> "$LOG_FILE" 2>&1
}

# Función para instalar/actualizar dependencias Go
update_dependencies() {
    log_message "${BLUE}📦 Verificando dependencias Go...${NC}"
    
    cd "$BACKEND_DIR" || handle_error "No se pudo acceder al directorio backend"
    
    # Verificar si hay go.mod
    if [ -f "go.mod" ]; then
        log_message "${BLUE}🔄 Actualizando dependencias Go...${NC}"
        go mod tidy || {
            log_message "${YELLOW}⚠️  Warning: Error en go mod tidy${NC}"
        }
        go mod download || {
            log_message "${YELLOW}⚠️  Warning: Error en go mod download${NC}"
        }
        log_message "${GREEN}✅ Dependencias actualizadas${NC}"
    else
        log_message "${YELLOW}⚠️  No se encontró go.mod en el directorio backend${NC}"
    fi
}

# Función para reiniciar servicios
restart_services() {
    log_message "${CYAN}🔄 Reiniciando servicios...${NC}"
    
    cd "$BACKEND_DIR" || handle_error "No se pudo acceder al directorio backend"
    
    # Hacer el script ejecutable si no lo es
    chmod +x restart_services.sh
    
    # Ejecutar el script de restart
    log_message "${BLUE}🚀 Ejecutando restart_services.sh...${NC}"
    
    # Ejecutar con timeout para evitar que se cuelgue
    timeout 300s ./restart_services.sh || {
        handle_error "Error o timeout ejecutando restart_services.sh"
    }
    
    log_message "${GREEN}✅ Servicios reiniciados correctamente${NC}"
}

# Función para verificar que los servicios están corriendo
verify_services() {
    log_message "${BLUE}🔍 Verificando estado de servicios...${NC}"
    
    # Lista de puertos críticos a verificar
    critical_ports=(8084 8085 8090 8092 8097)
    failed_services=0
    
    for port in "${critical_ports[@]}"; do
        if curl -s "http://localhost:$port/health" > /dev/null 2>&1 || 
           curl -s "http://localhost:$port" > /dev/null 2>&1; then
            log_message "${GREEN}  ✅ Servicio en puerto $port - OK${NC}"
        else
            log_message "${RED}  ❌ Servicio en puerto $port - FAILED${NC}"
            ((failed_services++))
        fi
        sleep 1
    done
    
    if [ $failed_services -eq 0 ]; then
        log_message "${GREEN}✅ Todos los servicios críticos están funcionando${NC}"
    else
        log_message "${YELLOW}⚠️  $failed_services servicios críticos no están respondiendo${NC}"
    fi
}

# Función para limpiar logs antiguos
cleanup_logs() {
    log_message "${BLUE}🧹 Limpiando logs antiguos...${NC}"
    
    # Mantener solo los últimos 100 backups
    cd "$REPO_DIR/backups" 2>/dev/null && {
        ls -t backup_*_info.txt 2>/dev/null | tail -n +101 | while read file; do
            rm -f "$file" 2>/dev/null
            log_message "  Eliminado backup antiguo: $file"
        done
    }
    
    # Rotar log de deployment si es muy grande (>10MB)
    if [ -f "$LOG_FILE" ] && [ $(stat -c%s "$LOG_FILE" 2>/dev/null || echo 0) -gt 10485760 ]; then
        mv "$LOG_FILE" "${LOG_FILE}.old"
        log_message "${GREEN}✅ Log rotado${NC}"
    fi
}

# =============================================================================
# FUNCIÓN PRINCIPAL
# =============================================================================

main() {
    log_message "${WHITE}=============================================================${NC}"
    log_message "${WHITE}🚀 INICIANDO DEPLOYMENT AUTOMÁTICO - $(date)${NC}"
    log_message "${WHITE}=============================================================${NC}"
    
    # Verificar prerequisitos
    verify_directory
    
    # Crear backup del estado actual
    create_backup
    
    # Actualizar código
    update_code
    
    # Actualizar dependencias
    update_dependencies
    
    # Reiniciar servicios
    restart_services
    
    # Esperar un poco antes de verificar
    log_message "${BLUE}⏳ Esperando 10 segundos para que los servicios se estabilicen...${NC}"
    sleep 10
    
    # Verificar servicios
    verify_services
    
    # Limpiar logs antiguos
    cleanup_logs
    
    log_message "${WHITE}=============================================================${NC}"
    log_message "${GREEN}🎉 DEPLOYMENT COMPLETADO EXITOSAMENTE - $(date)${NC}"
    log_message "${WHITE}=============================================================${NC}"
}

# Capturar errores y señales
trap 'handle_error "Script interrumpido"' INT TERM

# Ejecutar función principal
main

exit 0 