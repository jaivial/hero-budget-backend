#!/bin/bash

# =============================================================================
# SCRIPT DE AUTO-RESTART CON WEBHOOK PARA HERO BUDGET BACKEND VPS
# Automatiza git pull --rebase + restart de servicios al recibir webhooks
# =============================================================================

# Configuración
BASE_PATH="/opt/hero_budget/backend"
WEBHOOK_PORT=9000
LOG_FILE="/var/log/hero-budget-webhook.log"
PID_FILE="/var/run/hero-budget-webhook.pid"

# Configuración de colores para logs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# Función para logging con timestamp
log_message() {
    local level=$1
    local message=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local colored_message=""
    
    case $level in
        "INFO")
            colored_message="${GREEN}[INFO]${NC} $message"
            ;;
        "WARN")
            colored_message="${YELLOW}[WARN]${NC} $message"
            ;;
        "ERROR")
            colored_message="${RED}[ERROR]${NC} $message"
            ;;
        "DEBUG")
            colored_message="${BLUE}[DEBUG]${NC} $message"
            ;;
        *)
            colored_message="[UNKNOWN] $message"
            ;;
    esac
    
    echo -e "[$timestamp] $colored_message" | tee -a "$LOG_FILE"
}

# Función para manejar la actualización del código
handle_update() {
    log_message "INFO" "🔄 Iniciando proceso de actualización automática..."
    
    # Cambiar al directorio del proyecto
    cd "$BASE_PATH" || {
        log_message "ERROR" "❌ No se pudo acceder al directorio $BASE_PATH"
        return 1
    }
    
    log_message "INFO" "📂 Trabajando en directorio: $(pwd)"
    
    # Hacer stash de cambios locales si los hay
    if ! git diff --quiet || ! git diff --cached --quiet; then
        log_message "WARN" "⚠️ Detectados cambios locales, guardándolos en stash..."
        git stash push -m "Auto-stash antes de webhook $(date)" || {
            log_message "ERROR" "❌ Error al hacer stash de cambios locales"
            return 1
        }
    fi
    
    # Ejecutar git pull --rebase
    log_message "INFO" "📥 Ejecutando git pull --rebase..."
    if git pull --rebase origin main; then
        log_message "INFO" "✅ Git pull exitoso"
    else
        log_message "ERROR" "❌ Error en git pull --rebase"
        
        # Intentar abortar el rebase si falló
        git rebase --abort 2>/dev/null
        
        # Intentar un pull normal como fallback
        log_message "WARN" "🔄 Intentando pull normal como fallback..."
        if git pull origin main; then
            log_message "WARN" "⚠️ Pull normal exitoso, pero hubo conflictos en rebase"
        else
            log_message "ERROR" "❌ Falló tanto el rebase como el pull normal"
            return 1
        fi
    fi
    
    # Verificar que el script de restart existe
    if [ ! -f "$BASE_PATH/restart_services_vps.sh" ]; then
        log_message "ERROR" "❌ Script restart_services_vps.sh no encontrado"
        return 1
    fi
    
    # Dar permisos de ejecución al script
    chmod +x "$BASE_PATH/restart_services_vps.sh"
    
    # Ejecutar el restart de servicios
    log_message "INFO" "🚀 Ejecutando restart de servicios..."
    if "$BASE_PATH/restart_services_vps.sh"; then
        log_message "INFO" "✅ Servicios reiniciados exitosamente"
        log_message "INFO" "🎉 Actualización automática completada"
        return 0
    else
        log_message "ERROR" "❌ Error al reiniciar servicios"
        return 1
    fi
}

