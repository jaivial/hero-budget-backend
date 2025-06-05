#!/bin/bash

# ============================================================================
# Script para arreglar problemas de nginx y servicios que fallan
# ============================================================================

LOG_FILE="/tmp/fix_nginx_services.log"
exec > >(tee -a "$LOG_FILE") 2>&1

echo "=========================================="
echo "🔧 ARREGLANDO PROBLEMAS DE NGINX Y SERVICIOS"
echo "=========================================="
echo "Timestamp: $(date)"

# Función para matar procesos de servicios
kill_service_processes() {
    local service_name=$1
    echo "🔄 Matando procesos de $service_name..."
    
    # Matar procesos por puerto
    case $service_name in
        "google_auth") pkill -f "8081" ;;
        "signup") pkill -f "8082" ;;
        *) pkill -f "$service_name" ;;
    esac
    
    sleep 2
}

# Función para arreglar dependencias
fix_dependencies() {
    echo "🔧 Arreglando dependencias de Go..."
    
    # Ir al directorio raíz del backend
    cd /opt/hero_budget/backend
    
    # Arreglar signup - eliminar referencias a vendor
    echo "📝 Arreglando signup..."
    cd signup
    if [ -f "go.mod" ]; then
        # Eliminar replace directives problemáticas
        sed -i '/replace.*vendor/d' go.mod
        sed -i '/replace.*github.com\/chai2010\/webp/d' go.mod
        
        # Limpiar y actualizar dependencias
        /usr/local/go/bin/go mod tidy
        /usr/local/go/bin/go mod download
    fi
    
    # Arreglar google_auth
    echo "📝 Arreglando google_auth..."
    cd ../google_auth
    if [ -f "go.mod" ]; then
        /usr/local/go/bin/go mod tidy
        /usr/local/go/bin/go mod download
    fi
    
    cd /opt/hero_budget/backend
}

# Función mejorada para iniciar servicios
start_service_improved() {
    local service_name=$1
    local port=$2
    
    echo "🚀 Iniciando servicio: $service_name en puerto $port"
    
    # Matar procesos existentes
    kill_service_processes $service_name
    
    cd /opt/hero_budget/backend/$service_name
    
    # Verificar si el directorio existe
    if [ ! -d "/opt/hero_budget/backend/$service_name" ]; then
        echo "❌ Error: Directorio $service_name no existe"
        return 1
    fi
    
    # Compilar y ejecutar según el servicio
    case $service_name in
        "google_auth")
            echo "📦 Compilando google_auth con todos los archivos..."
            # Compilar todos los archivos .go del directorio
            nohup /usr/local/go/bin/go run *.go > /tmp/${service_name}.log 2>&1 &
            ;;
        "signup")
            echo "📦 Compilando signup con dependencias arregladas..."
            nohup /usr/local/go/bin/go run main.go > /tmp/${service_name}.log 2>&1 &
            ;;
        *)
            echo "📦 Compilando $service_name..."
            nohup /usr/local/go/bin/go run main.go > /tmp/${service_name}.log 2>&1 &
            ;;
    esac
    
    local pid=$!
    echo "🔍 PID del proceso: $pid"
    
    # Verificar que el servicio se inició correctamente
    sleep 3
    if ps -p $pid > /dev/null 2>&1; then
        echo "✅ Servicio $service_name iniciado correctamente"
        
        # Verificar que el puerto esté escuchando
        local retries=10
        while [ $retries -gt 0 ]; do
            if ss -tln | grep -q ":$port "; then
                echo "✅ Puerto $port está escuchando"
                return 0
            fi
            echo "⏳ Esperando que puerto $port esté disponible... ($retries intentos restantes)"
            sleep 2
            retries=$((retries-1))
        done
        
        echo "⚠️  Advertencia: Puerto $port no responde después de 20 segundos"
        tail -10 /tmp/${service_name}.log
    else
        echo "❌ Error: Servicio $service_name no se pudo iniciar"
        tail -10 /tmp/${service_name}.log
        return 1
    fi
}

