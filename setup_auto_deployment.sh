#!/bin/bash

# =============================================================================
# CONFIGURACIÓN COMPLETA DE DEPLOYMENT AUTOMÁTICO
# Hero Budget Backend - VPS Setup
# =============================================================================

# Configuración
VPS_HOST="srv736989.hstgr.cloud"
VPS_USER="root"
REPO_URL="https://github.com/jaivial/herobudget-backend.git"
BASE_DIR="/opt/hero_budget"

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

# Función para ejecutar comandos en el VPS
run_on_vps() {
    local command="$1"
    log "${BLUE}🔄 Ejecutando en VPS: $command${NC}"
    ssh -o StrictHostKeyChecking=no "$VPS_USER@$VPS_HOST" "$command"
}

# Función para copiar archivos al VPS
copy_to_vps() {
    local source="$1"
    local dest="$2"
    log "${BLUE}📁 Copiando $source a VPS:$dest${NC}"
    scp -o StrictHostKeyChecking=no "$source" "$VPS_USER@$VPS_HOST:$dest"
}

main() {
    log "${WHITE}=============================================================${NC}"
    log "${WHITE}🚀 CONFIGURANDO DEPLOYMENT AUTOMÁTICO - HERO BUDGET${NC}"
    log "${WHITE}=============================================================${NC}"
    
    log "${CYAN}📋 INFORMACIÓN DE CONFIGURACIÓN:${NC}"
    log "${WHITE}  • VPS: $VPS_HOST${NC}"
    log "${WHITE}  • Usuario: $VPS_USER${NC}"
    log "${WHITE}  • Repositorio: $REPO_URL${NC}"
    log "${WHITE}  • Directorio base: $BASE_DIR${NC}"
    
    # 1. Verificar conexión SSH
    log "\n${CYAN}🔑 Verificando conexión SSH...${NC}"
    if ! ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$VPS_USER@$VPS_HOST" "echo 'Conexión SSH exitosa'"; then
        log "${RED}❌ Error: No se pudo conectar al VPS${NC}"
        log "${YELLOW}Verifica que puedas conectarte manualmente con: ssh $VPS_USER@$VPS_HOST${NC}"
        exit 1
    fi
    
    # 2. Crear directorio webhook si no existe
    log "\n${CYAN}📁 Creando directorios necesarios...${NC}"
    run_on_vps "mkdir -p $BASE_DIR/webhook"
    run_on_vps "mkdir -p $BASE_DIR/backups"
    
    # 3. Copiar archivos del webhook al VPS
    log "\n${CYAN}📤 Copiando archivos de webhook al VPS...${NC}"
    copy_to_vps "backend/webhook/webhook_server.go" "$BASE_DIR/webhook/"
    copy_to_vps "backend/webhook/deploy.sh" "$BASE_DIR/webhook/"
    copy_to_vps "backend/webhook/install_webhook.sh" "$BASE_DIR/webhook/"
    copy_to_vps "backend/webhook/herobudget-webhook.service" "$BASE_DIR/webhook/"
    copy_to_vps "backend/webhook/go.mod" "$BASE_DIR/webhook/"
    copy_to_vps "backend/webhook/README.md" "$BASE_DIR/webhook/"
    
    # 4. Hacer scripts ejecutables
    log "\n${CYAN}🔧 Configurando permisos...${NC}"
    run_on_vps "chmod +x $BASE_DIR/webhook/install_webhook.sh"
    run_on_vps "chmod +x $BASE_DIR/webhook/deploy.sh"
    run_on_vps "chmod +x $BASE_DIR/backend/restart_services.sh"
    
    # 5. Ejecutar instalación del webhook
    log "\n${CYAN}⚙️  Ejecutando instalación del webhook...${NC}"
    run_on_vps "cd $BASE_DIR/webhook && ./install_webhook.sh"
    
    # 6. Generar secret seguro
    log "\n${CYAN}🔐 Generando secret de webhook...${NC}"
    SECRET=$(openssl rand -hex 20)
    log "${GREEN}Secret generado: $SECRET${NC}"
    
    # 7. Configurar secret en el servicio
    log "\n${CYAN}🔧 Configurando secret en el servicio...${NC}"
    run_on_vps "sed -i 's/Environment=GITHUB_WEBHOOK_SECRET=/Environment=GITHUB_WEBHOOK_SECRET=$SECRET/' /etc/systemd/system/herobudget-webhook.service"
    run_on_vps "systemctl daemon-reload"
    run_on_vps "systemctl restart herobudget-webhook"
    
    # 8. Verificar estado del servicio
    log "\n${CYAN}🔍 Verificando estado del servicio...${NC}"
    sleep 3
    run_on_vps "systemctl status herobudget-webhook --no-pager"
    
    # 9. Test del webhook
    log "\n${CYAN}🧪 Probando webhook...${NC}"
    if run_on_vps "curl -s http://localhost:9000/health"; then
        log "${GREEN}✅ Webhook respondiendo correctamente${NC}"
    else
        log "${RED}❌ Error: Webhook no está respondiendo${NC}"
    fi
    
    # 10. Mostrar información final
    log "\n${WHITE}=============================================================${NC}"
    log "${WHITE}🎉 CONFIGURACIÓN COMPLETADA${NC}"
    log "${WHITE}=============================================================${NC}"
    
    log "${CYAN}📋 INFORMACIÓN IMPORTANTE:${NC}"
    log "${WHITE}  • URL del webhook: http://$VPS_HOST:9000/webhook${NC}"
    log "${WHITE}  • Secret: $SECRET${NC}"
    log "${WHITE}  • Puerto del webhook: 9000${NC}"
    
    log "\n${CYAN}📋 PRÓXIMOS PASOS:${NC}"
    log "${WHITE}1. Configurar webhook en GitHub:${NC}"
    log "${WHITE}   - Ve a: https://github.com/jaivial/herobudget-backend/settings/hooks${NC}"
    log "${WHITE}   - Add webhook${NC}"
    log "${WHITE}   - Payload URL: http://$VPS_HOST:9000/webhook${NC}"
    log "${WHITE}   - Content type: application/json${NC}"
    log "${WHITE}   - Secret: $SECRET${NC}"
    log "${WHITE}   - Events: Solo 'Push events'${NC}"
    
    log "\n${WHITE}2. Verificar el sistema:${NC}"
    log "${WHITE}   ssh $VPS_USER@$VPS_HOST${NC}"
    log "${WHITE}   systemctl status herobudget-webhook${NC}"
    log "${WHITE}   curl http://localhost:9000/health${NC}"
    
    log "\n${WHITE}3. Hacer un test push:${NC}"
    log "${WHITE}   echo '// Test' >> README.md${NC}"
    log "${WHITE}   git add . && git commit -m 'Test auto-deploy' && git push${NC}"
    
    log "\n${CYAN}📋 COMANDOS ÚTILES:${NC}"
    log "${WHITE}  • Ver logs: ssh $VPS_USER@$VPS_HOST 'journalctl -u herobudget-webhook -f'${NC}"
    log "${WHITE}  • Ver deployments: ssh $VPS_USER@$VPS_HOST 'tail -f $BASE_DIR/webhook/deployment.log'${NC}"
    log "${WHITE}  • Reiniciar webhook: ssh $VPS_USER@$VPS_HOST 'systemctl restart herobudget-webhook'${NC}"
    
    log "\n${GREEN}🎯 LISTO! El deployment automático está configurado.${NC}"
    log "${GREEN}Cada push a 'main' activará automáticamente el deployment.${NC}"
}

# Verificar que estamos en el directorio correcto
if [ ! -f "backend/restart_services.sh" ]; then
    log "${RED}❌ Error: Este script debe ejecutarse desde la raíz del proyecto${NC}"
    log "${YELLOW}Cambia al directorio del proyecto y ejecuta: ./backend/setup_auto_deployment.sh${NC}"
    exit 1
fi

# Ejecutar función principal
main

exit 0 