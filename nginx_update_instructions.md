# Instrucciones para Actualizar Nginx - Endpoint de Eliminación de Cuenta

## Configuración Necesaria en el VPS

### 1. Conectar al VPS
```bash
ssh root@178.16.130.178
```

### 2. Backup de la configuración actual
```bash
cp /etc/nginx/sites-available/herobudget /etc/nginx/sites-available/herobudget.backup.$(date +%Y%m%d)
```

### 3. Editar configuración de nginx
```bash
nano /etc/nginx/sites-available/herobudget
```

### 4. Agregar las siguientes líneas en la sección del profile_management (puerto 8092)

Buscar la sección que contiene:
```nginx
# Profile Management Service
location ~ ^/(profile|update) {
    proxy_pass http://localhost:8092;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

Y asegurarse de que el endpoint DELETE esté cubierto. Si no lo está, agregar:
```nginx
# Profile Management Service - DELETE account endpoint
location ~ ^/profile/delete-account {
    proxy_pass http://localhost:8092;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    
    # Allow DELETE method
    proxy_method $request_method;
}
```

### 5. Verificar configuración
```bash
nginx -t
```

### 6. Recargar nginx
```bash
systemctl reload nginx
```

### 7. Verificar que el servicio profile_management esté corriendo
```bash
ps aux | grep profile_management
systemctl status herobudget
```

### 8. Probar el nuevo endpoint
```bash
# Test de conectividad (debe retornar 400 con mensaje de error, no 404)
curl -X DELETE https://herobudget.jaimedigitalstudio.com/profile/delete-account \
  -H "Content-Type: application/json" \
  -d '{"user_id":"999999"}'
```

## Resultado Esperado

- El endpoint `/profile/delete-account` debe estar accesible vía HTTPS
- Debe retornar respuesta del backend (no 404 de nginx)
- Los métodos DELETE deben estar permitidos
- El endpoint debe aparecer en los logs del backend cuando se acceda

## Troubleshooting

Si hay problemas:
1. Verificar logs de nginx: `tail -f /var/log/nginx/error.log`
2. Verificar logs del servicio: `journalctl -u herobudget -f`
3. Verificar que el puerto 8092 esté escuchando: `netstat -tlnp | grep 8092`
4. Reiniciar el servicio si es necesario: `systemctl restart herobudget` 