#!/bin/bash

# Script para verificar la base de datos en el servidor

echo "🔍 Verificando base de datos del sync service..."
echo ""

# Conectarse al servidor y verificar
ssh root@178.16.130.178 << 'EOF'
cd /opt/hero_budget/backend

echo "📂 Archivos de base de datos disponibles:"
ls -la *.db 2>/dev/null || echo "No se encontraron archivos .db"

echo ""
echo "🗄️ Verificando tablas en budget_data.db:"
if [ -f "budget_data.db" ]; then
    sqlite3 budget_data.db ".tables" 2>/dev/null || echo "Error al leer budget_data.db"
else
    echo "❌ No existe budget_data.db"
fi

echo ""
echo "🔍 Verificando contenido de tablas principales:"
if [ -f "budget_data.db" ]; then
    echo "- Tabla expenses:"
    sqlite3 budget_data.db "SELECT COUNT(*) FROM expenses;" 2>/dev/null || echo "  No existe la tabla expenses"
    
    echo "- Tabla incomes:"
    sqlite3 budget_data.db "SELECT COUNT(*) FROM incomes;" 2>/dev/null || echo "  No existe la tabla incomes"
    
    echo "- Tabla categories:"
    sqlite3 budget_data.db "SELECT COUNT(*) FROM categories;" 2>/dev/null || echo "  No existe la tabla categories"
    
    echo "- Tabla bills:"
    sqlite3 budget_data.db "SELECT COUNT(*) FROM bills;" 2>/dev/null || echo "  No existe la tabla bills"
fi

echo ""
echo "🔍 Buscando otras bases de datos:"
find . -name "*.db" -type f 2>/dev/null | head -20

EOF