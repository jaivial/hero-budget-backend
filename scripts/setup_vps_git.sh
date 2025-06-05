#!/bin/bash
# Script para configurar Git en el VPS
# Versión: 1.0
# Uso: ./setup_vps_git.sh

set -e

# Configuración
VPS_HOST="178.16.130.178"
VPS_USER="root"
VPS_PATH="/opt/hero_budget/backend"
REPO_URL="https://github.com/jaivial/herobudget-backend.git"

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

# Función principal
main() {
    echo "🔧 Configurando Git en el VPS"
    echo "=============================="
    
    log_info "Conectando al VPS..."
    
    ssh $VPS_USER@$VPS_HOST << 'EOF'
        cd /opt/hero_budget/backend
        
        echo "🔍 Verificando estado actual..."
        if [ -d ".git" ]; then
            echo "✅ Repositorio Git ya existe"
            git status --porcelain
        else
            echo "📦 Inicializando repositorio Git..."
            git init
            git config user.name "jaivial"
            git config user.email "jaimevillalcon@hotmail.com"
        fi
        
        echo ""
        echo "🔗 Configurando remote origin..."
        if git remote get-url origin >/dev/null 2>&1; then
            echo "✅ Remote origin ya configurado:"
            git remote -v
        else
            git remote add origin https://github.com/jaivial/herobudget-backend.git
            echo "✅ Remote origin configurado"
        fi
        
        echo ""
        echo "📋 Estado del repositorio:"
        echo "Branch actual: $(git branch --show-current 2>/dev/null || echo 'ninguno')"
        echo "Remote URL: $(git remote get-url origin 2>/dev/null || echo 'no configurado')"
        echo "Archivos sin seguimiento: $(git status --porcelain | wc -l)"
EOF
    
    log_success "Configuración Git completada"
    
    echo ""
    log_warning "NOTA IMPORTANTE:"
    echo "Para que el despliegue automático funcione, necesitas:"
    echo "1. Hacer el repositorio público, O"
    echo "2. Configurar un token de acceso personal en el VPS"
    echo ""
    echo "Para configurar token de acceso:"
    echo "1. Ve a GitHub → Settings → Developer settings → Personal access tokens"
    echo "2. Genera un token con permisos de 'repo'"
    echo "3. Ejecuta en el VPS:"
    echo "   git remote set-url origin https://TOKEN@github.com/jaivial/herobudget-backend.git"
}

# Ejecutar
main 