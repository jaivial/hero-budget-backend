#!/bin/bash

# =============================================================================
# SCRIPT PARA REINICIAR SERVICIOS CON NUEVOS ENDPOINTS IMPLEMENTADOS
# CONFIGURADO PARA VPS - ESTRUCTURA MODULAR ADAPTADA
# =============================================================================

# Configuración de rutas del VPS
BASE_PATH="/opt/hero_budget/backend"

# Configuración de variables de entorno Redis
export REDIS_ADDR=127.0.0.1:6379
export REDIS_PASSWORD=Jva-Mvc-5171

# Configuración de colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# Configuración de servicios y puertos para VPS
ALL_SERVICES=(
    "apple-auth:8100"
    "google_auth:8081"
    "signup:8082"
    "language_cookie:8083"
    "signin:8084"
    "fetch_dashboard:8085"
    "reset_password:8086"
    "dashboard_data:8087"
    "budget_management:8088"
    "savings_management:8089"
    "cash_bank_management:8090"
    "bills_management:8091"
    "profile_management:8092"
    "income_management:8093"
    "expense_management:8094"
    "transaction_delete_service:8095"
    "categories_management:8096"
    "money_flow_sync:8097"
    "budget_overview_fetch:8098"
    "user_locale:8099"
)

# Servicios críticos (se inician primero)
CRITICAL_SERVICES=(
    "google_auth:8081"
    "signin:8084"
    "fetch_dashboard:8085"
    "cash_bank_management:8090"
)

# Función para verificar puerto en uso
is_port_in_use() {
    local port=$1
    lsof -ti:$port >/dev/null 2>&1
}

# Función para detener procesos existentes
stop_all_services() {
    echo -e "${YELLOW}🛑 Deteniendo servicios existentes...${NC}"
    
    for service_info in "${ALL_SERVICES[@]}"; do
        local port=$(echo $service_info | cut -d':' -f2)
        local pid=$(lsof -ti:$port 2>/dev/null)
        if [ ! -z "$pid" ]; then
            echo -e "${YELLOW}  Deteniendo servicio en puerto $port (PID: $pid)${NC}"
            kill -9 $pid 2>/dev/null
        fi
    done
    
    sleep 2
    echo -e "${GREEN}✅ Servicios existentes detenidos${NC}"
}

# Función para verificar dependencias del sistema
check_system_dependencies() {
    echo -e "${YELLOW}🔍 Verificando dependencias del sistema...${NC}"
    
    if ! dpkg -l | grep -q libsqlite3-dev; then
        echo -e "${YELLOW}📦 Instalando libsqlite3-dev...${NC}"
        apt-get update && apt-get install -y libsqlite3-dev
    fi
    
    if ! dpkg -l | grep -q build-essential; then
        echo -e "${YELLOW}📦 Instalando build-essential...${NC}"
        apt-get install -y build-essential
    fi
    
    echo -e "${GREEN}✅ Dependencias verificadas${NC}"
}

