#!/bin/bash
# Script de Configuración SSH - Hero Budget Backend
# Versión: 1.0
# Uso: ./setup_ssh.sh

set -e

# Configuración
VPS_HOST="178.16.130.178"
VPS_USER="root"
USER_EMAIL="jaimevillalcon@hotmail.com"

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
    echo "🔑 Configuración SSH para Hero Budget Backend"
    echo "=============================================="
    echo "VPS: $VPS_USER@$VPS_HOST"
    echo "Email: $USER_EMAIL"
    echo ""
    
    # Verificar si ya existe clave SSH
    if [ -f ~/.ssh/id_rsa ]; then
        log_info "Clave SSH existente encontrada"
        echo "📍 Ubicación: ~/.ssh/id_rsa"
        echo "🔍 Clave pública:"
        cat ~/.ssh/id_rsa.pub
        echo ""
        
        read -p "¿Quieres usar la clave existente? (Y/n): " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Nn]$ ]]; then
            generate_new_key=true
        else
            generate_new_key=false
        fi
    else
        log_info "No se encontró clave SSH existente"
        generate_new_key=true
    fi
    
    # Generar nueva clave si es necesario
    if [ "$generate_new_key" = true ]; then
        log_info "Generando nueva clave SSH..."
        
        # Crear directorio .ssh si no existe
        mkdir -p ~/.ssh
        chmod 700 ~/.ssh
        
        # Hacer backup de clave existente si existe
        if [ -f ~/.ssh/id_rsa ]; then
            log_warning "Haciendo backup de clave existente..."
            cp ~/.ssh/id_rsa ~/.ssh/id_rsa.backup.$(date +%Y%m%d_%H%M%S)
            cp ~/.ssh/id_rsa.pub ~/.ssh/id_rsa.pub.backup.$(date +%Y%m%d_%H%M%S)
        fi
        
        # Generar nueva clave
        ssh-keygen -t rsa -b 4096 -C "$USER_EMAIL" -f ~/.ssh/id_rsa -N ""
        log_success "Nueva clave SSH generada"
    fi
    
    # Mostrar clave pública
    echo ""
    log_info "Tu clave pública SSH:"
    echo "===================="
    cat ~/.ssh/id_rsa.pub
    echo "===================="
    echo ""
    
    # Verificar conectividad básica al VPS
    log_info "Verificando conectividad básica al VPS..."
    if ping -c 1 -W 5 $VPS_HOST >/dev/null 2>&1; then
        log_success "VPS es accesible vía ping"
    else
        log_warning "VPS no responde a ping (puede ser normal si ICMP está bloqueado)"
    fi
    
    # Copiar clave al VPS
    echo ""
    log_info "Copiando clave SSH al VPS..."
    log_warning "Se te pedirá la contraseña del servidor"
    
    if ssh-copy-id $VPS_USER@$VPS_HOST; then
        log_success "Clave SSH copiada exitosamente"
    else
        log_error "Error copiando clave SSH"
        echo ""
        echo "🔧 Opciones de solución:"
        echo "1. Verificar que la contraseña del VPS sea correcta"
        echo "2. Verificar que el SSH esté habilitado en el VPS"
        echo "3. Copiar manualmente la clave:"
        echo ""
        echo "   Copia esta clave pública:"
        cat ~/.ssh/id_rsa.pub
        echo ""
        echo "   Luego ejecuta en el VPS:"
        echo "   ssh $VPS_USER@$VPS_HOST"
        echo "   mkdir -p ~/.ssh"
        echo "   echo '[PEGAR_CLAVE_AQUÍ]' >> ~/.ssh/authorized_keys"
        echo "   chmod 600 ~/.ssh/authorized_keys"
        echo "   chmod 700 ~/.ssh"
        echo ""
        exit 1
    fi
    
    # Verificar conexión sin contraseña
    echo ""
    log_info "Verificando conexión SSH sin contraseña..."
    if ssh -o ConnectTimeout=10 -o PasswordAuthentication=no $VPS_USER@$VPS_HOST "echo 'Conexión SSH sin contraseña OK - $(date)'"; then
        log_success "✅ ¡SSH configurado correctamente!"
        echo ""
        echo "🎉 Configuración completada exitosamente"
        echo "Ahora puedes:"
        echo "• Ejecutar scripts de despliegue sin contraseña"
        echo "• Usar Jenkins para CI/CD automático"
        echo "• Gestionar servicios remotamente"
        echo ""
    else
        log_error "La conexión sin contraseña falló"
        echo ""
        echo "🔧 Troubleshooting:"
        echo "1. Verificar permisos en el VPS:"
        echo "   ssh $VPS_USER@$VPS_HOST 'ls -la ~/.ssh/'"
        echo ""
        echo "2. Verificar configuración SSH del VPS:"
        echo "   ssh $VPS_USER@$VPS_HOST 'cat /etc/ssh/sshd_config | grep -E \"PubkeyAuthentication|AuthorizedKeysFile\"'"
        echo ""
        exit 1
    fi
}

# Ejecutar función principal
main "$@" 