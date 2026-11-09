# 🧩 Servicios: preparar el servidor, actualizaciones y backups { #services-server-setup-updates-and-backups }

La vista **Services** de la TUI (`versa`, pestaña *Services*) convierte a versaDeploy en un pequeño panel de servidor, sin agentes: detecta qué corre en el servidor, instala el stack que tu app necesita, aplica actualizaciones, avisa de problemas de seguridad, muestra logs y programa backups de la base de datos. Todo va por SSH: en el servidor no queda instalado nada más que los servicios que elijas.

> [!NOTE]
> La detección funciona en cualquier servidor Linux. **Instalar y los backups necesitan Debian/Ubuntu (apt) o RHEL/Rocky/Alma 8+ (dnf)**; las actualizaciones desde la vista (`U`/`S`/`Ctrl+R`) son solo para apt. En otros sistemas la vista es de solo lectura. Particularidades de la familia RHEL: [§5.1](#51-rhel-family-rockyalma).

---

## 1. Requisitos { #1-requirements }

| Qué | Por qué |
| :--- | :-- |
| Un entorno en `deploy.yml` con `ssh` y `remote_path` | La vista trabaja sobre el entorno activo. |
| `provision.ssh` con un usuario root (o con sudo sin contraseña) | Las instalaciones y actualizaciones corren como root. La detección usa la conexión normal de deploy. |
| El servidor en tu `known_hosts` | versa rechaza llaves de host desconocidas: `ssh-keyscan -p PUERTO HOST >> ~/.ssh/known_hosts`. |
| `<ssh.key_path>.pub` junto a tu llave privada | Se convierte en el `authorized_keys` del usuario de deploy cuando versa lo crea. |

Bloque mínimo:

```yaml
environments:
  production:
    ssh: { host: 203.0.113.10, user: deploy, key_path: ~/.ssh/id_ed25519 }
    remote_path: /var/www/myapp
    provision:
      ssh: { user: root }          # host, puerto y llave se heredan de ssh
      domain: myapp.com
```

---

## 2. De un servidor nuevo al primer deploy { #2-from-a-fresh-server-to-the-first-deploy }

1. Apunta el DNS del dominio al servidor y corre `ssh-keyscan` (mira arriba).
2. Ejecuta `versa` y abre **Services** (← / → o haz clic en la pestaña). El usuario de deploy todavía no existe: la vista detecta a través de la conexión root y muestra *"Deploy user … does not exist yet"*.
3. Revisa los servicios preseleccionados (vienen del backend de tu app, mira §4), ajusta los **Parámetros** y presiona **`I`** → **`Y`**. El log se ve en vivo.
4. O presiona **`P`** (*prepare for deploy*) para crear solo el usuario de deploy (con tu llave pública), sus reglas de sudoers y el directorio del proyecto.
5. Presiona **`w`** para guardar el bloque `provision:` y **`a`** para agregar a `deploy.yml` los comandos de `services_reload` sugeridos. Haz commit de `deploy.yml`.
6. Ve a **Operations**, activa **Initial deploy** y presiona **`D`**.

---

## 3. Teclas { #3-keys }

| Tecla | Acción |
| :-- | :----- |
| `↑` `↓` | Moverse entre servicios y parámetros |
| `Espacio` | Marcar / desmarcar un servicio (los grupos de web server y base de datos son excluyentes) |
| `Enter` | Sobre un servicio: marcarlo. Sobre un parámetro: editarlo. Sobre una versión: la siguiente versión |
| `e` | Escribir el valor de un parámetro (también para versiones) |
| `I` | Instalar / configurar la selección (pide confirmación) |
| `P` | Preparar el servidor para deploys: usuario de deploy + llave, sudoers, directorio |
| `U` | Actualizar el servicio bajo el cursor (solo sus paquetes) |
| `S` | Aplicar todas las actualizaciones de **seguridad** pendientes |
| `Ctrl+R` | Refrescar el índice de paquetes (`apt-get update`) |
| `L` | Ver las últimas líneas de log del servicio bajo el cursor (journal + archivos de log) |
| `B` | Hacer un backup de la base de datos ahora |
| `X` | Resetear la contraseña del usuario de la base de datos (explícito, mira §6) |
| `w` | Guardar los parámetros como `provision:` en `deploy.yml` (se conservan los comentarios) |
| `a` | Agregar a `deploy.yml` los comandos de `services_reload` que faltan |
| `F5` | Volver a detectar |
| `Esc` | Cerrar el log / cancelar una confirmación |

---

## 4. Qué detecta { #4-what-it-detects }

Una sola conexión SSH (shell POSIX, funciona también en servidores antiguos) informa:

- **Servicios**: estado (`● active`, `● stopped`, `○ absent`) y versión de nginx, Apache, PHP-FPM, FrankenPHP, PostgreSQL, MariaDB/MySQL, Redis, Dragonfly, Certbot, Python, Node.js, UFW, Fail2ban, el servicio de la app (desde `services`), el usuario de deploy, sus sudoers, el directorio del proyecto y el timer de backups.
- **Actualizaciones pendientes** por servicio (`↑ N update(s)`), en rojo cuando son de seguridad.
- **Aviso de seguridad** (también en el Dashboard): actualizaciones de seguridad pendientes, *reboot required*, unattended-upgrades apagado, índice de paquetes con más de 7 días.
- **Sitios en este servidor**: cada vhost activo con sus dominios y su document root. Un vhost hecho a mano o por certbot con otro nombre se asocia a tu app por su document root.
- **SSL**: el certificado que sirve tu dominio (revisado desde tu computador), con su fecha de vencimiento. En rojo si quedan menos de 14 días.
- **El backend de la app**: desde los `builds` de `deploy.yml`, o desde los archivos del repo cuando no hay:

| Backend | Se detecta por | Preselecciona |
| :------ | :------------ | :---------- |
| PHP | `builds.php` / `composer.json` | PHP-FPM + web server con FastCGI |
| Python | `builds.python` / `pyproject.toml`, `requirements.txt`, `Pipfile` | Python + reverse proxy a `web_port` + una entrada en [`services`](DEPLOY.md#services-services) |
| Go | `builds.go` / `go.mod` | reverse proxy + una entrada en `services` para el binario |
| Node | `package.json` con un script `start`, una entrada de `services` que corre `node`/`npm` (por ejemplo el `.output/server` de Nuxt), o `provision.node_major` (no un frontend estático) | Node.js + reverse proxy + una entrada en `services` que corre `npm start` |

Las apps Python, Go y Node corren como [`services`](DEPLOY.md#services-services) de `deploy.yml`: **cada deploy** las reinicia (systemd, OpenRC o SysV) y revierte si no se mantienen arriba. Cuando el entorno no tiene `services`, la vista propone uno (nombre, `exec` para Node, `PORT` = `proxy_port`) y `w` lo escribe en `deploy.yml`. El paso *Sudoers*, corrido como root, instala y habilita sus archivos de init y agrega las reglas que necesitan los deploys; hace lo mismo que `versa service <entorno> setup`. El usuario de deploy solo puede iniciarlas, detenerlas y reiniciarlas: nunca escribe un archivo que ejecute root.

Los servicios que ya están activos nunca se preseleccionan.

---

## 5. Servicios que se pueden instalar { #5-installable-services }

| Servicio | Qué hace |
| :------ | :----------- |
| Paquetes base | curl, git, unzip, build-essential, ufw, cron (para los schedulers de las apps)…, más `provision.packages` (sin actualizar el sistema: para eso `U`/`S`) |
| PHP-FPM + OPCache | PHP desde ondrej (Ubuntu) o sury (Debian), extensiones para tu base de datos y Redis, `php.ini` de producción y OPCache ajustado para deploys con symlink (`revalidate_path=1`) |
| FrankenPHP | Alternativa a PHP-FPM (`frankenphp: true`): el build estático oficial (versión fija, sha256 verificado: PHP 8.5 con pdo_pgsql/mysql, redis, intl, gd, opcache, zip…) en `/usr/local/bin`, `php` para hooks y cron (`frankenphp php-cli`, salvo que ya exista un PHP CLI), y la configuración del proyecto `/etc/frankenphp/<proyecto>.Caddyfile`: 127.0.0.1:`proxy_port` (por defecto 8000), `php_ini` de producción, IP del cliente y `HTTPS` desde nginx. La app corre como una entrada de `services` (`w` la propone) que los deploys **nunca reinician**: OPcache identifica los scripts por su ruta real (`opcache.revalidate_path`), así que las requests corren el release nuevo a los 2 s del cambio, cada una completa en un solo release, y después el deploy libera los scripts del release anterior con `opcache_reset()` (`reload`) por el socket privado del proyecto `<remote_path>/.frankenphp-opcache.sock`. No se pierde ninguna request (reiniciar FrankenPHP o `frankenphp reload` cortan las que están en curso). El vhost de nginx mantiene las reglas de versaWYS/Laravel y sirve los estáticos; solo `/index.php` pasa por el proxy, con la URI original, y FrankenPHP ejecuta `index.php` en cada request. Solo con nginx; x86_64 y arm64 |
| Apache2 / Nginx | Web server + el vhost del proyecto (desde las plantillas de versaWYS): headers de seguridad, rate limiting (nginx: 10 req/s por IP, burst 50, solo en las requests que llegan a la app; los estáticos nunca se limitan, tampoco detrás de un proxy: los chunks de Next/Nuxt), caché, archivos sensibles bloqueados, `$realpath_root` para PHP. Volver a correr el paso regenera el vhost (con respaldo) sin tocar el web server |
| SSL | `certbot certonly` para el dominio y los alias que apuntan al servidor, y luego el vhost SSL. Se salta con una indicación cuando el DNS no apunta acá. Con `ssl_self_signed: true` (staging, red local, Docker): un certificado autofirmado en `/etc/ssl/versa/<dominio>/`, en el que confía el propio servidor |
| PostgreSQL | Servidor + base de datos + usuario, regla en `pg_hba.conf` para 127.0.0.1 |
| MariaDB / MySQL | MariaDB, o el MySQL que ya tenga el servidor; base de datos (utf8mb4) + usuario |
| Redis | Escuchando en 127.0.0.1, protected mode, `maxmemory 256mb` |
| Dragonfly | Alternativa compatible con Redis (mismo protocolo y puerto, multihilo): el paquete oficial (versión fija, sha256 verificado), en 127.0.0.1, `maxmemory 256mb`, `cache_mode` (descarta como `allkeys-lru`), 1 hilo (Dragonfly necesita 256 MB por hilo: sube ambos juntos en `/etc/dragonfly/dragonfly.conf`), sin consultas de versión a internet. x86_64 y arm64 (Debian/Ubuntu); x86_64 en RHEL. Las apps siguen usando phpredis o cualquier cliente de Redis |
| Python 3 | El Python del sistema, u otra versión junto a él (Ubuntu: deadsnakes) |
| Node.js | NodeSource LTS |
| Usuario de deploy + llave SSH | Crea el usuario, lo agrega a `www-data` e instala tu llave pública |
| Sudoers | `reload`/`restart` del web server y PHP-FPM, más las reglas de los `services` del entorno, validado con `visudo -c`. Un archivo por proyecto |
| Directorio del proyecto | `remote_path` del usuario de deploy, grupo `www-data`, setgid. Los releases se extraen con `umask 002`, y un PHP-FPM nuevo recibe `UMask=0002` (drop-in de systemd), así el usuario de deploy (hooks, cron) y PHP-FPM pueden escribir los archivos del otro en todas las distribuciones |
| Backups de la base de datos | Timer diario de systemd que respalda la base de datos en `remote_path/backups` (§8) |
| Cron jobs | `provision.cron` en `/etc/cron.d/versa-<proyecto>`: cada tarea corre como el usuario de deploy en `current/app` (por ejemplo `app:schedule:run` de versaWYS). Se reescribe completo en cada ejecución |
| Firewall UFW | Permite SSH (el puerto en uso), 80 y 443 |
| Fail2ban | Solo bloquea abuso inequívoco, nunca el tráfico normal de una app (§6): logins SSH fallidos (jail `sshd` sobre el journal de systemd, el puerto SSH en uso, 5 intentos en 10 min) y escáneres de vulnerabilidades (`versa-scan`: 3 requests en 10 min a `.env`, `.git/`, phpMyAdmin, `vendor/phpunit`, `wp-login.php`… **que respondieron 404/403/444**: una ruta que una app sirve de verdad nunca cuenta). Los bloqueos empiezan en 1 h (escáneres: 1 día) y crecen hasta una semana para los reincidentes. Bloquea con UFW, firewalld o nftables, según lo que use el servidor. Nunca bloquea: localhost, la IP del administrador mientras prepara el servidor, `provision.fail2ban_ignore`. Una configuración por servidor (`/etc/fail2ban/jail.d/versa.conf`); un fail2ban configurado a mano no se toca |

### Distribuciones soportadas { #distro-support }

| Servicio | Debian / Ubuntu (apt) | RHEL / Rocky / Alma 8+ (dnf) | e2e |
| :------ | :-------------------- | :--------------------------- | :-- |
| Paquetes base, `provision.packages` | ✅ | ✅ (+ EPEL) | todos |
| PHP-FPM + OPCache | ✅ ondrej (Ubuntu) / sury (Debian) | ✅ AppStream, o Remi para otras versiones y phpredis | Ubuntu, Debian 8.4 · Rocky 8.3 · Alma 8.4 |
| FrankenPHP | ✅ | ✅ (nginx; revisa SELinux en servidores en modo enforcing) | Ubuntu (versaTodo + Laravel) |
| Nginx | ✅ | ✅ | Ubuntu, Rocky, Alma |
| Apache2 | ✅ | ❌ rechazado: usa nginx | Debian |
| SSL — Let's Encrypt | ✅ | ✅ (certbot desde EPEL) | — (necesita DNS público) |
| SSL — autofirmado | ✅ | ✅ | manual (AlmaLinux) |
| PostgreSQL | ✅ | ✅ | Ubuntu, Rocky |
| MariaDB / MySQL | ✅ | ✅ | Debian, Alma |
| Redis | ✅ | ✅ | Ubuntu, Rocky |
| Dragonfly | ✅ x86_64 + arm64 (.deb) | ✅ x86_64 (.rpm) | Debian, Alma |
| Python 3 + venv | ✅ del sistema, o deadsnakes (Ubuntu) | ✅ del sistema, o `python3.X` de AppStream | Ubuntu, Rocky, Alma |
| Node.js | ✅ NodeSource | ✅ módulo de AppStream | Ubuntu, Rocky, Alma |
| Usuario de deploy, sudoers, directorio, backups, cron jobs | ✅ | ✅ (+ etiquetas SELinux) | todos |
| UFW | ✅ | ❌ rechazado: firewalld (nginx abre 80/443) | — |
| Fail2ban | ✅ (UFW o nftables) | ✅ EPEL (firewalld o nftables) | todos (se bloquea un escáner real) |
| Actualizaciones `U` / `S` / `ctrl+r` | ✅ | ❌ `dnf upgrade` a mano | Ubuntu, Debian |

Los deploys (builds, releases, hooks, `services`, rollback) son iguales en ambas: solo necesitan SSH y herramientas POSIX en el servidor.

### 5.1 Familia RHEL (Rocky/Alma) { #51-rhel-family-rockyalma }

Los mismos pasos sobre `dnf`, con estas diferencias:

- **Web server: solo nginx.** Apache se instala solo en Debian/Ubuntu (el vhost va en `/etc/nginx/conf.d/<proyecto>.conf`). UFW se rechaza: el paso de nginx abre `http`/`https` en firewalld cuando está activo.
- **PHP y Node.js vienen de los module streams de AppStream** (`php:8.3`, `nodejs:22`): el paso revisa que los repositorios ofrezcan la versión pedida antes de instalar, y nunca cambia el stream de uno ya instalado. Cuando AppStream no tiene la versión de PHP (EL9 llega hasta 8.3) o la app necesita Redis/Dragonfly (phpredis), se agrega el **repositorio de Remi** (`php:remi-8.4`, como ondrej/sury en Debian). También se instalan `php-process` (posix/pcntl) y `php-sodium`: en RHEL no vienen en `php-common`.
- PHP-FPM escucha en `/run/php/php<v>-fpm.sock` y responde a `php<v>-fpm` igual que en Debian (tmpfiles.d mantiene `/run/php` entre reinicios). El pool sigue corriendo como `apache:apache` (su carpeta de sesiones es del grupo `apache`) y `apache` se une a `www-data`.
- **EPEL** se habilita para el módulo headers-more de nginx, certbot y htop (Rocky y Alma lo traen; en RHEL actívalo a mano).
- **SELinux**: `remote_path` se etiqueta `httpd_sys_rw_content_t`, `/run/php` `httpd_var_run_t`, se restaura `~/.ssh`, y se activa `httpd_can_network_connect` para las apps detrás de un proxy. El e2e corre en Docker sin SELinux, así que revisa `ausearch -m avc` en el primer servidor en modo enforcing.
- Actualizaciones: `dnf upgrade` a mano (`U`/`S` de la vista son solo para apt).

### Versiones { #versions }

Los parámetros de versión rotan con `Enter` entre las versiones **con soporte**, de la más nueva a la más antigua, siempre al menos tres:

- **PHP**: por ejemplo `8.5 8.4 8.3 8.2`
- **Node.js**: solo versiones LTS, por ejemplo `26 24 22`
- **Python**: `system` más, por ejemplo, `3.15 3.14 3.13 3.12 3.11` (otra distinta a la del sistema: en Ubuntu vía deadsnakes, en la familia RHEL cuando AppStream la empaqueta, por ejemplo `python3.12`)

La lista viene de [endoflife.date](https://endoflife.date) (una vez por sesión, con 4 s de límite) y, sin conexión, usa una lista incluida. Si la app usa otra versión de Python, configura también `builds.python.python_command: python3.13`.

---

## 6. Reglas de seguridad en servidores en uso { #6-safety-rules-on-servers-already-in-use }

Los scripts están hechos para volver a correrse en servidores de producción:

- **Nunca una alternativa**: con Apache instalado se rechaza nginx (y al revés); con Redis instalado se rechaza Dragonfly (y al revés: ambos usan el puerto 6379); con MySQL instalado se reutiliza, nunca se instala MariaDB encima; con una versión de PHP-FPM o Node.js instalada se rechaza otra; un solo PHP por servidor: con PHP-FPM instalado se rechaza FrankenPHP (y al revés). La TUI bloquea la selección y el script del servidor se detiene antes de `apt-get`.
- **Sin actualizaciones sorpresa**: los paquetes ya instalados se saltan, nunca se actualizan. Las actualizaciones solo ocurren con `U` / `S`.
- **Los servicios existentes mantienen su configuración y siguen corriendo**: no se cambia `php.ini`, OPCache, `postgresql.conf`, ni la configuración de MySQL/MariaDB o Redis, y no se reinician. Solo se agrega lo que falta (una extensión de PHP, un módulo de Apache), con una recarga suave cuando hace falta.
- **Los usuarios de base de datos existentes conservan su contraseña**: otras apps pueden usarlos. Se asegura la base de datos y sus permisos. Cambiar la contraseña es una acción explícita: **`X`** (y luego actualiza cada `.env` que la use).
- **Un vhost por dominio**: si otro vhost ya sirve el dominio, la instalación se detiene y lo nombra. Los vhosts se respaldan antes de reemplazarlos y se restauran si el nuevo falla `nginx -t` / `apachectl configtest`. El sitio por defecto solo se elimina en un web server instalado en esa misma ejecución.
- **Firewall**: un UFW activo mantiene su política y sus reglas; solo se permiten SSH, 80 y 443.
- **Fail2ban nunca bloquea tráfico normal**: ninguna jail cuenta los 429 (el rate limit de nginx), los 401/403/422 de APIs y paneles, los logins ni los webhooks de pago (Transbank, Mercado Pago, Flow…), así que un panel con mucho uso o clientes detrás de una misma IP (NAT) nunca quedan bloqueados. Solo cuentan los logins SSH fallidos y las requests a rutas de escáneres que no existen en el servidor. Un fail2ban configurado a mano mantiene sus jails.
- **`current/` nunca se crea a mano**: sigue siendo un enlace que crea el primer deploy.
- **Secretos**: la contraseña de la base de datos viaja en un archivo dentro de una carpeta temporal privada (0700) que se borra al terminar, nunca en una línea de comando. `w` nunca escribe `db_password` en `deploy.yml`: déjalo como `${DB_PASS}`.

---

## 7. Varias apps en un servidor { #7-several-apps-on-one-server }

Cada proyecto tiene su propio vhost (`sites-available/<proyecto>`), su archivo de sudoers (`/etc/sudoers.d/<usuario>-<proyecto>`), sus `services` y su timer de backups. Crea un entorno (o un archivo de configuración) por app, con su propio `project`, `remote_path` y `provision.domain`, e instala desde cada uno. *Sitios en este servidor* los lista todos.

Los dominios extra van en `provision.aliases` (por ejemplo `www.myapp.com`): se agregan al vhost, y al certificado cuando apuntan al servidor.

---

## 8. Backups { #8-backups }

El paso **Backups de la base de datos** instala `/usr/local/sbin/versa-backup-<proyecto>` y un timer diario de systemd (`versa-backup-<proyecto>.timer`, con hasta 30 min aleatorios, y se pone al día después de que el servidor estuvo apagado). Cada ejecución escribe `remote_path/backups/<bd>-<fecha>.sql.gz` (`pg_dump` / `mysqldump --single-transaction`) y conserva los `backup_keep` respaldos más nuevos (7 por defecto).

- `B` hace un backup ahora y muestra su tamaño.
- La carpeta es del usuario de deploy (`0700`; el directorio remoto es setgid `www-data`, así los respaldos quedan privados): descárgalos desde la vista **Files**.
- Restaurar es manual a propósito (sobrescribe la base de datos), por ejemplo `gunzip -c dump.sql.gz | sudo -u postgres psql mydb`.

---

## 9. Referencia de `provision` { #9-provision-reference }

| Campo | Por defecto | Descripción |
| :---- | :------ | :---------- |
| `ssh.user` / `ssh.key_path` / `ssh.port` | los de `ssh` | Conexión de administrador (root) para instalar y actualizar |
| `domain` | - | Dominio principal del vhost y del certificado |
| `aliases` | `[]` | Dominios extra (vhost + certificado) |
| `web_server` | el detectado, si no `nginx` | `nginx` o `apache` |
| `php_version` | la instalada, si no la más nueva con soporte | Versión de PHP-FPM |
| `frankenphp` | `false` | Correr la app PHP con FrankenPHP (un servicio en `proxy_port`, 8000 por defecto) en vez de PHP-FPM; mira [examples/deploy.laravel-frankenphp.yml](../examples/deploy.laravel-frankenphp.yml) |
| `python_version` | la del sistema | Python instalado junto al del sistema |
| `node_major` | `engines.node`, si no la LTS más nueva | Versión mayor de Node.js |
| `doc_root` | `current/app` | Document root relativo a `remote_path` (los releases guardan el código en `app/`; por ejemplo `current/app/public` para Laravel) |
| `database` | la detectada | `postgres`, `mariadb` o `none` |
| `db_name` / `db_user` | a partir de `project` | Nombres de la base de datos y del usuario (letras, dígitos, `_`) |
| `db_password` | - | Contraseña para un usuario **nuevo** (usa `${VAR}`; la TUI nunca la escribe) |
| `redis` / `dragonfly` / `ssl` / `ufw` / `fail2ban` / `backup` | `false` | Preseleccionar estos servicios (`redis` y `dragonfly` se excluyen entre sí) |
| `fail2ban_ignore` | `[]` | IPs o CIDRs que fail2ban nunca bloquea (oficina, monitoreo, CI), además de localhost y la IP del administrador |
| `backup_keep` | `7` | Respaldos que se conservan |
| `ssl_self_signed` | `false` | El paso SSL crea un certificado autofirmado en vez de pedirlo a Let's Encrypt |
| `packages` | `[]` | Paquetes del sistema extra que necesita la app (OCR, herramientas de imágenes…), instalados por el paso base, nunca actualizados |
| `cron` | `[]` | `"<min> <hora> <día> <mes> <día_semana> <comando>"` o `"@daily <comando>"`, corre como `deploy_user` en `current/app`; escribe `%` como `\%` |
| `deploy_user` | `ssh.user` | Usuario dueño de los releases |
| `backends` | los detectados | Los escribe la TUI como referencia |
| `proxy_port` | según builds / backend | Puerto de una app Python/Go/Node detrás del web server (se le pasa a su servicio como `PORT`), o el de FrankenPHP |

---

## 10. Tests { #10-testing }

`go test ./...` cubre el análisis de la detección, las plantillas, los bundles (`bash -n`), los conflictos y la vista de la TUI. El test end-to-end necesita un Docker capaz de correr contenedores con systemd:

```bash
go test -tags e2e -v -timeout 60m ./test/e2e/
```

Prepara cinco servidores en paralelo: **Ubuntu 24.04** (nginx + PostgreSQL + Redis), **Rocky Linux 9** (nginx + PostgreSQL + Redis sobre dnf, PHP 8.3 de Remi) y **AlmaLinux 9** (nginx + MariaDB + Dragonfly, PHP 8.4 de Remi), cada uno con cuatro apps de ejemplo (PHP, Python, Go y Node, cada una con su dominio; Ubuntu y Alma también Laravel, Next, Nuxt, Flask, FastAPI y Gin), **Debian 12** (Apache + MariaDB + Dragonfly) con versaTodo (`../versaTodo`, se salta si no está) y **franken** (Ubuntu con FrankenPHP: versaTodo y Laravel). Los despliega y revisa cada página como la carga un navegador (redirecciones, contenido y todos sus assets), que no falle ninguna request durante un deploy, fail2ban (bloquea un escáner real, nunca el tráfico del propio e2e), HTTP y sesiones de PHP, alias, backups, cron jobs y paquetes extra, phpredis contra el servidor de caché (y que solo escuche en 127.0.0.1), releases con permisos de grupo y el umask de PHP-FPM, logs, el listado de sitios, que volver a correr la instalación no cambie contraseñas, configuraciones ni procesos, y que se rechace un web server alternativo. `VERSA_E2E_KEEP=1` conserva los contenedores; `VERSA_E2E_SERVERS=rocky` corre solo esos servidores.
