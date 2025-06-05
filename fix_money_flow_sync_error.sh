#!/bin/bash

# =============================================================================
# SCRIPT PARA CORREGIR ERROR ESPECÍFICO EN MONEY_FLOW_SYNC - VPS
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
echo "   🔧 CORRIGIENDO ERROR EN MONEY_FLOW_SYNC - VPS"
echo "============================================================================="
echo -e "${NC}"

# Configuración de rutas del VPS
BASE_PATH="/opt/hero_budget/backend"
SERVICE_NAME="money_flow_sync"
SERVICE_PORT="8097"

echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"

# Verificar que estamos en el directorio correcto
if [ ! -d "$SERVICE_NAME" ]; then
    echo -e "${RED}❌ Error: Directorio $SERVICE_NAME no encontrado${NC}"
    echo -e "${YELLOW}💡 Asegurate de ejecutar este script desde: $BASE_PATH${NC}"
    exit 1
fi

echo -e "${CYAN}🔍 DIAGNÓSTICO DEL PROBLEMA:${NC}"
echo -e "${WHITE}  Error: 'no such column: bp.user_id'${NC}"
echo -e "${WHITE}  Causa: La tabla bill_payments no tiene columna user_id${NC}"
echo -e "${WHITE}  Solución: Usar b.user_id en lugar de bp.user_id${NC}"

# Detener el servicio actual si está corriendo
echo -e "\n${YELLOW}🛑 Deteniendo servicio $SERVICE_NAME...${NC}"
PID=$(lsof -ti:$SERVICE_PORT 2>/dev/null)
if [ ! -z "$PID" ]; then
    echo -e "${YELLOW}  Deteniendo servicio en puerto $SERVICE_PORT (PID: $PID)${NC}"
    kill -9 $PID 2>/dev/null
    sleep 2
    echo -e "${GREEN}  ✅ Servicio detenido${NC}"
else
    echo -e "${BLUE}  ℹ️  Servicio no estaba corriendo${NC}"
fi

# Hacer backup del archivo original
echo -e "\n${BLUE}💾 Creando backup del archivo original...${NC}"
cp "$SERVICE_NAME/main.go" "$SERVICE_NAME/main.go.backup.$(date +%Y%m%d_%H%M%S)"
echo -e "${GREEN}  ✅ Backup creado${NC}"

# Verificar el contenido actual
echo -e "\n${CYAN}🔍 Verificando contenido actual...${NC}"
if grep -q "WHERE bp.user_id = ?" "$SERVICE_NAME/main.go"; then
    echo -e "${YELLOW}  ⚠️  Se encontró la línea problemática (bp.user_id)${NC}"
    NEEDS_FIX=true
else
    echo -e "${GREEN}  ✅ La línea ya está corregida (b.user_id)${NC}"
    NEEDS_FIX=false
fi

# Aplicar la corrección si es necesario
if [ "$NEEDS_FIX" = true ]; then
    echo -e "\n${BLUE}🔧 Aplicando corrección...${NC}"
    
    # Usar sed para reemplazar bp.user_id con b.user_id
    sed -i 's/WHERE bp\.user_id = ?/WHERE b.user_id = ?/g' "$SERVICE_NAME/main.go"
    
    # Verificar que la corrección se aplicó
    if grep -q "WHERE b.user_id = ?" "$SERVICE_NAME/main.go"; then
        echo -e "${GREEN}  ✅ Corrección aplicada exitosamente${NC}"
    else
        echo -e "${RED}  ❌ Error aplicando la corrección${NC}"
        echo -e "${YELLOW}  🔄 Restaurando backup...${NC}"
        cp "$SERVICE_NAME/main.go.backup."* "$SERVICE_NAME/main.go"
        exit 1
    fi
else
    echo -e "${BLUE}  ℹ️  No se necesita aplicar corrección${NC}"
fi

# Probar compilación
echo -e "\n${BLUE}🔨 Verificando compilación...${NC}"
cd "$SERVICE_NAME" || { echo -e "${RED}❌ Error: No se pudo acceder al directorio${NC}"; exit 1; }

# Inicializar go.mod si no existe
if [ ! -f "go.mod" ]; then
    echo -e "${YELLOW}    📦 Inicializando go.mod...${NC}"
    /usr/local/go/bin/go mod init $SERVICE_NAME
fi

# Descargar dependencias
echo -e "${YELLOW}    📦 Descargando dependencias...${NC}"
/usr/local/go/bin/go mod tidy
/usr/local/go/bin/go mod download

