#!/bin/bash

# Script para forzar actualización específica del servicio expense_management

REMOTE_USER="root"
REMOTE_HOST="178.16.130.178"
REMOTE_DIR="/opt/hero_budget/backend"

echo "🔄 Actualizando código y reiniciando SOLO el servicio expense_management..."

ssh "$REMOTE_USER@$REMOTE_HOST" << 'EOF'
  cd /opt/hero_budget/backend || exit 1
  
  echo "🔄 Actualizando código desde repositorio..."
  git fetch origin
  git reset --hard origin/main
  
  echo "🛑 Deteniendo servicio expense_management..."
  pkill -f "expense_management" || echo "Servicio no estaba corriendo"
  sleep 2
  
  echo "🔧 Compilando servicio expense_management..."
  cd expense_management
  
  # Verificar que existe main.go
  if [ ! -f "main.go" ]; then
    echo "❌ Error: main.go no encontrado"
    exit 1
  fi
  
  # Compilar con todos los archivos Go
  /usr/local/go/bin/go mod tidy
  /usr/local/go/bin/go build -o expense_management_new *.go
  
  if [ $? -eq 0 ]; then
    echo "✅ Compilación exitosa"
    mv expense_management_new expense_management
    chmod +x expense_management
  else
    echo "❌ Error en compilación"
    exit 1
  fi
  
  echo "🚀 Iniciando servicio expense_management..."
  nohup ./expense_management --produccion > expense_management.log 2>&1 &
  
  # Esperar y verificar
  sleep 3
  if pgrep -f "expense_management" > /dev/null; then
    echo "✅ Servicio expense_management iniciado correctamente"
    echo "PID: $(pgrep -f expense_management)"
  else
    echo "❌ Error: Servicio no se inició"
    echo "Últimas líneas del log:"
    tail -10 expense_management.log
    exit 1
  fi
  
  echo "🔍 Verificando endpoint..."
  sleep 2
  curl -s "http://localhost:8094/expenses?user_id=test" | head -50
  
EOF

echo "✅ Actualización completada"