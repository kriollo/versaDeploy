# Ep. 2 — Deploy de Laravel con versaDeploy (de un VPS vacío a la app en línea)
# Antes de grabar: ./prep.sh 02 (servidor nuevo + proyecto Laravel recién creado)
# Esperas: Wait+Screen solo en la TUI; tras una salida con scroll VHS no ve la pantalla
# nueva, así que los comandos esperan la línea del cursor (la pregunta o el prompt ❯).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type "source setup.sh mitienda || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio llevamos una app Laravel desde nuestro computador a un VPS recién creado.
Sleep 2.5s
Sleep 4s
# CC: Este es un Laravel nuevo, con su primer commit. Nada especial.
Sleep 0.8s
Type "ls"
Enter
Sleep 4s

# ─── 1. El archivo deploy.yml ────────────────────────────────────────────────
# CC: versaDeploy se configura con un archivo: deploy.yml. Partimos del ejemplo para Laravel.
Sleep 0.7s
Ctrl+L
Type "curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.laravel.yml"
Sleep 400ms
Enter
Sleep 2s
# CC: Lo abrimos con un editor cualquiera. Aquí, nano.
Sleep 0.7s
Type "nano deploy.yml"
Enter
Sleep 3s
# CC: Arriba: el nombre del proyecto y cómo conectarse al servidor por SSH.
Ctrl+W
Type "ssh:"
Enter
Sleep 6s
# CC: provision describe lo que el servidor necesita: nginx, PHP, PostgreSQL, Redis, backups…
Ctrl+W
Type "provision:"
Enter
Sleep 7s
# CC: shared_paths: el .env y storage/ viven en el servidor y sobreviven a cada deploy.
Ctrl+W
Type "shared_paths:"
Enter
Sleep 7s
# CC: builds: composer y Vite corren en tu computador. Al servidor solo sube lo que cambió.
Ctrl+W
Type "builds:"
Enter
Sleep 7s
# CC: post_deploy: las migraciones y los caches de Laravel, después de activar la versión.
Ctrl+W
Type "post_deploy:"
Enter
Sleep 7s
# CC: Y un health check: si la app no responde, versa vuelve solo a la versión anterior.
Ctrl+W
Type "health_check:"
Enter
Sleep 6s

# CC: Cambiamos "myapp" por el nombre de nuestro proyecto en todo el archivo…
Alt+R
Sleep 500ms
Type "myapp"
Enter
Sleep 400ms
Type "mitienda"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: …y el dominio. En el video usamos mitienda.localhost: el VPS es un contenedor Docker.
Sleep 2.7s
Alt+R
Sleep 500ms
Type "mitienda.com"
Enter
Sleep 400ms
Type "mitienda.localhost"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: El host es la IP de tu VPS. Aquí: Docker en este computador, puerto 2299.
Sleep 2.0s
Ctrl+W
Type "host:"
Enter
Sleep 600ms
Ctrl+K
Type '      host: "127.0.0.1"                  # la IP de tu VPS'
Enter
Type "      port: 2299"
Enter
Sleep 3s
# CC: Sin dominio público no hay certificado SSL. En su lugar activamos fail2ban.
Ctrl+W
Type "ssl: true"
Enter
Sleep 600ms
Ctrl+K
Type "      fail2ban: true                     # bloquea ataques por SSH y escáneres"
Enter
Sleep 3s
# CC: El health check apunta a la ruta /up de Laravel.
Ctrl+W
Type "https://"
Enter
Sleep 600ms
Ctrl+K
Type '      url: "http://mitienda.localhost:8099/up"'
Enter
Sleep 3s
# CC: Guardamos (Ctrl+O) y salimos (Ctrl+X).
Sleep 1.2s
Ctrl+O
Sleep 400ms
Enter
Sleep 800ms
Ctrl+X
Sleep 1s

# CC: Así quedaron los cambios:
Ctrl+L
Type "grep -nE 'mitienda|127.0.0.1|2299|fail2ban|8099' deploy.yml"
Enter
Sleep 7s
# CC: La contraseña de la base de datos nunca va en el archivo: versa la lee de una variable.
Sleep 1.7s
Ctrl+L
Type "export DB_PASS='Sup3r-Secreta'"
Enter
Sleep 3s
# CC: Validamos la configuración…
Type "versa config validate production"
Enter
Sleep 3s
# CC: …y la guardamos en git: versa solo despliega lo que está commiteado.
Type "git add deploy.yml && git commit -qm 'Configura versaDeploy'"
Enter
Sleep 3s

