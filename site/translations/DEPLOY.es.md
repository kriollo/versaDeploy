# ⚙️ Configuración (`deploy.yml`) { #configuration-reference-deployyml }

Cada opción del archivo de configuración de versaDeploy, con sus valores por defecto y su comportamiento. Para un ejemplo comentado mira [deploy.example.yml](../deploy.example.yml), y por framework (Laravel con PHP-FPM o FrankenPHP, Next.js, Nuxt, Flask, FastAPI, Gin) [examples/](../examples/); para generar un archivo inicial corre `versa init`. Revisa un archivo cuando quieras con `versa config validate [entorno]`.

- [Estructura del archivo](#file-structure)
- [Variables de entorno en la configuración](#environment-variables-in-the-config)
- [Conexión SSH (`ssh`)](#ssh-connection-ssh)
- [Opciones del entorno](#environment-settings)
- [Builds (`builds`)](#builds-builds): [PHP](#php-php) · [Go](#go-go) · [Frontend](#frontend-frontend) · [Python](#python-python)
- [Archivos entre releases: compartidos, preservados, reutilizables](#files-between-releases-shared-preserved-reusable) · [`.env` y secretos](#environment-variables-and-secrets-env-configjson)
- [Hooks](#hooks)
- [Recarga de servicios (`services_reload`)](#service-reload-services_reload)
- [Services (`services`)](#services-services): correr un proceso Go/Python al iniciar el servidor y reiniciarlo en cada deploy
- [Health check (`health_check`)](#health-check-health_check)
- [Notificaciones (`notifications`)](#notifications-notifications)
- [Historial de deploys](#deploy-history) · [Producción desde tags (`deploy_tags`)](#production-from-tags-deploy_tags)
- [Rendimiento y límites](#performance-and-limits)
- [Modo local (sin SSH)](#local-mode-no-ssh)
- [Estructura de carpetas en el servidor](#server-directory-layout)
- [Opciones obsoletas](#deprecated-options)

---

## Estructura del archivo { #file-structure }

```yaml
project: "my-app"           # obligatorio

environments:
  production:               # el nombre que pasas a `versa deploy production`
    ssh: { ... }
    remote_path: "/var/www/my-app"
    builds: { ... }         # al menos un build activo
    shared_paths: [ ... ]
    post_deploy: [ ... ]
    # ...
  staging:
    # ...
```

| Campo          | Tipo   | Descripción                                                          |
| :------------- | :----- | :------------------------------------------------------------------- |
| `project`      | string | **Obligatorio.** Nombre del proyecto, usado en logs y notificaciones. |
| `environments` | map    | **Obligatorio.** Al menos un entorno. Cada clave es el nombre de un entorno. |

Todas las opciones de abajo van **dentro de un entorno**, con un nivel más de sangría que el nombre del entorno. Los entornos no heredan opciones entre sí.

Puedes tener varios archivos de configuración en un repositorio (por ejemplo uno por servidor: `deploy_server1.yml`, `deploy_server2.yml`); mira [Elegir el archivo de configuración](CLI_REFERENCE.md#choosing-the-configuration-file).

## Variables de entorno en la configuración { #environment-variables-in-the-config }

Antes de leer el archivo, `${NOMBRE}` y `$NOMBRE` se reemplazan por el valor de la variable de entorno en **tu** computador (vacío si no existe). Úsalo para dejar los secretos fuera del archivo:

```yaml
notifications:
  webhook_url: "${SLACK_WEBHOOK}"
```

Esto aplica a **todo el archivo, incluidos los hooks**. Para pasar una variable al shell del servidor, escribe `$$`, que queda como un `$` literal:

```yaml
post_deploy:
  - "echo $$HOME"                  # el servidor corre: echo $HOME
  - "cp .env.example $${APP_DIR}"  # el servidor corre: cp .env.example ${APP_DIR}
  - "awk '{print $1}' file"        # sin cambios: $1, $@, $?, $# … nunca se reemplazan
```

Un `$nombre` que no existe en tu computador muestra una advertencia al cargar el archivo (`$x is not set here, so it becomes empty`): casi siempre es una variable del servidor que necesita `$$`, por ejemplo `php -r '$$c = 1;'`.

Solo se reemplazan los nombres que empiezan con una letra o `_`, así que las variables posicionales y especiales del shell (`$1`, `$@`, `$?`, `$#`) pasan intactas.

---

## Conexión SSH (`ssh`) { #ssh-connection-ssh }

Obligatoria, salvo que el entorno use el [modo local](#local-mode-no-ssh).

| Campo               | Tipo   | Por defecto          | Descripción                                                                                                       |
| :------------------ | :----- | :------------------- | :---------------------------------------------------------------------------------------------------------------- |
| `host`              | string | —                    | **Obligatorio.** Nombre o IP del servidor.                                                                        |
| `user`              | string | —                    | **Obligatorio.** Usuario SSH. Necesita permiso de escritura en `remote_path`.                                     |
| `key_path`          | string | —                    | **Obligatorio.** Llave privada. `~/` se expande. En Linux/macOS la llave debe tener `chmod 600`.                  |
| `port`              | int    | `22`                 | Puerto SSH.                                                                                                       |
| `use_ssh_agent`     | bool   | `false`              | Probar también las llaves cargadas en tu agente SSH.                                                              |
| `known_hosts_file`  | string | `~/.ssh/known_hosts` | Archivo para verificar la llave de host del servidor. `~/` se expande.                                            |
| `strict_host_key`   | bool   | `false`              | No conectarse si la llave de host no se puede verificar (`known_hosts` falta o no se puede leer). Mira abajo.      |
| `legacy_algorithms` | bool   | `false`              | Activa intercambios de llaves SHA1, cifrados CBC y llaves de host `ssh-dss` para servidores muy antiguos (OpenSSH < 7, por ejemplo CentOS 5). Inseguro: solo cuando haga falta. |

Las rutas pueden ser estilo Unix (`~/.ssh/id_rsa`) o Windows (`C:\Users\me\.ssh\id_rsa`).

### Verificación de la llave de host { #host-key-verification }

versa verifica la identidad del servidor contra `known_hosts`, igual que `ssh`:

| Situación                                          | Resultado                                                                            |
| :------------------------------------------------- | :----------------------------------------------------------------------------------- |
| El servidor está en `known_hosts` y la llave coincide | Se conecta.                                                                       |
| El servidor está en `known_hosts` con otra llave   | **Se niega** (`key mismatch`): posible ataque man-in-the-middle, o el servidor se reinstaló. |
| `known_hosts` falta o no se puede leer             | Se conecta con la advertencia `Host key NOT verified`, o se niega si `strict_host_key: true`. |

La forma más fácil de agregar un servidor es conectarte una vez con el cliente `ssh` normal y aceptar la llave, o:

```bash
ssh-keyscan -p 22 your-server.com >> ~/.ssh/known_hosts
```

---

## Opciones del entorno { #environment-settings }

| Campo                | Tipo         | Por defecto                                        | Descripción                                                                                                                 |
| :------------------- | :----------- | :------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------------- |
| `remote_path`        | string       | —                                                  | **Obligatorio** (modo SSH). Ruta absoluta en el servidor; ahí viven los releases, `current`, `shared/` y `deploy.lock`.     |
| `ignored_paths`      | list         | `.git`, `tests`, `node_modules/.cache`, `vendor/bin` | Rutas del repositorio excluidas de la detección de cambios y **eliminadas del release después de compilar** (los builds sí las ven). Definir esta lista reemplaza los valores por defecto. |
| `shared_paths`       | list         | `[]`                                               | Rutas guardadas en `<remote_path>/shared` y enlazadas en cada release. Mira [abajo](#files-between-releases-shared-preserved-reusable). |
| `preserved_paths`    | list         | `[]`                                               | Rutas copiadas desde el release anterior sobre el nuevo (gana la copia del servidor).                                       |
| `pre_deploy_local`   | list         | `[]`                                               | [Hooks](#hooks) que corren en tu computador antes de compilar.                                                              |
| `pre_deploy_server`  | list         | `[]`                                               | [Hooks](#hooks) que corren en el servidor antes de activar el release.                                                      |
| `post_deploy`        | list         | `[]`                                               | [Hooks](#hooks) que corren en el servidor después de activar el release.                                                    |
| `on_failure`         | list         | `[]`                                               | [Hooks](#hooks) que corren en el servidor después de un deploy fallido (por ejemplo `php artisan up`).                      |
| `deploy_tags`        | string       | —                                                  | Solo desplegar commits con un tag de git que coincida con este glob, por ejemplo `"v*"`. Mira [abajo](#production-from-tags-deploy_tags). |
| `services_reload`    | list         | `[]`                                               | Comandos que corren en el servidor justo después de la activación, mira [abajo](#service-reload-services_reload).          |
| `health_check`       | map          | —                                                  | Mira [Health check](#health-check-health_check).                                                                            |
| `notifications`      | map          | —                                                  | Mira [Notificaciones](#notifications-notifications).                                                                        |
| `route_files`        | list         | `[]`                                               | Archivos cuyo cambio marca `route_cache_regenerate: true` en el `manifest.json` del release.                                |
| `hook_timeout`       | int (s)      | `300`                                              | Límite de cada hook remoto, y de `versa exec`.                                                                              |
| `deploy_timeout`     | int (s)      | `600`                                              | Límite del deploy completo, revisado entre fases.                                                                           |
| `releases_to_keep`   | int          | `5`                                                | Releases que se mantienen en el servidor; los más antiguos se borran después de cada deploy. También es el límite de cuán atrás puedes volver. |
| `upload_workers`     | int          | `4`                                                | Subidas SFTP en paralelo.                                                                                                   |
| `chunk_size_mb`      | int          | `10`                                               | Tamaño de cada parte del archivo que se sube.                                                                               |
| `incremental_upload` | bool         | `false`                                            | Subir solo los archivos que cambiaron; mira [Rendimiento](#performance-and-limits).                                         |
| `services`           | list         | `[]`                                               | Procesos que corre el sistema de init del servidor, mira [Services](#services-services).                                    |
| `local`, `local_path`| bool, string | `false`, —                                         | [Modo local](#local-mode-no-ssh).                                                                                           |

---

## Builds (`builds`) { #builds-builds }

Debe haber al menos un build activo. Todos los builds activos corren en paralelo en tu computador, dentro de una copia del repositorio (`HEAD`), así que las herramientas necesarias (`composer`, `go`, `npm`/`pnpm`, `python3` …) deben estar instaladas en local. Cada builder corre solo cuando cambiaron archivos que le importan (o con `--force`).

Las rutas de `root` son relativas a la raíz del repositorio.

### PHP (`php`) { #php-php }

| Campo              | Tipo   | Por defecto                                                              | Descripción                                                                 |
| :----------------- | :----- | :----------------------------------------------------------------------- | :-------------------------------------------------------------------------- |
| `enabled`          | bool   | `false`                                                                  | Activa el builder de PHP.                                                   |
| `root`             | string | `""`                                                                     | Carpeta con el `composer.json`.                                             |
| `composer_command` | string | `composer install --no-dev --optimize-autoloader --classmap-authoritative` | Comando de instalación.                                                   |
| `reusable_paths`   | list   | `vendor`                                                                 | Se reutilizan desde el release anterior cuando `composer.json`/`composer.lock` no cambiaron. `vendor` siempre se incluye. |

Las dependencias se reinstalan cuando cambia `composer.json` o `composer.lock`.

### Go (`go`) { #go-go }

| Campo         | Tipo   | Por defecto | Descripción                                                                                         |
| :------------ | :----- | :---------- | :-------------------------------------------------------------------------------------------------- |
| `enabled`     | bool   | `false`     | Activa el builder de Go.                                                                            |
| `root`        | string | `""`        | Carpeta con el `go.mod`.                                                                            |
| `target_os`   | string | —           | **Obligatorio.** Por ejemplo `linux`.                                                               |
| `target_arch` | string | —           | **Obligatorio.** Por ejemplo `amd64`, `arm64`, `386`.                                               |
| `binary_name` | string | —           | **Obligatorio.** Nombre del archivo generado.                                                       |
| `deploy_path` | string | `bin`       | Carpeta del binario **relativa a la raíz del release** (no a `app/`): el binario queda en `<remote_path>/current/<deploy_path>/<binary_name>`. Usa rutas distintas para varios servicios. |
| `build_flags` | string | `""`        | Opciones extra de `go build`.                                                                       |
| `cgo`         | bool   | `false`     | Compilar con `CGO_ENABLED=1`. Por defecto los binarios son estáticos (`CGO_ENABLED=0`).             |

Para correr el binario como un servicio que arranca con el servidor y se reinicia en cada deploy, agrégalo a [`services`](#services-services).

Antes de compilar, versa compara el target con el servidor (`uname`) y con el kernel mínimo que soporta tu versión de Go, y se detiene antes si el binario no podría correr ahí. Go se recompila solo cuando cambian archivos `.go`, `go.mod` o `go.sum`.

### Frontend (`frontend`) { #frontend-frontend }

| Campo                | Tipo   | Por defecto                   | Descripción                                                                                                       |
| :------------------- | :----- | :---------------------------- | :---------------------------------------------------------------------------------------------------------------- |
| `enabled`            | bool   | `false`                       | Activa el builder de frontend.                                                                                    |
| `root`               | string | `""`                          | Carpeta con el `package.json`.                                                                                    |
| `npm_command`        | string | `npm ci --only=production`    | Comando de instalación. Corre cuando cambió `package.json` o su lockfile (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`), o cuando falta `node_modules`. Para compilar normalmente necesitas las dependencias de desarrollo: `npm ci`, `pnpm install`. |
| `compile_command`    | string | —                             | **Obligatorio.** Comando de build, por ejemplo `npm run build`. Si contiene `{file}`, corre una vez por cada archivo de frontend que cambió, con `{file}` reemplazado. |
| `cleanup_dev_deps`   | bool   | `false`                       | Después de compilar, borra `node_modules` y corre `production_command`.                                          |
| `production_command` | string | `pnpm install --production`   | Instalación solo de producción que usa `cleanup_dev_deps`.                                                        |
| `reusable_paths`     | list   | `node_modules`                | Se reutilizan desde el release anterior cuando `package.json` y su lockfile no cambiaron. `node_modules` siempre se incluye. |
| `server_install`     | string | —                             | Instala `node_modules` **en el servidor** con este comando (por ejemplo `npm ci --omit=dev`) en vez de enviar el tuyo: los módulos nativos (sharp, bcrypt, better-sqlite3…) compilados en Windows o macOS no corren en Linux. Corre antes del cambio de release (si falla, el deploy se detiene); se reutiliza cuando el lockfile no cambió. Envía el lockfile (no lo ignores) e instala Node en el servidor (Services → Node.js). Reemplaza a `cleanup_dev_deps`. |

#### Apps SSR (Next.js, Nuxt, SvelteKit, Remix, Astro…) { #ssr-apps-nextjs-nuxt-sveltekit-remix-astro }

Se compilan en tu computador como cualquier frontend y corren en el servidor como una entrada de [`services`](#services-services) detrás del web server ([`provision.proxy_port`](SERVICES.md#9-provision-reference)). nginx limita la tasa de peticiones a las páginas de la app, nunca a sus archivos estáticos.

```yaml
builds:
  frontend:
    enabled: true
    npm_command: "npm ci"
    compile_command: "npm run build"
    server_install: "npm ci --omit=dev"     # Next: node_modules para el sistema del servidor
services:
  - name: myshop
    exec: "npm start"                       # next start / node .output/server/index.mjs (Nuxt)
    environment: { PORT: "3000", NODE_ENV: production }
provision:
  proxy_port: 3000
  node_major: "22"
```

- **Nuxt / Nitro, SvelteKit (adapter-node), Astro (adaptador node)** incluyen sus dependencias en el resultado del build: omite `server_install` y deja fuera `node_modules`, por ejemplo `compile_command: "npm run build && rm -rf node_modules"`, `exec: "node .output/server/index.mjs"`.
- **Next.js** necesita `node_modules` (o `output: "standalone"` con `exec: "node .next/standalone/server.js"`, copiando además `public/` y `.next/static` dentro).
- Las variables de build (`NEXT_PUBLIC_*`, `NUXT_PUBLIC_*`) vienen de tu computador; las de ejecución, de `environment` o `env_file`.
- Cada deploy reinicia el servicio: unos segundos sin servicio, igual que los servicios Go y Python.

Probado de punta a punta con Next.js 16 y Nuxt 4 en Ubuntu 24.04 y AlmaLinux 9.

### Python (`python`) { #python-python }

Las dependencias de Python se instalan **en el servidor**, en un virtualenv dentro de cada release (`<release>/app/<root>/<venv_path>`). Un virtualenv creado en tu computador no funcionaría ahí (otro sistema, otras rutas y paquetes compilados). Cuando los requirements no cambiaron, se reutiliza el virtualenv del release anterior (con hardlinks) en vez de reinstalar.

Por eso el servidor necesita Python 3 con el módulo `venv` (Debian/Ubuntu: `apt install python3-venv`), además de `poetry` o `pipenv` si los usas. Tu computador no necesita Python, salvo para `build_binary`.

| Campo                    | Tipo   | Por defecto        | Descripción                                                                         |
| :----------------------- | :----- | :----------------- | :---------------------------------------------------------------------------------- |
| `enabled`                | bool   | `false`            | Activa el builder de Python.                                                        |
| `root`                   | string | `""`               | Carpeta del proyecto Python.                                                        |
| `python_command`         | string | `python3`          | Python **del servidor** con que se crea el virtualenv.                              |
| `package_manager`        | string | `pip`              | `pip`, `poetry` o `pipenv` (los dos últimos deben estar instalados en el servidor). |
| `requirements_file`      | string | `requirements.txt` | Las dependencias se reinstalan cuando cambia este archivo, `pyproject.toml`, `poetry.lock`, `Pipfile` o `Pipfile.lock`. |
| `extra_requirements`     | list   | `[]`               | Archivos de requirements adicionales (pip).                                         |
| `venv_path`              | string | `.venv`            | Ruta del virtualenv, relativa a `root`.                                             |
| `reusable_paths`         | list   | `[venv_path]`      | Se reutilizan desde el release anterior cuando los requirements no cambiaron.       |
| `install_dev_deps`       | bool   | `false`            | Instalar también las dependencias de desarrollo (poetry/pipenv).                    |
| `use_cache`              | bool   | `false`            | Permitir que pip use su caché (si no, `--no-cache-dir`).                            |
| `pypi_mirror`            | string | `""`               | URL de un índice de paquetes propio.                                                |
| `torch_index`            | string | `""`               | Índice de PyTorch, por ejemplo `https://download.pytorch.org/whl/cpu`.              |
| `web_server`             | bool   | `false`            | Genera `run_server.sh` (mira abajo).                                                |
| `web_framework`          | string | —                  | `fastapi`/`uvicorn`, `gunicorn`, `django`, `flask`.                                 |
| `entry_point`            | string | —                  | Archivo del módulo de la app (por ejemplo `main.py`, `api/main.py`). Obligatorio con `build_binary`. |
| `web_host` / `web_port`  | string / int | `0.0.0.0` / `8000` | Dirección donde escucha.                                                      |
| `web_workers`            | int    | —                  | Procesos worker (uvicorn, gunicorn; Django usa 2 por defecto, gunicorn 4).          |
| `web_threads`            | int    | —                  | Threads por worker (gunicorn).                                                      |
| `run_command`            | string | —                  | Reemplaza el comando de inicio generado.                                            |
| `build_binary`           | bool   | `false`            | Genera un ejecutable independiente con PyInstaller (requiere `entry_point` y `binary_name`). **Solo cuando versa corre en Linux**: PyInstaller no hace compilación cruzada. |
| `binary_name`            | string | —                  | Nombre del archivo de PyInstaller.                                                  |
| `extra_pyinstaller_args` | string | `""`               | Opciones extra de PyInstaller.                                                      |
| `service_name`           | string | —                  | **Obsoleto**: se convierte en `services: [{name: ...}]` automáticamente. Usa [`services`](#services-services). |
| `stop_command`, `websocket`, `ws_protocol`, `ws_channel_layer`, `source_path`, `deploy_path` | | | Se aceptan pero no se usan. |

#### `run_server.sh` { #run_serversh }

Con `web_server: true`, versa escribe `run_server.sh` en la raíz Python de cada release. El script entra a su propia carpeta, usa el virtualenv del release y hace `exec` del servidor, para que el sistema de init siga al proceso correcto:

| `web_framework`         | Comando                                                                       |
| :---------------------- | :---------------------------------------------------------------------------- |
| `fastapi` / `uvicorn`   | `.venv/bin/python -m uvicorn <entry>:app --host H --port P [--workers N]`      |
| `gunicorn`              | `.venv/bin/python -m gunicorn <entry>:app -w N -b H:P [--threads T]`           |
| `django`                | `.venv/bin/python -m gunicorn <project>.wsgi:application -w N -b H:P` (busca `<project>/wsgi.py`; si no, usa `manage.py runserver` con una advertencia) |
| `flask`                 | `.venv/bin/python -m flask run --host H --port P` (servidor de desarrollo: en producción usa `run_command` con gunicorn) |
| ninguno, con `entry_point` | `.venv/bin/python <entry_point>`                                           |

`<entry>` es `entry_point` como módulo (`api/main.py` → `api.main`). `gunicorn`/`uvicorn` deben estar en tus requirements. El script no corre migraciones ni `collectstatic`: ponlas en hooks `post_deploy`, por ejemplo `".venv/bin/python manage.py migrate --noinput"`.

Para correrlo como un servicio que arranca con el servidor, agrégalo a [`services`](#services-services).

---

## Archivos entre releases: compartidos, preservados, reutilizables { #files-between-releases-shared-preserved-reusable }

Cada deploy crea una carpeta de release nueva. Tres mecanismos traspasan cosas de un release al siguiente:

| Opción                    | Dónde viven los datos           | En cada deploy                                                                                          | Uso típico                               |
| :------------------------ | :------------------------------ | :------------------------------------------------------------------------------------------------------ | :--------------------------------------- |
| `shared_paths`            | `<remote_path>/shared/<ruta>`   | La ruta en el release se reemplaza por un enlace a `shared/<ruta>`. Lo que traía el repositorio ahí se descarta. | `.env`, `storage`, `public/uploads`, logs |
| `preserved_paths`         | Dentro de cada release          | Se copian desde el release anterior sobre el nuevo. El primer deploy usa la versión del repositorio.     | Archivos de configuración editados en el servidor |
| `builds.*.reusable_paths` | Dentro de cada release          | Se enlazan con hardlinks (`cp -al`) desde el release anterior cuando las dependencias no cambiaron, en vez de reinstalar. | `vendor`, `node_modules`, `.venv` |

Todas las rutas son relativas a la carpeta de la app (la raíz del repositorio).

Una ruta compartida que todavía no existe en `shared/` se crea **una sola vez**, en el deploy que la necesita por primera vez:

| El release tiene…                     | `shared/<ruta>` queda como                                |
| :------------------------------------ | :-------------------------------------------------------- |
| la ruta misma (el `storage/` de Laravel, un `config.json` por defecto) | una copia de ella        |
| `<ruta>.example` (`.env.example`)     | una copia del ejemplo, permisos `0640`                    |
| ninguna de las dos, y el nombre empieza con `.env` | un archivo vacío, permisos `0640`            |
| ninguna de las dos                    | una carpeta vacía con escritura para el grupo             |

El deploy avisa de cada ruta que creó. Desde ahí, un deploy nunca sobrescribe `shared/`.

### Variables de entorno y secretos (`.env`, `config.json`) { #environment-variables-and-secrets-env-configjson }

Tu `.env` local nunca se sube: los releases se compilan desde un clon del repositorio, donde `.env` está ignorado. El servidor tiene el suyo.

**Dónde vive.** Una entrada de `shared_paths` es una ruta relativa a la raíz del repositorio, el mismo lugar donde el framework la busca en tu computador. Con `- ".env"`:

```
/var/www/myapp/
├── shared/.env                         ← el archivo real (uno por servidor, sobrevive a los deploys)
└── current → releases/20261108_1200/
    └── app/.env → /var/www/myapp/shared/.env   (enlace creado en cada deploy)
```

- **Apps PHP** (Laravel, versaWYS) leen `app/.env` igual que en tu computador; el enlace las lleva a `shared/.env`.
- **Services** (`env_file: ".env"`, relativo a `shared/`): el sistema de init carga `shared/.env` en el entorno del proceso antes de iniciarlo, así `os.Getenv`, `process.env` y `os.environ` ven las variables sin leer ningún archivo (el `.output` de Nuxt y los binarios Go dependen de esto; `next start` además lee `app/.env` por su cuenta).
- **Monorepo** con la app en una subcarpeta: `shared_paths: ["backend/.env"]` y `env_file: "backend/.env"`.

Para configurarlo y cambiarlo:

1. Pon el archivo en `shared_paths` (`- ".env"`). El primer deploy crea `shared/.env` desde `.env.example`.
2. Edítalo desde la TUI: vista **Shared** → el archivo → `Enter` → `e` → `Ctrl+S` lo guarda en el servidor (el archivo mantiene sus permisos). Los archivos de `preserved_paths` (como el `config.json` de versaWYS) también aparecen ahí. También puedes editar cualquier archivo desde la vista **Files** (`Enter`, `e`), o subir uno con `u`.
3. Aplícalo: los services lo leen al iniciar (`versa service <entorno> restart`); los frameworks PHP que cachean su configuración necesitan volver a correr su comando de caché (`php artisan optimize`): **Operations → h** vuelve a correr los hooks `post_deploy`.

| De dónde viene una variable                     | Uso                                                                 |
| :---------------------------------------------- | :------------------------------------------------------------------ |
| `shared/.env` en el servidor                    | Secretos y opciones de cada servidor. Las apps PHP lo leen; los services lo cargan con [`env_file`](#services-services). |
| `services[].environment`                        | Opciones no secretas de un servicio (`PORT`, `NODE_ENV`).           |
| Tu computador, al compilar                      | Valores incluidos en el frontend al compilar (`NEXT_PUBLIC_*`, `VITE_*`): el build corre en local. |
| `${VAR}` en `deploy.yml`                        | Valores para versa mismo (`provision.db_password`), expandidos en local. No los pongas en hooks remotos (mira [Hooks](#hooks)). |

Hay configuraciones listas por framework en [`examples/`](../examples/).

---

## Hooks { #hooks }

Comandos que corren en momentos fijos del deploy. Cada entrada es un comando, o un bloque `parallel:` cuyos comandos corren al mismo tiempo:

```yaml
pre_deploy_local:
  - "npm test"

pre_deploy_server:
  - "php artisan down || true"

post_deploy:
  - "php artisan migrate --force"
  - parallel:
      - "php artisan config:cache"
      - "php artisan route:cache"
  - "php artisan up"
```

| Hook                | Corre en     | Carpeta de trabajo         | Cuándo                                             | Si falla                                                            |
| :------------------ | :----------- | :------------------------- | :------------------------------------------------- | :------------------------------------------------------------------ |
| `pre_deploy_local`  | Tu computador | Raíz del repositorio      | Antes de revisar y clonar el repositorio. No corre con `--dry-run`. | El deploy se cancela. No se tocó nada en el servidor.  |
| `pre_deploy_server` | Servidor     | `<release nuevo>/app`      | Después de subir, antes de cambiar `current`       | Solo una advertencia; el deploy sigue.                              |
| `post_deploy`       | Servidor     | `<release nuevo>/app`      | Después de cambiar `current` y de `services_reload` | `current` vuelve al release anterior y el deploy falla (en el primer deploy no hay a dónde volver). |
| `on_failure`        | Servidor     | `current/app` (después del rollback: el release activo) | Cuando un deploy falla después de conectarse al servidor | Solo una advertencia. |

- Los hooks remotos corren en el shell de login del usuario SSH, con un límite de `hook_timeout` segundos cada uno.
- Los hooks locales (`pre_deploy_local`, y `post_deploy` en modo local) corren con `sh -c`. **En Windows, `sh` debe estar en el `PATH`** (viene con Git for Windows: agrega `C:\Program Files\Git\bin`).
- En el primer deploy (`--initial-deploy`), versa pregunta antes de correr `post_deploy`, para que configures el `.env` primero ([cómo](#environment-variables-and-secrets-env-configjson)).
- `on_failure` deshace lo que un hook anterior dejó a medias, por ejemplo el modo mantenimiento:

  ```yaml
  pre_deploy_server:
    - "php artisan down --retry=15"   # storage/ es compartido: afecta al sitio en vivo
  post_deploy:
    - "php artisan migrate --force"
    - "php artisan up"
  on_failure:
    - "php artisan up"                # nunca dejar el sitio abajo después de un deploy fallido
  ```
- Vuelve a correr `post_deploy` sin desplegar: `versa hooks <entorno> [índices...]`.
- `$NOMBRE` se [expande en local](#environment-variables-in-the-config); escribe `$$NOMBRE` para una variable del shell del servidor.
- No pongas `${SECRETO}` en hooks remotos: la línea de comandos expandida es visible para otros usuarios del servidor (`ps`). Lee los secretos en el servidor, por ejemplo `$$DB_PASS` desde el `.env` de la app en `shared/`.

---

## Recarga de servicios (`services_reload`) { #service-reload-services_reload }

Comandos que corren en el servidor justo después de cambiar `current` (antes de `post_deploy`), y después de cada rollback (`versa rollback`, la TUI, y los rollbacks automáticos tras un hook `post_deploy` o un health check fallido). Cada uno tiene 30 s de límite; las fallas son advertencias y nunca provocan un rollback.

```yaml
services_reload:
  - "sudo systemctl reload php8.2-fpm"
  - "sudo systemctl reload nginx"
```

> [!IMPORTANT]
> **Hay que recargar PHP-FPM después de cada deploy.** OPcache y el `realpath_cache` de PHP recuerdan a dónde apuntaba `current`, así que sin recarga PHP sigue sirviendo el release viejo por minutos. El usuario SSH necesita `sudo` sin contraseña para estos comandos; mira la [Guía completa](GETTING_STARTED.md#2-prepare-the-server).

Córrelos cuando quieras con `versa services-reload <entorno>`.

---

## Services (`services`) { #services-services }

Procesos de larga duración, como una API en Go o un servidor web en Python, que deben seguir corriendo, arrancar cuando inicia el servidor y tomar cada release nuevo. Para cada servicio versa:

1. Escribe sus archivos en `<remote_path>/.versa/`: un script wrapper (`<nombre>.sh`) más una unit de systemd, un script OpenRC y un script de init SysV. Todos apuntan a `<remote_path>/current`, así siguen siendo válidos entre deploys y rollbacks.
2. Detecta el sistema de init del servidor e instala el archivo que corresponde: `/etc/systemd/system/<nombre>.service` (systemd), o `/etc/init.d/<nombre>` (OpenRC, o SysV init en RHEL/CentOS 5-6 y otros servidores sin systemd). Solo lo copia cuando cambió.
3. Lo habilita al iniciar el servidor (`systemctl enable`, `rc-update add`, `chkconfig --add` o `update-rc.d`).
4. Lo reinicia después de los hooks `post_deploy`, espera `start_wait` segundos y revisa que siga corriendo. Si no, el deploy **vuelve atrás**.
5. Lo reinicia de nuevo después de cada rollback (`versa rollback`, la TUI, y los rollbacks automáticos).

```yaml
services:
  - name: "api"                      # binario Go: exec lo usa por defecto
    env_file: ".env"                 # <remote_path>/shared/.env
    environment:
      PORT: "8080"

  - name: "web"                      # Python con web_server: exec usa run_server.sh por defecto
```

| Campo         | Tipo    | Por defecto              | Descripción                                                                                                  |
| :------------ | :------ | :----------------------- | :----------------------------------------------------------------------------------------------------------- |
| `name`        | string  | —                        | **Obligatorio.** Nombre en el sistema de init (`systemctl status <nombre>`). Letras, dígitos, `-`, `_`, `.`, `@`. |
| `exec`        | string  | mira abajo               | Línea de comandos. Un `./` al inicio es relativo a `<remote_path>/current`: `./bin/go/api --port 8080`.      |
| `working_dir` | string  | `app` (`app/<python.root>` en proyectos solo Python) | Carpeta, relativa a `<remote_path>/current`, donde arranca el proceso.                     |
| `user`        | string  | `ssh.user`               | Usuario Unix con que corre el proceso.                                                                       |
| `env_file`    | string  | —                        | Archivo bajo `<remote_path>/shared/` con líneas `CLAVE=valor`, cargado en el entorno del proceso.            |
| `environment` | map     | —                        | Variables de entorno extra, escritas en el wrapper de `.versa/` (permisos 0600 cuando el servicio corre como `ssh.user`, 0644 si no: en ese caso pon los secretos en `env_file`). |
| `start_wait`  | int (s) | `3`                      | Segundos de espera tras (re)iniciar antes de revisar que el proceso siga corriendo.                         |
| `stop_timeout`| int (s) | `30`                     | Segundos que un stop espera a que el proceso termine tras SIGTERM antes de matarlo (SIGKILL). Súbelo si tu app necesita más tiempo para cerrarse limpiamente. |
| `reload`      | string  | —                        | Comando que corre (como el usuario SSH) **en vez de un reinicio** en deploys y rollbacks mientras el servicio está arriba: una recarga suave que no corta requests (por ejemplo `kill -HUP` para gunicorn; FrankenPHP: su socket de `opcache_reset()`, propuesto por la vista Services). Un servicio detenido, o una recarga fallida, se reinicia. `versa service <entorno> restart` siempre reinicia. |

**`exec` por defecto** (cuando se omite):

| Builds activos                         | Corre                                                      |
| :------------------------------------- | :--------------------------------------------------------- |
| `go`                                   | `<remote_path>/current/<go.deploy_path>/<go.binary_name>`   |
| `python` + `build_binary`              | el binario de PyInstaller                                   |
| `python` + `web_server`                | `/bin/sh <raíz python>/run_server.sh`                       |
| `python` + `entry_point`               | `<venv>/bin/python <entry_point>`                           |
| `go` **y** `python`, o ninguno         | nada: define `exec` (un servicio por proceso).              |

Tu programa debería escribir sus logs en stdout/stderr. Los logs van al journal con systemd (`versa service <entorno> logs`) y a `/var/log/<nombre>.log` en los demás.

### Permisos (configuración como root y sudo) { #permissions-root-setup-and-sudo }

Los archivos de init (unit de systemd, script OpenRC o SysV) los instala **solo root**. `.versa/` pertenece al usuario de deploy. Si el usuario de deploy pudiera copiar con sudo un archivo de ahí a `/etc`, podría escribir cualquier unit, y las units pueden correr como cualquier usuario: en la práctica, como root. Corre esto una vez por servidor, como root:

```bash
versa service production setup | ssh root@203.0.113.10 sh
```

Instala y habilita cada servicio, y escribe `/etc/sudoers.d/versa-<proyecto>` (validado con `visudo -c`). Ese archivo solo permite al usuario de deploy hacer `start` / `stop` / `restart` / `enable` / `disable` de los services, `daemon-reload`, y leer su journal con argumentos fijos. La vista Services hace lo mismo en su paso **Sudoers**. `versa info <entorno>` lo revisa.

Vuelve a correrlo cuando cambie `name`, `user`, `env_file` o `stop_timeout` de un servicio: esas opciones viven en el archivo instalado. Hasta entonces, los deploys se detienen con *"needs root setup"*. `exec`, `environment` y `working_dir` viven en el wrapper de `.versa/`, así que sus cambios se despliegan normalmente. Cuando `ssh.user` es `root`, los deploys instalan los archivos por sí mismos.

Un servicio no puede correr como `root` salvo que `ssh.user` sea `root`: el usuario de deploy escribe el código que corre. Pon el web server delante para puertos menores a 1024.

`versa service <entorno> sudoers` sigue mostrando solo las líneas de sudoers, con las rutas reales de los binarios del servidor:

- Siempre incluye `Defaults:<usuario> !requiretty`: RHEL/CentOS hasta la 7 traen `Defaults requiretty`, que hace que sudo se niegue a correr por SSH sin terminal (así corre versa los comandos).
- En servidores sin `/etc/sudoers.d` (el sudo 1.7 de RHEL/CentOS 5), las líneas van al final de `/etc/sudoers` (`setup` te las muestra para agregarlas con `visudo`).
- Los comandos usan las rutas absolutas de este servidor (por ejemplo `/sbin/chkconfig`), y versa los corre con esas mismas rutas: el sudo antiguo no tiene `secure_path`, así que `sudo chkconfig` no coincidiría con una regla para `/sbin/chkconfig`.

### Sistemas de init { #init-systems }

| Servidor                                       | Init                      | Supervisión                                                         |
| :--------------------------------------------- | :------------------------ | :------------------------------------------------------------------ |
| Ubuntu 16.04+, Debian 8+, RHEL/CentOS 7+, …    | systemd                   | Se reinicia solo si se cae (`Restart=always`).                      |
| Alpine, Gentoo                                 | OpenRC                    | Arranca al iniciar el servidor; no se reinicia si se cae.           |
| RHEL/CentOS 5-6, Debian antiguo, sistemas busybox | SysV init (`/etc/init.d`) | Arranca al iniciar el servidor (pidfile + `nohup`); no se reinicia si se cae. |

Administra los services a mano con [`versa service`](CLI_REFERENCE.md#versa-service).

---

## Preparación del servidor (`provision`) { #server-provisioning-provision }

Bloque opcional que solo lee la vista **Services** de la TUI, que instala y mantiene el stack del servidor. Los deploys lo ignoran. Contiene la conexión de administrador (root) y el stack deseado:

```yaml
provision:
  ssh: { user: root }        # hereda host, puerto y llave de ssh
  domain: myapp.com
  aliases: [www.myapp.com]
  web_server: nginx          # nginx | apache
  php_version: "8.4"
  database: postgres         # postgres | mariadb | none
  db_name: myapp
  db_user: myapp_user
  db_password: ${DB_PASS}    # solo para crear un usuario nuevo; la TUI nunca lo escribe
  redis: true
  ssl: true
  backup: true
```

Para apps Python, Go y Node la vista propone la entrada de [`services`](#services-services) e instala sus reglas de sudoers, así cada deploy instala y reinicia el servicio. Todos los campos y las reglas de seguridad están en **[Servicios del servidor](SERVICES.md#9-provision-reference)**.

## Health check (`health_check`) { #health-check-health_check }

Verifica el release nuevo después de `post_deploy` y del reinicio de los services. Si fallan todos los intentos, `current` vuelve al release anterior, se recargan los servicios y el deploy falla.

```yaml
health_check:
  url: "https://myapp.com/health"   # HTTP GET desde TU computador
  expected_status: 200
  command: "php artisan about"      # y/o un comando en el servidor, en <release nuevo>/app; exit 0 = sano
  timeout: 10
  retries: 3
  retry_delay: 2
```

| Campo             | Por defecto | Descripción                                                         |
| :---------------- | :---------- | :------------------------------------------------------------------ |
| `url`             | —           | URL pedida desde el computador que corre versa.                     |
| `expected_status` | `200`       | Código HTTP que se considera sano.                                  |
| `command`         | —           | Comando remoto; sano si termina con 0. Si están los dos, ambos deben pasar. |
| `timeout`         | `10`        | Segundos por intento.                                               |
| `retries`         | `3`         | Intentos antes de rendirse.                                         |
| `retry_delay`     | `2`         | Segundos entre intentos.                                            |

---

## Notificaciones (`notifications`) { #notifications-notifications }

Envía un `POST` JSON cuando termina un deploy. Funciona con los incoming webhooks de Slack y Microsoft Teams (campo `text`), Discord (campo `content`) y Telegram (`chat_id` + `text`). No se envía con `--dry-run`.

```yaml
notifications:
  webhook_url: "${DEPLOY_WEBHOOK}"
  on_success: false   # en silencio mientras los deploys funcionan…
  on_failure: true    # …cada falla, y "recovered" en el siguiente éxito
```

- **Fallas**: llevan el error y las últimas 20 líneas del log del deploy (`log_tail`, también en `text`).
- **Recuperado:** con `on_failure: true`, el primer éxito después de un deploy fallido se envía aunque `on_success` sea `false` (estado `recovered`), así una corrección nunca pasa inadvertida. El resultado anterior sale del [historial](#deploy-history) del servidor.
- **Telegram:** crea un bot con @BotFather y usa

  ```yaml
  notifications:
    webhook_url: "https://api.telegram.org/bot${TELEGRAM_TOKEN}/sendMessage"
    chat_id: "-1001234567890"
    on_failure: true
  ```

Contenido:

```json
{
  "text": "[SUCCESS] my-app → production: success (release 20261001-183807, 42s)",
  "content": "…same as text…",
  "project": "my-app",
  "environment": "production",
  "release": "20261001-183807",
  "commit": "9f2c1e7…",
  "status": "success",
  "error": "",
  "duration_s": 42.1,
  "timestamp": "2026-10-01T18:38:49Z",
  "tags": ["v1.2.0"],
  "log_tail": ""
}
```

---

## Historial de deploys { #deploy-history }

Cada deploy que llega al servidor queda registrado ahí, exitoso o no, con su log completo, y también cada rollback (desde `versa rollback` o la TUI: quién volvió a qué release): `<remote_path>/history/history.jsonl` (los últimos 100 deploys) y `history/<id>.log` (los últimos 20). Los deploys fallidos también se registran, aunque el rollback borra su release. Los logs pueden contener la salida de los hooks, por eso los archivos tienen permisos `0600`.

```text
$ versa history production
ID               STATUS   TIME    COMMIT    TAG           BY
20261108-153012  success     48s  a41f09c3  v1.2.0        ana@laptop
20261108-150140  FAILED      31s  c93a11f0                ana@laptop
                 └ post-deploy hook failed (rolled back to 20261107-101203): …
$ versa history production 20261108-150140    # el log completo de ese deploy
```

La TUI muestra los últimos 10 en **Operations → t**, y los logs en **Files → history/**. `BY` es el `usuario@host` del computador que desplegó; define `VERSA_DEPLOYER` para ponerle tu propio nombre (en CI: `VERSA_DEPLOYER: ${{ github.actor }}`).

---

## Producción desde tags (`deploy_tags`) { #production-from-tags-deploy_tags }

Con `deploy_tags`, un entorno solo despliega commits que tienen un tag de git que coincide, así producción cambia solo cuando publicas una versión, mientras staging despliega cualquier commit:

```yaml
environments:
  staging:
    # … cada commit
  production:
    deploy_tags: "v*"      # glob: v*, v[0-9]*, release-*
```

```bash
git tag v1.2.0 && git push --tags
versa deploy production          # se rechaza si HEAD no tiene un tag v*
```

El tag aparece en el historial y en las notificaciones. Desde CI, [examples/github-actions.yml](../examples/github-actions.yml) despliega staging en cada push a `main` y producción en cada tag `v*`.

---

## Rendimiento y límites { #performance-and-limits }

- **Detección de cambios**: si nada cambió desde el `deploy.lock` del servidor, el deploy se detiene antes de compilar. Los builders solo corren para los lenguajes cuyos archivos cambiaron.
- **Subida**: el release se comprime en partes de `chunk_size_mb` y lo suben `upload_workers` workers en paralelo; luego se descomprime con `tar` en el servidor. versa revisa antes el espacio libre en disco.
- **`incremental_upload: true`**: el release nuevo parte como una copia con hardlinks del anterior y solo se suben los archivos que cambiaron. Mucho más rápido en proyectos grandes con cambios pequeños.

  > [!WARNING]
  > Con hardlinks, ambos releases comparten los mismos archivos. Los hooks no deben modificar archivos del release **en el mismo lugar** (por ejemplo `echo >> archivo`, `sed -i`), o el release anterior también cambia y los rollbacks dejan de ser limpios. Escribir un archivo nuevo y renombrarlo es seguro.

- **`deploy_timeout`** detiene un deploy que tarda demasiado en total; **`hook_timeout`** limita cada hook.

---

## Modo local (sin SSH) { #local-mode-no-ssh }

Para hosting sin SSH (hosting compartido con FTP o un administrador de archivos). versa compila el proyecto y escribe un release **completo** en `local_path`, reemplazando lo que había. Después subes tú el contenido de `local_path/app`.

```yaml
environments:
  hosting:
    local: true
    local_path: "./local-releases/hosting"
    builds:
      php:
        enabled: true
    post_deploy:
      - "php versaCLI cache:clear"   # corre en local, en local_path/app
```

- No se necesitan `ssh` ni `remote_path`.
- Sin historial de releases: no hay rollback, `shared_paths`, `preserved_paths` ni `pre_deploy_server` (se ignoran con una advertencia), ni `services`.
- Cada deploy es un build completo.
- No apuntes `local_path` a una carpeta donde escribe tu build (por ejemplo el `dist` del frontend).
- Solo funcionan `deploy` y `config validate`; mira [Comandos](CLI_REFERENCE.md#which-commands-work-in-local-mode).

---

## Estructura de carpetas en el servidor { #server-directory-layout }

```
<remote_path>/
├── current -> <remote_path>/releases/20261001-183807   # lo que sirve tu web server
├── releases/
│   ├── 20260930-170212/
│   └── 20261001-183807/
│       ├── app/            # tu repositorio (compilado); aquí corren los hooks
│       ├── bin/            # binarios Go (go.deploy_path)
│       └── manifest.json   # metadatos del release
├── shared/                 # shared_paths
├── history/                # cada deploy: history.jsonl + <id>.log
├── deploy.lock             # hashes y metadatos del último deploy
├── .versa/                 # archivos de services generados (wrapper, unit de systemd, scripts de init)
└── .versa.lock/            # existe solo mientras corre un deploy
```

Apunta tu web server a `<remote_path>/current/app/...` (por ejemplo `/var/www/app/current/app/public`). Los nombres de los releases son fechas UTC.

---

## Opciones obsoletas { #deprecated-options }

| Opción                | Reemplazo                                                                                           |
| :-------------------- | :-------------------------------------------------------------------------------------------------- |
| `python.service_name` | `services: [{name: ...}]` (se migra automáticamente, con una advertencia). El archivo `.service` antiguo en la carpeta de la app ya no se escribe. |
| `hook_execution_mode` | `before_switch` → mueve los hooks a `pre_deploy_server` (se hace automáticamente, con una advertencia). `after_switch` → solo quita la opción. No se puede combinar con `pre_deploy_local`/`pre_deploy_server`. |
