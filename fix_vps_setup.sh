#!/bin/bash

# =============================================================================
# SCRIPT PARA SOLUCIONAR PROBLEMAS EN EL VPS
# Instala Go, corrige configuraciones y verifica el sistema
# =============================================================================

# Configuración
VPS_HOST="srv736989.hstgr.cloud"
VPS_USER="root"
BASE_DIR="/opt/hero_budget"
SECRET="d2f031325780e13d4ca4be6e44b7e99b462e6e05"

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

main() {
    log "${WHITE}=============================================================${NC}"
    log "${WHITE}🔧 SOLUCIONANDO PROBLEMAS EN EL VPS - HERO BUDGET${NC}"
    log "${WHITE}=============================================================${NC}"
    
    # 1. Instalar Go en el VPS
    log "\n${CYAN}📦 Instalando Go en el VPS...${NC}"
    run_on_vps "apt update"
    run_on_vps "apt install -y wget curl"
    
    # Descargar e instalar Go 1.21
    run_on_vps "cd /tmp && wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz"
    run_on_vps "rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go1.21.0.linux-amd64.tar.gz"
    
    # Configurar PATH para Go
    run_on_vps "echo 'export PATH=\$PATH:/usr/local/go/bin' >> /etc/profile"
    run_on_vps "export PATH=\$PATH:/usr/local/go/bin"
    
    # Verificar instalación de Go
    log "\n${CYAN}🔍 Verificando instalación de Go...${NC}"
    if run_on_vps "/usr/local/go/bin/go version"; then
        log "${GREEN}✅ Go instalado correctamente${NC}"
    else
        log "${RED}❌ Error instalando Go${NC}"
        exit 1
    fi
    
    # 2. Detener servicio defectuoso
    log "\n${CYAN}🛑 Deteniendo servicio defectuoso...${NC}"
    run_on_vps "systemctl stop herobudget-webhook 2>/dev/null || true"
    run_on_vps "systemctl disable herobudget-webhook 2>/dev/null || true"
    
    # 3. Crear servicio corregido
    log "\n${CYAN}📝 Creando servicio systemd corregido...${NC}"
    
    # Crear el archivo de servicio corregido directamente en el VPS
    run_on_vps "cat > /etc/systemd/system/herobudget-webhook.service << 'EOF'
[Unit]
Description=Hero Budget Webhook Server
Documentation=https://github.com/jaivial/herobudget-backend
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root

# Directorio de trabajo
WorkingDirectory=/opt/hero_budget/webhook

# Comando para ejecutar
ExecStart=/usr/local/go/bin/go run webhook_server.go
ExecReload=/bin/kill -HUP \$MAINPID

# Reiniciar automáticamente si falla
Restart=always
RestartSec=10

# Variables de entorno
Environment=PATH=/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Environment=GITHUB_WEBHOOK_SECRET=$SECRET

# Logs
StandardOutput=journal
StandardError=journal
SyslogIdentifier=herobudget-webhook

# Límites de recursos
LimitNOFILE=65536
LimitNPROC=4096

# Timeout
TimeoutStartSec=30
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
EOF"
    
    # 4. Configurar directorio webhook
    log "\n${CYAN}📁 Configurando directorio webhook...${NC}"
    run_on_vps "cd $BASE_DIR/webhook && /usr/local/go/bin/go mod init herobudget-webhook"
    run_on_vps "cd $BASE_DIR/webhook && /usr/local/go/bin/go mod tidy"
    
    # 5. Configurar permisos
    log "\n${CYAN}🔐 Configurando permisos...${NC}"
    run_on_vps "chmod +x $BASE_DIR/webhook/deploy.sh"
    run_on_vps "chmod +x $BASE_DIR/backend/restart_services.sh"
    run_on_vps "chown -R root:root $BASE_DIR/webhook"
    
    # 6. Configurar firewall
    log "\n${CYAN}🔥 Configurando firewall...${NC}"
    run_on_vps "ufw allow 9000/tcp comment 'Hero Budget Webhook' 2>/dev/null || iptables -A INPUT -p tcp --dport 9000 -j ACCEPT"
    
    # 7. Habilitar y iniciar servicio
    log "\n${CYAN}🚀 Habilitando y iniciando servicio...${NC}"
    run_on_vps "systemctl daemon-reload"
    run_on_vps "systemctl enable herobudget-webhook"
    run_on_vps "systemctl start herobudget-webhook"
    
    # 8. Esperar un poco para que se inicie
    log "\n${CYAN}⏳ Esperando 5 segundos para que el servicio se inicie...${NC}"
    sleep 5
    
    # 9. Verificar estado del servicio
    log "\n${CYAN}🔍 Verificando estado del servicio...${NC}"
    run_on_vps "systemctl status herobudget-webhook --no-pager"
    
    # 10. Probar el webhook
    log "\n${CYAN}🧪 Probando webhook...${NC}"
    if run_on_vps "curl -s http://localhost:9000/health | grep -q 'Webhook server is running'"; then
        log "${GREEN}✅ Webhook respondiendo correctamente${NC}"
    else
        log "${YELLOW}⚠️  Probando respuesta del webhook...${NC}"
        run_on_vps "curl -v http://localhost:9000/health"
    fi
    
    # 11. Verificar logs si hay problemas
    log "\n${CYAN}📄 Últimos logs del servicio:${NC}"
    run_on_vps "journalctl -u herobudget-webhook -n 10 --no-pager"
    
    # 12. Probar endpoints
    log "\n${CYAN}🔍 Probando endpoints disponibles...${NC}"
    run_on_vps "curl -s http://localhost:9000/health || echo 'Health endpoint no responde'"
    run_on_vps "curl -s http://localhost:9000/logs | head -n 5 || echo 'Logs endpoint no responde'"
    
    # 13. Verificar puertos
    log "\n${CYAN}🔍 Verificando puertos...${NC}"
    run_on_vps "netstat -tuln | grep :9000 || echo 'Puerto 9000 no está abierto'"
    
    # 14. Verificar procesos Go
    log "\n${CYAN}🔍 Verificando procesos Go...${NC}"
    run_on_vps "ps aux | grep 'go run' | grep -v grep || echo 'No hay procesos Go corriendo'"
    
    log "\n${WHITE}=============================================================${NC}"
    log "${WHITE}🎉 CORRECCIÓN COMPLETADA${NC}"
    log "${WHITE}=============================================================${NC}"
    
    log "${CYAN}📋 INFORMACIÓN FINAL:${NC}"
    log "${WHITE}  • Go instalado en: /usr/local/go/bin/go${NC}"
    log "${WHITE}  • Servicio: herobudget-webhook${NC}"
    log "${WHITE}  • Puerto: 9000${NC}"
    log "${WHITE}  • Secret: $SECRET${NC}"
    
    log "\n${CYAN}📋 COMANDOS DE VERIFICACIÓN:${NC}"
    log "${WHITE}  • Estado: ssh $VPS_USER@$VPS_HOST 'systemctl status herobudget-webhook'${NC}"
    log "${WHITE}  • Logs: ssh $VPS_USER@$VPS_HOST 'journalctl -u herobudget-webhook -f'${NC}"
    log "${WHITE}  • Test: ssh $VPS_USER@$VPS_HOST 'curl http://localhost:9000/health'${NC}"
    
    log "\n${CYAN}📋 PARA CONFIGURAR GITHUB WEBHOOK:${NC}"
    log "${WHITE}  • URL: http://srv736989.hstgr.cloud:9000/webhook${NC}"
    log "${WHITE}  • Secret: $SECRET${NC}"
    log "${WHITE}  • Content-type: application/json${NC}"
    log "${WHITE}  • Events: Push events${NC}"
    
    log "\n${GREEN}🎯 Sistema corregido y listo para usar!${NC}"
}

# Ejecutar función principal
main

exit 0 