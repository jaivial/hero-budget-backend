#!/bin/bash

# Script para actualizar masivamente los servicios restantes
echo "🚀 Actualizando servicios restantes con configuración de flags..."

SERVICES_TO_UPDATE=(
    "categories_management"
    "savings_management"
    "budget_management"
    "profile_management"
    "transaction_delete_service"
    "money_flow_sync"
    "budget_overview_fetch"
    "user_locale"
    "language_cookie"
    "dashboard_data"
)

for service in "${SERVICES_TO_UPDATE[@]}"; do
    echo "🔧 Actualizando: $service"
    
    main_file="$service/main.go"
    
    if [ ! -f "$main_file" ]; then
        echo "  ❌ Archivo no encontrado: $main_file"
        continue
    fi
    
    # Backup
    cp "$main_file" "${main_file}.bak_mass"
    
    # 1. Agregar imports necesarios si no existen
    if ! grep -q '"flag"' "$main_file"; then
        # Insertar flag después de la línea import (
        sed -i.tmp '/^import (/a\
	"flag"' "$main_file"
    fi
    
    if ! grep -q 'github.com/joho/godotenv' "$main_file"; then
        # Insertar godotenv después de la línea import (
        sed -i.tmp '/^import (/a\
	"github.com/joho/godotenv"' "$main_file"
    fi
    
    # 2. Remover path/filepath si existe (ya que no lo usaremos)
    sed -i.tmp 's/"path\/filepath"//' "$main_file"
    
    # 3. Reemplazar la lógica de init que contiene la configuración hardcodeada
    # Crear archivo temporal con la nueva lógica
    cat > /tmp/new_init_logic.txt << 'EOF'
func init() {
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
	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database at %s: %v", dbPath, err)
	}

	log.Println("Database connection established successfully")
EOF

    # 4. Usar awk para hacer el reemplazo más preciso
    awk '
    BEGIN { in_init = 0; init_done = 0 }
    
    # Detectar inicio de func init()
    /^func init\(\) \{/ && !init_done {
        in_init = 1
        system("cat /tmp/new_init_logic.txt")
        next
    }
    
    # Si estamos dentro de init, buscar el final
    in_init && /^\}$/ {
        in_init = 0
        init_done = 1
        print
        next
    }
    
    # Si estamos dentro de init, saltar líneas
    in_init { next }
    
    # Imprimir todas las demás líneas
    { print }
    ' "$main_file" > "${main_file}.new"
    
    mv "${main_file}.new" "$main_file"
    
    # 5. Agregar función helper al final si no existe
    if ! grep -q 'func getEnvOrDefault' "$main_file"; then
        echo '
// getEnvOrDefault returns the value of an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}' >> "$main_file"
    fi
    
    # Limpiar archivos temporales
    rm -f "${main_file}.tmp" /tmp/new_init_logic.txt
    
    echo "  ✅ $service actualizado"
done

echo "🎉 Actualización masiva completada"