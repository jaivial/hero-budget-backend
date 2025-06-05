# 🔑 Configuración GitHub - Hero Budget Backend

Esta guía explica cómo configurar el acceso al repositorio GitHub para el despliegue automático.

## 📋 Estado Actual

- **Repositorio**: https://github.com/jaivial/herobudget-backend.git
- **Branch principal**: main
- **VPS Git**: ✅ Configurado
- **Token**: ⚠️ Pendiente configuración

## 🔐 Opciones de Configuración

### Opción A: Repositorio Público (Recomendado para desarrollo)

1. **Ir a GitHub** → Repositorio → Settings → General
2. **Scroll down** hasta "Danger Zone"
3. **Click** en "Change repository visibility"
4. **Seleccionar** "Public"
5. **Confirmar** el cambio

**Ventajas:**
- ✅ No necesita token
- ✅ Despliegue automático funciona inmediatamente
- ✅ Fácil de configurar

### Opción B: Token de Acceso Personal (Para repositorio privado)

#### 1. Generar Token en GitHub

1. **GitHub** → Settings → Developer settings
2. **Personal access tokens** → Tokens (classic)
3. **Generate new token** → Generate new token (classic)
4. **Configurar**:
   - **Note**: "Hero Budget Backend Deploy"
   - **Expiration**: 90 days (o sin expiración)
   - **Scopes**: ✅ `repo` (Full control of private repositories)
5. **Generate token** y **copiar**

#### 2. Configurar en VPS

```bash
# Opción 1: Usando el script automático
./scripts/configure_github_token.sh TU_TOKEN_AQUI

# Opción 2: Manual
ssh root@178.16.130.178
cd /opt/hero_budget/backend
git remote set-url origin https://jaivial:TU_TOKEN@github.com/jaivial/herobudget-backend.git
```

#### 3. Verificar Configuración

```bash
# En el VPS
git ls-remote origin main

# Debería mostrar la información del branch sin errores
```

## 🧪 Probar Despliegue Automático

Una vez configurado GitHub (opción A o B):

```bash
# Desde local
./scripts/deploy_backend.sh --force

# El script debería:
# 1. ✅ Conectar al VPS
# 2. ✅ Crear backup
# 3. ✅ Actualizar código desde GitHub
# 4. ✅ Compilar microservicios
# 5. ✅ Reiniciar servicios
# 6. ✅ Verificar endpoints
```

## 🔍 Troubleshooting

### Error: "Write access to repository not granted"

**Causa**: Token sin permisos o inválido
**Solución**:
1. Verificar que el token tenga scope `repo`
2. Regenerar token si es necesario
3. Verificar que el usuario tenga acceso al repositorio

### Error: "Repository not found"

**Causa**: Repositorio privado sin token válido
**Solución**:
1. Hacer repositorio público (Opción A)
2. Configurar token correctamente (Opción B)

### Error de conexión SSH

**Causa**: SSH no configurado
**Solución**:
```bash
./scripts/setup_ssh.sh
```

## 📝 Scripts Disponibles

| Script | Propósito | Uso |
|--------|-----------|-----|
| `configure_github_token.sh` | Configurar token GitHub | `./scripts/configure_github_token.sh TOKEN` |
| `setup_vps_git.sh` | Configurar Git en VPS | `./scripts/setup_vps_git.sh` |
| `deploy_backend.sh` | Despliegue automático | `./scripts/deploy_backend.sh --force` |

## 🚀 Flujo de Desarrollo Recomendado

### 1. Desarrollo Local
```bash
# Hacer cambios en el código
git add .
git commit -m "feat: nueva funcionalidad"
git push origin main
```

### 2. Despliegue Automático
```bash
# Desplegar cambios al VPS
./scripts/deploy_backend.sh
```

### 3. Verificación
```bash
# Verificar estado del sistema
./scripts/manage_services.sh health
```

## 🔒 Seguridad

### Para Repositorio Público
- ✅ No exponer credenciales en el código
- ✅ Usar variables de entorno para secrets
- ✅ Configurar .gitignore apropiadamente

### Para Repositorio Privado
- ✅ Token con permisos mínimos necesarios
- ✅ Rotar tokens regularmente
- ✅ No compartir tokens en documentación

## 📋 Checklist de Configuración

- [ ] SSH configurado (`./scripts/setup_ssh.sh`)
- [ ] Git configurado en VPS (`./scripts/setup_vps_git.sh`)
- [ ] GitHub configurado (público o con token)
- [ ] Despliegue probado (`./scripts/deploy_backend.sh --force`)
- [ ] Servicios verificados (`./scripts/manage_services.sh health`)

---

**💡 Recomendación**: Para desarrollo inicial, usar repositorio público. Para producción, usar repositorio privado con token de acceso personal. 