# Función para crear el servidor webhook usando Python
create_webhook_server() {
    cat > /tmp/webhook_server.py << 'EOF'
import http.server
import socketserver
import json
import subprocess
import sys
import os
from urllib.parse import urlparse

class WebhookHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == '/webhook':
            content_length = int(self.headers.get('Content-Length', 0))
            post_data = self.rfile.read(content_length)
            
            try:
                payload = json.loads(post_data.decode('utf-8'))
                
                if payload.get('ref') == 'refs/heads/main':
                    pusher_name = payload.get('pusher', {}).get('name', 'unknown')
                    print(f'Webhook recibido: Push a main por {pusher_name}')
                    
                    # Ejecutar la actualización
                    script_path = os.environ.get('SCRIPT_PATH', '/opt/hero_budget/backend/auto-restart-vps.sh')
                    result = subprocess.run([script_path, 'update'], 
                                          capture_output=True, text=True)
                    
                    if result.returncode == 0:
                        response = {'status': 'success', 'message': 'Update completed'}
                        self.send_response(200)
                    else:
                        response = {'status': 'error', 'message': 'Update failed', 'error': result.stderr}
                        self.send_response(500)
                        
                    self.send_header('Content-type', 'application/json')
                    self.end_headers()
                    self.wfile.write(json.dumps(response).encode())
                else:
                    self.send_response(200)
                    self.send_header('Content-type', 'application/json')
                    self.end_headers()
                    response = {'status': 'ignored', 'message': 'Not a push to main'}
                    self.wfile.write(json.dumps(response).encode())
                    
            except Exception as e:
                print(f'Error procesando webhook: {e}')
                self.send_response(400)
                self.send_header('Content-type', 'application/json')
                self.end_headers()
                response = {'status': 'error', 'message': str(e)}
                self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(404)
            self.end_headers()
    
    def do_GET(self):
        if self.path == '/health':
            self.send_response(200)
            self.send_header('Content-type', 'application/json')
            self.end_headers()
            response = {'status': 'healthy', 'service': 'hero-budget-webhook'}
            self.wfile.write(json.dumps(response).encode())
        else:
            self.send_response(404)
            self.end_headers()
    
    def log_message(self, format, *args):
        log_file = os.environ.get('LOG_FILE', '/var/log/hero-budget-webhook.log')
        with open(log_file, 'a') as f:
            f.write(f'[{self.log_date_time_string()}] {format % args}\n')

if __name__ == '__main__':
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9000
    with socketserver.TCPServer(('', port), WebhookHandler) as httpd:
        print(f'Servidor webhook iniciado en puerto {port}')
        httpd.serve_forever()
EOF
}

# Función para iniciar el servidor webhook
start_webhook_server() {
    log_message "INFO" "🌐 Iniciando servidor webhook en puerto $WEBHOOK_PORT..."
    
    # Verificar si el puerto está ocupado y liberarlo
    local port_pid=$(lsof -ti:$WEBHOOK_PORT 2>/dev/null)
    if [ ! -z "$port_pid" ]; then
        log_message "WARN" "⚠️ Puerto $WEBHOOK_PORT ocupado por PID $port_pid, liberando..."
        kill -9 $port_pid 2>/dev/null
        sleep 1
    fi
    
    # Limpiar procesos Python residuales del webhook
    pkill -f webhook_server.py 2>/dev/null
    
    # Crear el script del servidor Python
    create_webhook_server
    
    # Establecer variables de entorno para el servidor Python
    export SCRIPT_PATH="$BASE_PATH/auto-restart-vps.sh"
    export LOG_FILE="$LOG_FILE"
    
    # Iniciar el servidor en background
    nohup python3 /tmp/webhook_server.py "$WEBHOOK_PORT" > "$LOG_FILE" 2>&1 &
    local webhook_pid=$!
    
    # Esperar un momento para verificar que se inició correctamente
    sleep 2
    
    # Verificar que el proceso sigue ejecutándose
    if kill -0 "$webhook_pid" 2>/dev/null; then
        echo $webhook_pid > "$PID_FILE"
        log_message "INFO" "✅ Servidor webhook iniciado exitosamente (PID: $webhook_pid)"
        log_message "INFO" "📡 Endpoint webhook: http://tu-vps-ip:$WEBHOOK_PORT/webhook"
        log_message "INFO" "🏥 Health check: http://tu-vps-ip:$WEBHOOK_PORT/health"
        
        # Verificar que el puerto está realmente escuchando
        if lsof -i:$WEBHOOK_PORT > /dev/null 2>&1; then
            log_message "INFO" "🔌 Puerto $WEBHOOK_PORT confirmado como activo"
        else
            log_message "ERROR" "❌ Puerto $WEBHOOK_PORT no está escuchando, revisar logs"
            return 1
        fi
        
        return 0
    else
        log_message "ERROR" "❌ El proceso del webhook falló al iniciarse, revisar logs"
        rm -f "$PID_FILE"
        return 1
    fi
}

# Función para detener el servidor webhook
stop_webhook_server() {
    log_message "INFO" "🛑 Deteniendo servidor webhook..."
    
    # Detener por PID file si existe
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_message "INFO" "🛑 Deteniendo servidor webhook (PID: $pid)"
            kill "$pid"
            sleep 1
            # Si no se detuvo, forzar
            if kill -0 "$pid" 2>/dev/null; then
                kill -9 "$pid" 2>/dev/null
            fi
        fi
        rm -f "$PID_FILE"
    fi
    
    # Limpiar todos los procesos del webhook por si acaso
    local port_pid=$(lsof -ti:$WEBHOOK_PORT 2>/dev/null)
    if [ ! -z "$port_pid" ]; then
        log_message "WARN" "⚠️ Proceso adicional encontrado en puerto $WEBHOOK_PORT (PID: $port_pid), eliminando..."
        kill -9 $port_pid 2>/dev/null
    fi
    
    # Limpiar procesos Python del webhook
    pkill -f webhook_server.py 2>/dev/null
    
    # Verificar que el puerto quedó libre
    if ! lsof -i:$WEBHOOK_PORT > /dev/null 2>&1; then
        log_message "INFO" "✅ Servidor webhook detenido correctamente"
    else
        log_message "WARN" "⚠️ Puerto $WEBHOOK_PORT aún ocupado, puede requerir intervención manual"
    fi
}

