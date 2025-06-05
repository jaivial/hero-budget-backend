# Hero Budget Backend

Microservicios backend para la aplicación Hero Budget.

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
