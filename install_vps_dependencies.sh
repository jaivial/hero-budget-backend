#!/bin/bash

# =============================================================================
# SCRIPT PARA INSTALAR DEPENDENCIAS NECESARIAS EN VPS
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
echo "   📦 INSTALANDO DEPENDENCIAS PARA SERVICIOS GO - VPS"
echo "============================================================================="
echo -e "${NC}"

# Actualizar repositorios
echo -e "${YELLOW}🔄 Actualizando repositorios...${NC}"
apt-get update

# Instalar dependencias esenciales
echo -e "${YELLOW}📦 Instalando dependencias esenciales...${NC}"

# SQLite3 development libraries
if ! dpkg -l | grep -q libsqlite3-dev; then
    echo -e "${BLUE}  🔧 Instalando libsqlite3-dev...${NC}"
    apt-get install -y libsqlite3-dev
else
    echo -e "${GREEN}  ✅ libsqlite3-dev ya está instalado${NC}"
fi

# Build essentials
if ! dpkg -l | grep -q build-essential; then
    echo -e "${BLUE}  🔧 Instalando build-essential...${NC}"
    apt-get install -y build-essential
else
    echo -e "${GREEN}  ✅ build-essential ya está instalado${NC}"
fi

# GCC compiler
if ! command -v gcc &> /dev/null; then
    echo -e "${BLUE}  🔧 Instalando gcc...${NC}"
    apt-get install -y gcc
else
    echo -e "${GREEN}  ✅ gcc ya está instalado${NC}"
fi

# PKG-Config
if ! command -v pkg-config &> /dev/null; then
    echo -e "${BLUE}  🔧 Instalando pkg-config...${NC}"
    apt-get install -y pkg-config
else
    echo -e "${GREEN}  ✅ pkg-config ya está instalado${NC}"
fi

# Curl (si no está instalado)
if ! command -v curl &> /dev/null; then
    echo -e "${BLUE}  🔧 Instalando curl...${NC}"
    apt-get install -y curl
else
    echo -e "${GREEN}  ✅ curl ya está instalado${NC}"
fi

# Git (si no está instalado)
if ! command -v git &> /dev/null; then
    echo -e "${BLUE}  🔧 Instalando git...${NC}"
    apt-get install -y git
else
    echo -e "${GREEN}  ✅ git ya está instalado${NC}"
fi

echo -e "\n${CYAN}🔍 Verificando instalación de Go...${NC}"

if command -v go &> /dev/null; then
    local go_version=$(go version)
    echo -e "${GREEN}✅ Go instalado: $go_version${NC}"
    
    # Verificar CGO
    if go env CGO_ENABLED | grep -q "1"; then
        echo -e "${GREEN}✅ CGO habilitado${NC}"
    else
        echo -e "${YELLOW}⚠️  CGO deshabilitado - configurando...${NC}"
        export CGO_ENABLED=1
    fi
else
    echo -e "${RED}❌ Go no está instalado${NC}"
    echo -e "${YELLOW}💡 Instalar Go siguiendo: https://golang.org/doc/install${NC}"
fi

echo -e "\n${CYAN}🔍 Verificando librerías SQLite3...${NC}"

# Verificar ubicaciones comunes de SQLite3
sqlite_locations=(
    "/usr/lib/x86_64-linux-gnu/libsqlite3.so"
    "/usr/local/lib/libsqlite3.so"
    "/usr/lib/libsqlite3.so"
    "/lib/x86_64-linux-gnu/libsqlite3.so.0"
)

found_sqlite=false
for location in "${sqlite_locations[@]}"; do
    if [ -f "$location" ]; then
        echo -e "${GREEN}✅ SQLite3 library encontrada en: $location${NC}"
        found_sqlite=true
        break
    fi
done

if [ "$found_sqlite" = false ]; then
    echo -e "${RED}❌ SQLite3 library no encontrada${NC}"
    echo -e "${BLUE}🔧 Instalando sqlite3...${NC}"
    apt-get install -y sqlite3 libsqlite3-0
fi

echo -e "\n${CYAN}🔍 Verificando permisos de directorio...${NC}"

# Verificar y corregir permisos
if [ -d "/opt/hero_budget" ]; then
    echo -e "${BLUE}🔧 Configurando permisos para /opt/hero_budget...${NC}"
    chown -R root:root /opt/hero_budget
    chmod -R 755 /opt/hero_budget
    echo -e "${GREEN}✅ Permisos configurados${NC}"
else
    echo -e "${YELLOW}⚠️  Directorio /opt/hero_budget no encontrado${NC}"
fi

# Limpiar cache
echo -e "\n${YELLOW}🧹 Limpiando cache de apt...${NC}"
apt-get autoremove -y
apt-get autoclean

echo -e "\n${WHITE}"
echo "============================================================================="
echo "   ✅ DEPENDENCIAS INSTALADAS CORRECTAMENTE"
echo "============================================================================="
echo -e "${NC}"

echo -e "${GREEN}🎉 DEPENDENCIAS INSTALADAS:${NC}"
echo -e "${WHITE}  • libsqlite3-dev${NC}"
echo -e "${WHITE}  • build-essential${NC}"
echo -e "${WHITE}  • gcc${NC}"
echo -e "${WHITE}  • pkg-config${NC}"
echo -e "${WHITE}  • curl${NC}"
echo -e "${WHITE}  • git${NC}"

echo -e "\n${CYAN}📋 SIGUIENTE PASO:${NC}"
echo -e "${WHITE}  cd /opt/hero_budget/backend${NC}"
echo -e "${WHITE}  ./restart_services_vps.sh${NC}"

echo "" 