# Función para verificar el estado del servidor
status_webhook_server() {
    if [ -f "$PID_FILE" ]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            log_message "INFO" "✅ Servidor webhook ejecutándose (PID: $pid)"
            
            # Verificar conectividad
            if curl -s "http://localhost:$WEBHOOK_PORT/health" > /dev/null; then
                log_message "INFO" "🏥 Health check: OK"
            else
                log_message "WARN" "⚠️ Health check: FAILED"
            fi
        else
            log_message "WARN" "⚠️ PID $pid no encontrado, servidor no está ejecutándose"
            rm -f "$PID_FILE"
        fi
    else
        log_message "INFO" "ℹ️ Servidor webhook no está ejecutándose"
    fi
}

# Función para mostrar ayuda
show_help() {
    echo -e "${WHITE}"
    echo "============================================================================="
    echo "   🚀 HERO BUDGET - AUTO RESTART VPS WEBHOOK"
    echo "============================================================================="
    echo -e "${NC}"
    echo "Uso: $0 {start|stop|restart|status|update|help}"
    echo ""
    echo "Comandos:"
    echo "  start   - Iniciar el servidor webhook"
    echo "  stop    - Detener el servidor webhook"
    echo "  restart - Reiniciar el servidor webhook"
    echo "  status  - Verificar estado del servidor"
    echo "  update  - Ejecutar actualización manual (git pull + restart servicios)"
    echo "  help    - Mostrar esta ayuda"
    echo ""
    echo "Configuración:"
    echo "  Puerto webhook: $WEBHOOK_PORT"
    echo "  Directorio base: $BASE_PATH"
    echo "  Archivo log: $LOG_FILE"
    echo "  Archivo PID: $PID_FILE"
    echo ""
    echo "Para configurar webhook en GitHub:"
    echo "  1. Ve a Settings > Webhooks en tu repositorio"
    echo "  2. Agrega: http://tu-vps-ip:$WEBHOOK_PORT/webhook"
    echo "  3. Content type: application/json"
    echo "  4. Eventos: Just the push event"
    echo ""
}

# Función para configurar el entorno
setup_environment() {
    # Crear directorio de logs si no existe
    sudo mkdir -p "$(dirname "$LOG_FILE")"
    sudo touch "$LOG_FILE"
    sudo chown "$USER:$USER" "$LOG_FILE"
    
    # Verificar que Python3 está instalado
    if ! command -v python3 &> /dev/null; then
        log_message "ERROR" "❌ Python3 no está instalado. Instalando..."
        sudo apt update && sudo apt install -y python3
    fi
    
    # Verificar que curl está instalado
    if ! command -v curl &> /dev/null; then
        log_message "ERROR" "❌ curl no está instalado. Instalando..."
        sudo apt update && sudo apt install -y curl
    fi
    
    # Verificar directorio base
    if [ ! -d "$BASE_PATH" ]; then
        log_message "ERROR" "❌ Directorio base $BASE_PATH no existe"
        exit 1
    fi
}

# Procesar argumentos de línea de comandos
case "${1:-help}" in
    start)
        setup_environment
        log_message "INFO" "🚀 Iniciando sistema de webhook..."
        
        # Verificar si ya está ejecutándose
        if [ -f "$PID_FILE" ]; then
            local pid=$(cat "$PID_FILE")
            if kill -0 "$pid" 2>/dev/null; then
                log_message "WARN" "⚠️ Servidor webhook ya está ejecutándose (PID: $pid)"
                exit 1
            else
                rm -f "$PID_FILE"
            fi
        fi
        
        start_webhook_server
        ;;
        
    stop)
        log_message "INFO" "🛑 Deteniendo sistema de webhook..."
        stop_webhook_server
        ;;
        
    restart)
        log_message "INFO" "🔄 Reiniciando sistema de webhook..."
        stop_webhook_server
        sleep 2
        setup_environment
        start_webhook_server
        ;;
        
    status)
        status_webhook_server
        ;;
        
    update)
        log_message "INFO" "🔧 Ejecutando actualización manual..."
        handle_update
        ;;
        
    help)
        show_help
        ;;
        
    *)
        echo "Comando no reconocido: $1"
        show_help
        exit 1
        ;;
esac 