#!/bin/bash
# Script pre-commit para backend Hero Budget

echo "🔍 Ejecutando verificaciones pre-commit..."

# Verificar formato Go
if command -v gofmt &> /dev/null; then
    UNFORMATTED=$(gofmt -l .)
    if [ -n "$UNFORMATTED" ]; then
        echo "❌ Archivos sin formatear:"
        echo "$UNFORMATTED"
        echo "Ejecuta: gofmt -w ."
        exit 1
    fi
fi

# Verificar tests (si existen)
if [ -d "tests" ]; then
    echo "🧪 Ejecutando tests..."
    go test ./... || exit 1
fi

echo "✅ Verificaciones pre-commit completadas"