# Función para iniciar un servicio
start_service() {
    local service_name=$1
    local port=$2
    local service_path="${BASE_PATH}/${service_name}"
    
    echo -e "${CYAN}🚀 Iniciando $service_name en puerto $port...${NC}"
    
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Error: Directorio $service_path no encontrado${NC}"
        return 1
    fi
    
    cd "$service_path" || return 1
    
    # Verificar si existe main.go o archivos main_part*.go (para servicios divididos)
    if [ ! -f "main.go" ] && [ ! -f "main_part1.go" ]; then
        echo -e "${RED}❌ Error: main.go o main_part1.go no encontrado en $service_path${NC}"
        cd "$BASE_PATH"
        return 1
    fi
    
    # Limpiar archivos conflictivos específicos para este servicio
    cd "$BASE_PATH"
    clean_conflicting_files "$service_name"
    cd "$service_path"
    
    # Inicializar go.mod si no existe
    if [ ! -f "go.mod" ]; then
        /usr/local/go/bin/go mod init $service_name >> "/tmp/${service_name}.log" 2>&1
    fi
    
    # Descargar dependencias
    /usr/local/go/bin/go mod tidy >> "/tmp/${service_name}.log" 2>&1
    /usr/local/go/bin/go mod download >> "/tmp/${service_name}.log" 2>&1
    
    # Verificar compilación con manejo mejorado de errores
    if ! /usr/local/go/bin/go build -o "/tmp/test_${service_name}" . >> "/tmp/${service_name}.log" 2>&1; then
        echo -e "${RED}    ❌ Error de compilación para $service_name${NC}"
        echo -e "${YELLOW}    📋 Error de compilación:${NC}"
        # Mostrar las últimas líneas del log para debugging
        tail -5 "/tmp/${service_name}.log" | sed 's/^/    /'
        cd "$BASE_PATH"
        return 1
    fi
    rm -f "/tmp/test_${service_name}"
    
    # Ejecutar en background
    nohup env CGO_ENABLED=1 /usr/local/go/bin/go run . > "/tmp/${service_name}.log" 2>&1 &
    local pid=$!
    
    echo -e "${GREEN}  ✅ $service_name iniciado (PID: $pid)${NC}"
    cd "$BASE_PATH"
    sleep 1
}

# Lista de servicios que usan Redis
REDIS_SERVICES=(
    "apple-auth"
    "google_auth"
    "signup"
    "signin"
    "expense_management"
    "income_management"
    "reset_password"
    "profile_management"
    "fetch_dashboard"
    "categories_management"
    "cash_bank_management"
    "budget_overview_fetch"
    "budget_management"
    "dashboard_data"
)

# Función para verificar conexión Redis en un servicio
check_redis_connection() {
    local service_name=$1
    local log_file="/tmp/${service_name}.log"
    
    if [[ " ${REDIS_SERVICES[@]} " =~ " ${service_name} " ]]; then
        if [ -f "$log_file" ]; then
            if grep -q "Successfully connected to Redis" "$log_file" 2>/dev/null; then
                echo -e "${GREEN}    🟢 Redis conectado${NC}"
                return 0
            elif grep -q "Failed to connect to Redis" "$log_file" 2>/dev/null; then
                echo -e "${RED}    🔴 Redis desconectado${NC}"
                return 1
            else
                echo -e "${YELLOW}    🟡 Redis estado desconocido${NC}"
                return 2
            fi
        else
            echo -e "${YELLOW}    🟡 Log no encontrado${NC}"
            return 2
        fi
    else
        echo -e "${BLUE}    🟦 Sin Redis${NC}"
        return 0
    fi
}

# Función para verificar servicio con estado Redis
verify_service() {
    local service_name=$1
    local port=$2
    
    if is_port_in_use "$port"; then
        local pid=$(lsof -ti:$port 2>/dev/null)
        echo -e "${GREEN}  ✅ $service_name está activo (Puerto: $port, PID: $pid)${NC}"
        check_redis_connection "$service_name"
        return 0
    else
        echo -e "${RED}  ❌ $service_name no está activo en puerto $port${NC}"
        return 1
    fi
}

