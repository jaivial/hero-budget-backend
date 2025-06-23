#!/bin/bash

# =============================================================================
# SCRIPT PARA CORREGIR RUTAS DE EXPENSE EN NGINX
# =============================================================================

VPS_HOST="178.16.130.178"
NGINX_CONFIG="/etc/nginx/sites-available/herobudget"

echo "🔧 Corrigiendo rutas de expense en Nginx..."

# Crear backup
ssh root@$VPS_HOST "cp $NGINX_CONFIG ${NGINX_CONFIG}.$(date +%Y%m%d_%H%M%S)"

# Crear archivo temporal con las correcciones
cat << 'EOF' > /tmp/nginx_expense_fix.txt
    # EXPENSE ENDPOINTS (Ruta de prefijo) - CORREGIDO
    location /expense/ {
        proxy_pass http://backend_expense/expense/;
        proxy_set_header Content-Type application/json;
        proxy_set_header Accept application/json;
    }

    location /expense {
        proxy_pass http://backend_expense/expense;
        proxy_set_header Content-Type application/json;
        proxy_set_header Accept application/json;
    }

    # LEGACY: Mantener expenses por compatibilidad
    location /expenses/ {
        proxy_pass http://backend_expense/expense/;
        proxy_set_header Content-Type application/json;
        proxy_set_header Accept application/json;
    }

    location /expenses {
        proxy_pass http://backend_expense/expense;
        proxy_set_header Content-Type application/json;
        proxy_set_header Accept application/json;
    }
EOF

# Copiar el archivo temporal al VPS
scp /tmp/nginx_expense_fix.txt root@$VPS_HOST:/tmp/

# Aplicar la corrección usando awk
ssh root@$VPS_HOST "awk '
BEGIN { in_expense_section = 0; skip_until_bills = 0 }

# Detectar inicio de sección EXPENSE
/# EXPENSE ENDPOINTS/ {
    in_expense_section = 1
    skip_until_bills = 1
    # Imprimir la corrección completa
    while ((getline line < "/tmp/nginx_expense_fix.txt") > 0) {
        print line
    }
    close("/tmp/nginx_expense_fix.txt")
    next
}

# Detectar inicio de sección BILLS para detener el skip
/# BILLS ENDPOINTS/ {
    skip_until_bills = 0
    in_expense_section = 0
}

# Skip líneas entre EXPENSE y BILLS
skip_until_bills == 1 && /# BILLS ENDPOINTS/ {
    skip_until_bills = 0
    in_expense_section = 0
}

# Imprimir líneas que no están en la sección de expense a saltar
skip_until_bills == 0 { print }

' $NGINX_CONFIG > /tmp/herobudget_fixed.conf && mv /tmp/herobudget_fixed.conf $NGINX_CONFIG"

# Verificar configuración
echo "🔍 Verificando configuración Nginx..."
if ssh root@$VPS_HOST "nginx -t"; then
    echo "✅ Configuración Nginx válida"
    
    # Recargar Nginx
    echo "🔄 Recargando Nginx..."
    ssh root@$VPS_HOST "systemctl reload nginx"
    
    if [ $? -eq 0 ]; then
        echo "✅ Nginx recargado exitosamente"
        echo ""
        echo "🎯 RUTAS CORREGIDAS:"
        echo "  • /expense/add → backend_expense/expense/add"
        echo "  • /expenses/add → backend_expense/expense/add (legacy)"
        echo ""
        echo "🧪 PROBAR ENDPOINTS:"
        echo "  curl -X POST 'https://herobudget.jaimedigitalstudio.com/expense/add' \\"
        echo "       -H 'Content-Type: application/json' \\"
        echo "       -d '{\"user_id\":\"test\",\"amount\":50,\"date\":\"2025-06-23\",\"category\":\"food\",\"payment_method\":\"cash\"}'"
    else
        echo "❌ Error recargando Nginx"
        exit 1
    fi
else
    echo "❌ Error en configuración Nginx"
    echo "📋 Restaurando backup..."
    ssh root@$VPS_HOST "cp ${NGINX_CONFIG}.backup $NGINX_CONFIG"
    exit 1
fi

# Limpiar archivos temporales
rm -f /tmp/nginx_expense_fix.txt
ssh root@$VPS_HOST "rm -f /tmp/nginx_expense_fix.txt /tmp/herobudget_fixed.conf"

echo "🎉 Corrección completada exitosamente"