# Ep. 5 — versaWYS en FrankenPHP con versaDeploy (del composer create-project al panel en línea)
# Antes de grabar: ./prep.sh 05 (servidor nuevo; el proyecto lo crea el video)
# Esperas: Wait+Screen solo en la TUI; los comandos esperan el prompt ❯ (ver README).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type "source setup.sh proyectos || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio desplegamos un panel hecho con versaWYS, y esta vez sin PHP-FPM: con FrankenPHP.
Sleep 3.5s
Sleep 4s
# CC: Partimos de una carpeta vacía. En el computador solo hace falta PHP, Composer, Node y pnpm.
Sleep 2.7s
Type "ls"
Enter
Sleep 4s

# ─── 1. El proyecto ──────────────────────────────────────────────────────────
# CC: versaWYS tiene instalador en Composer. Creamos el proyecto…
Sleep 0.5s
Type "composer create-project versawys/versawys mi-panel"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
# CC: …e instalamos y compilamos el frontend: Vue, TypeScript y Tailwind.
Sleep 0.3s
Ctrl+L
Type "cd mi-panel && pnpm install && pnpm compile"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 3s
# los avatares por defecto, hasta que el .gitignore de versaWYS los incluya (ya corregido en su repo)
Hide
Type "git init -q && git add -f public/dashboard/img/avatars"
Enter
Sleep 1s
Ctrl+L
Show
# CC: config.json tiene las claves y la base de datos: nunca va en git. Los ajustes sin secretos, sí.
Sleep 2.0s
Ctrl+L
Type "ls versaWYS/config"
Enter
Sleep 4s
# CC: Por ejemplo, la caché en Redis. Este archivo se aplica en cada deploy.
Type `echo '{ "redis": { "enabled": true }, "cache": { "driver": "redis" } }' > versaWYS/config.compartida.json`
Enter
Sleep 4s

# ─── 2. El archivo deploy.yml ────────────────────────────────────────────────
# CC: versaDeploy trae un ejemplo para versaWYS en FrankenPHP. Lo descargamos.
Ctrl+L
Type "curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.versawys-frankenphp.yml"
Sleep 400ms
Enter
Sleep 2s
# CC: frankenphp: true en lugar de PHP-FPM. Y el cron que corre las tareas de app/schedule.php.
Ctrl+L
Type "grep -nE 'frankenphp|cron|schedule' deploy.yml"
Enter
Sleep 7s
# CC: config.json, los logs y la caché viven en el servidor y sobreviven a cada deploy.
Ctrl+L
Type "grep -nA8 'shared_paths:' deploy.yml"
Enter
Sleep 7s
# CC: FrankenPHP corre como un servicio, y después de activar cada versión, app:deploy:finish.
Ctrl+L
Type "grep -nE 'services:|exec:|reload:|post_deploy|deploy:finish' deploy.yml"
Enter
Sleep 7s
# CC: Ponemos el nombre del proyecto y el dominio. Aquí, mipanel.localhost: el VPS es un contenedor Docker.
Sleep 1.7s
Ctrl+L
Type `sed -i 's/myapp\.com/mipanel.localhost/g; s/myapp/mipanel/g' deploy.yml`
Enter
Sleep 3s
# CC: El host es la IP de tu VPS; en el video, Docker en el puerto 2299.
Sleep 0.3s
Type `sed -i 's/host: "mipanel.localhost"/host: "127.0.0.1"\n      port: 2299/' deploy.yml`
Enter
Sleep 3s
# CC: Sin dominio público no hay SSL: en su lugar, fail2ban. Y el health check, a la ruta /up.
Type `sed -i 's/ssl: true/fail2ban: true/; s|https://mipanel.localhost/up|http://mipanel.localhost:8099/up|' deploy.yml`
Enter
Sleep 3s
# CC: Así quedaron los cambios:
Ctrl+L
Type "grep -nE '^project|127.0.0.1|2299|domain|fail2ban|8099' deploy.yml"
Enter
Sleep 7s
# CC: La contraseña de la base de datos, en una variable. Validamos y guardamos todo en git.
Ctrl+L
Type "export DB_PASS='Sup3r-Secreta'"
Enter
Type "versa config validate production"
Enter
Sleep 2s
Type "git init -q && git add -A && git commit -qm 'Primer commit'"
Enter
Sleep 3s

