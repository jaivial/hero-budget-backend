#!/bin/bash

# Script para actualizar configuración de base de datos en todos los servicios restantes
# Agrega soporte para flags --dev y --produccion

SERVICES=(
    "bills_management"
    "expense_management" 
    "income_management"
    "cash_bank_management"
    "categories_management"
    "savings_management"
    "budget_management"
    "profile_management"
    "transaction_delete_service"
    "money_flow_sync"
    "budget_overview_fetch"
    "user_locale"
    "language_cookie"
    "fetch_dashboard"
    "dashboard_data"
)

echo "🚀 Actualizando configuración de base de datos en 15 servicios..."

for service in "${SERVICES[@]}"; do
    echo "🔧 Procesando servicio: $service"
    
    # Buscar archivo principal
    if [ -f "$service/main.go" ]; then
        main_file="$service/main.go"
    elif [ -f "$service/main_part1.go" ]; then
        main_file="$service/main_part1.go"
    else
        echo "  ❌ No se encontró archivo principal en $service"
        continue
    fi
    
    echo "  📝 Modificando: $main_file"
    
    # Backup del archivo original
    cp "$main_file" "${main_file}.backup"
    
    # Verificar si ya tiene flag import
    if ! grep -q '"flag"' "$main_file"; then
        echo "  ➕ Agregando import flag"
        # Insertar después de la línea import (
        sed -i.tmp '/^import (/a\
	"flag"
' "$main_file"
        rm "${main_file}.tmp" 2>/dev/null
    fi
    
    # Verificar si ya tiene godotenv import
    if ! grep -q 'github.com/joho/godotenv' "$main_file"; then
        echo "  ➕ Agregando import godotenv"
        sed -i.tmp '/^import (/a\
	"github.com/joho/godotenv"
' "$main_file"
        rm "${main_file}.tmp" 2>/dev/null
    fi
    
    # Buscar la función init y el patrón de base de datos
    if grep -q "filepath.Join.*google_auth.*users.db\|\.\.\/google_auth\/users\.db" "$main_file"; then
        echo "  🔄 Actualizando lógica de base de datos en init()"
        
        # Crear archivo temporal con nueva lógica
        cat > /tmp/new_db_logic.txt << 'EOF'
	// Parse command line flags
	devMode := flag.Bool("dev", false, "Run in development mode")
	prodMode := flag.Bool("produccion", false, "Run in production mode")
	flag.Parse()

	// Load environment variables from .env file in parent directory
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		log.Printf("Continuing with system environment variables...")
	} else {
		log.Println("Successfully loaded environment variables from ../.env")
	}

	// Determine database path based on environment flag
	var dbPath string
	if *prodMode {
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)
	} else if *devMode {
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)
	} else {
		// Default to development mode if no flag specified
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)
	}

	var err error
EOF
        
        # Encontrar línea de func init() y reemplazar lógica de DB
        awk '
        /^func init\(\) \{/ { 
            print $0
            system("cat /tmp/new_db_logic.txt")
            skip_until_db = 1
            next
        }
        skip_until_db && /db, err = sql\.Open/ {
            print "\t// Open the database connection"
            print "\tdb, err = sql.Open(\"sqlite3\", dbPath)"
            print "\tif err != nil {"
            print "\t\tlog.Fatalf(\"Failed to open database at %s: %v\", dbPath, err)"
            print "\t}"
            print ""
            print "\t// Test the connection"
            print "\tif err = db.Ping(); err != nil {"
            print "\t\tlog.Fatalf(\"Failed to ping database at %s: %v\", dbPath, err)"
            print "\t}"
            skip_until_db = 0
            skip_lines = 1
            next
        }
        skip_until_db && (/dbPath :=|log\.Printf.*Using database|cwd, err := os\.Getwd|filepath\.Join|os\.Stat.*dbPath/) { next }
        skip_lines && (/if err != nil|log\.Fatalf.*Failed to open|log\.Fatalf.*Failed to ping/) { next }
        { print; skip_lines = 0 }
        ' "$main_file" > "${main_file}.new"
        
        mv "${main_file}.new" "$main_file"
        rm /tmp/new_db_logic.txt 2>/dev/null
    fi
    
    # Agregar función helper al final si no existe
    if ! grep -q 'getEnvOrDefault' "$main_file"; then
        echo "  ➕ Agregando función getEnvOrDefault"
        echo '
// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}' >> "$main_file"
    fi
    
    echo "  ✅ $service actualizado exitosamente"
done

echo "🎉 Todos los servicios han sido actualizados con configuración de base de datos dinámica"
echo "📋 Servicios procesados: ${#SERVICES[@]}"