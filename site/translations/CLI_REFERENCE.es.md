# 📖 Comandos (CLI y TUI) { #cli-reference }

Cada comando de `versa`, sus argumentos y opciones, y lo que hace en tu computador y en el servidor.

- [Convenciones](#conventions)
- [Opciones globales](#global-flags)
- [Elegir el archivo de configuración](#choosing-the-configuration-file)
- Comandos
  - Configuración: [`init`](#versa-init) · [`config validate`](#versa-config-validate) · [`ssh-test` / `info`](#versa-ssh-test-versa-info)
  - Desplegar: [`deploy`](#versa-deploy) · [`diff`](#versa-diff) · [`rollback`](#versa-rollback) · [`status`](#versa-status) · [`history`](#versa-history)
  - Operar el servidor: [`exec`](#versa-exec) · [`logs`](#versa-logs) · [`hooks`](#versa-hooks) · [`services-reload`](#versa-services-reload) · [`service`](#versa-service)
  - La herramienta: [`version`](#versa-version) · [`self-update`](#versa-self-update)
- [Interfaz interactiva (TUI)](#interactive-tui)
- [Qué comandos funcionan en modo local](#which-commands-work-in-local-mode)
- [Códigos de salida y errores](#exit-codes-and-error-output)

---

## Convenciones { #conventions }

```
versa <comando> <entorno> [argumentos] [opciones]
```

- `<entorno>` es una clave bajo `environments:` de tu archivo de configuración (`production`, `staging`, …). Es obligatorio en cada comando que toca un servidor.
- Corre `versa` **desde la raíz de tu repositorio Git**. versa despliega el repositorio de la carpeta actual y busca ahí el archivo de configuración.
- `versa <comando> --help` muestra la ayuda de cualquier comando.

## Opciones globales { #global-flags }

Disponibles en todos los comandos.

| Opción             | Por defecto  | Descripción                                                                                 |
| :----------------- | :----------- | :------------------------------------------------------------------------------------------ |
| `--config <ruta>`  | `deploy.yml` | Archivo de configuración a usar. Desactiva la búsqueda automática (mira abajo).             |
| `--verbose`        | `false`      | Salida más detallada.                                                                       |
| `--debug`          | `false`      | Salida de depuración (pasos internos, comandos remotos).                                    |
| `--log-file <ruta>`| —            | Escribe también el log en este archivo. Útil para guardar un historial: `--log-file deploy.log`. |
| `--no-gui`         | `false`      | Sin comando, muestra la ayuda en vez de abrir la TUI.                                       |
| `--gui`            | `false`      | Abre la TUI. Se mantiene por compatibilidad: `versa` a secas ya lo hace.                    |
| `-v`, `--version`  | —            | Muestra la versión (igual que `versa version`).                                             |
| `-h`, `--help`     | —            | Ayuda de cualquier comando.                                                                 |

## Elegir el archivo de configuración { #choosing-the-configuration-file }

Si no pasas `--config`, versa busca en la carpeta actual archivos con alguno de estos nombres (`.yml` o `.yaml`):

```
deploy.yml   deploy_*.yml   versa_deploy.yml   versa_deploy_*.yml   *_deploy.yml   *_deploy_*.yml
```

| Archivos encontrados | Qué pasa                                                              |
| :------------------- | :-------------------------------------------------------------------- |
| Ninguno              | Se usa `deploy.yml` (y el comando falla si no existe).                |
| Uno                  | Se usa ese archivo.                                                   |
| Varios               | versa los lista y te pide elegir uno por número.                      |

En scripts y CI pasa siempre `--config`, así versa nunca se detiene a preguntar:

```bash
versa deploy production --config versa_deploy_frontend.yml
```

> [!NOTE]
> `versa init` no busca archivos: siempre escribe en la ruta de `--config` (por defecto `deploy.yml`).

---

## `versa init` { #versa-init }

Crea una configuración inicial comentada en la carpeta actual.

```bash
versa init                       # pregunta: [1] VPS/servidor por SSH, [2] hosting local sin SSH
versa init --local               # sin preguntar, genera una configuración en modo local
versa init --config deploy_staging.yml
```

| Opción    | Por defecto | Descripción                                                                    |
| :-------- | :---------- | :----------------------------------------------------------------------------- |
| `--local` | `false`     | Genera un entorno en [modo local](DEPLOY.md#local-mode-no-ssh) sin preguntar.  |

Falla si el archivo ya existe. Edita el archivo generado y luego corre `versa config validate`. También puedes partir de los [Ejemplos](../examples.md).

---

## `versa config validate` { #versa-config-validate }

Lee y valida el archivo de configuración **sin conectarse a ningún servidor**.

```bash
versa config validate               # valida todos los entornos
versa config validate production    # además muestra un resumen de un entorno
```

Revisa los campos obligatorios, que `ssh.key_path` exista (y en Linux/macOS, que tenga `chmod 600`), que `remote_path` sea absoluto, que haya al menos un build activo y que cada build activo tenga sus campos obligatorios. Con un nombre de entorno muestra:

```
✅ deploy.yml is valid (2 environment(s): production, staging)

Environment "production":
  mode:        ssh
  ssh:         deploy@10.0.0.5:22
  remote_path: /var/www/app
  builds:      php, frontend
  hooks:       pre_deploy_local=1 pre_deploy_server=0 post_deploy=2
```

---

## `versa ssh-test` (`versa info`) { #versa-ssh-test-versa-info }

`info` es un alias de `ssh-test`. Se conecta al servidor, muestra lo que encuentra y comprueba que SFTP funcione.

```bash
versa info production
```

```
🔍 Testing SSH connection to production (deploy@10.0.0.5)...
✅ SSH connection established successfully!
🔍 Probing server...
   Host:            web01
   OS:              Ubuntu 22.04.4 LTS
   Kernel:          5.15.0-105-generic x86_64
   Init / libc:     systemd / glibc
   CPU:             Intel(R) Xeon(R) (4 cores)
   RAM:             2.1G / 7.8G
   Disk (deploy):   12G / 40G (30%)
   php:             8.2.18
   node:            —
   tools:           tar gzip timeout perl ...
🔍 Testing SFTP subsystem...
✅ SFTP subsystem working.
```

Córrelo primero en un servidor nuevo: muestra los problemas de SSH, de llave de host y de permisos antes de un deploy real. Cuando el entorno tiene [`services`](DEPLOY.md#services-services), revisa además que el usuario SSH pueda reiniciarlos con sudo sin contraseña. Avisa si falta `tar` en el servidor, porque los deploys lo necesitan. La revisión es POSIX pura y funciona también en servidores antiguos (RHEL/CentOS 5, busybox).

---

## `versa deploy` { #versa-deploy }

Compila el estado commiteado de tu repositorio y lo despliega en uno o más entornos.

```bash
versa deploy production
versa deploy production --initial-deploy     # primer deploy a este entorno
versa deploy production --dry-run            # muestra qué cambiaría; no despliega nada
versa deploy production --force              # vuelve a desplegar aunque no haya cambios
versa deploy staging,production              # varios entornos, uno tras otro
```

| Opción               | Por defecto | Descripción                                                                                                                                                                    |
| :------------------- | :---------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--initial-deploy`   | `false`     | Permite desplegar cuando el servidor aún no tiene `deploy.lock` (primer deploy). Además pide confirmación antes de correr los hooks `post_deploy`, para que configures el `.env` primero. |
| `--dry-run`          | `false`     | Se conecta, detecta los cambios y los lista, y se detiene. No compila ni sube nada, y no corre los hooks `pre_deploy_local`.                                                 |
| `--force`            | `false`     | Despliega aunque ningún archivo haya cambiado desde el último deploy (por ejemplo para volver a desplegar después de un rollback).                                          |
| `--skip-dirty-check` | `false`     | No rechaza el deploy cuando hay cambios sin commitear. Esos cambios igual **no** se despliegan: versa siempre despliega el último commit (`HEAD`).                          |

### Qué hace un deploy { #what-a-deploy-does }

1. **Revisiones locales**: las herramientas de build están en el `PATH`, la carpeta es un repositorio Git, los hooks `pre_deploy_local` pasan, y no hay cambios sin commitear (salvo `--skip-dirty-check`).
2. **Clonar**: `HEAD` se clona en una carpeta temporal. Solo se despliegan archivos commiteados.
3. **Conectar y bloquear**: abre SSH y crea `<remote_path>/.versa.lock` para que no corran dos deploys a la vez.
4. **Detectar cambios**: compara los hashes SHA256 de cada archivo con el `deploy.lock` del servidor. Sin cambios, versa muestra `No changes detected` y se detiene (salvo `--force`).
5. **Revisión previa**: revisa el servidor (sistema, `tar` y, para builds de Go, si el target corresponde al kernel y la arquitectura del servidor).
6. **Compilar**: corre los builders activos (PHP/Go/Frontend/Python) en paralelo.
7. **Subir**: comprime el release en partes y las sube por SFTP en paralelo, a `releases/<versión>/`. Con `incremental_upload: true`, solo se envían los archivos que cambiaron.
8. **Preparar**: enlaza los `shared_paths`, reutiliza dependencias del release anterior (`vendor`, `node_modules`, `.venv` …) con hardlinks, restaura los `preserved_paths` y, para Python, crea el virtualenv en el servidor cuando no se pudo reutilizar.
9. **Activar**: corre los hooks `pre_deploy_server`, cambia `current → releases/<versión>` de forma atómica, corre `services_reload`, corre los hooks `post_deploy`, instala y reinicia los [`services`](DEPLOY.md#services-services) (revisando que se mantengan arriba), y luego el `health_check`.
10. **Terminar**: escribe el nuevo `deploy.lock`, borra los releases antiguos más allá de `releases_to_keep`, envía la notificación, registra el deploy en el historial y libera el bloqueo.

Si falla un hook `post_deploy`, un servicio o el health check, `current` vuelve al release anterior (y los services se reinician sobre él). Mira las reglas de los hooks en [Configuración](DEPLOY.md#hooks).

### Varios entornos { #multiple-environments }

`versa deploy staging,production` despliega en cada entorno en orden, con un build separado por entorno, y se detiene en el primer error. Para compilar una vez y enviar el mismo artefacto a varios servidores (un archivo de configuración por servidor), usa el multi-deploy de la [TUI](#interactive-tui) (vista Switch Config).

### Entornos en modo local { #local-mode-environments }

Para un entorno con `local: true`, `deploy` compila el proyecto y escribe un release completo en `local_path` (reemplazando lo que había), y luego corre los hooks `post_deploy` en local. Sube tú el contenido de `local_path/app` a tu hosting.

---

## `versa diff` { #versa-diff }

Lista los archivos que enviaría un deploy: el `HEAD` commiteado comparado con el `deploy.lock` del servidor.

```bash
versa diff production
```

Es un deploy en modo dry-run que además se salta la revisión de cambios sin commitear (no importan, porque solo se compara `HEAD`). La salida agrupa los archivos por tipo (PHP, Twig, Go, Frontend, Python, Otros) y dice qué manifiestos de dependencias cambiaron (`composer`, `package.json`, `go.mod`, requirements de Python, rutas) y provocarán un rebuild. Solo para entornos SSH.

---

## `versa rollback` { #versa-rollback }

Vuelve a apuntar `current` a un release anterior que todavía está en el servidor.

```bash
versa rollback production                       # al release más nuevo anterior al activo
versa rollback production --to 20261001-183807  # a un release específico
```

| Opción           | Por defecto | Descripción                                                                                       |
| :--------------- | :---------- | :------------------------------------------------------------------------------------------------ |
| `--to <release>` | —           | Release a activar. Los nombres son fechas UTC `AAAAMMDD-HHMMSS`; lístalos con `versa status`.     |

- El rollback solo cambia el enlace: no se compila nada y los hooks `post_deploy` **no** corren. Las migraciones de base de datos no se revierten.
- Después del cambio corre `services_reload` y se reinician los [`services`](DEPLOY.md#services-services), para que PHP-FPM y tus procesos Go/Python sirvan de inmediato el release restaurado.
- Solo se pueden restaurar releases que sigan en el servidor (mira `releases_to_keep`). Un deploy que falla después del cambio (`post_deploy`, services o health check) vuelve atrás y borra su propio release, para que no se pueda restaurar por error.
- Sin `--to`, un segundo rollback va más atrás, nunca hacia un release más nuevo.
- `deploy.lock` no cambia. Si después quieres volver a desplegar el mismo commit, usa `versa deploy <entorno> --force`.
- El rollback queda registrado en [`versa history`](#versa-history) (quién, cuándo, a qué release), igual que los rollbacks desde la TUI.

---

## `versa status` { #versa-status }

Muestra el release activo y todos los releases del servidor.

```bash
versa status production
```

```
[INFO] Status for production:
[INFO] Current release: 20261001-183807
[INFO] Available releases: 3
[INFO]     20260930-101500
[INFO]     20260930-170212
[INFO]   → 20261001-183807
```

`→` marca el release al que apunta `current`.

---

## `versa history` { #versa-history }

Lista los deploys registrados en el servidor, del más nuevo al más antiguo (también los fallidos y los rollbacks), o muestra el log completo de un deploy.

```bash
versa history production                     # los últimos 20 (--limit N)
versa history production 20261108-150140     # el log de ese deploy
```

```
ID               STATUS   TIME    COMMIT    TAG           BY
20261108-153012  success     48s  a41f09c3  v1.2.0        ana@laptop
20261108-150140  FAILED      31s  c93a11f0                ana@laptop
                 └ post-deploy hook failed (rolled back to 20261107-101203): …
```

El servidor guarda los últimos 100 deploys y los logs de los últimos 20 en `<remote_path>/history/` ([detalles](DEPLOY.md#deploy-history)). Los deploys hechos antes de versa 1.15.0 no aparecen.

---

## `versa exec` { #versa-exec }

Corre un comando en el servidor por SSH y muestra su salida.

```bash
versa exec production "df -h"
versa exec production "sudo systemctl status php8.2-fpm"
versa exec production "tail -50 /var/log/nginx/error.log | grep 502"
```

- Todo lo que va después del entorno se une con espacios en un solo comando. **Ponlo entre comillas** para que tu shell local no interprete pipes, redirecciones ni `&&`.
- Corre en el shell de login del usuario SSH, empezando en su carpeta home (no en el release). Usa `cd /var/www/app/current/app && …` si necesitas la carpeta de la app.
- Tiene un límite de `hook_timeout` segundos (300 por defecto).

---

## `versa logs` { #versa-logs }

Sigue un archivo de log en el servidor (`tail -f`). Presiona `Ctrl+C` para detenerlo.

```bash
versa logs production                                   # <remote_path>/current/app/storage/logs/laravel.log
versa logs production /var/log/nginx/error.log
versa logs production /var/log/syslog --lines 200
```

| Argumento / opción | Por defecto                                             | Descripción                                   |
| :----------------- | :------------------------------------------------------ | :-------------------------------------------- |
| `[ruta]`           | `<remote_path>/current/app/storage/logs/laravel.log`    | Ruta absoluta del archivo en el servidor.     |
| `--lines <n>`      | `50`                                                    | Líneas de historial antes de empezar a seguir. |

La ruta por defecto es el log de Laravel. Para otros frameworks, pasa siempre la ruta.

---

## `versa hooks` { #versa-hooks }

Vuelve a correr los hooks `post_deploy` sobre el release activo, sin desplegar.

```bash
versa hooks production          # todos los hooks post_deploy, en orden
versa hooks production 0 2      # solo los hooks #0 y #2 (desde 0, en el orden de la configuración)
```

Los hooks corren en `<release activo>/app`, con `hook_timeout`. Un bloque `parallel:` cuenta como un índice. A diferencia de un deploy, aquí una falla detiene el comando pero **no** hace rollback.

---

## `versa services-reload` { #versa-services-reload }

Corre los comandos de `services_reload` del entorno (por ejemplo recargar PHP-FPM y el web server) sin desplegar.

```bash
versa services-reload production
```

Cada comando tiene 30 s de límite. Las fallas se muestran como advertencias. Útil cuando PHP sigue sirviendo código viejo, o después de cambiar la configuración de PHP o del web server.

---

## `versa service` { #versa-service }

Administra los procesos listados en [`services`](DEPLOY.md#services-services) (una API en Go, un servidor web en Python…). Los deploys ya los instalan, habilitan y reinician; usa esto para revisarlos o actuar a mano.

```bash
versa service production                     # estado de cada servicio (acción por defecto)
versa service production restart
versa service production logs --name api     # sigue el log; Ctrl+C para salir
versa service production setup | ssh root@SERVIDOR sh   # una vez, como root: instala los services + sudoers
```

| Acción      | Qué hace                                                                                           |
| :---------- | :------------------------------------------------------------------------------------------------- |
| `status`    | Si cada servicio está corriendo (`systemctl status`, o el `status` del script de init). Por defecto. |
| `start`     | Lo inicia, espera `start_wait` segundos y revisa que siga corriendo.                               |
| `stop`      | Lo detiene.                                                                                        |
| `restart`   | Lo reinicia y revisa que se mantenga arriba.                                                       |
| `reload`    | Corre el comando `reload` del servicio si está arriba (sin cortar requests), si no lo reinicia; luego revisa que se mantenga arriba. Es lo que hacen los deploys y los rollbacks. |
| `logs`      | Sigue el log: `journalctl -u <nombre>` con systemd, `/var/log/<nombre>.log` en los demás.          |
| `install`   | Reescribe los archivos del servicio y lo habilita al iniciar, sin desplegar. Con un usuario SSH que no es root solo revisa que root haya instalado la versión actual (si no: *needs root setup*). |
| `uninstall` | Lo detiene, lo deshabilita al iniciar y lo quita del sistema de init.                              |
| `setup`     | Muestra el script que **root** corre una vez en el servidor (`versa service <entorno> setup \| ssh root@SERVIDOR sh`): instala y habilita los services y escribe `/etc/sudoers.d/versa-<proyecto>`. Vuelve a correrlo después de cambiar `name`, `user`, `env_file` o `stop_timeout` de un servicio. |
| `sudoers`   | Muestra las líneas de sudoers que necesita el usuario SSH, con las rutas de los binarios de este servidor. |

| Opción        | Por defecto | Descripción                                                                 |
| :------------ | :---------- | :-------------------------------------------------------------------------- |
| `--name <n>`  | todos       | Actúa sobre un solo servicio. Obligatorio para `logs` cuando hay varios.    |
| `--lines <n>` | `50`        | `logs`: líneas de historial antes de empezar a seguir.                      |

Salvo que `ssh.user` sea `root`, los comandos corren con `sudo -n`: mira [Permisos](DEPLOY.md#permissions-root-setup-and-sudo).

---

## `versa version` { #versa-version }

```bash
versa version      # versaDeploy 1.7.0
```

## `versa self-update` { #versa-self-update }

Revisa el [último release de GitHub](https://github.com/kriollo/versaDeploy/releases) y, si es más nuevo, descarga el binario para tu sistema y arquitectura, verifica su checksum SHA256 cuando está publicado, y reemplaza el `versa` en uso.

```bash
versa self-update
```

Necesitas permiso de escritura en la carpeta donde está instalado `versa` (en Linux, `sudo` si está en `/usr/local/bin`).

---

## Interfaz interactiva (TUI) { #interactive-tui }

Correr `versa` sin comando abre una interfaz a pantalla completa (`versa --no-gui` muestra la ayuda en su lugar). Carga el archivo de configuración de la carpeta actual si hay uno; si no, usa la vista **Switch Config** para elegirlo. La conexión SSH a un entorno se abre la primera vez que la necesitas.

**En todas partes**

| Tecla            | Acción                                          |
| :--------------- | :---------------------------------------------- |
| `←` / `→`        | Vista anterior / siguiente                      |
| `Tab`            | Ir a la barra lateral (elegir vista o entorno)  |
| `↑` / `↓`, `Enter` | Moverse, seleccionar                          |
| `F5`             | Refrescar la vista actual                       |
| `c`              | Conectar / reconectar al entorno activo         |
| `D`              | Ir a Operations para desplegar                  |
| `q`, `Ctrl+C`    | Salir                                           |

**Mouse**: haz clic en una pestaña para abrirla, en un entorno de la barra lateral para cambiarte a él, y usa la rueda para desplazarte (las listas mueven su selección). Mientras la TUI usa el mouse, la mayoría de las terminales seleccionan texto con `Shift` + arrastrar.

**Vistas**

| Vista             | Qué muestra                                      | Teclas                                                                                                                       |
| :---------------- | :----------------------------------------------- | :--------------------------------------------------------------------------------------------------------------------------- |
| **Dashboard**     | Datos del servidor (igual que `versa info`), release activo, runtimes y herramientas, alertas de seguridad | `F5` refrescar                                                                 |
| **Releases**      | Releases del servidor, con el activo marcado     | `Enter` ver sus archivos · `R` volver a él (pide `y`/`n`; corre `services_reload` y reinicia los `services`)                 |
| **Files**         | Explorador de archivos remoto (sigue enlaces)    | `Enter` abrir carpeta / ver archivo · `e` editar el archivo de texto abierto (`Ctrl+S` guarda, mantiene sus permisos; `Esc` cancela) · `Backspace` subir · `d` descargar · `u` subir archivos (`Espacio` marca, `Enter` abre carpeta) · `Delete` borrar · `Esc` cerrar |
| **Shared**        | Contenido de `<remote_path>/shared`, los `preserved_paths` (por ejemplo `config.json`) y los `shared_paths` que aún no existen | `Enter` abrir carpeta / archivo (luego `e` edita `.env`, `config.json`…; [cómo aplicarlo](DEPLOY.md#environment-variables-and-secrets-env-configjson)) |
| **Operations**    | Opciones de deploy y log en vivo                 | `↑`/`↓` + `Enter` activan Dry run, Force, Initial deploy, Skip dirty check, Debug, archivo de log · `D` deploy · `R` rollback · `s` prueba SSH · `t` estado y los últimos 10 deploys · `h` volver a correr hooks · `l` services reload · `u` self-update · `Esc` cerrar log |
| **Terminal**      | Shell remoto en el servidor                      | `Enter` empezar a escribir (hasta entonces ←/→ cambian de vista) · `Tab` autocompletar · `Esc` dejar de escribir, otra vez para salir |
| **Services**      | Stack del servidor: servicios instalados, versiones, actualizaciones, seguridad, sitios, SSL ([Servicios del servidor](SERVICES.md)) | `Espacio` marcar · `Enter` editar / siguiente versión · `I` instalar · `P` preparar para deploys · `U` actualizar · `S` actualizaciones de seguridad · `Ctrl+R` apt update · `L` logs · `B` backup ahora · `X` resetear contraseña de BD · `w` guardar `provision` · `a` agregar `services_reload` |
| **Config**        | El archivo de configuración cargado              | `e` editar · `Ctrl+S` guardar · `Esc` cancelar                                                                               |
| **Switch Config** | Archivos de configuración de la carpeta          | `Enter` cargar · `Espacio` marcar varios · `m` multi-deploy de los marcados con un solo build · `Esc` volver                  |

---

## Qué comandos funcionan en modo local { #which-commands-work-in-local-mode }

Los entornos con `local: true` no tienen servidor, así que solo aplican estos comandos:

| Comando                        | Modo local                                     |
| :----------------------------- | :--------------------------------------------- |
| `deploy`                       | ✅ Compila y escribe el release en `local_path` |
| `config validate`              | ✅                                             |
| `diff`, `rollback`, `status`, `history`, `exec`, `hooks`, `services-reload`, `service`, `logs`, `ssh-test`/`info` | ❌ Solo entornos SSH |

---

## Códigos de salida y errores { #exit-codes-and-error-output }

`versa` termina con `0` si todo salió bien y con `1` ante cualquier error, así que sirve en CI y scripts. Los errores conocidos se muestran con un código, el detalle y, cuando se puede, una sugerencia para resolverlo:

```
[ERROR] Deployment lock already held (since 2026-11-09 21:50)
Code: DEPLOYMENT_FAILED

Suggestion: Another deployment is in progress, or one was cut off (a crash, a lost
connection). If no one else is deploying, free it: versa exec <env> "rmdir /var/www/app/.versa.lock"
```

Mira [Problemas comunes](TROUBLESHOOTING.md) para los errores frecuentes.
