#!/bin/bash

# =============================================================================
# SCRIPT PARA APLICAR NUEVA CONFIGURACIÓN NGINX - HERO BUDGET
# Corrige todos los problemas identificados en los tests de producción
# =============================================================================

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
echo "   🔧 APLICANDO NUEVA CONFIGURACIÓN NGINX - HERO BUDGET"
echo "   🎯 Objetivo: Solucionar problemas de endpoints de producción"
echo "============================================================================="
echo -e "${NC}"

# Función para backup de configuración actual
backup_current_config() {
    echo -e "${YELLOW}📦 Creando backup de configuración actual...${NC}"
    
    # Crear directorio de backup si no existe
    sudo mkdir -p /opt/hero_budget/nginx_backups
    
    # Backup con timestamp
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local backup_file="/opt/hero_budget/nginx_backups/herobudget_backup_${timestamp}.conf"
    
    if [ -f "/etc/nginx/sites-available/herobudget" ]; then
        sudo cp /etc/nginx/sites-available/herobudget "$backup_file"
        echo -e "${GREEN}  ✅ Backup creado: $backup_file${NC}"
    else
        echo -e "${RED}  ❌ No se encontró configuración actual${NC}"
    fi
}

# Función para aplicar la nueva configuración
apply_new_config() {
    echo -e "${BLUE}🚀 Aplicando nueva configuración nginx...${NC}"
    
    # Verificar que el archivo corregido existe
    if [ ! -f "nginx-herobudget-updated.conf" ]; then
        echo -e "${RED}  ❌ No se encontró archivo nginx-herobudget-updated.conf${NC}"
        echo -e "${YELLOW}  📝 Ejecute este script desde el directorio /opt/hero_budget/backend${NC}"
        return 1
    fi
    
    # Corregir errores tipográficos antes de aplicar
    sed 's/proxy_Set_header/proxy_set_header/g' nginx-herobudget-updated.conf > /tmp/herobudget_fixed.conf
    
    # Copiar la nueva configuración
    sudo cp /tmp/herobudget_fixed.conf /etc/nginx/sites-available/herobudget
    
    # Verificar sintaxis
    echo -e "${YELLOW}🔍 Verificando sintaxis de nginx...${NC}"
    if sudo nginx -t; then
        echo -e "${GREEN}  ✅ Sintaxis nginx correcta${NC}"
        
        # Recargar nginx
        echo -e "${CYAN}🔄 Recargando nginx...${NC}"
        if sudo systemctl reload nginx; then
            echo -e "${GREEN}  ✅ Nginx recargado exitosamente${NC}"
        else
            echo -e "${RED}  ❌ Error al recargar nginx${NC}"
            return 1
        fi
    else
        echo -e "${RED}  ❌ Error en sintaxis nginx${NC}"
        echo -e "${YELLOW}  📝 Revisando configuración...${NC}"
        sudo nginx -t
        return 1
    fi
}

# Función para verificar la aplicación
verify_config() {
    echo -e "${CYAN}🧪 Verificando configuración aplicada...${NC}"
    
    echo -e "${YELLOW}  📡 Probando health check general...${NC}"
    if curl -s https://herobudget.jaimedigitalstudio.com/health > /dev/null; then
        echo -e "${GREEN}    ✅ Health check principal OK${NC}"
    else
        echo -e "${RED}    ❌ Health check principal FAILED${NC}"
    fi
    
    echo -e "${YELLOW}  📡 Probando health check savings...${NC}"
    if curl -s https://herobudget.jaimedigitalstudio.com/savings/health > /dev/null; then
        echo -e "${GREEN}    ✅ Health check savings OK${NC}"
    else
        echo -e "${YELLOW}    ⚠️ Health check savings pendiente (puede necesitar servicio activo)${NC}"
    fi
    
    echo -e "${YELLOW}  📡 Probando health check budget-overview...${NC}"
    if curl -s https://herobudget.jaimedigitalstudio.com/budget-overview/health > /dev/null; then
        echo -e "${GREEN}    ✅ Health check budget-overview OK${NC}"
    else
        echo -e "${YELLOW}    ⚠️ Health check budget-overview pendiente (puede necesitar servicio activo)${NC}"
    fi
}

# Función principal
main() {
    echo -e "${CYAN}🎯 Iniciando proceso de actualización nginx...${NC}\n"
    
    # Verificar directorio actual
    if [ ! -f "nginx-herobudget-updated.conf" ]; then
        echo -e "${RED}❌ Archivo nginx-herobudget-updated.conf no encontrado${NC}"
        echo -e "${YELLOW}📂 Asegúrese de ejecutar desde /opt/hero_budget/backend${NC}"
        exit 1
    fi
    
    # Paso 1: Backup
    backup_current_config
    echo ""
    
    # Paso 2: Aplicar nueva configuración
    if apply_new_config; then
        echo ""
        
        # Paso 3: Verificar
        verify_config
        echo ""
        
        echo -e "${WHITE}"
        echo "============================================================================="
        echo "   ✅ CONFIGURACIÓN NGINX APLICADA"
        echo "============================================================================="
        echo -e "${NC}"
        
        echo -e "${GREEN}🎉 CORRECCIONES APLICADAS:${NC}"
        echo -e "${WHITE}  • ✅ Health checks añadidos: /savings/health, /budget-overview/health${NC}"
        echo -e "${WHITE}  • ✅ Endpoint corregido: /update/locale${NC}"
        echo -e "${WHITE}  • ✅ Endpoint añadido: /money-flow/data${NC}"
        echo -e "${WHITE}  • ✅ Métodos corregidos: GET /budget-overview, /transactions/history${NC}"
        echo -e "${WHITE}  • ✅ Errores tipográficos corregidos${NC}"
        
        echo -e "\n${CYAN}📋 PRÓXIMOS PASOS:${NC}"
        echo -e "${WHITE}  1. Ejecutar tests de producción: ./tests/endpoints/test_production_endpoints.sh${NC}"
        echo -e "${WHITE}  2. Verificar que todos los servicios estén corriendo: ./restart_services_vps.sh${NC}"
        echo -e "${WHITE}  3. Revisar logs si hay errores: tail -f /var/log/nginx/herobudget_error.log${NC}"
        
        echo -e "\n${GREEN}🚀 ¡Configuración lista para testing!${NC}"
    else
        echo -e "\n${RED}❌ Error aplicando configuración${NC}"
        exit 1
    fi
}

# Verificar que se ejecuta como root o con sudo
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}❌ Este script debe ejecutarse como root o con sudo${NC}"
    echo -e "${YELLOW}Uso: sudo ./apply_nginx_config.sh${NC}"
    exit 1
fi

# Ejecutar función principal
main 