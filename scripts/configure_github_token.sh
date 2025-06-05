#!/bin/bash
# Script para configurar token de GitHub en VPS
# Versión: 1.0
# Uso: ./configure_github_token.sh [TOKEN]

set -e

# Configuración
VPS_HOST="178.16.130.178"
VPS_USER="root"
VPS_PATH="/opt/hero_budget/backend"
GITHUB_USER="jaivial"
REPO_NAME="herobudget-backend"

# Colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Verificar argumentos
if [ $# -eq 0 ]; then
    echo "🔑 Configuración de Token GitHub"
    echo "================================"
    echo ""
    echo "Uso: $0 [TOKEN]"
    echo ""
    echo "Donde TOKEN es tu Personal Access Token de GitHub"
    echo ""
    echo "Para generar el token:"
    echo "1. Ve a GitHub → Settings → Developer settings → Personal access tokens"
    echo "2. Genera un token con permisos 'repo'"
    echo "3. Copia el token y úsalo con este script"
    echo ""
    exit 1
fi

TOKEN="$1"

# Función principal
main() {
    echo "🔑 Configurando Token GitHub en VPS"
    echo "===================================="
    echo "Usuario: $GITHUB_USER"
    echo "Repositorio: $REPO_NAME"
    echo "Token: ${TOKEN:0:20}..." # Mostrar solo los primeros 20 caracteres
    echo ""
    
    log_info "Conectando al VPS y configurando token..."
    
    # Configurar el remote con token
    ssh $VPS_USER@$VPS_HOST << EOF
        cd $VPS_PATH
        
        echo "🔗 Configurando remote con token..."
        git remote set-url origin https://${GITHUB_USER}:${TOKEN}@github.com/${GITHUB_USER}/${REPO_NAME}.git
        
        echo "🧪 Probando acceso al repositorio..."
        if git ls-remote origin main >/dev/null 2>&1; then
            echo "✅ Token configurado correctamente"
            echo "✅ Acceso al repositorio verificado"
        else
            echo "❌ Error: No se puede acceder al repositorio"
            echo "Verifica que el token tenga permisos 'repo'"
            exit 1
        fi
        
        echo ""
        echo "📋 Configuración actual:"
        echo "Remote URL: \$(git remote get-url origin | sed 's/:[^@]*@/:***@/')"
EOF
    
    if [ $? -eq 0 ]; then
        log_success "Token configurado exitosamente"
        echo ""
        log_info "Ahora puedes usar el despliegue automático:"
        echo "  ./scripts/deploy_backend.sh --force"
    else
        log_error "Error configurando el token"
        echo ""
        log_warning "Posibles soluciones:"
        echo "1. Verifica que el token sea válido"
        echo "2. Asegúrate de que tenga permisos 'repo'"
        echo "3. Verifica que el repositorio existe y tienes acceso"
    fi
}

# Ejecutar
main 