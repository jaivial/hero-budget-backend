#!/bin/bash

# =============================================================================
# SCRIPT ESPECÍFICO PARA REPARAR GOOGLE_AUTH SERVICE
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

BASE_PATH="/opt/hero_budget/backend"
SERVICE_PATH="$BASE_PATH/google_auth"

echo -e "${WHITE}"
echo "==========================================================================="
echo "   🔧 REPARANDO GOOGLE_AUTH SERVICE - DIAGNÓSTICO COMPLETO"
echo "==========================================================================="
echo -e "${NC}"

# Función para detener google_auth existente
stop_google_auth() {
    echo -e "${YELLOW}🛑 Deteniendo google_auth existente...${NC}"
    
    PID=$(lsof -ti:8081 2>/dev/null)
    if [ ! -z "$PID" ]; then
        echo -e "${YELLOW}  Deteniendo proceso en puerto 8081 (PID: $PID)${NC}"
        kill -9 $PID 2>/dev/null
        sleep 2
    fi
    
    # Buscar procesos de google_auth
    PIDS=$(pgrep -f "google_auth" 2>/dev/null)
    if [ ! -z "$PIDS" ]; then
        echo -e "${YELLOW}  Deteniendo procesos google_auth: $PIDS${NC}"
        kill -9 $PIDS 2>/dev/null
    fi
    
    echo -e "${GREEN}✅ Google_auth detenido${NC}"
}

# Función de diagnóstico
diagnose_google_auth() {
    echo -e "${CYAN}🔍 DIAGNÓSTICO DE GOOGLE_AUTH:${NC}"
    
    # Verificar directorio
    if [ ! -d "$SERVICE_PATH" ]; then
        echo -e "${RED}❌ Directorio no encontrado: $SERVICE_PATH${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ Directorio encontrado: $SERVICE_PATH${NC}"
    
    # Verificar main.go
    if [ ! -f "$SERVICE_PATH/main.go" ]; then
        echo -e "${RED}❌ main.go no encontrado${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ main.go encontrado${NC}"
    
    # Verificar go.mod
    if [ ! -f "$SERVICE_PATH/go.mod" ]; then
        echo -e "${RED}❌ go.mod no encontrado${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ go.mod encontrado${NC}"
    
    # Verificar Go instalación
    if ! command -v /usr/local/go/bin/go &> /dev/null; then
        echo -e "${RED}❌ Go no encontrado en /usr/local/go/bin/go${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ Go encontrado: $(/usr/local/go/bin/go version)${NC}"
    
    return 0
}

# Función para reparar dependencias
fix_dependencies() {
    echo -e "${CYAN}🔧 REPARANDO DEPENDENCIAS:${NC}"
    
    cd "$SERVICE_PATH" || return 1
    
    # Limpiar cache de módulos
    echo -e "${YELLOW}  🧹 Limpiando cache de módulos...${NC}"
    /usr/local/go/bin/go clean -modcache
    
    # Verificar y reparar go.mod
    echo -e "${YELLOW}  📦 Verificando go.mod...${NC}"
    /usr/local/go/bin/go mod verify
    
    # Descargar dependencias con CGO habilitado
    echo -e "${YELLOW}  ⬇️ Descargando dependencias...${NC}"
    env CGO_ENABLED=1 /usr/local/go/bin/go mod download
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Error descargando dependencias${NC}"
        return 1
    fi
    
    # Tidy módulos
    echo -e "${YELLOW}  🔧 Ejecutando go mod tidy...${NC}"
    /usr/local/go/bin/go mod tidy
    
    echo -e "${GREEN}✅ Dependencias reparadas${NC}"
    return 0
}

