#!/bin/bash

# =============================================================================
# SCRIPT DE INSTALACIÓN DEL SISTEMA DE WEBHOOKS
# Configura el deployment automático para Hero Budget Backend
# =============================================================================

# Configuración
WEBHOOK_DIR="/opt/hero_budget/webhook"
SERVICE_NAME="herobudget-webhook"
PORT="9000"

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# Función para logging
log() {
    echo -e "${1}"
}

# Función para manejar errores
error_exit() {
    log "${RED}❌ ERROR: $1${NC}"
    exit 1
}

# Función para verificar si el usuario es root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        error_exit "Este script debe ejecutarse como root"
    fi
}

# Función para verificar prerequisitos
check_prerequisites() {
    log "${BLUE}🔍 Verificando prerequisitos...${NC}"
    
    # Verificar Go
    if ! command -v go &> /dev/null; then
        error_exit "Go no está instalado. Instala Go primero."
    fi
    
    # Verificar Git
    if ! command -v git &> /dev/null; then
        error_exit "Git no está instalado. Instala Git primero."
    fi
    
    # Verificar curl
    if ! command -v curl &> /dev/null; then
        error_exit "curl no está instalado. Instala curl primero."
    fi
    
    # Verificar systemctl
    if ! command -v systemctl &> /dev/null; then
        error_exit "systemd no está disponible."
    fi
    
    log "${GREEN}✅ Prerequisitos verificados${NC}"
}

# Función para crear directorios necesarios
setup_directories() {
    log "${BLUE}📁 Configurando directorios...${NC}"
    
    # Crear directorio webhook si no existe
    mkdir -p "$WEBHOOK_DIR"
    chmod 755 "$WEBHOOK_DIR"
    
    # Crear directorio de logs
    mkdir -p "/var/log/herobudget"
    chmod 755 "/var/log/herobudget"
    
    # Crear directorio de backups
    mkdir -p "/opt/hero_budget/backups"
    chmod 755 "/opt/hero_budget/backups"
    
    log "${GREEN}✅ Directorios configurados${NC}"
}

# Función para configurar el firewall
setup_firewall() {
    log "${BLUE}🔥 Configurando firewall...${NC}"
    
    # Verificar si ufw está instalado
    if command -v ufw &> /dev/null; then
        # Permitir el puerto del webhook
        ufw allow $PORT/tcp comment "Hero Budget Webhook"
        log "${GREEN}✅ Puerto $PORT permitido en UFW${NC}"
    else
        log "${YELLOW}⚠️  UFW no está instalado, configuración manual del firewall requerida${NC}"
    fi
    
    # Si usas iptables directamente
    if command -v iptables &> /dev/null; then
        # Verificar si la regla ya existe
        if ! iptables -L INPUT -n | grep -q ":$PORT "; then
            iptables -A INPUT -p tcp --dport $PORT -j ACCEPT
            log "${GREEN}✅ Regla iptables añadida para puerto $PORT${NC}"
        else
            log "${GREEN}✅ Regla iptables para puerto $PORT ya existe${NC}"
        fi
    fi
}

# Función para configurar el servicio systemd
setup_systemd_service() {
    log "${BLUE}⚙️  Configurando servicio systemd...${NC}"
    
    # Copiar archivo de servicio
    cp "$WEBHOOK_DIR/herobudget-webhook.service" "/etc/systemd/system/"
    
    # Recargar systemd
    systemctl daemon-reload
    
    # Habilitar el servicio
    systemctl enable $SERVICE_NAME
    
    log "${GREEN}✅ Servicio systemd configurado${NC}"
}

# Función para compilar el webhook server
compile_webhook() {
    log "${BLUE}🔨 Compilando webhook server...${NC}"
    
    cd "$WEBHOOK_DIR" || error_exit "No se pudo acceder al directorio webhook"
    
    # Inicializar módulo Go si no existe
    if [ ! -f "go.mod" ]; then
        go mod init herobudget-webhook
    fi
    
    # Descargar dependencias
    go mod tidy
    
    # Compilar
    go build -o webhook_server webhook_server.go
    chmod +x webhook_server
    
    log "${GREEN}✅ Webhook server compilado${NC}"
}

# Función para configurar permisos
setup_permissions() {
    log "${BLUE}🔐 Configurando permisos...${NC}"
    
    # Hacer scripts ejecutables
    chmod +x "$WEBHOOK_DIR/deploy.sh"
    chmod +x "/opt/hero_budget/backend/restart_services.sh"
    
    # Configurar permisos del directorio webhook
    chown -R root:root "$WEBHOOK_DIR"
    chmod -R 755 "$WEBHOOK_DIR"
    
    log "${GREEN}✅ Permisos configurados${NC}"
}

