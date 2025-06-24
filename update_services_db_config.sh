#!/bin/bash

# Script para actualizar configuración de base de datos en todos los servicios
# Agrega soporte para flags --dev y --produccion

SERVICES=(
    "signup"
    "reset_password"
    "apple-auth"
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

for service in "${SERVICES[@]}"; do
    echo "🔧 Actualizando servicio: $service"
    
    # Buscar archivo main.go o archivos principales
    if [ -f "$service/main.go" ]; then
        main_file="$service/main.go"
    elif [ -f "$service/main_part1.go" ]; then
        main_file="$service/main_part1.go"
    else
        echo "  ❌ No se encontró archivo principal en $service"
        continue
    fi
    
    echo "  📝 Modificando: $main_file"
    
    # Agregar import flag si no existe
    if ! grep -q '"flag"' "$main_file"; then
        sed -i.bak '/import (/a\
	"flag"
' "$main_file"
    fi
    
    # Agregar import godotenv si no existe
    if ! grep -q 'github.com/joho/godotenv' "$main_file"; then
        sed -i.bak '/import (/a\
	"github.com/joho/godotenv"
' "$main_file"
    fi
    
    # Buscar y reemplazar la lógica de base de datos en func init()
    if grep -q 'filepath.Join.*google_auth.*users.db' "$main_file"; then
        # Patron para servicios que usan filepath.Join con google_auth
        sed -i.bak '/func init()/,/^}$/{
            /Parse command line flags/,/^}$/d
            /devMode := flag.Bool/,/^}$/d
            /prodMode := flag.Bool/,/^}$/d
            /Load environment variables/,/^}$/d
            /Determine database path/,/^}$/d
        }' "$main_file"
        
        # Insertar nueva lógica después de func init() {
        sed -i.bak '/func init() {/a\
	// Parse command line flags\
	devMode := flag.Bool("dev", false, "Run in development mode")\
	prodMode := flag.Bool("produccion", false, "Run in production mode")\
	flag.Parse()\
\
	// Load environment variables from .env file in parent directory\
	if err := godotenv.Load("../.env"); err != nil {\
		log.Printf("Warning: Error loading .env file: %v", err)\
		log.Printf("Continuing with system environment variables...")\
	} else {\
		log.Println("Successfully loaded environment variables from ../.env")\
	}\
\
	// Determine database path based on environment flag\
	var dbPath string\
	if *prodMode {\
		dbPath = getEnvOrDefault("DB_PROD_PATH", "/opt/hero_budget/database/hero_budget.db")\
		log.Printf("🏭 Running in PRODUCTION mode - Database: %s", dbPath)\
	} else if *devMode {\
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")\
		log.Printf("🔧 Running in DEVELOPMENT mode - Database: %s", dbPath)\
	} else {\
		// Default to development mode if no flag specified\
		dbPath = getEnvOrDefault("DB_DEV_PATH", "./users.db")\
		log.Printf("🔧 Running in DEVELOPMENT mode (default) - Database: %s", dbPath)\
	}\
' "$main_file"
    fi
    
    # Agregar función helper al final del archivo si no existe
    if ! grep -q 'getEnvOrDefault' "$main_file"; then
        echo '
// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}' >> "$main_file"
    fi
    
    echo "  ✅ $service actualizado"
done

echo "🎉 Todos los servicios han sido actualizados con configuración de base de datos dinámica"