# Función para crear configuración nginx optimizada
create_optimized_nginx_config() {
    echo "📝 Creando configuración nginx optimizada..."
    
    cat > /tmp/nginx-herobudget-optimized.conf << 'EOF'
upstream backend_main {
    server 127.0.0.1:8083 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_signin {
    server 127.0.0.1:8084 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_signup {
    server 127.0.0.1:8082 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_google_auth {
    server 127.0.0.1:8081 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_reset_password {
    server 127.0.0.1:8086 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_dashboard {
    server 127.0.0.1:8087 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_categories {
    server 127.0.0.1:8088 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_income {
    server 127.0.0.1:8089 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_expense {
    server 127.0.0.1:8090 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_bills {
    server 127.0.0.1:8091 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_cash_bank {
    server 127.0.0.1:8092 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_budget_overview {
    server 127.0.0.1:8093 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_budget_management {
    server 127.0.0.1:8094 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_savings {
    server 127.0.0.1:8095 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_money_flow {
    server 127.0.0.1:8096 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_language {
    server 127.0.0.1:8097 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

upstream backend_dashboard_data {
    server 127.0.0.1:8098 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 80;
    server_name srv736989.hstgr.cloud;

    # Configuración optimizada de timeouts
    proxy_connect_timeout       60s;
    proxy_send_timeout          60s;
    proxy_read_timeout          60s;
    proxy_buffering             on;
    proxy_buffer_size           8k;
    proxy_buffers               32 8k;
    proxy_busy_buffers_size     16k;

    # Headers comunes
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_http_version 1.1;
    proxy_set_header Connection "";

    # === RUTAS PRINCIPALES ===
    location / {
        proxy_pass http://backend_main;
    }

    location /health {
        proxy_pass http://backend_main;
    }

    # === AUTENTICACIÓN ===
    location /signin {
        proxy_pass http://backend_signin;
    }

    location /signup/ {
        proxy_pass http://backend_signup;
    }

    location /auth/google {
        proxy_pass http://backend_google_auth;
    }

    location /update/locale {
        proxy_pass http://backend_google_auth;
    }

    location /reset-password {
        proxy_pass http://backend_reset_password;
    }

    # === GESTIÓN DE DATOS ===
    location /categories/ {
        proxy_pass http://backend_categories;
    }

    location /income/ {
        proxy_pass http://backend_income;
    }

    location /expense/ {
        proxy_pass http://backend_expense;
    }

    location /bills/ {
        proxy_pass http://backend_bills;
    }

    location /cash-bank/ {
        proxy_pass http://backend_cash_bank;
    }

    # === ANÁLISIS Y REPORTES ===
    location /budget-overview {
        proxy_pass http://backend_budget_overview;
    }

    location /budget-overview/ {
        proxy_pass http://backend_budget_overview;
    }

    location /budget/ {
        proxy_pass http://backend_budget_management;
    }

    location /savings/ {
        proxy_pass http://backend_savings;
    }

    location /money-flow/ {
        proxy_pass http://backend_money_flow;
    }

    # === CONFIGURACIÓN Y UTILIDADES ===
    location /language/ {
        proxy_pass http://backend_language;
    }

    location /dashboard/ {
        proxy_pass http://backend_dashboard_data;
    }

    # === INFORMACIÓN DE USUARIO ===
    location /user/ {
        proxy_pass http://backend_main;
    }

    location /profile/ {
        proxy_pass http://backend_main;
    }

    location /transactions/history {
        proxy_pass http://backend_main;
    }

    # Logs de errores detallados
    error_log /var/log/nginx/herobudget_error.log debug;
    access_log /var/log/nginx/herobudget_access.log;
}
EOF

    echo "✅ Configuración nginx optimizada creada"
}

# ============================================================================
# EJECUCIÓN PRINCIPAL
# ============================================================================

echo "📋 Paso 1: Arreglando dependencias..."
fix_dependencies

echo ""
echo "📋 Paso 2: Reiniciando servicios problemáticos..."

# Reiniciar solo los servicios que están fallando
start_service_improved "google_auth" 8081
start_service_improved "signup" 8082

echo ""
echo "📋 Paso 3: Creando configuración nginx optimizada..."
create_optimized_nginx_config

echo ""
echo "📋 Paso 4: Aplicando nueva configuración nginx..."
if [ -f "/etc/nginx/sites-available/herobudget" ]; then
    cp /etc/nginx/sites-available/herobudget /etc/nginx/sites-available/herobudget.backup.$(date +%Y%m%d_%H%M%S)
    echo "✅ Backup de configuración nginx creado"
fi

cp /tmp/nginx-herobudget-optimized.conf /etc/nginx/sites-available/herobudget

# Verificar configuración nginx
if nginx -t; then
    echo "✅ Configuración nginx válida"
    systemctl reload nginx
    echo "✅ Nginx recargado"
else
    echo "❌ Error en configuración nginx"
    exit 1
fi

echo ""
echo "📋 Paso 5: Verificando servicios..."
sleep 5

echo "🔍 Puertos activos:"
ss -tln | grep -E ':(808[1-9]|809[0-8])'

echo ""
echo "🔍 Procesos Go ejecutándose:"
ps aux | grep 'go run' | grep -v grep | wc -l

echo ""
echo "=========================================="
echo "✅ CORRECCIÓN COMPLETADA"
echo "=========================================="
echo ""
echo "📊 Próximos pasos:"
echo "1. Ejecutar test de endpoints: ./test_production_endpoints.sh"
echo "2. Revisar logs si hay errores: tail -f /tmp/*.log"
echo "3. Verificar nginx: nginx -t && systemctl status nginx"
echo ""
echo "📝 Log completo en: $LOG_FILE" 