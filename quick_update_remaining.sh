#!/bin/bash

echo "🚀 Actualizando servicios restantes con configuración de DB..."

# Lista de servicios a actualizar con sus archivos principales
declare -A SERVICES
SERVICES["cash_bank_management"]="main.go"
SERVICES["categories_management"]="main.go"
SERVICES["savings_management"]="main.go"
SERVICES["budget_management"]="main.go"
SERVICES["profile_management"]="main.go"
SERVICES["transaction_delete_service"]="main.go"
SERVICES["money_flow_sync"]="main.go"
SERVICES["budget_overview_fetch"]="main.go"
SERVICES["user_locale"]="main.go"
SERVICES["fetch_dashboard"]="main.go"
SERVICES["dashboard_data"]="main.go"

for service in "${!SERVICES[@]}"; do
    file="${SERVICES[$service]}"
    service_file="$service/$file"
    
    echo "🔧 Actualizando: $service_file"
    
    if [ ! -f "$service_file" ]; then
        echo "  ❌ Archivo no encontrado"
        continue
    fi
    
    # Backup
    cp "$service_file" "${service_file}.bak2"
    
    # Agregar imports flag y godotenv si no existen
    if ! grep -q '"flag"' "$service_file"; then
        sed -i.tmp '/import (/a\
	"flag"' "$service_file"
    fi
    
    if ! grep -q 'github.com/joho/godotenv' "$service_file"; then
        sed -i.tmp '/import (/a\
	"github.com/joho/godotenv"' "$service_file"
    fi
    
    # Reemplazar filepath.Join pattern
    if grep -q "filepath.Join.*google_auth.*users.db" "$service_file"; then
        # Crear un replacement text
        replacement='	// Parse command line flags
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

	var err error'
        
        # Usar awk para reemplazar desde filepath.Join hasta db, err = sql.Open
        awk -v repl="$replacement" '
        /filepath\.Join.*google_auth.*users\.db/ {
            print repl
            # Skip lines until we find db, err = sql.Open
            while ((getline) > 0) {
                if (/db, err = sql\.Open/) {
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
                    break
                }
            }
            next
        }
        { print }
        ' "$service_file" > "${service_file}.new"
        
        mv "${service_file}.new" "$service_file"
    fi
    
    # Agregar función helper si no existe
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
    
    # Limpiar archivos temporales
    rm "${service_file}.tmp" 2>/dev/null
    
    echo "  ✅ $service actualizado"
done

echo "🎉 Todos los servicios restantes han sido actualizados"