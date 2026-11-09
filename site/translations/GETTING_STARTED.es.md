# 🚀 Guía completa { #getting-started }

De cero a tu primer deploy, paso a paso. El ejemplo despliega una app PHP (Laravel) en un servidor Ubuntu con Nginx/Apache + PHP-FPM; otros stacks siguen los mismos pasos con otra sección `builds` (mira la [Configuración](DEPLOY.md#builds-builds)).

1. [Requisitos](#1-prerequisites)
2. [Preparar el servidor](#2-prepare-the-server)
3. [Crear la configuración](#3-create-the-configuration)
4. [Revisar todo antes de desplegar](#4-check-everything-before-deploying)
5. [Primer deploy](#5-first-deploy)
6. [El día a día](#6-everyday-use)
7. [Apps Go o Python: correrlas como servicio](#7-go-or-python-apps-run-them-as-a-service)

---

## 1. Requisitos { #1-prerequisites }

### En tu computador (donde corres `versa`) { #on-your-machine-where-you-run-versa }

- **versa** instalado: mira [Instalación](INSTALL.md).
- **Git**: versa despliega el último commit del repositorio desde donde lo corres.
- **Las herramientas de build de tu proyecto**, en el `PATH`: `composer` para PHP, `npm`/`pnpm`/`yarn` para el frontend, `go` para Go. Los builds corren en local, no en el servidor. (Las dependencias de Python son la excepción: se instalan en el servidor.)
- **Windows**: los hooks locales (`pre_deploy_local`) usan `sh`. Instala Git for Windows y agrega `C:\Program Files\Git\bin` al `PATH`.
- Una **llave SSH** que pueda entrar al servidor.

### En el servidor { #on-the-server }

- Linux con SSH y SFTP. Las distribuciones antiguas (por ejemplo CentOS 5) también funcionan, mira [`legacy_algorithms`](DEPLOY.md#ssh-connection-ssh).
- `tar` y `gzip` (vienen en prácticamente todas las distribuciones).
- Un usuario que pueda escribir en el directorio del deploy.

## 2. Preparar el servidor { #2-prepare-the-server }

> [!TIP]
> **Deja que versa lo haga.** Con acceso root por SSH, la vista **Services** de la TUI hace toda esta sección por ti: usuario de deploy con tu llave, directorio, sudoers, vhost del web server, PHP, base de datos, SSL y firewall. Solo `P` prepara un servidor nuevo para recibir deploys. Mira **[Servicios del servidor](SERVICES.md)**. Los pasos de abajo son para hacerlo a mano.

### 2.1 Usuario de deploy y llave SSH { #21-deploy-user-and-ssh-key }

Desde tu computador, copia tu llave pública al servidor y acepta su llave de host una vez:

```bash
ssh-copy-id deploy@your-server.com        # o agrega el .pub a ~/.ssh/authorized_keys en el servidor
ssh deploy@your-server.com                # responde "yes" para guardar el servidor en ~/.ssh/known_hosts
```

> [!TIP]
> Conectarte una vez con `ssh` guarda el servidor en `~/.ssh/known_hosts`, que versa usa para verificar el servidor. Sin eso versa igual se conecta, pero avisa `Host key NOT verified`.

### 2.2 Directorio del deploy { #22-deploy-directory }

```bash
sudo mkdir -p /var/www/my-app
sudo chown -R deploy:www-data /var/www/my-app
```

versa crea adentro `releases/`, `shared/` y el enlace `current`.

### 2.3 Archivos compartidos { #23-shared-files }

Los archivos que deben sobrevivir entre releases (como `.env`) viven en `shared/`. No necesitas crearlos: el primer deploy crea cada uno que falte, una sola vez, a partir del release (por ejemplo el `storage/` de Laravel), si no desde `<ruta>.example` (`.env.example`), y si no un `.env` vacío (permisos 0640). Después editas el `.env` en el servidor, por ejemplo desde la vista **Shared** de la TUI. Si quieres tenerlo listo antes del primer deploy, créalo a mano:

```bash
mkdir -p /var/www/my-app/shared
nano /var/www/my-app/shared/.env
```

### 2.4 Web server { #24-web-server }

Cada release se guarda como `releases/<versión>/app/`, y `current` apunta al release activo. Por eso tu web root es **`/var/www/my-app/current/app/<carpeta pública>`**.

**Nginx**: usa `$realpath_root` para que PHP resuelva la ruta real del release, no el enlace:

```nginx
server {
    root /var/www/my-app/current/app/public;

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_pass unix:/run/php/php8.2-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT $realpath_root;
    }
}
```

**Apache**:

```apache
<VirtualHost *:80>
    DocumentRoot /var/www/my-app/current/app/public

    <Directory /var/www/my-app/current/app/public>
        Options +FollowSymLinks
        AllowOverride All
        Require all granted
    </Directory>

    # No servir archivos estáticos viejos desde la caché del kernel tras un cambio
    EnableSendfile Off
    EnableMMAP Off

    <FilesMatch \.php$>
        SetHandler "proxy:unix:/run/php/php8.2-fpm.sock|fcgi://localhost"
    </FilesMatch>
</VirtualHost>
```

### 2.5 Recargar servicios sin contraseña { #25-allow-reloading-services-without-a-password }

PHP-FPM guarda en caché a dónde apunta `current`, así que hay que recargarlo después de cada deploy (versa lo hace con [`services_reload`](DEPLOY.md#service-reload-services_reload)). Permite que el usuario de deploy lo haga sin contraseña, con `sudo visudo -f /etc/sudoers.d/deploy`:

```
deploy ALL=(ALL) NOPASSWD: /bin/systemctl reload php8.2-fpm
deploy ALL=(ALL) NOPASSWD: /bin/systemctl reload nginx
```

## 3. Crear la configuración { #3-create-the-configuration }

En la raíz de tu repositorio:

```bash
versa init
```

Elige `1` (servidor con SSH). Esto crea un `deploy.yml` comentado. Edítalo; una configuración mínima de Laravel se ve así (también puedes partir de los [Ejemplos](../examples.md)):

```yaml
project: "my-app"

environments:
  production:
    ssh:
      host: "your-server.com"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"

    remote_path: "/var/www/my-app"

    builds:
      php:
        enabled: true
      frontend:
        enabled: true
        npm_command: "npm ci"
        compile_command: "npm run build"

    shared_paths:
      - ".env"
      - "storage"

    services_reload:
      - "sudo systemctl reload php8.2-fpm"

    post_deploy:
      - "php artisan migrate --force"
      - "php artisan config:cache"
```

Haz commit de `deploy.yml` (y de todo lo que quieras desplegar): versa solo despliega archivos commiteados.

## 4. Revisar todo antes de desplegar { #4-check-everything-before-deploying }

```bash
versa config validate production    # el archivo es válido (sin conectarse)
versa info production               # SSH, llave de host, SFTP y datos del servidor
```

Corrige lo que reporten antes de seguir; [Problemas comunes](TROUBLESHOOTING.md) cubre los errores habituales.

## 5. Primer deploy { #5-first-deploy }

El servidor todavía no tiene `deploy.lock`, así que dile a versa que es el primer deploy:

```bash
versa deploy production --initial-deploy
```

versa compila el proyecto, lo sube, enlaza los `shared_paths` (creando `shared/.env` desde `.env.example` si no existe), cambia `current` y recarga PHP-FPM. Antes de correr los hooks `post_deploy` pide confirmación:

- `shared/.env` ya tiene los datos de producción → responde `y`.
- Se acaba de crear desde `.env.example` → responde `N`, edítalo (`versa` → **Shared** → `.env` → `Enter` → `e` → `Ctrl+S`) y luego corre `versa deploy production --force` para que corran las migraciones.

## 6. El día a día { #6-everyday-use }

```bash
git commit -am "Nueva funcionalidad"
versa deploy production             # compila y sube solo si algo cambió
```

| Quiero…                                     | Comando                                     |
| :------------------------------------------ | :------------------------------------------ |
| Ver qué se desplegaría                      | `versa diff production`                     |
| Ver los releases en el servidor             | `versa status production`                   |
| Ver quién desplegó qué, y el log de cada deploy | `versa history production`              |
| Volver al release anterior                  | `versa rollback production`                 |
| Volver a un release específico              | `versa rollback production --to 20261001-183807` |
| Volver a desplegar aunque nada cambió       | `versa deploy production --force`           |
| Seguir el log de la app                     | `versa logs production`                     |
| Correr un comando en el servidor            | `versa exec production "php artisan queue:restart"` |
| Ver / reiniciar un servicio Go o Python     | `versa service production [restart]`        |
| Hacer todo esto desde una interfaz          | `versa`                                     |

## 7. Apps Go o Python: correrlas como servicio { #7-go-or-python-apps-run-them-as-a-service }

A una app PHP la sirve PHP-FPM, pero un binario Go o un servidor Python es un proceso que tiene que estar corriendo. Decláralo en `services` y versa se encarga del resto: lo instala en el sistema de init del servidor (systemd, o `/etc/init.d` en servidores antiguos), lo arranca al iniciar el servidor, lo reinicia en cada deploy y rollback, y revierte el deploy si no se mantiene arriba.

**API en Go:**

```yaml
environments:
  production:
    # ssh, remote_path …
    builds:
      go:
        enabled: true
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "api"
        deploy_path: "bin"
    shared_paths:
      - ".env"
    services:
      - name: "myapi"
        env_file: ".env"            # /var/www/my-app/shared/.env
        environment:
          PORT: "8080"
```

**API en Python (FastAPI):** las dependencias se instalan en el servidor, en un virtualenv por release. El servidor necesita `python3` y `python3-venv`.

```yaml
    builds:
      python:
        enabled: true
        web_server: true
        web_framework: "fastapi"
        entry_point: "main.py"      # main.py define `app`
        web_port: 8000
    services:
      - name: "myapi"
        env_file: ".env"
```

Después:

```bash
versa service production setup | ssh root@203.0.113.10 sh   # una vez, como root: instala los services + sudoers
versa info production               # confirma que sudo está bien
versa deploy production
versa service production            # ¿está corriendo?
versa service production logs       # seguir su salida
```

Pon un reverse proxy (Nginx) delante del puerto si el servicio debe ser accesible desde afuera. Todas las opciones: [Services](DEPLOY.md#services-services).

### Siguientes pasos { #next-steps }

- [Comandos (CLI y TUI)](CLI_REFERENCE.md): cada comando y opción.
- [Configuración](DEPLOY.md): hooks, health checks, notificaciones, subidas incrementales, modo local.
