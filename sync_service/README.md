# HeroBudget Sync Service

Servicio de sincronización para funcionalidad offline de HeroBudget. Este servicio maneja la sincronización masiva de operaciones offline cuando se restaura la conectividad.

## 🚀 Inicio Rápido

### Prerrequisitos

- Go 1.21 o superior
- SQLite3
- Acceso a la base de datos principal de HeroBudget

### Instalación y Ejecución

1. **Ejecutar con script de inicio (Recomendado):**
   ```bash
   cd backend/sync_service
   ./start_sync_service.sh
   ```

2. **Ejecución manual:**
   ```bash
   cd backend/sync_service
   go mod tidy
   go get github.com/gorilla/mux
   go get github.com/mattn/go-sqlite3
   go run main.go
   ```

### Configuración

El servicio puede configurarse mediante variables de entorno o el archivo `config.env`:

```bash
# Puerto del servicio
export SYNC_SERVICE_PORT=8101

# Ruta a la base de datos
export DATABASE_PATH="../budget_data.db"
```

## 📡 API Endpoints

### POST /sync/batch
Sincronización masiva de operaciones offline.

**Request Body:**
```json
{
  "user_id": "usuario123",
  "device_id": "device456",
  "last_sync_timestamp": "2023-12-01T10:00:00Z",
  "client_timestamp": "2023-12-01T10:30:00Z",
  "operations": [
    {
      "id": "op1",
      "operation_type": "create",
      "table_name": "expenses_local",
      "data": {
        "amount": 50.00,
        "category": "Food",
        "description": "Lunch"
      },
      "sequence_number": 1,
      "created_at": "2023-12-01T10:15:00Z",
      "dependencies": []
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "message": "Sync completed successfully. Processed 1 operations.",
  "processed_operations": 1,
  "successful_operations": 1,
  "failed_operations": 0,
  "server_updates": [],
  "conflict_resolutions": [],
  "new_sync_timestamp": "2023-12-01T10:35:00Z",
  "errors": []
}
```

### GET /sync/health
Health check del servicio.

**Response:**
```json
{
  "status": "healthy",
  "service": "sync-service",
  "timestamp": "2023-12-01T10:35:00Z"
}
```

## 🏗️ Arquitectura

### Componentes Principales

1. **SyncService**: Maneja las solicitudes HTTP y coordinación general
2. **Operations Processor**: Procesa operaciones individuales en orden secuencial
3. **Conflict Resolver**: Resuelve conflictos entre datos locales y del servidor
4. **Database Manager**: Maneja conexiones y operaciones de base de datos
5. **Logging System**: Registra todas las operaciones para auditoría

### Flujo de Sincronización

1. **Recepción**: Cliente envía lote de operaciones offline
2. **Validación**: Verificar datos requeridos y formato
3. **Ordenamiento**: Ordenar operaciones por sequence_number
4. **Procesamiento**: Ejecutar operaciones una por una
5. **Logging**: Registrar resultados en tabla de auditoría
6. **Respuesta**: Enviar resultados al cliente

### Tablas de Base de Datos

#### sync_metadata
```sql
CREATE TABLE sync_metadata (
    user_id TEXT PRIMARY KEY,
    device_id TEXT,
    last_sync_timestamp TEXT,
    last_client_timestamp TEXT,
    total_operations_processed INTEGER DEFAULT 0,
    last_sync_device TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);
```

#### sync_operations_log
```sql
CREATE TABLE sync_operations_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    operation_type TEXT NOT NULL,
    table_name TEXT NOT NULL,
    status TEXT NOT NULL,
    error_message TEXT,
    processed_at TEXT DEFAULT CURRENT_TIMESTAMP,
    server_id INTEGER,
    conflict_resolution TEXT
);
```

## 🔧 Configuración Avanzada

### Variables de Entorno

| Variable | Descripción | Valor por Defecto |
|----------|-------------|-------------------|
| `SYNC_SERVICE_PORT` | Puerto del servicio | `8101` |
| `DATABASE_PATH` | Ruta a la base de datos | `../budget_data.db` |
| `LOG_LEVEL` | Nivel de logging | `info` |
| `MAX_BATCH_SIZE` | Tamaño máximo de lote | `100` |
| `DB_TIMEOUT` | Timeout de base de datos (s) | `30` |

### Mapeo de Tablas

El servicio mapea tablas locales a tablas del servidor:

- `expenses_local` → `expenses`
- `incomes_local` → `incomes`
- `bills_local` → `bills`
- `categories_local` → `categories`

## 🚨 Manejo de Errores

### Tipos de Error

1. **Errores de Validación**: Datos faltantes o formato incorrecto
2. **Errores de Base de Datos**: Problemas de conexión o SQL
3. **Errores de Operación**: Fallos al procesar operaciones específicas
4. **Errores de Conflicto**: Conflictos entre datos locales y del servidor

### Estrategias de Recuperación

- **Reintentos**: Operaciones fallidas se pueden reintentar
- **Rollback Parcial**: Solo operaciones exitosas se confirman
- **Logging Detallado**: Todos los errores se registran para debugging

## 📊 Monitoreo

### Logs

El servicio genera logs detallados de:
- Solicitudes de sincronización
- Operaciones procesadas
- Errores y advertencias
- Métricas de rendimiento

### Health Check

Monitorear el endpoint `/sync/health` para verificar el estado del servicio.

## 🔒 Seguridad

### Consideraciones

- **Validación de Entrada**: Todos los datos se validan antes del procesamiento
- **SQL Injection**: Se usan prepared statements para todas las consultas
- **CORS**: Configuración de CORS para control de acceso
- **Rate Limiting**: Considerar implementar rate limiting en producción

### Recomendaciones de Producción

1. Ejecutar detrás de un proxy reverso (nginx)
2. Implementar HTTPS
3. Configurar logs centralizados
4. Monitoreo de métricas
5. Backups regulares de la base de datos

## 🛠️ Desarrollo

### Compilación

```bash
go build -o sync_service main.go
```

### Testing

```bash
# Unit tests (cuando estén implementados)
go test ./...

# Test manual del endpoint de health
curl http://localhost:8101/sync/health
```

### Debugging

Para debugging detallado, configurar:
```bash
export LOG_LEVEL=debug
```

## 📋 TODO / Mejoras Futuras

- [ ] Tests unitarios e integración
- [ ] Rate limiting
- [ ] Autenticación JWT
- [ ] Métricas de Prometheus
- [ ] Docker containerization
- [ ] Clustering support
- [ ] Real-time sync notifications

## 🤝 Contribución

1. Fork el proyecto
2. Crear feature branch
3. Commit los cambios
4. Push al branch
5. Crear Pull Request

## 📄 Licencia

Este proyecto es parte de HeroBudget y sigue la misma licencia del proyecto principal.