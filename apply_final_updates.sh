#!/bin/bash

# Script final para aplicar configuración de DB a servicios restantes

SERVICES_TO_UPDATE=(
    "income_management/main.go"
    "cash_bank_management/main.go"
    "categories_management/main.go"
    "savings_management/main.go"
    "budget_management/main.go"
    "profile_management/main.go"
    "transaction_delete_service/main.go"
    "money_flow_sync/main.go"
    "budget_overview_fetch/main.go"
    "user_locale/main.go"
    "fetch_dashboard/main.go"
    "dashboard_data/main.go"
)

echo "🔧 Aplicando configuración de DB a servicios restantes..."

for service_file in "${SERVICES_TO_UPDATE[@]}"; do
    echo "  📝 Actualizando: $service_file"
    
    # Verificar si el archivo existe
    if [ ! -f "$service_file" ]; then
        echo "    ❌ Archivo no encontrado: $service_file"
        continue
    fi
    
    # Backup del archivo
    cp "$service_file" "${service_file}.bak"
    
    # Variables para el servicio actual
    service_dir=$(dirname "$service_file")
    
    # Aplicar cambios usando sed para reemplazar patrones específicos
    sed -i.tmp '
    # Agregar imports necesarios después de import (
    /^import ($/,/^)$/ {
        /^import ($/a\
	"flag"\
	"github.com/joho/godotenv"
    }
    
    # Reemplazar la lógica de init que contiene filepath.Join con google_auth
    /filepath\.Join.*google_auth.*users\.db/ {
        # Iniciar reemplazo desde func init()
        :start
        N
        s/\(func init() {\n.*\)\(filepath\.Join.*google_auth.*users\.db.*\n.*log\.Printf.*Using database.*\n.*\)\(db, err = sql\.Open.*\)/\1\
\t\/\/ Parse command line flags\
\tdevMode := flag.Bool("dev", false, "Run in development mode")\
\tprodMode := flag.Bool("produccion", false, "Run in production mode")\
\tflag.Parse()\
\
\t\/\/ Load environment variables from .env file in parent directory\
\tif err := godotenv.Load("../.env"); err != nil {\
\t\tlog.Printf("Warning: Error loading .env file: %v", err)\
\t\tlog.Printf("Continuing with system environment variables...")\
\t} else {\
\t\tlog.Println("Successfully loaded environment variables from ../.env")\
\t}\
\
\t\/\/ Determine database path based on environment flag\
\tvar dbPath string\
\tif *prodMode {\
\t\tdbPath = getEnvOrDefault("DB_PROD_PATH", "\/opt\/hero_budget\/database\/hero_budget.db")\
\t\tlog.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)\
\t} else if *devMode {\
\t\tdbPath = getEnvOrDefault("DB_DEV_PATH", ".\/users.db")\
\t\tlog.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)\
\t} else {\
\t\t\/\/ Default to development mode if no flag specified\
\t\tdbPath = getEnvOrDefault("DB_DEV_PATH", ".\/users.db")\
\t\tlog.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)\
\t}\
\
\tvar err error\
\t\/\/ Open the database connection\
\tdb, err = sql.Open("sqlite3", dbPath)\
\tif err != nil {\
\t\tlog.Fatalf("Failed to open database at %s: %v", dbPath, err)\
\t}\
\
\t\/\/ Test the connection\
\tif err = db.Ping(); err != nil {\
\t\tlog.Fatalf("Failed to ping database at %s: %v", dbPath, err)\
\t}/
        t end
        b start
        :end
    }
    ' "$service_file"
    
    # Limpiar archivo temporal
    rm "${service_file}.tmp" 2>/dev/null
    
    # Verificar si ya existe getEnvOrDefault
    if ! grep -q 'func getEnvOrDefault' "$service_file"; then
        echo '
// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}' >> "$service_file"
    fi
    
    echo "    ✅ $service_file actualizado"
done

echo "🎉 Configuración aplicada a todos los servicios restantes"