# ─── 3. Preparar el servidor ─────────────────────────────────────────────────
# CC: Abrimos versa y vamos a la vista Services.
Ctrl+L
Type "versa"
Enter
Hide
Wait+Screen@30s /Dashboard/
Show
Sleep 2s
Right@400ms 6
Hide
Wait+Screen@60s /Base packages/
Show
Sleep 3s
# CC: FrankenPHP aparece marcado y PHP-FPM no: un solo PHP por servidor. Además nginx, PostgreSQL y Redis.
Sleep 1.2s
Sleep 7s
# CC: …el usuario de deploy, los backups, el cron y fail2ban. I instala todo por SSH, sin agentes.
Sleep 2s
Type "I"
Sleep 4s
Type "y"
Sleep 4s
# CC: (Unos minutos después…)
Sleep 0.3s
Hide
Wait+Screen@900s /Provisioning finished/
Show
Sleep 2s
# CC: Listo. FrankenPHP quedó instalado, con PHP 8.5 y su configuración para este proyecto.
Sleep 0.1s
Sleep 5s
Escape
Sleep 1s
Type "q"
Sleep 500ms

# ─── 4. Primer deploy e instalación ──────────────────────────────────────────
# CC: El primer deploy. versa compila en tu computador: composer y el frontend.
Ctrl+L
Type "versa deploy production --initial-deploy"
Enter
Sleep 5s
Hide
Wait@600s /Run post_deploy hooks\?/
Show
Sleep 2s
# CC: app:deploy:finish necesita la app instalada. Esta primera vez lo saltamos: N.
Sleep 3s
Type "n"
Enter
Hide
Wait@180s /❯\s*$/
Show
Sleep 3s
# CC: Ahora instalamos versaWYS en el servidor: la base de datos, la empresa y el superadministrador.
Ctrl+L
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:install --db-driver=postgresql --db-name=mipanel --db-user=mipanel_user --db-pass=$DB_PASS --empresa-nombre='Mi Empresa' --empresa-rut=76.543.210-3 --admin-name=Admin --admin-email=admin@miempresa.cl --admin-password='Cl4ve-Segura#2026'"`
Enter
Hide
Wait@180s /❯\s*$/
Show
Sleep 5s
# CC: Migraciones, seeders y el administrador listos. El panel ya responde:
Ctrl+L
Type "curl -sI http://mipanel.localhost:8099/admin/login | head -1"
Enter
Sleep 4s

# ─── 5. Deploy con app:deploy:finish ─────────────────────────────────────────
# CC: Ahora el deploy completo. --force porque el código no cambió.
Ctrl+L
Type "versa deploy production --force"
Enter
Sleep 4s
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
# CC: app:deploy:finish aplicó los ajustes, migró, armó la caché de Twig y revisó el servidor. Todo en verde.
Sleep 8s

# ─── 6. Lo que corre en el servidor ──────────────────────────────────────────
# CC: El servicio de FrankenPHP está corriendo. Los deploys no lo reinician: no se corta ninguna petición.
Sleep 0.4s
Ctrl+L
Type "versa service production"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 6s
# CC: El cron llama cada minuto al programador de versaWYS: las tareas y la cola de trabajos.
Ctrl+L
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:schedule:list"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 7s
# CC: Y la caché ya está en Redis.
Ctrl+L
Type `versa exec production "redis-cli --scan | head -4"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 4s
# CC: Para una migración larga: modo mantenimiento. El sitio responde 503 y el equipo entra con su clave.
Ctrl+L
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:down --secret=equipo"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s
Type "curl -sI http://mipanel.localhost:8099/ | head -1"
Enter
Sleep 4s
# CC: app:up lo vuelve a abrir.
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:up"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s

# CC: Desde ahora: commit y versa deploy production. Solo sube lo que cambió.
Sleep 5s
# CC: En el próximo episodio: qué hacer cuando un deploy falla. ¡Nos vemos!
Sleep 0.6s
Sleep 4s