# Función para generar configuración de ejemplo
generate_config() {
    log "${BLUE}📝 Generando configuración de ejemplo...${NC}"
    
    cat > "$WEBHOOK_DIR/webhook_config.example" << EOF
# =============================================================================
# CONFIGURACIÓN DEL WEBHOOK - HERO BUDGET
# =============================================================================

# 1. CONFIGURAR SECRET EN GITHUB:
#    - Ve a tu repositorio en GitHub
#    - Settings > Webhooks > Add webhook
#    - Payload URL: http://tu-servidor.com:9000/webhook
#    - Content type: application/json
#    - Secret: [generar-un-secret-seguro]

# 2. CONFIGURAR SECRET EN EL SERVIDOR:
#    export GITHUB_WEBHOOK_SECRET="tu-secret-aqui"
#    
#    O añadir al archivo de servicio:
#    Environment=GITHUB_WEBHOOK_SECRET=tu-secret-aqui

# 3. URLs DISPONIBLES:
#    - POST /webhook  - Recibe webhooks de GitHub
#    - GET  /health   - Health check
#    - GET  /logs     - Ver logs recientes

# 4. LOGS:
#    - Servicio: journalctl -u herobudget-webhook -f
#    - Deployment: tail -f /opt/hero_budget/webhook/deployment.log

# 5. COMANDOS ÚTILES:
#    - Reiniciar servicio: systemctl restart herobudget-webhook
#    - Ver estado: systemctl status herobudget-webhook
#    - Ver logs: journalctl -u herobudget-webhook -f
EOF

    log "${GREEN}✅ Configuración de ejemplo generada en webhook_config.example${NC}"
}

# Función para probar la configuración
test_configuration() {
    log "${BLUE}🧪 Probando configuración...${NC}"
    
    # Verificar que el puerto está libre
    if netstat -tuln | grep -q ":$PORT "; then
        log "${YELLOW}⚠️  Puerto $PORT ya está en uso${NC}"
        log "${BLUE}ℹ️  Procesos usando el puerto:${NC}"
        lsof -i :$PORT
    fi
    
    # Verificar archivos necesarios
    required_files=(
        "$WEBHOOK_DIR/webhook_server.go"
        "$WEBHOOK_DIR/deploy.sh"
        "/opt/hero_budget/backend/restart_services.sh"
        "/etc/systemd/system/herobudget-webhook.service"
    )
    
    for file in "${required_files[@]}"; do
        if [ -f "$file" ]; then
            log "${GREEN}  ✅ $file${NC}"
        else
            log "${RED}  ❌ $file - NO ENCONTRADO${NC}"
        fi
    done
}

# Función para iniciar el servicio
start_service() {
    log "${BLUE}🚀 Iniciando servicio webhook...${NC}"
    
    # Iniciar el servicio
    systemctl start $SERVICE_NAME
    
    # Esperar un poco
    sleep 3
    
    # Verificar estado
    if systemctl is-active --quiet $SERVICE_NAME; then
        log "${GREEN}✅ Servicio webhook iniciado correctamente${NC}"
        
        # Probar health check
        if curl -s http://localhost:$PORT/health > /dev/null; then
            log "${GREEN}✅ Health check exitoso${NC}"
        else
            log "${YELLOW}⚠️  Health check falló${NC}"
        fi
    else
        log "${RED}❌ Error iniciando el servicio${NC}"
        log "${BLUE}Ver logs con: journalctl -u $SERVICE_NAME -n 20${NC}"
    fi
}

# Función para mostrar resumen final
show_summary() {
    log "${WHITE}=============================================================${NC}"
    log "${WHITE}🎉 INSTALACIÓN DEL WEBHOOK COMPLETADA${NC}"
    log "${WHITE}=============================================================${NC}"
    
    log "${CYAN}📋 INFORMACIÓN IMPORTANTE:${NC}"
    log "${WHITE}  • Servicio: $SERVICE_NAME${NC}"
    log "${WHITE}  • Puerto: $PORT${NC}"
    log "${WHITE}  • Directorio: $WEBHOOK_DIR${NC}"
    log "${WHITE}  • URL Webhook: http://tu-servidor:$PORT/webhook${NC}"
    
    log "${CYAN}📋 PRÓXIMOS PASOS:${NC}"
    log "${WHITE}  1. Configurar secret: export GITHUB_WEBHOOK_SECRET='tu-secret'${NC}"
    log "${WHITE}  2. Configurar webhook en GitHub (ver webhook_config.example)${NC}"
    log "${WHITE}  3. Reiniciar servicio: systemctl restart $SERVICE_NAME${NC}"
    
    log "${CYAN}📋 COMANDOS ÚTILES:${NC}"
    log "${WHITE}  • Estado: systemctl status $SERVICE_NAME${NC}"
    log "${WHITE}  • Logs: journalctl -u $SERVICE_NAME -f${NC}"
    log "${WHITE}  • Test: curl http://localhost:$PORT/health${NC}"
    log "${WHITE}  • Logs deployment: tail -f $WEBHOOK_DIR/deployment.log${NC}"
}

# =============================================================================
# FUNCIÓN PRINCIPAL
# =============================================================================

main() {
    log "${WHITE}=============================================================${NC}"
    log "${WHITE}🚀 INSTALANDO SISTEMA DE WEBHOOKS - HERO BUDGET${NC}"
    log "${WHITE}=============================================================${NC}"
    
    check_root
    check_prerequisites
    setup_directories
    setup_firewall
    setup_systemd_service
    compile_webhook
    setup_permissions
    generate_config
    test_configuration
    start_service
    show_summary
    
    log "${GREEN}🎉 Instalación completada exitosamente${NC}"
}

# Ejecutar instalación
main

exit 0 