# 🚀 Sistema de Deployment Automático - Hero Budget Backend

Este sistema permite el deployment automático del backend cuando se hace push al repositorio de GitHub.

## 📋 Descripción

El sistema funciona de la siguiente manera:

1. **Push a GitHub** → **Webhook** → **VPS recibe notificación**
2. **VPS ejecuta `deploy.sh`** → **`git pull --rebase`** → **`restart_services.sh`**
3. **Servicios actualizados** → **Sistema funcionando con el nuevo código**

## 🏗️ Arquitectura

```
GitHub Repository
       ↓ (webhook)
   Webhook Server (Puerto 9000)
       ↓ (ejecuta)
   Deploy Script
       ↓ (actualiza código)
   Git Pull --rebase
       ↓ (reinicia)
   Restart Services Script
       ↓ (resultado)
   Servicios Actualizados
```

## 📂 Estructura de Archivos

```
/opt/hero_budget/webhook/
├── webhook_server.go           # Servidor webhook principal
├── deploy.sh                   # Script de deployment
├── install_webhook.sh          # Script de instalación
├── herobudget-webhook.service  # Archivo de servicio systemd
├── go.mod                      # Dependencias Go
├── webhook_config.example      # Configuración de ejemplo
├── deployment.log              # Logs de deployments
└── README.md                   # Esta documentación
```

## 🛠️ Instalación

### 1. Ejecutar el Script de Instalación

```bash
# En el VPS como root
cd /opt/hero_budget/backend/webhook/
chmod +x install_webhook.sh
./install_webhook.sh
```

### 2. Configurar Secret de GitHub

```bash
# Generar un secret seguro
export GITHUB_WEBHOOK_SECRET=$(openssl rand -hex 20)
echo "Secret generado: $GITHUB_WEBHOOK_SECRET"

# Configurar en el sistema
echo "Environment=GITHUB_WEBHOOK_SECRET=$GITHUB_WEBHOOK_SECRET" >> /etc/systemd/system/herobudget-webhook.service

# Recargar y reiniciar
systemctl daemon-reload
systemctl restart herobudget-webhook
```

### 3. Configurar Webhook en GitHub

1. Ve a tu repositorio: https://github.com/jaivial/herobudget-backend
2. Settings → Webhooks → Add webhook
3. Configurar:
   - **Payload URL**: `http://srv736989.hstgr.cloud:9000/webhook`
   - **Content type**: `application/json`
   - **Secret**: El secret generado anteriormente
   - **Which events**: Solo "Push events"
   - **Active**: ✅ Marcado

## 🔧 Configuración del VPS

### Variables de Entorno

```bash
# Secret del webhook (requerido para producción)
export GITHUB_WEBHOOK_SECRET="tu-secret-aqui"
```

### Servicio Systemd

```bash
# Ver estado
systemctl status herobudget-webhook

# Iniciar/parar/reiniciar
systemctl start herobudget-webhook
systemctl stop herobudget-webhook
systemctl restart herobudget-webhook

# Ver logs en tiempo real
journalctl -u herobudget-webhook -f
```

## 📡 Endpoints Disponibles

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/webhook` | POST | Recibe webhooks de GitHub |
| `/health` | GET | Health check del servicio |
| `/logs` | GET | Ver últimos 50 logs de deployment |

### Ejemplos de Uso

```bash
# Health check
curl http://localhost:9000/health

# Ver logs recientes
curl http://localhost:9000/logs

# Simular webhook (solo para testing)
curl -X POST http://localhost:9000/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -d '{"ref":"refs/heads/main","repository":{"full_name":"jaivial/herobudget-backend"}}'
```

## 📝 Logs y Monitoreo

### Logs del Servicio Webhook

```bash
# Logs en tiempo real
journalctl -u herobudget-webhook -f

# Últimos 50 logs
journalctl -u herobudget-webhook -n 50

# Logs con filtro de fecha
journalctl -u herobudget-webhook --since "1 hour ago"
```

### Logs de Deployment

```bash
# Ver logs de deployment
tail -f /opt/hero_budget/webhook/deployment.log

