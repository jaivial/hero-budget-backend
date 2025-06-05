#!/bin/bash
# Preparar estructura en VPS para backend Hero Budget

VPS_HOST="178.16.130.178"
VPS_USER="root"

echo "🚀 Preparando estructura en VPS..."

ssh $VPS_USER@$VPS_HOST << 'ENDSSH'
# Crear directorios necesarios
mkdir -p /opt/hero_budget/{backend,backups,logs}

# Configurar permisos
chown -R root:root /opt/hero_budget
chmod -R 755 /opt/hero_budget

# Crear directorio para logs de microservicios
mkdir -p /var/log/herobudget

echo "✅ Estructura VPS preparada"
ENDSSH
