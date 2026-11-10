# Ep. 4 — Deploy de Nuxt con SSR (y un rollback) con versaDeploy
# Antes de grabar: ./prep.sh 04 (servidor nuevo + el sitio Nuxt de ejemplo)
# Esperas: Wait+Screen solo en la TUI; los comandos esperan el prompt ❯ (ver README).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type "source setup.sh mi-sitio || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio desplegamos un sitio Nuxt con SSR: un proceso Node detrás de nginx.
Sleep 3.1s
Sleep 4s
# CC: Es un Nuxt 4 nuevo. Muestra el nombre del sitio, que leerá del .env del servidor.
Type "cat app/app.vue"
Enter
Sleep 7s

# ─── 1. El archivo deploy.yml ────────────────────────────────────────────────
# CC: Partimos del ejemplo de versaDeploy para Nuxt.
Ctrl+L
Type "curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.nuxt.yml"
Sleep 400ms
Enter
Sleep 2s
Type "nano deploy.yml"
Enter
Sleep 2s
# CC: Arriba: el nombre del proyecto y cómo conectarse al servidor por SSH.
Ctrl+W
Type "ssh:"
Enter
Sleep 6s
# CC: provision: nginx como proxy hacia el puerto 3000, Node.js 22 y sin base de datos.
Ctrl+W
Type "provision:"
Enter
Sleep 7s
# CC: shared_paths: el .env vive en el servidor y sobrevive a cada deploy.
Ctrl+W
Type "shared_paths:"
Enter
Sleep 6s
# CC: builds: npm ci y npm run build corren en tu computador. Al servidor sube el resultado, .output.
Sleep 1.1s
Ctrl+W
Type "builds:"
Enter
Sleep 7s
# CC: services: versa corre .output/server con Node como servicio de systemd, en el puerto 3000.
Sleep 0.5s
Ctrl+W
Type "services:"
Enter
Sleep 7s
# CC: Y el health check: si la página no responde, versa vuelve a la versión anterior.
Sleep 0.6s
Ctrl+W
Type "health_check:"
Enter
Sleep 5s

# CC: Cambiamos "mysite" por el nombre de nuestro proyecto en todo el archivo…
Alt+R
Sleep 500ms
Type "mysite"
Enter
Sleep 400ms
Type "misitio"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: …y el dominio. Aquí misitio.localhost: el VPS es un contenedor Docker.
Sleep 1.8s
Alt+R
Sleep 500ms
Type "misitio.com"
Enter
Sleep 400ms
Type "misitio.localhost"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: El host es la IP de tu VPS. Aquí: Docker en este computador, puerto 2299.
Sleep 1.7s
Ctrl+W
Type `host: "misitio`
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
# CC: Y el health check, con el puerto de Docker.
Ctrl+W
Type "https://"
Enter
Sleep 600ms
Ctrl+K
Type '      url: "http://misitio.localhost:8099/"'
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

# CC: Así quedaron los cambios. Validamos y hacemos commit.
Ctrl+L
Type "grep -nE 'misitio|127.0.0.1|2299|fail2ban|8099' deploy.yml"
Enter
Sleep 5s
Type "versa config validate production && git add deploy.yml && git commit -qm 'Configura versaDeploy'"
Enter
Sleep 5s

# ─── 2. Preparar el servidor ─────────────────────────────────────────────────
# CC: Preparamos el servidor desde la vista Services.
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
Sleep 2s
# CC: versa detectó el servidor Node y marcó Node.js, nginx, el usuario de deploy y fail2ban.
Sleep 1.3s
Sleep 7s
# CC: I instala todo, como root por SSH.
Type "I"
Sleep 3s
Type "y"
Sleep 5s
# CC: (Unos minutos después…)
Sleep 0.3s
Hide
Wait+Screen@900s /Provisioning finished/
Show
Sleep 2s
# CC: Listo: Node.js 22 desde NodeSource, nginx con el proxy, y el servicio de la app preparado.
Sleep 1.2s
Sleep 5s
Escape
Sleep 1s
Type "q"
Sleep 500ms

# ─── 3. Primer deploy ────────────────────────────────────────────────────────
# CC: El primer deploy: compila con npm run build, sube .output y arranca el servicio.
Ctrl+L
Type "versa deploy production --initial-deploy"
Enter
Sleep 6s
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
# CC: El servicio quedó corriendo y el health check pasó.
Sleep 5s
Ctrl+L
Type "curl -s http://misitio.localhost:8099/ | grep -o '<h1>.*</h1>'"
Enter
Sleep 4s

# ─── 4. El .env en el servidor ───────────────────────────────────────────────
# CC: El nombre del sitio sale del .env del servidor. Lo cambiamos en la vista Shared.
Ctrl+L
Type "versa"
Enter
Hide
Wait+Screen@30s /Dashboard/
Show
Right@400ms 3
Hide
Wait+Screen@30s /\.env/
Show
Sleep 2s
Enter
Hide
Wait+Screen@30s /NUXT_PUBLIC_SITE_NAME/
Show
Sleep 2s
Type "e"
Sleep 1s
Ctrl+A
Ctrl+K
Type `NUXT_PUBLIC_SITE_NAME="Mi tienda online"`
Sleep 2s
Ctrl+S
Sleep 3s
Escape
Sleep 500ms
Type "q"
Sleep 500ms
# CC: Nuxt lee el .env al arrancar: reiniciamos el servicio.
Ctrl+L
Type "versa service production restart"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 2s
Type "curl -s http://misitio.localhost:8099/ | grep -o '<h1>.*</h1>'"
Enter
Sleep 4s

# ─── 5. Un cambio… y un rollback ─────────────────────────────────────────────
# CC: Ahora un cambio en el código: commit y deploy.
Ctrl+L
Type "sed -i 's/Nuxt con SSR/Nuxt 4 con SSR, versión 2/' app/app.vue"
Enter
Type "git commit -qam 'Nueva versión' && versa deploy production"
Enter
Sleep 5s
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
Ctrl+L
Type "curl -s http://misitio.localhost:8099/ | grep -o '<p>.*</p>'"
Enter
Sleep 4s
# CC: ¿Algo salió mal? versa rollback vuelve a la versión anterior al instante: solo cambia un symlink.
Type "versa rollback production"
Enter
Hide
Wait@120s /❯\s*$/
Show
Sleep 3s
Type "curl -s http://misitio.localhost:8099/ | grep -o '<p>.*</p>'"
Enter
Sleep 4s
# CC: La versión anterior volvió en segundos. El historial registra los deploys y el rollback.
Type "versa history production"
Enter
Sleep 6s

# CC: Eso es todo: Laravel, FastAPI y Nuxt en un VPS, con versaDeploy. ¡Gracias por ver!
Sleep 2.7s
Sleep 5s
