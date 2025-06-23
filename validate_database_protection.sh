#!/bin/bash

# =============================================================================
# SCRIPT DE VALIDACIÓN DE PROTECCIÓN DE BASE DE DATOS
# Verifica que las bases de datos estén protegidas correctamente
# =============================================================================

# Configuración de colores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

echo -e "${CYAN}🔍 VALIDACIÓN DE PROTECCIÓN DE BASE DE DATOS${NC}"
echo -e "${WHITE}============================================${NC}"

# Función para validar localmente
validate_local() {
    echo -e "\n${BLUE}📍 Validando configuración local...${NC}"
    
    # 1. Verificar que .gitignore incluye .db (buscar en directorio padre)
    if grep -q "^\*.db" ../.gitignore; then
        echo -e "${GREEN}✅ .gitignore incluye protección para archivos .db${NC}"
    else
        echo -e "${RED}❌ .gitignore NO incluye protección para archivos .db${NC}"
        return 1
    fi
    
    # 2. Verificar que no hay archivos .db siendo trackeados
    if git ls-files | grep -q "\.db$"; then
        echo -e "${RED}❌ Archivos .db encontrados en git tracking:${NC}"
        git ls-files | grep "\.db$"
        return 1
    else
        echo -e "${GREEN}✅ No hay archivos .db siendo trackeados por git${NC}"
    fi
    
    # 3. Verificar que deploybackend.sh tiene protecciones (buscar en directorio padre)
    if grep -q "git update-index --assume-unchanged" ../deploybackend.sh && \
       grep -q "# RESTAURAR: Todas las bases de datos DESPUÉS de todos los comandos git" ../deploybackend.sh; then
        echo -e "${GREEN}✅ deploybackend.sh incluye protecciones de base de datos${NC}"
    else
        echo -e "${RED}❌ deploybackend.sh NO incluye protecciones adecuadas${NC}"
        return 1
    fi
    
    # 4. Verificar que existen archivos .db locales
    local db_count=$(find . -name "*.db" -type f | wc -l)
    if [ $db_count -gt 0 ]; then
        echo -e "${GREEN}✅ Encontrados $db_count archivos .db locales${NC}"
        find . -name "*.db" -type f | head -5
    else
        echo -e "${YELLOW}⚠️  No se encontraron archivos .db locales${NC}"
    fi
}

# Función para validar en VPS
validate_vps() {
    echo -e "\n${BLUE}🌐 Validando configuración en VPS...${NC}"
    
    local REMOTE_USER="root"
    local REMOTE_HOST="178.16.130.178"
    local REMOTE_DIR="/opt/hero_budget/backend"
    
    ssh "$REMOTE_USER@$REMOTE_HOST" << EOF
        cd "$REMOTE_DIR" || { echo "Error: No se pudo navegar a $REMOTE_DIR"; exit 1; }
        
        echo "🔍 Verificando archivos .db en VPS..."
        
        # 1. Verificar que existen archivos .db
        DB_COUNT=\$(find . -name "*.db" -type f | wc -l)
        if [ \$DB_COUNT -gt 0 ]; then
            echo -e "${GREEN}✅ Encontrados \$DB_COUNT archivos .db en VPS${NC}"
            
            # 2. Verificar integridad de las bases de datos
            echo "🔍 Verificando integridad de bases de datos..."
            for db_file in \$(find . -name "*.db" -type f); do
                if sqlite3 "\$db_file" "PRAGMA integrity_check;" 2>/dev/null | grep -q "ok"; then
                    echo -e "${GREEN}✅ \$db_file - Íntegra${NC}"
                else
                    echo -e "${RED}❌ \$db_file - Error de integridad${NC}"
                fi
            done
            
            # 3. Verificar contenido de la base de datos principal
            if [ -f "google_auth/users.db" ]; then
                USER_COUNT=\$(sqlite3 "google_auth/users.db" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "0")
                echo -e "${BLUE}📊 Usuarios en base de datos: \$USER_COUNT${NC}"
                
                if [ \$USER_COUNT -gt 0 ]; then
                    echo "👥 Tipos de usuarios:"
                    sqlite3 "google_auth/users.db" "SELECT type, COUNT(*) FROM users GROUP BY type;" 2>/dev/null || echo "No se pudo obtener información de tipos"
                fi
            fi
            
        else
            echo -e "${RED}❌ No se encontraron archivos .db en VPS${NC}"
            exit 1
        fi
        
        # 4. Verificar que .db no están siendo trackeados por git
        if git ls-files | grep -q "\.db$"; then
            echo -e "${RED}❌ Archivos .db encontrados en git tracking en VPS${NC}"
            git ls-files | grep "\.db$"
            exit 1
        else
            echo -e "${GREEN}✅ No hay archivos .db siendo trackeados por git en VPS${NC}"
        fi
EOF
    
    local vps_result=$?
    if [ $vps_result -eq 0 ]; then
        echo -e "${GREEN}✅ Validación de VPS completada exitosamente${NC}"
    else
        echo -e "${RED}❌ Validación de VPS falló${NC}"
        return 1
    fi
}

