#!/bin/bash

# =============================================================================
# CONFIGURACIÓN DE DEPLOYMENT PARA SYNC SERVICE
# Configuraciones específicas para entornos local y VPS
# =============================================================================

# Detectar entorno
if [[ "$HOSTNAME" == *"vps"* ]] || [[ -d "/opt/hero_budget" ]]; then
    ENVIRONMENT="vps"
else
    ENVIRONMENT="local"
fi

# Configuración por entorno
if [ "$ENVIRONMENT" == "vps" ]; then
    # Configuración VPS
    export SYNC_SERVICE_PORT=8101
    export DATABASE_PATH="/opt/hero_budget/backend/budget_data.db"
    export LOG_LEVEL=info
    export CORS_ALLOWED_ORIGINS="*"
    export MAX_BATCH_SIZE=100
    export DB_TIMEOUT=30
    export GO_PATH="/usr/local/go/bin/go"
    
    echo "🌐 Configuración VPS cargada"
    echo "   • Puerto: $SYNC_SERVICE_PORT"
    echo "   • Base de datos: $DATABASE_PATH"
    echo "   • Log level: $LOG_LEVEL"
else
    # Configuración Local
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
    
    export SYNC_SERVICE_PORT=8101
    export DATABASE_PATH="$PROJECT_ROOT/budget_data.db"
    export LOG_LEVEL=debug
    export CORS_ALLOWED_ORIGINS="http://localhost:*"
    export MAX_BATCH_SIZE=50
    export DB_TIMEOUT=15
    export GO_PATH="go"
    
    echo "🏠 Configuración LOCAL cargada"
    echo "   • Puerto: $SYNC_SERVICE_PORT"
    echo "   • Base de datos: $DATABASE_PATH"
    echo "   • Log level: $LOG_LEVEL"
fi

# Configuración común
export CORS_ALLOWED_METHODS="GET,POST,PUT,DELETE,OPTIONS"
export CORS_ALLOWED_HEADERS="Content-Type,Authorization"
export MAX_RETRY_ATTEMPTS=3
export RETRY_DELAY_SECONDS=5
export LOG_RETENTION_DAYS=30
export CLEANUP_INTERVAL_HOURS=24

# Función para verificar dependencias
verify_dependencies() {
    echo "🔍 Verificando dependencias..."
    
    # Verificar Go
    if ! command -v "$GO_PATH" &> /dev/null; then
        echo "❌ Go no encontrado en: $GO_PATH"
        return 1
    fi
    
    # Verificar SQLite
    if ! command -v sqlite3 &> /dev/null; then
        echo "❌ SQLite3 no encontrado"
        return 1
    fi
    
    # Verificar directorio de base de datos
    DB_DIR=$(dirname "$DATABASE_PATH")
    if [ ! -d "$DB_DIR" ]; then
        echo "⚠️  Directorio de base de datos no existe: $DB_DIR"
        echo "   Creando directorio..."
        mkdir -p "$DB_DIR"
    fi
    
    echo "✅ Dependencias verificadas"
    return 0
}

# Función para crear directorio de logs
setup_logging() {
    local log_dir
    if [ "$ENVIRONMENT" == "vps" ]; then
        log_dir="/opt/hero_budget/logs/sync_service"
    else
        log_dir="$(dirname "$DATABASE_PATH")/logs/sync_service"
    fi
    
    if [ ! -d "$log_dir" ]; then
        mkdir -p "$log_dir"
        echo "📁 Directorio de logs creado: $log_dir"
    fi
    
    export LOG_DIR="$log_dir"
}

# Función para mostrar configuración actual
show_config() {
    echo ""
    echo "🔧 CONFIGURACIÓN SYNC SERVICE ($ENVIRONMENT)"
    echo "============================================"
    echo "Puerto: $SYNC_SERVICE_PORT"
    echo "Base de datos: $DATABASE_PATH"
    echo "Log level: $LOG_LEVEL"
    echo "Directorio logs: $LOG_DIR"
    echo "Max batch size: $MAX_BATCH_SIZE"
    echo "DB timeout: $DB_TIMEOUT segundos"
    echo "CORS origins: $CORS_ALLOWED_ORIGINS"
    echo ""
}

# Ejecutar configuración automáticamente
verify_dependencies
setup_logging

# Mostrar configuración si se ejecuta directamente
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    show_config
fi