# Función para compilar y probar
test_compilation() {
    echo -e "${CYAN}🔨 PROBANDO COMPILACIÓN:${NC}"
    
    cd "$SERVICE_PATH" || return 1
    
    # Compilar sin ejecutar
    echo -e "${YELLOW}  🔨 Compilando google_auth...${NC}"
    env CGO_ENABLED=1 /usr/local/go/bin/go build -o google_auth_test main.go
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Error de compilación${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✅ Compilación exitosa${NC}"
    
    # Limpiar archivo de prueba
    rm -f google_auth_test
    
    return 0
}

# Función para iniciar google_auth correctamente
start_google_auth() {
    echo -e "${CYAN}🚀 INICIANDO GOOGLE_AUTH:${NC}"
    
    cd "$SERVICE_PATH" || return 1
    
    # Eliminar log anterior
    rm -f /tmp/google_auth.log
    
    # Iniciar con configuración completa
    echo -e "${YELLOW}  🚀 Ejecutando google_auth...${NC}"
    nohup env CGO_ENABLED=1 GOMAXPROCS=2 /usr/local/go/bin/go run main.go > /tmp/google_auth.log 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ Google_auth iniciado (PID: $pid)${NC}"
    echo -e "${WHITE}      Logs: /tmp/google_auth.log${NC}"
    
    # Esperar un poco
    sleep 3
    
    return 0
}

# Función para verificar servicio
verify_google_auth() {
    echo -e "${CYAN}🔍 VERIFICANDO GOOGLE_AUTH:${NC}"
    
    # Verificar que el proceso esté corriendo
    if ! pgrep -f "google_auth" > /dev/null; then
        echo -e "${RED}❌ Proceso google_auth no está corriendo${NC}"
        echo -e "${YELLOW}📋 Revisando logs:${NC}"
        tail -10 /tmp/google_auth.log
        return 1
    fi
    
    # Verificar puerto
    if ! lsof -i:8081 > /dev/null 2>&1; then
        echo -e "${RED}❌ Puerto 8081 no está en uso${NC}"
        return 1
    fi
    echo -e "${GREEN}✅ Puerto 8081 está siendo usado${NC}"
    
    # Probar endpoint de health
    sleep 2
    local response=$(curl -s -w "%{http_code}" -o /dev/null "http://localhost:8081/health" 2>/dev/null)
    
    if [ "$response" = "200" ]; then
        echo -e "${GREEN}✅ Google_auth está respondiendo correctamente (Status: $response)${NC}"
        return 0
    else
        echo -e "${RED}❌ Google_auth no está respondiendo (Status: $response)${NC}"
        echo -e "${YELLOW}📋 Últimos logs:${NC}"
        tail -10 /tmp/google_auth.log
        return 1
    fi
}

# EJECUCIÓN PRINCIPAL
echo -e "${GREEN}📂 Trabajando en: $SERVICE_PATH${NC}"

# Paso 1: Detener servicio existente
stop_google_auth

# Paso 2: Diagnóstico
if ! diagnose_google_auth; then
    echo -e "${RED}💥 Diagnóstico falló - abortando${NC}"
    exit 1
fi

# Paso 3: Reparar dependencias
if ! fix_dependencies; then
    echo -e "${RED}💥 Reparación de dependencias falló${NC}"
    exit 1
fi

# Paso 4: Probar compilación
if ! test_compilation; then
    echo -e "${RED}💥 Prueba de compilación falló${NC}"
    exit 1
fi

# Paso 5: Iniciar servicio
if ! start_google_auth; then
    echo -e "${RED}💥 Inicio del servicio falló${NC}"
    exit 1
fi

# Paso 6: Verificar servicio
if ! verify_google_auth; then
    echo -e "${RED}💥 Verificación del servicio falló${NC}"
    exit 1
fi

echo -e "\n${WHITE}"
echo "==========================================================================="
echo "   ✅ GOOGLE_AUTH REPARADO Y FUNCIONANDO"
echo "==========================================================================="
echo -e "${NC}"

echo -e "${GREEN}🎉 ESTADO FINAL:${NC}"
echo -e "${WHITE}  • Servicio: google_auth${NC}"
echo -e "${WHITE}  • Puerto: 8081${NC}"
echo -e "${WHITE}  • Estado: ✅ Funcionando${NC}"
echo -e "${WHITE}  • Health: http://localhost:8081/health${NC}"
echo -e "${WHITE}  • Auth: http://localhost:8081/auth/google${NC}"

echo -e "\n${CYAN}📋 COMANDOS ÚTILES:${NC}"
echo -e "${WHITE}  • Ver logs: tail -f /tmp/google_auth.log${NC}"
echo -e "${WHITE}  • Probar health: curl http://localhost:8081/health${NC}"
echo -e "${WHITE}  • Verificar puerto: lsof -i:8081${NC}"

echo "" 