# Función para mostrar estadísticas de usuarios
show_user_stats() {
    echo -e "\n${BLUE}📊 Estadísticas de usuarios (VPS)...${NC}"
    
    local REMOTE_USER="root"
    local REMOTE_HOST="178.16.130.178"
    local REMOTE_DIR="/opt/hero_budget/backend"
    
    ssh "$REMOTE_USER@$REMOTE_HOST" << EOF
        cd "$REMOTE_DIR" || exit 1
        
        if [ -f "google_auth/users.db" ]; then
            echo -e "${CYAN}🏦 Base de datos principal: google_auth/users.db${NC}"
            
            # Estadísticas generales
            TOTAL_USERS=\$(sqlite3 "google_auth/users.db" "SELECT COUNT(*) FROM users;" 2>/dev/null || echo "0")
            echo -e "${WHITE}Total de usuarios: \$TOTAL_USERS${NC}"
            
            if [ \$TOTAL_USERS -gt 0 ]; then
                echo -e "\n${WHITE}Por tipo de autenticación:${NC}"
                sqlite3 "google_auth/users.db" "
                    SELECT 
                        COALESCE(type, 'NULL') as tipo,
                        COUNT(*) as cantidad,
                        printf('%.1f%%', (COUNT(*) * 100.0 / (SELECT COUNT(*) FROM users))) as porcentaje
                    FROM users 
                    GROUP BY type 
                    ORDER BY COUNT(*) DESC;
                " 2>/dev/null | while IFS='|' read tipo cantidad porcentaje; do
                    echo -e "  • \$tipo: \$cantidad usuarios (\$porcentaje)"
                done
                
                echo -e "\n${WHITE}Usuarios con email verificado:${NC}"
                VERIFIED=\$(sqlite3 "google_auth/users.db" "SELECT COUNT(*) FROM users WHERE verified_email = 1;" 2>/dev/null || echo "0")
                echo -e "  • Verificados: \$VERIFIED"
                
                echo -e "\n${WHITE}Usuarios creados recientemente (últimos 7 días):${NC}"
                RECENT=\$(sqlite3 "google_auth/users.db" "SELECT COUNT(*) FROM users WHERE created_at >= datetime('now', '-7 days');" 2>/dev/null || echo "0")
                echo -e "  • Nuevos usuarios: \$RECENT"
            fi
        else
            echo -e "${RED}❌ No se encontró la base de datos principal${NC}"
        fi
EOF
}

# Función principal
main() {
    echo -e "${CYAN}Iniciando validación de protección de base de datos...${NC}\n"
    
    case "$1" in
        "local")
            validate_local
            ;;
        "vps")
            validate_vps
            ;;
        "stats")
            show_user_stats
            ;;
        "all"|"")
            validate_local && validate_vps && show_user_stats
            ;;
        *)
            echo -e "${YELLOW}Uso: $0 [local|vps|stats|all]${NC}"
            echo -e "${WHITE}  local  - Validar solo configuración local${NC}"
            echo -e "${WHITE}  vps    - Validar solo configuración en VPS${NC}"
            echo -e "${WHITE}  stats  - Mostrar estadísticas de usuarios${NC}"
            echo -e "${WHITE}  all    - Validar todo (por defecto)${NC}"
            exit 1
            ;;
    esac
    
    local result=$?
    
    if [ $result -eq 0 ]; then
        echo -e "\n${GREEN}🎉 VALIDACIÓN COMPLETADA EXITOSAMENTE${NC}"
        echo -e "${WHITE}Las bases de datos están protegidas correctamente.${NC}"
    else
        echo -e "\n${RED}❌ VALIDACIÓN FALLÓ${NC}"
        echo -e "${WHITE}Se encontraron problemas en la protección de bases de datos.${NC}"
        exit 1
    fi
}

# Ejecutar función principal con argumentos
main "$@"