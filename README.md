# Hero Budget Backend

Hero Budget Backend es un conjunto de microservicios desarrollados en Go para gestionar las funcionalidades del backend de la aplicación Hero Budget.

## 🏗️ Arquitectura

El backend está compuesto por múltiples microservicios independientes:

- **google_auth**: Autenticación con Google OAuth
- **signin**: Inicio de sesión de usuarios
- **signup**: Registro de usuarios
- **profile_management**: Gestión de perfiles
- **dashboard_data**: Datos del dashboard
- **expense_management**: Gestión de gastos
- **income_management**: Gestión de ingresos
- **budget_management**: Gestión de presupuestos
- **savings_management**: Gestión de ahorros
- **bills_management**: Gestión de facturas
- **categories_management**: Gestión de categorías
- **cash_bank_management**: Gestión de efectivo y bancos
- **money_flow_sync**: Sincronización de flujo de dinero
- **budget_overview_fetch**: Obtención de resumen de presupuesto
- **transaction_delete_service**: Servicio de eliminación de transacciones
- **recurring_bills_management**: Gestión de facturas recurrentes
- **fetch_dashboard**: Obtención de datos del dashboard
- **reset_password**: Restablecimiento de contraseña
- **language_cookie**: Gestión de cookies de idioma
- **user_locale**: Gestión de configuración regional
- **webhook**: Webhooks para integración

## 🔧 Configuración

### Variables de Entorno

Cada servicio puede requerir variables de entorno específicas. Para configurar correctamente:

1. **Crear archivo .env** en el directorio `/backend`:
```bash
cp .env.example .env
```

2. **Configurar Google OAuth** (requerido para google_auth):
```bash
GOOGLE_CLIENT_ID=tu_google_client_id
GOOGLE_CLIENT_SECRET=tu_google_client_secret
GOOGLE_REDIRECT_URL=https://tudominio.com/auth/google/callback
```

3. **Para más detalles de configuración en VPS**, consulta: [VPS_ENV_SETUP.md](docs/VPS_ENV_SETUP.md)

### Compilación

Para compilar todos los servicios:

```bash
./compile_all_services.sh
```

### Ejecución

Para ejecutar todos los servicios:

```bash
./restart_services_vps.sh
```

## 🚀 Deployment

### VPS Setup

Para configurar el backend en un VPS:

1. Sigue las instrucciones en [VPS_ENV_SETUP.md](docs/VPS_ENV_SETUP.md)
2. Ejecuta el script de configuración: `./fix_vps_setup.sh`
3. Configura nginx con: `./apply_nginx_config.sh`

### Webhook Setup

Para configurar auto-deployment con GitHub:

1. Revisa [CONFIGURAR_GITHUB_WEBHOOK.md](CONFIGURAR_GITHUB_WEBHOOK.md)
2. Ejecuta: `./setup_auto_deployment.sh`

## 📊 Monitoreo

### Health Checks

Cada servicio expone un endpoint `/health` para verificar su estado:

```bash
# Verificar todos los servicios
./check_services_status.sh

# Verificar servicio específico
curl http://localhost:8081/health  # Google Auth
```

### Logs

```bash
# Ver logs de todos los servicios
./check_vps_services.sh

# Logs específicos con systemd
journalctl -u hero-budget-[service-name]
```

## 🛠️ Troubleshooting

Para diagnosticar problemas:

```bash
# Diagnóstico completo
./diagnose_vps_services.sh

# Verificar puertos
./verificar_puertos_servicios.sh

# Test de endpoints
./test_backend_routes.sh
```

### Guías de Solución

- [VPS_TROUBLESHOOTING_GUIDE.md](VPS_TROUBLESHOOTING_GUIDE.md)
- [nginx_fixes_report.md](nginx_fixes_report.md)
- [CORRECCIONES_REALIZADAS.md](CORRECCIONES_REALIZADAS.md)

## 🔒 Seguridad

- Las credenciales sensibles se manejan mediante variables de entorno
- Los archivos `.env` están excluidos del control de versiones
- Se utilizan permisos restrictivos para archivos de configuración
- Validación de tokens OAuth implementada

## 📚 Documentación Adicional

