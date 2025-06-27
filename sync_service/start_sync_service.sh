#!/bin/bash

# HeroBudget Sync Service Startup Script
# Script para inicializar y ejecutar el servicio de sincronización offline

set -e

# Cargar configuración de deployment
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
source "$SCRIPT_DIR/deploy_config.sh"

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🔄 HeroBudget Sync Service Startup${NC}"
echo "=================================="

# Función para logging
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Verificar que Go está instalado usando configuración
if ! command -v "$GO_PATH" &> /dev/null; then
    log_error "Go no está instalado en: $GO_PATH. Por favor instalar Go 1.21 o superior."
    exit 1
fi

# Verificar versión de Go
GO_VERSION=$($GO_PATH version | awk '{print $3}' | sed 's/go//')
log_info "Go version detectada: $GO_VERSION (path: $GO_PATH)"

# Navegar al directorio del servicio
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR"

log_info "Directorio de trabajo: $SCRIPT_DIR"

# Crear módulo Go si no existe
if [ ! -f "go.mod" ]; then
    log_info "Inicializando módulo Go..."
    $GO_PATH mod init sync_service
fi

# Instalar dependencias
log_info "Instalando dependencias..."
$GO_PATH get github.com/gorilla/mux@latest
$GO_PATH get github.com/mattn/go-sqlite3@latest

# Limpiar y actualizar módulos
log_info "Actualizando módulos..."
$GO_PATH mod tidy

# Intentar compilación
log_info "Compilando servicio de sincronización..."
if $GO_PATH build -o sync_service main.go; then
    log_info "✅ Compilación exitosa"
    BINARY_PATH="./sync_service"
elif $GO_PATH build -o sync_service .; then
    log_info "✅ Compilación exitosa (módulo completo)"
    BINARY_PATH="./sync_service"
else
    log_warn "⚠️  Compilación falló, intentando ejecución directa..."
    BINARY_PATH=""
fi

# Verificar base de datos
if [ ! -f "$DATABASE_PATH" ]; then
    log_warn "Base de datos no encontrada en $DATABASE_PATH"
    log_info "El servicio creará las tablas necesarias automáticamente"
fi

# Configurar variables de entorno
export SYNC_SERVICE_PORT="$SYNC_SERVICE_PORT"
export DATABASE_PATH="$DATABASE_PATH"

# Mostrar configuración
echo ""
log_info "📋 Configuración del servicio:"
echo "   • Puerto: $SYNC_SERVICE_PORT"
echo "   • Base de datos: $DATABASE_PATH"
echo "   • Nivel de log: $LOG_LEVEL"
echo ""

# Función para manejar señales de interrupción
cleanup() {
    log_info "🛑 Deteniendo servicio de sincronización..."
    exit 0
}

trap cleanup SIGINT SIGTERM

# Iniciar servicio
log_info "🚀 Iniciando servicio de sincronización..."
echo ""

if [ -n "$BINARY_PATH" ] && [ -f "$BINARY_PATH" ]; then
    # Ejecutar binario compilado
    log_info "Ejecutando binario compilado..."
    exec "$BINARY_PATH"
else
    # Ejecutar con go run como fallback
    log_info "Ejecutando con Go runtime..."
    exec $GO_PATH run main.go
fi