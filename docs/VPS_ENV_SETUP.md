# Configuración de Variables de Entorno en VPS - Hero Budget Backend

Esta guía te ayudará a configurar las variables de entorno necesarias para el servicio de Google OAuth en tu VPS.

## 📋 Prerequisitos

- Acceso SSH al VPS
- Permisos de administrador (sudo)
- Servicio de Google OAuth configurado en Google Cloud Console

## 🔧 Variables de Entorno Requeridas

### Para Google OAuth Service

```bash
GOOGLE_CLIENT_ID=tu_google_client_id_aqui
GOOGLE_CLIENT_SECRET=tu_google_client_secret_aqui
GOOGLE_REDIRECT_URL=https://tudominio.com/auth/google/callback
```

## 🚀 Configuración en VPS

### Paso 1: Crear el archivo .env

```bash
# Navegar al directorio del backend
cd /path/to/hero-budget/backend

# Crear el archivo .env
sudo nano .env
```

### Paso 2: Agregar las variables de entorno

Copia y pega el siguiente contenido en el archivo `.env`:

```bash
# Google OAuth Configuration
GOOGLE_CLIENT_ID=204913639838-4m6soe15a1e1tssnfupuj1mcbd6arj82.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=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
GOOGLE_REDIRECT_URL=https://tudominio.com/auth/google/callback

# Database Configuration (opcional)
DB_PATH=./users.db

# Server Configuration (opcional)
PORT=8081
```

### Paso 3: Configurar permisos de seguridad

```bash
# Establecer permisos restrictivos para el archivo .env
sudo chmod 600 .env
sudo chown root:root .env
```

### Paso 4: Verificar configuración

```bash
# Verificar que el archivo existe y tiene el contenido correcto
cat .env

# Verificar permisos
ls -la .env
```

## 🔄 Configuración con systemd

Si usas systemd para administrar el servicio:

### Paso 1: Crear archivo de configuración de entorno

```bash
sudo nano /etc/systemd/system/hero-budget-google-auth.env
```

### Paso 2: Agregar variables

```bash
GOOGLE_CLIENT_ID=204913639838-4m6soe15a1e1tssnfupuj1mcbd6arj82.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=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
GOOGLE_REDIRECT_URL=https://tudominio.com/auth/google/callback
```

### Paso 3: Modificar servicio systemd

```bash
sudo nano /etc/systemd/system/hero-budget-google-auth.service
```

Agregar la línea `EnvironmentFile`:

```ini
[Unit]
Description=Hero Budget Google Auth Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/path/to/hero-budget/backend/google_auth
EnvironmentFile=/etc/systemd/system/hero-budget-google-auth.env
ExecStart=/path/to/hero-budget/backend/google_auth/main
Restart=always

[Install]
WantedBy=multi-user.target
```

## 🔒 Configuración con Docker (alternativa)

Si usas Docker:

### Paso 1: Crear docker-compose.yml

```yaml
version: '3.8'
services:
  google-auth:
    build: .
    ports:
      - "8081:8081"
    environment:
      - GOOGLE_CLIENT_ID=204913639838-4m6soe15a1e1tssnfupuj1mcbd6arj82.apps.googleusercontent.com
      - GOOGLE_CLIENT_SECRET=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
      - GOOGLE_REDIRECT_URL=https://tudominio.com/auth/google/callback
    volumes:
      - ./data:/app/data
```

## ✅ Verificación de la Configuración

### Verificar que el servicio lee las variables

```bash
# Ejecutar el servicio manualmente para probar
cd /path/to/hero-budget/backend/google_auth
./main
```

Si la configuración es correcta, deberías ver:

```
2024/01/XX XX:XX:XX Registering routes:
2024/01/XX XX:XX:XX - POST /auth/google
2024/01/XX XX:XX:XX - POST /update/locale
2024/01/XX XX:XX:XX - GET /health
2024/01/XX XX:XX:XX Server started on :8081
```

### Test de conectividad

```bash
# Probar el endpoint de health
curl http://localhost:8081/health
```

## 🚨 Seguridad Importante

1. **Nunca** commitees el archivo `.env` en Git
2. Agrega `.env` a tu `.gitignore`
3. Usa permisos restrictivos (600) para archivos con secrets
4. Considera usar un gestor de secretos para producción
5. Rota las credenciales periódicamente

## 🔧 Troubleshooting

### Error: "GOOGLE_CLIENT_ID environment variable is required"

- Verificar que el archivo `.env` existe
- Verificar que las variables están definidas sin espacios
- Verificar permisos del archivo

### Error: "Invalid token"

- Verificar que `GOOGLE_CLIENT_ID` es correcto
- Verificar configuración en Google Cloud Console

### Error de permisos

```bash
# Restaurar permisos correctos
sudo chmod 600 .env
sudo chown $USER:$USER .env
```

## 📞 Soporte

Si encuentras problemas, revisa:
1. Logs del servicio: `journalctl -u hero-budget-google-auth`
2. Configuración de Google Cloud Console
3. Firewall y puertos abiertos
4. Variables de entorno cargadas correctamente 