- [Configuración de VPS](docs/VPS_ENV_SETUP.md)
- [Configuración de GitHub](docs/GITHUB_SETUP.md)
- [Changelog](docs/CHANGELOG.md)

## 🤝 Contribución

1. Crea una rama para tu feature
2. Realiza los cambios necesarios
3. Asegúrate de que las variables de entorno estén configuradas
4. Ejecuta las pruebas
5. Crea un Pull Request

## 📝 Notas de Desarrollo

- Cada servicio debe mantener menos de 200 líneas por archivo
- Se utiliza SQLite para almacenamiento de datos
- Los puertos están estandarizados por servicio
- Se implementa logging estructurado en todos los servicios

## Estructura de Microservicios

- `google_auth/` - Autenticación con Google OAuth
- `expense_management/` - Gestión de gastos
- `income_management/` - Gestión de ingresos
- `budget_management/` - Gestión de presupuestos
- `dashboard_data/` - Datos del dashboard
- `bills_management/` - Gestión de facturas
- `profile_management/` - Gestión de perfiles
- `categories_management/` - Gestión de categorías
- `savings_management/` - Gestión de ahorros
- `cash_bank_management/` - Gestión de efectivo/banco
- `money_flow_sync/` - Sincronización de flujo de dinero
- `budget_overview_fetch/` - Resumen de presupuesto
- `transaction_delete_service/` - Eliminación de transacciones

## Tecnologías

- **Lenguaje:** Go 1.21+
- **Base de datos:** PostgreSQL
- **Servidor web:** Nginx
- **Autenticación:** Google OAuth 2.0
- **Despliegue:** VPS con Ubuntu 24.04

## Scripts de Gestión

### Configuración SSH
```bash
./scripts/setup_ssh.sh
```

### Gestión de Servicios
```bash
# Verificar estado del sistema
./scripts/manage_services.sh health

# Ver estado detallado
./scripts/manage_services.sh status

# Ver logs en tiempo real
./scripts/manage_services.sh logs

# Reiniciar servicios
./scripts/manage_services.sh restart

# Crear backup manual
./scripts/manage_services.sh backup
```

### Despliegue
```bash
# Despliegue estándar
./scripts/deploy_backend.sh

# Despliegue forzado sin backup
./scripts/deploy_backend.sh --force --no-backup

# Despliegue de branch específico
./scripts/deploy_backend.sh --branch=develop
```

## Desarrollo

### Configuración inicial
1. Configura SSH: `./scripts/setup_ssh.sh`
2. Verifica conectividad: `./scripts/manage_services.sh health`
3. Haz cambios en el código
4. Despliega: `./scripts/deploy_backend.sh`

### Estructura del proyecto
```
backend/
├── scripts/              # Scripts de gestión
├── config/              # Configuraciones
├── docs/                # Documentación
├── tests/               # Tests
├── go.mod              # Dependencias Go
├── main.go             # Aplicación principal
├── schema.sql          # Esquema de DB
└── [microservicios]/   # Cada microservicio en su carpeta
```

## CI/CD

El proyecto usa Jenkins para despliegue automático:
- Push a `main` → Despliega a producción
- Push a `develop` → Build y tests
- Push a `staging` → Despliega a staging

Ver `docs/CI_CD_GUIDE.md` para configuración completa.

## URLs de Producción

- **Sitio principal:** https://herobudget.jaimedigitalstudio.com/
- **Auth Google:** https://herobudget.jaimedigitalstudio.com/auth/google
- **Dashboard:** https://herobudget.jaimedigitalstudio.com/dashboard

## Troubleshooting

### Servicios no responden
```bash
./scripts/manage_services.sh health
ssh root@178.16.130.178 "journalctl -u herobudget -n 50"
```

### Error de conexión SSH
```bash
./scripts/setup_ssh.sh
```

### Despliegue falla
```bash
./scripts/manage_services.sh status
./scripts/deploy_backend.sh --help
```
# Test webhook martes,  3 de junio de 2025, 18:01:38 CEST
# Test webhook martes,  3 de junio de 2025, 18:01:45 CEST
# Webhook test martes,  3 de junio de 2025, 18:12:03 CEST