# Función para mostrar estado de servicios
show_services_status() {
    echo -e "\n${WHITE}📊 ESTADO ACTUAL DE LOS SERVICIOS - VPS${NC}"
    echo -e "${WHITE}=============================================================================${NC}"
    
    local active_count=0
    for ((i=0; i<${#ALL_SERVICES[@]}; i++)); do
        local service_info="${ALL_SERVICES[$i]}"
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        local status_text="🔴 INACTIVO"
        local status_color="${RED}"
        
        if is_port_in_use "$port"; then
            status_text="🟢 ACTIVO"
            status_color="${GREEN}"
            ((active_count++))
        fi
        
        printf "${BLUE}%2d)${NC} %-35s ${YELLOW}:%s${NC} ${status_color}%s${NC}\n" \
            $((i+1)) "$service_name" "$port" "$status_text"
    done
    
    echo -e "\n${WHITE}📈 RESUMEN: ${GREEN}$active_count${NC}/${BLUE}${#ALL_SERVICES[@]}${NC} servicios activos${NC}"
}

# Función para mostrar ayuda
show_help() {
    echo -e "\n${WHITE}🔧 GESTIÓN DE MICROSERVICIOS GO - VPS${NC}"
    echo -e "${CYAN}Uso: $0 [OPCIÓN]${NC}"
    echo -e "\n${YELLOW}Opciones:${NC}"
    echo -e "  ${GREEN}-a, --all${NC}     Reiniciar TODOS los servicios automáticamente"
    echo -e "  ${GREEN}-s, --status${NC}  Mostrar estado actual de todos los servicios"
    echo -e "  ${GREEN}-h, --help${NC}    Mostrar esta ayuda"
    echo -e "\n${YELLOW}Ejemplos:${NC}"
    echo -e "  $0                 # Reiniciar todos los servicios"
    echo -e "  $0 --status        # Ver estado de servicios"
}


# Función para actualizar código desde repositorio (deshabilitada)
update_code_from_repository() {
    echo -e "\n${CYAN}🔄 Usando código actual (sin actualización automática)...${NC}"
    echo -e "${GREEN}✅ Continuando con código actual${NC}"
}

# Función para limpiar archivos conflictivos específicos de un servicio
clean_conflicting_files() {
    local service_name=$1
    
    case "$service_name" in
        "bills_management")
            # Eliminar main.go en bills_management si existe (debe usar main_part*.go)
            if [ -f "bills_management/main.go" ]; then
                echo -e "${YELLOW}  Limpiando bills_management/main.go conflictivo${NC}"
                rm -f "bills_management/main.go"
            fi
            ;;
        "transaction_delete_service")
            # Limpiar duplicaciones en transaction_delete_service
            if [ -f "transaction_delete_service/main.go" ]; then
                # Lista de funciones y tipos que pueden estar duplicados
                duplicated_items=(
                    "type TransactionDetails"
                    "func getTransactionDetails"
                    "func deleteTransaction"
                    "func updatePreviousBalanceForPeriod"
                    "func parsePeriodIdentifier"
                    "func calculatePeriodIdentifier"
                    "func recalculateAllBalances"
                    "func updatePeriodBalance"
                    "func updateSubsequentPeriods"
                )
                
                # Verificar si hay duplicaciones en main.go
                has_duplications=false
                for item in "${duplicated_items[@]}"; do
                    if grep -q "^${item}" "transaction_delete_service/main.go" 2>/dev/null; then
                        has_duplications=true
                        break
                    fi
                done
                
                if [ "$has_duplications" = true ]; then
                    echo -e "${YELLOW}  Limpiando duplicaciones en transaction_delete_service/main.go${NC}"
                    # Reemplazar con la versión limpia del repositorio directamente
                    cat > "transaction_delete_service/main.go" << 'EOF'
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// Transaction deletion request structure
type DeleteTransactionRequest struct {
	UserID          string `json:"user_id"`
	TransactionID   int    `json:"transaction_id"`
	TransactionType string `json:"transaction_type"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

var db *sql.DB

func init() {
	var err error

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct absolute path to the database file
	dbPath := filepath.Join(cwd, "..", "google_auth", "users.db")
	log.Printf("Using database at: %s", dbPath)

	// Open the database connection
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Transaction Delete Service - Database connection established successfully")
}

func main() {
	// CORS middleware function
	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		}
	}

	// Health check endpoint
	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		response := ApiResponse{
			Success: true,
			Message: "Transaction Delete Service is running",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))

	// Delete transaction endpoint
	http.HandleFunc("/transactions/delete", corsMiddleware(handleDeleteTransaction))

	port := "8095" // Unique port for transaction delete service
	log.Printf("Transaction Delete Service starting on port %s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest DeleteTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&deleteRequest); err != nil {
		log.Printf("Error decoding request body: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Invalid request format",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if deleteRequest.UserID == "" || deleteRequest.TransactionID <= 0 || deleteRequest.TransactionType == "" {
		response := ApiResponse{
			Success: false,
			Message: "Missing required fields: user_id, transaction_id, or transaction_type",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Printf("Deleting transaction ID %d of type %s for user %s",
		deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)

	// Get transaction details before deletion for balance recalculation
	transaction, err := getTransactionDetails(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
	if err != nil {
		log.Printf("Error fetching transaction details: %v", err)
		response := ApiResponse{
			Success: false,
			Message: "Transaction not found or access denied",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle special case: expense with bill_id (corresponds to a bill payment)
	if deleteRequest.TransactionType == "expense" && transaction.BillID != nil {
		err = handleExpenseWithBillDeletion(*transaction)
		if err != nil {
			log.Printf("Error handling expense with bill deletion: %v", err)
			response := ApiResponse{
				Success: false,
				Message: "Failed to handle expense with bill deletion",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
	} else {
		// For regular transactions (income, bills, expenses without bill_id)
		// Delete the transaction first
		err = deleteTransaction(deleteRequest.TransactionID, deleteRequest.TransactionType, deleteRequest.UserID)
		if err != nil {
			log.Printf("Error deleting transaction: %v", err)
			response := ApiResponse{
				Success: false,
				Message: "Failed to delete transaction",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Recalculate balances for all time periods
		err = recalculateAllBalances(deleteRequest.UserID, transaction.Date, transaction.Amount, transaction.PaymentMethod, deleteRequest.TransactionType, transaction.BillID)
		if err != nil {
			log.Printf("Error recalculating balances: %v", err)
			// Don't fail the request if balance recalculation fails, just log it
		}
	}

	response := ApiResponse{
		Success: true,
		Message: "Transaction deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
EOF
                fi
            fi
            ;;
    esac
}

# Función para actualizar dependencias de todos los servicios
update_all_dependencies() {
    echo -e "\n${CYAN}🔧 Actualizando dependencias Redis en todos los servicios...${NC}"
    
    # Lista de servicios que tienen Redis implementado
    local redis_services=(
        "apple-auth"
        "bills_management"
        "budget_management" 
        "budget_overview_fetch"
        "cash_bank_management"
        "categories_management"
        "dashboard_data"
        "expense_management"
        "fetch_dashboard"
        "google_auth"
        "income_management"
        "language_cookie"
        "money_flow_sync"
        "profile_management"
        "reset_password"
        "savings_management"
        "signin"
        "signup"
        "transaction_delete_service"
        "user_locale"
    )
    
    for service in "${redis_services[@]}"; do
        if [ -d "$service" ]; then
            echo -e "${BLUE}  🔧 Actualizando dependencias: $service${NC}"
            cd "$service"
            
            # Verificar y actualizar go.mod si es necesario
            if ! grep -q "github.com/redis/go-redis/v9" go.mod 2>/dev/null; then
                echo -e "${YELLOW}    📦 Agregando dependencia Redis...${NC}"
                /usr/local/go/bin/go get github.com/redis/go-redis/v9 > /dev/null 2>&1
            fi
            
            # Actualizar dependencias
            /usr/local/go/bin/go mod tidy > /dev/null 2>&1
            /usr/local/go/bin/go mod download > /dev/null 2>&1
            
            cd "$BASE_PATH"
        fi
    done
    
    echo -e "${GREEN}✅ Dependencias actualizadas${NC}"
}

# Función principal de reinicio
restart_all_services() {
    echo -e "\n${WHITE}=== REINICIANDO TODOS LOS SERVICIOS - VPS ===${NC}"
    
    # Actualizar código desde repositorio
    update_code_from_repository
    
    # Actualizar dependencias Redis
    update_all_dependencies
    
    # Verificar directorio base
    if [ ! -d "$BASE_PATH" ]; then
        echo -e "${RED}❌ Error: El directorio base $BASE_PATH no existe${NC}"
        exit 1
    fi
    
    cd "$BASE_PATH" || exit 1
    echo -e "${GREEN}📂 Trabajando desde: $(pwd)${NC}"
    
    # Verificar dependencias y detener servicios
    check_system_dependencies
    stop_all_services
    
    # Iniciar servicios críticos primero
    echo -e "\n${CYAN}📋 INICIANDO SERVICIOS CRÍTICOS:${NC}"
    for service_info in "${CRITICAL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        start_service "$service_name" "$port"
    done
    
    # Iniciar resto de servicios
    echo -e "\n${CYAN}📋 INICIANDO RESTO DE SERVICIOS:${NC}"
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        
        if [[ ! " ${CRITICAL_SERVICES[@]} " =~ " ${service_info} " ]]; then
            start_service "$service_name" "$port"
        fi
    done
    
    # Verificación final
    echo -e "\n${WHITE}=== VERIFICACIÓN FINAL ===${NC}"
    echo -e "${YELLOW}⏳ Esperando 5 segundos para inicialización...${NC}"
    sleep 5
    
    local active_count=0
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        local port=$(echo $service_info | cut -d':' -f2)
        
        if verify_service "$service_name" "$port"; then
            ((active_count++))
        fi
    done
    
    # Contar servicios con Redis conectado
    local redis_connected=0
    local redis_total=0
    
    echo -e "\n${WHITE}=== VERIFICACIÓN REDIS ===${NC}"
    for service_info in "${ALL_SERVICES[@]}"; do
        local service_name=$(echo $service_info | cut -d':' -f1)
        
        if [[ " ${REDIS_SERVICES[@]} " =~ " ${service_name} " ]]; then
            ((redis_total++))
            if check_redis_connection "$service_name" >/dev/null; then
                ((redis_connected++))
                echo -e "${GREEN}✅ $service_name: Redis conectado${NC}"
            else
                echo -e "${RED}❌ $service_name: Redis desconectado${NC}"
            fi
        fi
    done
    
    # Mostrar resumen final
    echo -e "\n${WHITE}"
    echo "============================================================================="
    echo "   ✅ SERVICIOS REINICIADOS - VPS"
    echo "   📊 Servicios activos: $active_count/${#ALL_SERVICES[@]}"
    echo "   🟢 Redis conectado: $redis_connected/$redis_total servicios"
    echo "============================================================================="
    echo -e "${NC}"
    
    if [ $active_count -eq ${#ALL_SERVICES[@]} ]; then
        if [ $redis_connected -eq $redis_total ]; then
            echo -e "${GREEN}🎉 TODOS LOS SERVICIOS Y REDIS ESTÁN FUNCIONANDO PERFECTAMENTE${NC}"
        else
            echo -e "${YELLOW}⚠️  SERVICIOS ACTIVOS PERO ALGUNOS SIN REDIS ($redis_connected/$redis_total)${NC}"
        fi
    else
        echo -e "${YELLOW}⚠️  $active_count/${#ALL_SERVICES[@]} servicios funcionando${NC}"
        echo -e "${YELLOW}💡 Revisar logs en: /tmp/[servicio].log${NC}"
    fi
    
    # Mostrar URLs de servicios
    echo -e "\n${CYAN}📋 ENDPOINTS PRINCIPALES:${NC}"
    echo -e "${WHITE}  • Cash Update: http://localhost:8090/cash-bank/cash/update${NC}"
    echo -e "${WHITE}  • Bank Update: http://localhost:8090/cash-bank/bank/update${NC}"
    echo -e "${WHITE}  • Profile Update: http://localhost:8092/update/locale${NC}"
    echo -e "${WHITE}  • Money Flow: http://localhost:8097/money-flow/data${NC}"
    echo -e "${WHITE}  • User Locale Get: http://localhost:8099/user_locale/get${NC}"
    echo -e "${WHITE}  • User Locale Update: http://localhost:8099/user_locale/update${NC}"
}

# Función principal
main() {
    echo -e "${WHITE}"
    echo "============================================================================="
    echo "   🔄 GESTIÓN DE MICROSERVICIOS GO - VPS"
    echo "============================================================================="
    echo -e "${NC}"
    
    # Verificar argumentos de línea de comandos
    case "$1" in
        "--all"|"-a"|"")
            restart_all_services
            ;;
        "--status"|"-s")
            show_services_status
            ;;
        "--help"|"-h")
            show_help
            ;;
        *)
            echo -e "${RED}❌ Opción desconocida: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Ejecutar función principal con argumentos
main "$@" 