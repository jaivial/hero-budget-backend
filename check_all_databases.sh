#!/bin/bash

# Script para verificar todas las bases de datos

echo "🔍 Verificando todas las bases de datos en el servidor..."
echo ""

ssh root@178.16.130.178 << 'EOF'
cd /opt/hero_budget/backend

echo "📊 Verificando google_auth/users.db:"
if [ -f "google_auth/users.db" ]; then
    echo "Tablas encontradas:"
    sqlite3 google_auth/users.db ".tables" 2>/dev/null
    echo ""
    echo "Verificando tabla expenses:"
    sqlite3 google_auth/users.db "SELECT COUNT(*) as total FROM expenses;" 2>/dev/null || echo "No existe expenses"
    echo "Verificando tabla categories:"
    sqlite3 google_auth/users.db "SELECT COUNT(*) as total FROM categories;" 2>/dev/null || echo "No existe categories"
fi

echo ""
echo "📊 Verificando bills_management/users.db:"
if [ -f "bills_management/users.db" ]; then
    echo "Tablas encontradas:"
    sqlite3 bills_management/users.db ".tables" 2>/dev/null
fi

echo ""
echo "🔍 Buscando archivo hero_budget.db:"
find /opt/hero_budget -name "hero_budget.db" -type f 2>/dev/null

echo ""
echo "🔍 Verificando /opt/hero_budget/database/hero_budget.db:"
if [ -f "/opt/hero_budget/database/hero_budget.db" ]; then
    echo "✅ Encontrado! Verificando tablas:"
    sqlite3 /opt/hero_budget/database/hero_budget.db ".tables" | tr ' ' '\n' | sort
    echo ""
    echo "Conteo de registros:"
    echo "- expenses: $(sqlite3 /opt/hero_budget/database/hero_budget.db "SELECT COUNT(*) FROM expenses;" 2>/dev/null)"
    echo "- incomes: $(sqlite3 /opt/hero_budget/database/hero_budget.db "SELECT COUNT(*) FROM incomes;" 2>/dev/null)"
    echo "- categories: $(sqlite3 /opt/hero_budget/database/hero_budget.db "SELECT COUNT(*) FROM categories;" 2>/dev/null)"
    echo "- bills: $(sqlite3 /opt/hero_budget/database/hero_budget.db "SELECT COUNT(*) FROM bills;" 2>/dev/null)"
else
    echo "❌ No encontrado en /opt/hero_budget/database/"
fi

EOF