# Ver últimos deployments
tail -n 100 /opt/hero_budget/webhook/deployment.log
```

### Logs de Servicios

```bash
# Logs de servicios individuales (generados por restart_services.sh)
tail -f /tmp/cash_bank_management.log
tail -f /tmp/profile_management.log
# etc.
```

## 🔍 Troubleshooting

### Problema: Webhook no recibe requests

**Verificar:**
1. Puerto 9000 abierto en firewall
2. Servicio corriendo: `systemctl status herobudget-webhook`
3. Configuración en GitHub correcta

**Solución:**
```bash
# Verificar puerto
netstat -tuln | grep :9000

# Verificar firewall
ufw status | grep 9000

# Probar local
curl http://localhost:9000/health
```

### Problema: Deployment falla

**Verificar logs:**
```bash
tail -f /opt/hero_budget/webhook/deployment.log
journalctl -u herobudget-webhook -f
```

**Verificar permisos:**
```bash
ls -la /opt/hero_budget/backend/restart_services.sh
ls -la /opt/hero_budget/webhook/deploy.sh
```

### Problema: Servicios no se reinician

**Verificar:**
```bash
# Estado de servicios
./tests/endpoints/test_endpoints_final_solution.sh

# Procesos corriendo
ps aux | grep "go run"

# Puertos en uso
netstat -tuln | grep -E ":(808[0-9]|809[0-9])"
```

## 🚀 Testing del Sistema

### Test Manual del Webhook

```bash
# 1. Hacer un cambio en el código
echo "// Test comment" >> /opt/hero_budget/backend/main.go

# 2. Commit y push
git add .
git commit -m "Test deployment"
git push origin main

# 3. Verificar logs
tail -f /opt/hero_budget/webhook/deployment.log

# 4. Verificar servicios
curl http://localhost:8090/health
```

### Test Local del Deployment

```bash
# Ejecutar deploy script manualmente
cd /opt/hero_budget/webhook
./deploy.sh
```

## 📊 Monitoreo de Performance

### Métricas del Sistema

```bash
# Uso de CPU y memoria del webhook
ps aux | grep webhook_server

# Estado de todos los servicios
systemctl --type=service --state=running | grep hero

# Espacio en disco
df -h /opt/hero_budget
```

### Alertas Recomendadas

1. **Webhook Service Down**: Monitor `systemctl is-active herobudget-webhook`
2. **Deployment Failures**: Monitor logs para errores
3. **Disk Space**: Monitor espacio en `/opt/hero_budget`
4. **Service Health**: Monitor endpoints de salud de servicios

## 🔒 Seguridad

### Recomendaciones

1. **Secret Strong**: Usar secret de al menos 40 caracteres
2. **Firewall**: Solo permitir puerto 9000 desde GitHub IPs
3. **Logs**: Rotar logs regularmente
4. **Permisos**: Verificar permisos de archivos críticos

### GitHub IPs (para firewall restrictivo)

```bash
# Permitir solo IPs de GitHub (opcional)
curl -s https://api.github.com/meta | jq -r '.hooks[]' | while read ip; do
  ufw allow from $ip to any port 9000
done
```

## 📋 Comandos Útiles

```bash
# Estado completo del sistema
systemctl status herobudget-webhook
ps aux | grep "go run"
netstat -tuln | grep -E ":(900[0-9]|808[0-9]|809[0-9])"

# Logs combinados
tail -f /opt/hero_budget/webhook/deployment.log & journalctl -u herobudget-webhook -f

# Reinicio completo del sistema
systemctl restart herobudget-webhook
cd /opt/hero_budget/backend && ./restart_services.sh

# Backup manual antes de cambios importantes
cd /opt/hero_budget && git stash push -m "Manual backup $(date)"
```

## 🎯 Flujo de Trabajo Recomendado

1. **Desarrollo Local** → Hacer cambios en código
2. **Commit** → `git commit -m "descripción"`
3. **Push** → `git push origin main`
4. **Automático** → Webhook recibe notificación
5. **Automático** → Deploy script se ejecuta
6. **Automático** → Servicios se reinician
7. **Verificación** → Comprobar que todo funciona

## 📞 Soporte

Si tienes problemas:

1. Revisar logs: `journalctl -u herobudget-webhook -f`
2. Revisar deployment: `tail -f /opt/hero_budget/webhook/deployment.log`
3. Verificar servicios: `./tests/endpoints/test_endpoints_final_solution.sh`
4. Test manual: `./deploy.sh`

---

**Desarrollado para Hero Budget Backend**  
**Automatización de deployment con webhooks de GitHub** 