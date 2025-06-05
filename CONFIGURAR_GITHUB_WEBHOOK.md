# 🔧 Configuración del Webhook en GitHub

## ✅ Sistema Completamente Configurado

El servidor webhook está funcionando correctamente en el VPS:
- **URL:** `http://srv736989.hstgr.cloud:9000/webhook`
- **Secret:** `bfcf5f938bd2d3f1e9da684d11f0fd26f3a7be44`
- **Estado:** ✅ Activo y funcionando
- **Repositorio:** `https://github.com/jaivial/herobudget-backend.git`

## 📋 Configurar Webhook en GitHub

### 1. Ir a la configuración del repositorio
1. Ve a: https://github.com/jaivial/herobudget-backend/settings/hooks
2. Haz clic en **"Add webhook"**

### 2. Configurar el webhook con estos datos exactos:

**Payload URL:**
```
http://srv736989.hstgr.cloud:9000/webhook
```

**Content type:**
```
application/json
```

**Secret:**
```
bfcf5f938bd2d3f1e9da684d11f0fd26f3a7be44
```

**Which events would you like to trigger this webhook?**
- ✅ Selecciona **"Just the push event"**
- ✅ Asegúrate de que **"Active"** esté marcado

### 3. Guardar
- Haz clic en **"Add webhook"**

## 🧪 Probar el Sistema

### Opción 1: Test desde GitHub
1. Después de crear el webhook, GitHub mostrará una pestaña **"Recent Deliveries"**
2. Haz clic en **"Redeliver"** en cualquier delivery para probarlo

### Opción 2: Hacer un commit de prueba AL REPOSITORIO CORRECTO
```bash
# IMPORTANTE: Hacer push al repositorio herobudget-backend
git remote add backend https://github.com/jaivial/herobudget-backend.git
echo "// Test auto-deploy $(date)" >> test_webhook.md
git add test_webhook.md
git commit -m "test: Probar deployment automático"
git push backend main
```

## 🔍 Verificar que Funciona

### Ver logs del webhook:
```bash
ssh root@srv736989.hstgr.cloud 'journalctl -u herobudget-webhook -f'
```

### Ver logs del deployment:
```bash
ssh root@srv736989.hstgr.cloud 'tail -f /opt/hero_budget/webhook/deployment.log'
```

### Verificar servicios después del deployment:
```bash
ssh root@srv736989.hstgr.cloud '/opt/hero_budget/backend/restart_services.sh'
```

## 🚀 Flujo Automático Completo

Una vez configurado, el flujo será:

1. **Desarrollador hace push** → GitHub (repositorio herobudget-backend)
2. **GitHub envía webhook** → VPS (puerto 9000)
3. **Webhook server recibe notificación** → Ejecuta deployment
4. **Script de deployment:**
   - Hace `git pull --rebase`
   - Ejecuta `restart_services.sh`
   - Reinicia todos los microservicios
5. **Servicios actualizados** → Sistema funcionando con nuevo código

## 📋 Comandos Útiles

**Ver estado del webhook:**
```bash
ssh root@srv736989.hstgr.cloud 'systemctl status herobudget-webhook'
```

**Reiniciar webhook si es necesario:**
```bash
ssh root@srv736989.hstgr.cloud 'systemctl restart herobudget-webhook'
```

**Probar webhook manualmente:**
```bash
curl http://srv736989.hstgr.cloud:9000/health
```

**Ver logs en tiempo real:**
```bash
ssh root@srv736989.hstgr.cloud 'journalctl -u herobudget-webhook -f'
```

## ⚠️ IMPORTANTE

**Para que el webhook funcione, debes hacer push al repositorio `herobudget-backend`:**

```bash
# Añadir el repositorio como remote
git remote add backend https://github.com/jaivial/herobudget-backend.git

# Hacer push para activar el webhook
git push backend main
```

## 🎯 ¡Listo!

El sistema de deployment automático está completamente configurado y funcionando. Cada push a `https://github.com/jaivial/herobudget-backend.git` rama `main` desencadenará automáticamente:

✅ Pull del código más reciente  
✅ Restart de todos los servicios  
✅ Logs detallados del proceso  
✅ Manejo de errores robusto

**Próximo paso:** Configurar el webhook en GitHub con la información proporcionada arriba. 