# ─── 2. Preparar el servidor ─────────────────────────────────────────────────
# CC: Abrimos la interfaz de versa. Es solo escribir: versa
Sleep 1.2s
Ctrl+L
Type "versa"
Enter
Hide
Wait+Screen@30s /Dashboard/
Show
Sleep 3s
# CC: Todavía no se puede conectar: el usuario deploy no existe. La vista Services prepara el servidor.
Sleep 1.9s
Right@400ms 6
Hide
Wait+Screen@60s /Base packages/
Show
Sleep 3s
# CC: versa detectó Ubuntu y marcó lo que pide deploy.yml: nginx, PHP 8.4, PostgreSQL, Redis…
Sleep 2.8s
Sleep 6s
# CC: …el usuario de deploy con tu llave SSH, los backups diarios, el cron de Laravel y fail2ban.
Sleep 1.4s
Sleep 6s
# CC: Con I se instala todo. Se conecta como root por SSH: en el servidor no se instala ningún agente.
Type "I"
Sleep 4s
Type "y"
Sleep 6s
# CC: (Unos minutos después…)
Sleep 0.3s
Hide
Wait+Screen@900s /Provisioning finished/
Show
Sleep 2s
# CC: Listo: el servidor tiene todo lo necesario. Y si lo vuelves a correr, no rompe nada.
Sleep 0.5s
Sleep 5s
Escape
Sleep 1s
Type "q"
Sleep 500ms

# ─── 3. Primer deploy ────────────────────────────────────────────────────────
# CC: Ahora el primer deploy. --initial-deploy porque el servidor está vacío.
Ctrl+L
Type "versa deploy production --initial-deploy"
Enter
Sleep 7s
# CC: versa compila en tu computador (composer y Vite), comprime y sube por SFTP…
Sleep 4.2s
Hide
Wait@600s /Run post_deploy hooks\?/
Show
Sleep 2s
# CC: Las migraciones necesitan el .env configurado. Las saltamos esta primera vez: N.
Sleep 4s
Type "n"
Enter
Hide
Wait@180s /❯\s*$/
Show
Sleep 3s
# CC: El código ya está en el servidor. Falta el .env de producción.
Sleep 1.3s
Sleep 4s

# ─── 4. El .env en el servidor ───────────────────────────────────────────────
# CC: versa creó el .env del servidor a partir de .env.example. Lo editamos en la vista Shared.
Sleep 1.1s
Ctrl+L
Type "versa"
Enter
Hide
Wait+Screen@30s /Dashboard/
Show
Right@400ms 3
Hide
Wait+Screen@30s /storage/
Show
Sleep 3s
Enter
Hide
Wait+Screen@30s /APP_NAME/
Show
Sleep 2s
# CC: e para editar. Producción, sin debug, y la URL de la app.
Type "e"
Sleep 1s
Down
Ctrl+A
Ctrl+K
Type "APP_ENV=production"
Down 2
Ctrl+A
Ctrl+K
Type "APP_DEBUG=false"
Down
Ctrl+A
Ctrl+K
Type "APP_URL=http://mitienda.localhost:8099"
Sleep 2s
# CC: La base de datos: PostgreSQL, con el usuario y la contraseña que creó versa.
Down 18
Ctrl+A
Ctrl+K
Type "DB_CONNECTION=pgsql"
Down
Ctrl+A
Ctrl+K
Type "DB_HOST=127.0.0.1"
Down
Ctrl+A
Ctrl+K
Type "DB_PORT=5432"
Down
Ctrl+A
Ctrl+K
Type "DB_DATABASE=mitienda"
Down
Ctrl+A
Ctrl+K
Type "DB_USERNAME=mitienda_user"
Down
Ctrl+A
Ctrl+K
Type "DB_PASSWORD=Sup3r-Secreta"
Sleep 2s
# CC: Sesiones y cache en Redis.
Down 2
Ctrl+A
Ctrl+K
Type "SESSION_DRIVER=redis"
Down 10
Ctrl+A
Ctrl+K
Type "CACHE_STORE=redis"
Sleep 2s
# CC: Ctrl+S lo guarda en el servidor. Este archivo nunca sale de ahí ni entra a git.
Sleep 1.4s
Ctrl+S
Sleep 4s
Escape
Sleep 500ms
Type "q"
Sleep 500ms

# CC: Falta la APP_KEY. versa exec corre un comando en el servidor:
Ctrl+L
Type "versa exec production 'cd /var/www/mitienda/current/app && php artisan key:generate --force'"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s

# ─── 5. Deploy ───────────────────────────────────────────────────────────────
# CC: Ahora sí: deploy. --force porque el código no cambió desde el anterior.
Ctrl+L
Type "versa deploy production --force"
Enter
Sleep 5s
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
# CC: Corrieron las migraciones y los caches. El health check pasó. ¡Deploy exitoso!
Sleep 1.2s
Sleep 6s

# CC: Comprobemos la app:
Ctrl+L
Type "curl -s http://mitienda.localhost:8099 | grep -o '<title>.*</title>'"
Enter
Sleep 3s
# CC: Y el historial: quién desplegó, qué commit, cuánto tardó.
Type "versa history production"
Enter
Sleep 6s

# CC: Desde ahora, cada cambio es: commit y versa deploy production. Solo sube lo que cambió.
Sleep 3.0s
Sleep 5s
# CC: Si algo sale mal: versa rollback production vuelve a la versión anterior al instante.
Sleep 0.7s
Sleep 5s
# CC: En el próximo episodio: FastAPI. ¡Nos vemos!
Sleep 1.1s
Sleep 4s
