# 🩺 Problemas comunes { #troubleshooting }

Errores frecuentes y cómo resolverlos. Agrega `--debug` a cualquier comando para ver más detalle (y `--log-file deploy.log` para guardar la salida). `versa info <entorno>` es la forma más rápida de revisar SSH, la llave de host y SFTP.

- [Configuración](#configuration)
- [Conexión SSH](#ssh-connection)
- [Deploy](#deploy)
- [Builds](#builds)
- [Hooks](#hooks)
- [Services](#services)
- [Vista Services (preparar el servidor)](#services-view-server-setup)
- [Después del deploy](#after-the-deploy)

---

## Configuración { #configuration }

### `Multiple configuration files found. Please select one` { #multiple-configuration-files-found-please-select-one }

Hay más de un archivo en la carpeta que calza con los nombres de configuración (`deploy*.yml`, `*_deploy*.yml`, …). Elige uno, o pásalo explícitamente (obligatorio en scripts y CI): `--config deploy_production.yml`.

### `environment 'X' not found in configuration` { #environment-x-not-found-in-configuration }

El nombre después del comando debe ser una clave bajo `environments:`. Revisa cómo está escrito y qué archivo se cargó (`versa config validate`).

### `environment keys are mis-indented` { #environment-keys-are-mis-indented }

Una opción (`ssh`, `remote_path`, `builds` …) está a la misma sangría que el nombre del entorno. Cada opción va un nivel más adentro que su entorno.

### `ssh key not found` / `SSH key has insecure permissions` { #ssh-key-not-found-ssh-key-has-insecure-permissions }

`key_path` debe apuntar a una llave privada que exista. En Linux y macOS la llave debe ser privada para tu usuario:

```bash
chmod 600 ~/.ssh/id_rsa
```

### `remote_path must be an absolute path` { #remote_path-must-be-an-absolute-path }

Usa una ruta completa que empiece con `/`: `remote_path: "/var/www/app"`.

### `at least one build type must be enabled` { #at-least-one-build-type-must-be-enabled }

Activa al menos uno de `builds.php`, `builds.go`, `builds.frontend`, `builds.python`.

### Un hook o un valor perdió su parte `$...` { #a-hook-or-value-lost-its-part }

`${NOMBRE}`/`$NOMBRE` en cualquier parte de la configuración se reemplaza con las variables de entorno **de tu computador**, también dentro de los hooks. Para una variable del shell del servidor escribe `$$NOMBRE` (por ejemplo `echo $$HOME`). Mira [Variables de entorno](DEPLOY.md#environment-variables-in-the-config).

---

## Conexión SSH { #ssh-connection }

### `Host key NOT verified (open ...known_hosts: ...)` { #host-key-not-verified-open-known_hosts }

versa no pudo leer el archivo `known_hosts`, así que se conectó sin verificar la identidad del servidor. Para resolverlo:

1. Conéctate una vez con `ssh usuario@host` y acepta la llave, o ejecuta `ssh-keyscan -p 22 host >> ~/.ssh/known_hosts`.
2. Si configuraste `known_hosts_file`, revisa la ruta. `~/` funciona (versa ≤ 1.7.0 no expandía `~` en este campo; actualiza con `versa self-update` o quita la línea para usar el valor por defecto).
3. Para rechazar conexiones sin verificar, usa `ssh.strict_host_key: true`.

### `knownhosts: key mismatch` { #knownhosts-key-mismatch }

La llave que presentó el servidor no coincide con `known_hosts`. O el servidor se reinstaló / regeneró sus llaves, o algo está interceptando la conexión. Confírmalo con quien administra el servidor, luego borra la entrada vieja y agrega la nueva:

```bash
ssh-keygen -R your-server.com
ssh user@your-server.com      # acepta la llave nueva
```

### `ssh: handshake failed: ssh: no common algorithm for ...` { #ssh-handshake-failed-ssh-no-common-algorithm-for }

El servidor solo soporta algoritmos antiguos (típico de OpenSSH < 7, por ejemplo CentOS 5/6). Actívalos para ese entorno:

```yaml
ssh:
  legacy_algorithms: true
```

Para `no common algorithm for host key ... peer offered: [ssh-rsa ssh-dss]` con una entrada de tipo `ssh-rsa` en `known_hosts`, actualiza versa (se corrigió después de 1.7.0) o usa `legacy_algorithms: true`.

### `SSH Authentication failed` { #ssh-authentication-failed }

- La llave pública no está en `~/.ssh/authorized_keys` de ese `user` en el servidor, o `key_path` apunta a otra llave. Prueba `ssh -i <key_path> usuario@host`.
- En el servidor, `~/.ssh` debe tener permisos `700` y `authorized_keys` `600`.
- Si tu llave está en un agente, usa `use_ssh_agent: true`.

### Timeout de conexión / `connection refused` { #connection-timeout-connection-refused }

Revisa `host`, `port`, firewalls y VPN. `versa info <entorno>` muestra si la conexión TCP llega o no.

---

## Deploy { #deploy }

### `working directory has uncommitted changes` { #working-directory-has-uncommitted-changes }

versa despliega el último commit, y se niega a correr si tienes cambios sin commitear para que no creas que se desplegaron. Haz commit, o usa `--skip-dirty-check` (los cambios sin commitear igual **no** se despliegan).

### `No changes detected - skipping deployment` { #no-changes-detected-skipping-deployment }

Nada cambió desde el último deploy registrado en el `deploy.lock` del servidor. ¿Hiciste commit? Para desplegar igual (por ejemplo después de un rollback, o para reconstruir dependencias) usa `--force`.

### `deploy.lock not found on remote server` { #deploylock-not-found-on-remote-server }

Es el primer deploy a este entorno (o cambió `remote_path`). Usa `versa deploy <entorno> --initial-deploy`.

### `Deployment lock already held` { #deployment-lock-already-held }

Hay otro deploy corriendo, o uno anterior se interrumpió (Ctrl+C, conexión perdida) y dejó el bloqueo. Si estás seguro de que nadie más está desplegando:

```bash
versa exec production "rmdir /var/www/app/.versa.lock"
```

### `deployment aborted: timeout of 600s exceeded` { #deployment-aborted-timeout-of-600s-exceeded }

El deploy completo tardó más que `deploy_timeout`. Súbelo para proyectos grandes o conexiones lentas, o activa `incremental_upload`.

### Falta espacio en disco en el servidor { #not-enough-disk-space-on-the-server }

versa revisa el espacio libre antes de subir. Libera espacio, o baja `releases_to_keep` para guardar menos releases viejos.

### `tar not found` en el servidor { #tar-not-found-on-the-server }

Instala `tar` (y `gzip`) en el servidor; los deploys los necesitan.

---

## Builds { #builds }

### `PHP build tool 'composer' not found` / `Frontend build tool '...' not found` / `Go compiler not found` { #php-build-tool-composer-not-found-frontend-build-tool-not-found-go-compiler-not-found }

Los builds corren en **tu** computador. Instala la herramienta y revisa que esté en el `PATH` de la terminal desde donde corres versa (abre una terminal nueva después de cambiar el `PATH`).

### No se reinstalaron las dependencias { #dependencies-werent-reinstalled }

Las dependencias se reinstalan cuando cambia un manifiesto o su lockfile (`composer.json`/`composer.lock`, `package.json`/`package-lock.json`/`pnpm-lock.yaml`/`yarn.lock`, `go.mod`/`go.sum`, requirements de Python/`pyproject.toml`/`poetry.lock`/`Pipfile`/`Pipfile.lock`), y solo si ese archivo está commiteado. Para forzar una reinstalación limpia de `vendor`/`node_modules`, despliega con `--force`.

### Go: el target no coincide con el servidor { #go-target-doesnt-match-the-server }

Antes de compilar, versa compara `go.target_os`/`target_arch` con el servidor y con el kernel mínimo que soporta tu versión de Go. Usa el `target_arch` correcto (`uname -m` en el servidor: `x86_64` → `amd64`, `aarch64` → `arm64`), o compila con un Go más antiguo para kernels muy viejos.

### Falta un archivo en el servidor { #a-file-is-missing-on-the-server }

- ¿Está commiteado? Solo se despliegan archivos versionados y commiteados.
- ¿Está en `ignored_paths`? Esos se borran del release.
- ¿Está en `shared_paths`? Entonces se usa la copia del servidor en `shared/`.

---

## Hooks { #hooks }

### `post-deploy hook failed (rolled back to ...)` { #post-deploy-hook-failed-rolled-back-to }

Un comando `post_deploy` terminó con error, así que `current` volvió a la versión anterior. Córrelo a mano en el release nuevo para ver por qué:

```bash
versa exec production "cd /var/www/app/releases/<versión>/app && php artisan migrate --force"
```

Causas frecuentes: el comando no está en el `PATH` del shell no interactivo (usa la ruta completa, por ejemplo `/usr/bin/php8.2`), falta el `.env`, o un problema de permisos. Vuelve a correr los hooks sobre el release activo con `versa hooks <entorno>`.

### `pre_deploy_local hook failed: exec: "sh": executable file not found` (Windows) { #pre_deploy_local-hook-failed-exec-sh-executable-file-not-found-windows }

Los hooks locales corren con `sh`. Instala Git for Windows y agrega `C:\Program Files\Git\bin` al `PATH`.

### Un hook se cortó a los 300 s { #a-hook-was-killed-after-300-s }

Sube `hook_timeout` (en segundos).

---

## Services { #services }

### `needs root setup` / `the deploy user needs passwordless sudo` { #needs-root-setup-the-deploy-user-needs-passwordless-sudo }

El servicio todavía no está instalado, o cambió su `name`, `user`, `env_file` o `stop_timeout`. Otra causa: el usuario SSH no puede correr los comandos del sistema de init con `sudo -n`. Root instala los services y el archivo de sudoers:

```bash
versa service production setup | ssh root@SERVIDOR sh
```

`versa info <entorno>` te dice si quedó resuelto. Los servidores preparados antes de la v1.9.1 tienen una regla de sudoers que le permitía al usuario de deploy instalar archivos de unidad. Correr `setup` reemplaza `/etc/sudoers.d/versa-<proyecto>` y la elimina. Si guardaste esas líneas en otro archivo, bórralo.

### `sudo: sorry, you must have a tty to run sudo` { #sudo-sorry-you-must-have-a-tty-to-run-sudo }

El servidor tiene `Defaults requiretty` (por defecto en RHEL/CentOS hasta la 7). Agrega la línea `Defaults:<usuario> !requiretty` que imprime `versa service <entorno> sudoers`.

### `sudo: sorry, a password is required` aunque la línea de sudoers está { #sudo-sorry-a-password-is-required-although-the-sudoers-line-is-there }

La regla tiene que calzar exactamente con el comando y su ruta. Genérala de nuevo en ese servidor con `versa service <entorno> setup | ssh root@SERVIDOR sh` (usa las rutas del servidor). En RHEL/CentOS 5 no existe `/etc/sudoers.d`: las líneas van en `/etc/sudoers`.

### `services failed to start (rolled back to ...)` / `<nombre> is not running` { #services-failed-to-start-rolled-back-to-name-is-not-running }

El proceso terminó antes de `start_wait` segundos, así que versa volvió al release anterior. La salida sobre el error muestra las últimas líneas del log. Después:

```bash
versa service production logs --name <nombre>      # log completo
versa exec production "cd /var/www/app/current/app && /bin/sh /var/www/app/.versa/<nombre>.sh"   # correrlo a mano
```

Causas habituales: falta una variable en `shared/.env`, el puerto ya está en uso (un proceso viejo arrancado a mano: detenlo), un binario compilado para otra arquitectura, o falta un paquete de Python. Si tu app tarda más en fallar o en arrancar, sube `start_wait`.

### `failed to prepare the Python virtualenv` { #failed-to-prepare-the-python-virtualenv }

- `python3 not found on the server`: instala Python 3, o pon en `python.python_command` su nombre o ruta (por ejemplo `python3.11`).
- `could not create the virtualenv`: instala el módulo venv (`apt install python3-venv` en Debian/Ubuntu).
- Un error de `pip install`: el paquete necesita herramientas de compilación o librerías del sistema en el servidor (`build-essential`, `python3-dev`, `libpq-dev` …).
- `poetry`/`pipenv not found`: instálalo en el servidor, o usa `package_manager: pip`.

### `python.build_binary needs versa to run on Linux` { #pythonbuild_binary-needs-versa-to-run-on-linux }

PyInstaller compila para el sistema donde corre, así que un binario hecho en Windows o macOS no funciona en el servidor. Despliega desde Linux (WSL o CI), o quita `build_binary` y deja que versa corra el código con un virtualenv en el servidor.

### El servicio no arranca después de reiniciar el servidor (SysV / OpenRC) { #the-service-doesnt-start-after-a-reboot-sysv-openrc }

Revisa que esté habilitado: `chkconfig --list <nombre>` (RHEL/CentOS) o `rc-update show` (Alpine). versa lo habilita al instalarlo; corre `versa service <entorno> install` para repetirlo. Sin systemd, los procesos que se caen no se reinician solos.

---

## Vista Services (preparar el servidor) { #services-view-server-setup }

### `Apache is already installed — nginx not installed` (o al revés) { #apache-is-already-installed-nginx-not-installed-or-the-opposite }

versa nunca instala una alternativa a lo que el servidor ya usa: elige el web server que ya tiene. Lo mismo vale para una segunda versión de PHP-FPM o Node.js, y para MariaDB sobre MySQL (se reutiliza el MySQL existente). Mira **[Servicios del servidor](SERVICES.md#6-safety-rules-on-servers-already-in-use)**.

### `<dominio> is already served by: …` { #domain-is-already-served-by }

Otro vhost (hecho a mano o por certbot, por ejemplo `myapp-le-ssl.conf`) responde por ese dominio, así que no se agrega un segundo. Úsalo, o desactívalo antes del paso del web server.

### `no public key: the deploy user is created with <ssh.key_path>.pub` { #no-public-key-the-deploy-user-is-created-with-sshkey_pathpub }

`P` y el paso *Deploy user* instalan tu llave pública en el servidor. Créala junto a la privada: `ssh-keygen -y -f ~/.ssh/id_ed25519 > ~/.ssh/id_ed25519.pub`.

### El usuario de deploy no puede conectarse a un servidor nuevo { #the-deploy-user-cant-connect-to-a-fresh-server }

Todavía no existe. Pon `provision.ssh.user: root`, abre **Services** y presiona `P`.

### Una app perdió acceso a la base de datos después de volver a correr la instalación { #an-app-lost-database-access-after-a-re-run }

Volver a correr la instalación nunca cambia la contraseña de un usuario de base de datos existente. Solo `X` (reset explícito) lo hace: actualiza el `.env` de cada app que use ese usuario.

### `SSL skipped: Let's Encrypt would fail` { #ssl-skipped-lets-encrypt-would-fail }

El dominio todavía no apunta al servidor. Corrige el DNS y vuelve a correr el paso de SSL.

### `release is missing required PHP dependencies` { #release-is-missing-required-php-dependencies }

versa revisa que la carpeta vendor de composer haya llegado al release, siguiendo `config.vendor-dir` de `composer.json` (por ejemplo `versaWYS/vendor`). Revisa que esa carpeta no esté en `ignored_paths`.

## Después del deploy { #after-the-deploy }

### El sitio sigue mostrando la versión anterior { #the-site-still-shows-the-old-version }

PHP-FPM guarda en caché a dónde apuntaba `current`. Agrega la recarga a `services_reload` y córrela ahora:

```yaml
services_reload:
  - "sudo systemctl reload php8.2-fpm"
```

```bash
versa services-reload production
```

Con Nginx, usa además `$realpath_root` en `SCRIPT_FILENAME`/`DOCUMENT_ROOT`. Mira la [Guía completa](GETTING_STARTED.md#24-web-server).

### 404 / document root equivocado { #404-wrong-document-root }

La app vive en `current/app/`, así que el web root es `<remote_path>/current/app/public` (o la carpeta pública de tu framework), no `<remote_path>/current/public`.

### `.env` se convirtió en un directorio vacío { #env-became-an-empty-directory }

Con versa anterior a la 1.14, una ruta compartida que no existía en `shared/` en el primer deploy se creaba como directorio (ahora el primer deploy crea `.env` desde `.env.example`, o vacío). Corrígelo en el servidor:

```bash
rmdir /var/www/app/shared/.env && nano /var/www/app/shared/.env
```

### Hice rollback, pero PHP sigue sirviendo el código nuevo { #rolled-back-but-php-still-serves-the-new-code }

Los rollbacks corren `services_reload`; revisa que recargue PHP-FPM (mira arriba) y que sus comandos funcionen (una falla se muestra como advertencia). Vuelve a correrlo con `versa services-reload <entorno>`.