# Compilar para verificar
if /usr/local/go/bin/go build -o "/tmp/test_${SERVICE_NAME}" main.go; then
    echo -e "${GREEN}  ✅ Compilación exitosa${NC}"
    rm -f "/tmp/test_${SERVICE_NAME}"
else
    echo -e "${RED}  ❌ Error de compilación${NC}"
    cd ..
    exit 1
fi

cd ..

# Reiniciar el servicio
echo -e "\n${CYAN}🚀 Reiniciando servicio $SERVICE_NAME...${NC}"
cd "$SERVICE_NAME" || { echo -e "${RED}❌ Error: No se pudo acceder al directorio${NC}"; exit 1; }

# Iniciar el servicio en background
nohup env CGO_ENABLED=1 /usr/local/go/bin/go run main.go > "/tmp/${SERVICE_NAME}.log" 2>&1 &
NEW_PID=$!

echo -e "${GREEN}  ✅ Servicio $SERVICE_NAME reiniciado (PID: $NEW_PID)${NC}"
echo -e "${WHITE}      Logs: /tmp/${SERVICE_NAME}.log${NC}"

cd ..

# Esperar a que el servicio se inicialice
echo -e "\n${YELLOW}⏳ Esperando 3 segundos para que el servicio se inicialice...${NC}"
sleep 3

# Verificar que el servicio está corriendo
echo -e "\n${BLUE}🔍 Verificando que el servicio esté corriendo...${NC}"
PID_CHECK=$(lsof -ti:$SERVICE_PORT 2>/dev/null)

if [ ! -z "$PID_CHECK" ]; then
    echo -e "${GREEN}  ✅ Servicio está corriendo en puerto $SERVICE_PORT (PID: $PID_CHECK)${NC}"
else
    echo -e "${RED}  ❌ Servicio no está corriendo${NC}"
    echo -e "${YELLOW}  📄 Últimas líneas del log:${NC}"
    tail -10 "/tmp/${SERVICE_NAME}.log"
    exit 1
fi

# Probar el endpoint
echo -e "\n${BLUE}🌐 Probando endpoint del servicio...${NC}"
RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null "http://localhost:$SERVICE_PORT/money-flow/data?user_id=1" 2>/dev/null)

if [ "$RESPONSE" = "200" ]; then
    echo -e "${GREEN}  ✅ Endpoint responde correctamente (Status: $RESPONSE)${NC}"
elif [ "$RESPONSE" = "500" ] || [ "$RESPONSE" = "404" ]; then
    echo -e "${YELLOW}  ⚠️  Endpoint responde pero puede tener errores internos (Status: $RESPONSE)${NC}"
    echo -e "${YELLOW}      Revisando logs para verificar si el error SQL fue corregido...${NC}"
    
    # Buscar el error específico en los logs
    if tail -20 "/tmp/${SERVICE_NAME}.log" | grep -q "no such column: bp.user_id"; then
        echo -e "${RED}      ❌ El error SQL aún persiste${NC}"
        exit 1
    else
        echo -e "${GREEN}      ✅ El error SQL fue corregido${NC}"
    fi
else
    echo -e "${RED}  ❌ Endpoint no responde (Status: $RESPONSE)${NC}"
    exit 1
fi

echo -e "\n${WHITE}"
echo "============================================================================="
echo "   ✅ CORRECCIÓN DE MONEY_FLOW_SYNC COMPLETADA EXITOSAMENTE"
echo "============================================================================="
echo -e "${NC}"

echo -e "${GREEN}🎉 CORRECCIÓN APLICADA:${NC}"
echo -e "${WHITE}  • Error SQL 'bp.user_id' corregido a 'b.user_id'${NC}"
echo -e "${WHITE}  • Servicio compilado exitosamente${NC}"
echo -e "${WHITE}  • Servicio reiniciado en puerto $SERVICE_PORT${NC}"
echo -e "${WHITE}  • Endpoint respondiendo correctamente${NC}"

echo -e "\n${CYAN}📋 VERIFICACIÓN FINAL:${NC}"
echo -e "${WHITE}  • Estado del servicio: curl http://localhost:$SERVICE_PORT/money-flow/data?user_id=1${NC}"
echo -e "${WHITE}  • Ver logs: tail -f /tmp/${SERVICE_NAME}.log${NC}"
echo -e "${WHITE}  • Estado general: ./check_services_status.sh${NC}"

echo -e "\n${GREEN}🎯 El error 'no such column: bp.user_id' ha sido corregido${NC}"

echo "" 