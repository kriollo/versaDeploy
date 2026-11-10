# Primeros pasos

De un VPS recién creado a tu app en línea. Si prefieres verlo, cada paso está en los [videos](videos.md).

!!! tip "Lo que necesitas"
    - Un **VPS** con Ubuntu, Debian, Rocky, Alma o RHEL, y acceso **root por SSH** (lo da cualquier proveedor: Hetzner, DigitalOcean, Vultr, OVH…).
    - Tu proyecto en **git**, en tu computador, con lo que use para compilar: `composer`, `npm`, `go`… (Python se instala en el servidor).
    - Opcional: un **dominio** apuntando a la IP del VPS, para tener HTTPS con Let's Encrypt.

## 1. Instala versa

=== "Linux y macOS"

    ```bash
    curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh
    versa version
    ```

=== "Windows"

    Descarga `versa_windows_amd64.exe` desde [Releases](https://github.com/kriollo/versaDeploy/releases/latest), renómbralo a `versa.exe` y ponlo en una carpeta de tu `PATH`. Para los hooks locales instala Git for Windows. Detalles en [Instalación](reference/INSTALL.md).

## 2. Tu llave SSH en el servidor

Si aún no tienes una llave SSH, créala y cópiala al servidor (una sola vez):

```bash
ssh-keygen -t ed25519                 # Enter a todo
ssh-copy-id root@IP_DE_TU_VPS         # pide la contraseña de root por última vez
ssh root@IP_DE_TU_VPS 'echo ok'       # debe entrar sin contraseña
```

## 3. El archivo `deploy.yml`

En la carpeta de tu proyecto, parte del ejemplo de tu framework:

```bash
curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.laravel.yml
```

Hay ejemplos para Laravel, Next.js, Nuxt, FastAPI, Flask y Gin en [Ejemplos](examples.md). Ábrelo y cambia lo marcado con `# change`:

- `project`: el nombre de tu proyecto.
- `ssh.host`: la IP o el dominio de tu VPS.
- `provision.domain`: el dominio de tu app (o déjalo y usa la IP mientras tanto).
- `remote_path`, `db_name`, `db_user`: normalmente basta con cambiar `myapp` por tu nombre.

Las contraseñas no van en el archivo: `db_password: "${DB_PASS}"` se lee de una variable.

```bash
export DB_PASS='una-contraseña-segura'
versa config validate production
git add deploy.yml && git commit -m "Configura versaDeploy"
```

!!! info "versa despliega lo commiteado"
    versa toma el último commit, no tus archivos sin guardar: haz commit antes de cada deploy.

## 4. Prepara el servidor

```bash
versa
```

Se abre la interfaz. Con **→** ve a la vista **Services**: versa detecta el servidor y marca lo que tu app necesita (nginx, PHP, la base de datos, el usuario `deploy` con tu llave, backups, fail2ban…). Revisa, y presiona **I** y luego **Y** para instalar. Toma unos minutos. Sal con **q**.

Sin la interfaz (en un script o en CI) es lo mismo: `versa provision production` muestra lo que va a instalar y pide confirmación (`--dry-run` solo lo muestra).

Si el servidor ya tiene cosas instaladas, versa las respeta: no reemplaza nginx por Apache, no instala un segundo PHP, no reinicia servicios ni cambia contraseñas existentes.

## 5. Primer deploy

```bash
versa deploy production --initial-deploy
```

versa compila en tu computador, sube el proyecto y lo activa. En el primer deploy pregunta si correr los hooks `post_deploy` (las migraciones): responde **N**, porque falta el `.env`.

## 6. El `.env` de producción

El primer deploy creó el `.env` del servidor a partir de tu `.env.example`. Edítalo:

```bash
versa
```

Vista **Shared** → `.env` → **Enter** → **e** para editar → **Ctrl+S** para guardar. Pon los datos de producción (en Laravel: `APP_ENV=production`, `APP_DEBUG=false`, `APP_URL`, `DB_*`…). Si tu framework necesita una clave (Laravel: `APP_KEY`), puedes generarla en el servidor:

```bash
versa exec production 'cd /var/www/miapp/current/app && php artisan key:generate --force'
```

## 7. Deploy

```bash
versa deploy production --force
```

Ahora sí corren las migraciones y los caches, y el health check confirma que la app responde. ¡Listo!

## Desde ahora

```bash
git commit -am "Mi cambio"
versa deploy production     # solo sube lo que cambió
versa history production    # quién desplegó qué y cuándo
versa rollback production   # volver a la versión anterior, al instante
```

Siguientes pasos:

- **HTTPS**: con un dominio apuntando al VPS, marca **SSL** en la vista Services.
- **Avisos** a Slack, Discord o Telegram, **staging**, **tags** y **CI**: [Configuración](reference/DEPLOY.md).
- Cómo funciona todo por dentro: [Flujos](flows.md). ¿Algo falló? [Problemas comunes](reference/TROUBLESHOOTING.md).
