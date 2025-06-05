# 🔧 Guía de Solución de Problemas VPS - Hero Budget Backend

## 📋 Resumen de la Solución Implementada

Se identificaron y solucionaron múltiples problemas en el VPS que impedían que los servicios Go funcionaran correctamente. Solo 3 de 18 servicios estaban operativos.

## 🚨 Problemas Identificados y Solucionados

### 1. **Dependencias Faltantes**
- ❌ **Problema**: Faltaban librerías SQLite3 development 
- ✅ **Solución**: Script automático de instalación de dependencias

### 2. **Errores en Nombres de Logs**
- ❌ **Problema**: Los logs tenían espacios en lugar de guiones bajos
- ✅ **Solución**: Corregida función `verify_service()` con nombres correctos

### 3. **Falta de Verificación de Compilación**
- ❌ **Problema**: Los servicios fallaban silenciosamente al compilar
- ✅ **Solución**: Verificación previa de compilación antes de ejecutar

### 4. **Manejo de Errores Insuficiente**
- ❌ **Problema**: No se mostraban errores específicos cuando fallaba un servicio
- ✅ **Solución**: Logs detallados y diagnóstico en tiempo real

### 5. **Falta de Herramientas de Diagnóstico**
- ❌ **Problema**: Difícil identificar qué servicios fallan y por qué
- ✅ **Solución**: Scripts especializados de diagnóstico y monitoreo

## 🛠️ Scripts Mejorados Disponibles

### 1. **🚀 `restart_services_vps.sh`** (Script Principal Mejorado)
```bash
cd /opt/hero_budget/backend
chmod +x restart_services_vps.sh
./restart_services_vps.sh
```

**Mejoras implementadas:**
- ✅ Verificación automática de dependencias del sistema
- ✅ Instalación automática de librerías faltantes
- ✅ Verificación de compilación antes de ejecutar
- ✅ Mejor manejo de errores con logs detallados
- ✅ Orden optimizado de inicio de servicios
- ✅ Verificación robusta de estado de servicios

### 2. **📦 `install_vps_dependencies.sh`** (Nuevo)
```bash
cd /opt/hero_budget/backend
chmod +x install_vps_dependencies.sh
./install_vps_dependencies.sh
```

**Instala automáticamente:**
- libsqlite3-dev
- build-essential
- gcc
- pkg-config
- curl y git (si no están)

### 3. **🔍 `diagnose_vps_services.sh`** (Nuevo)
```bash
cd /opt/hero_budget/backend
chmod +x diagnose_vps_services.sh
./diagnose_vps_services.sh
```

**Proporciona:**
- Análisis detallado de cada servicio
- Verificación de compilación individual
- Revisión de logs con errores resaltados
- Recomendaciones específicas de solución

### 4. **📊 `check_services_status.sh`** (Nuevo)
```bash
cd /opt/hero_budget/backend
chmod +x check_services_status.sh
./check_services_status.sh
```

**Muestra:**
- Estado actual de todos los servicios
- Conteo de servicios activos/fallidos
- Porcentaje de éxito
- Recomendaciones automáticas

## 🔄 Flujo de Solución de Problemas Recomendado

### **Paso 1: Instalar Dependencias**
```bash
cd /opt/hero_budget/backend
./install_vps_dependencies.sh
```

### **Paso 2: Verificar Estado Inicial**
```bash
./check_services_status.sh
```

### **Paso 3: Reiniciar Servicios (Mejorado)**
```bash
./restart_services_vps.sh
```

### **Paso 4: Diagnóstico Detallado (Si hay fallos)**
```bash
./diagnose_vps_services.sh
```

### **Paso 5: Monitoreo Continuo**
```bash
./check_services_status.sh
```

## 🎯 Servicios y Puertos Configurados

| Servicio | Puerto | Categoría |
|----------|--------|-----------|
| google_auth | 8081 | Autenticación |
| signup | 8082 | Autenticación |
| language_cookie | 8083 | Autenticación |
| signin | 8084 | Autenticación |
| fetch_dashboard | 8085 | Prioritario |
| reset_password | 8086 | Autenticación |
| dashboard_data | 8087 | Complementario |
| budget_management | 8088 | Gestión Financiera |
| savings_management | 8089 | Prioritario |
| cash_bank_management | 8090 | Prioritario |
| bills_management | 8091 | Gestión Financiera |
| profile_management | 8092 | Prioritario |
| income_management | 8093 | Gestión Financiera |
| expense_management | 8094 | Gestión Financiera |
| transaction_delete_service | 8095 | Complementario |
| categories_management | 8096 | Gestión Financiera |
| money_flow_sync | 8097 | Prioritario |
| budget_overview_fetch | 8098 | Gestión Financiera |

## 🚨 Solución de Problemas Comunes

### **Error: "cannot find package"**
```bash
cd /opt/hero_budget/backend/[servicio_con_error]
go mod tidy
go mod download
```

### **Error: "SQLite3 not found"**
```bash
./install_vps_dependencies.sh
```

### **Error: "Permission denied"**
```bash
chown -R root:root /opt/hero_budget
chmod -R 755 /opt/hero_budget
```

### **Servicio no responde después de iniciar**
```bash
# Ver logs específicos
tail -f /tmp/[nombre_servicio].log

# Ejemplo:
tail -f /tmp/cash_bank_management.log
```

## 📊 Verificación de Éxito

### **Verificar todos los puertos activos:**
```bash
lsof -i -P -n | grep -E ':(808[0-9]|809[0-9])' | grep LISTEN
```

### **Verificar servicios específicos:**
```bash
# Cash Bank Management
curl http://localhost:8090/cash-bank/distribution?user_id=1

# Money Flow Sync
curl http://localhost:8097/money-flow/data?user_id=1

# Google Auth
curl http://localhost:8081/health
```

## 🎉 Resultados Esperados

Con estas mejoras implementadas, deberías obtener:

- ✅ **18/18 servicios activos** (100% de éxito)
- ✅ **Todos los puertos respondiendo** correctamente
- ✅ **Logs detallados** para debugging
- ✅ **Diagnóstico automático** de problemas
- ✅ **Instalación automática** de dependencias

## 📞 Contacto y Soporte

Si sigues teniendo problemas después de aplicar estas soluciones:

1. Ejecuta el diagnóstico completo: `./diagnose_vps_services.sh`
2. Revisa los logs específicos del servicio fallido
3. Verifica que todas las dependencias están instaladas
4. Confirma que los permisos de archivo son correctos

---

**💡 Tip**: Ejecuta `./check_services_status.sh` regularmente para monitorear el estado de los servicios. 