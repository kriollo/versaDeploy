# Ep. 3 — Deploy de una API FastAPI + PostgreSQL con versaDeploy
# Antes de grabar: ./prep.sh 03 (servidor nuevo + la API de ejemplo)
# Esperas: Wait+Screen solo en la TUI; los comandos esperan el prompt ❯ (ver README).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type "source setup.sh mi-api || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio desplegamos una API hecha con FastAPI, con su base de datos PostgreSQL.
Sleep 2.7s
Sleep 4s
# CC: Es una API pequeña: lee su configuración del entorno y /health comprueba la base de datos.
Type "cat main.py requirements.txt"
Enter
Sleep 7s

# ─── 1. El archivo deploy.yml ────────────────────────────────────────────────
# CC: Partimos del ejemplo de versaDeploy para FastAPI.
Ctrl+L
Type "curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.fastapi.yml"
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
# CC: provision: nginx como proxy hacia el puerto 8000 de la API, PostgreSQL y backups diarios.
Ctrl+W
Type "provision:"
Enter
Sleep 7s
# CC: shared_paths: el .env vive en el servidor, fuera de git, y sobrevive a cada deploy.
Ctrl+W
Type "shared_paths:"
Enter
Sleep 6s
# CC: builds: el virtualenv se crea en el servidor con requirements.txt, y la API corre con uvicorn.
Sleep 0.2s
Ctrl+W
Type "builds:"
Enter
Sleep 7s
# CC: services: versa la instala como servicio de systemd, con las variables del .env.
Ctrl+W
Type "services:"
Enter
Sleep 7s
# CC: Y el health check: si /docs no responde después del deploy, versa vuelve a la versión anterior.
Sleep 0.5s
Ctrl+W
Type "health_check:"
Enter
Sleep 6s

# CC: Cambiamos "myapi" por el nombre de nuestro proyecto en todo el archivo…
Alt+R
Sleep 500ms
Type "myapi"
Enter
Sleep 400ms
Type "miapi"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: …y el dominio. Aquí miapi.localhost: el VPS es un contenedor Docker.
Sleep 2.0s
Alt+R
Sleep 500ms
Type "api.example.com"
Enter
Sleep 400ms
Type "miapi.localhost"
Enter
Sleep 600ms
Type "A"
Sleep 3s
# CC: El host es la IP de tu VPS. Aquí: Docker en este computador, puerto 2299.
Sleep 1.8s
Ctrl+W
Type `host: "miapi`
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
Type '      url: "http://miapi.localhost:8099/docs"'
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
Type "grep -nE 'miapi|127.0.0.1|2299|fail2ban|8099' deploy.yml"
Enter
Sleep 6s
# CC: La contraseña de la base de datos va en una variable, nunca en el archivo.
Ctrl+L
Type "export DB_PASS='Sup3r-Secreta'"
Enter
Sleep 2s
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
# CC: versa detectó la API FastAPI y marcó Python con virtualenv, nginx, PostgreSQL y fail2ban.
Sleep 0.1s
Sleep 7s
# CC: I instala todo, como root por SSH.
Type "I"
Sleep 3s
Type "y"
Sleep 5s
# CC: (Unos minutos después…)
Hide
Wait+Screen@900s /Provisioning finished/
Show
Sleep 2s
# CC: Listo. Además creó la base de datos miapi y su usuario, con la contraseña de DB_PASS.
Sleep 0.5s
Sleep 5s
Escape
Sleep 1s
Type "q"
Sleep 500ms

# ─── 3. Primer deploy ────────────────────────────────────────────────────────
# CC: El primer deploy. versa sube el código, crea el virtualenv en el servidor y arranca el servicio.
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

# CC: La API responde…
Ctrl+L
Type "curl -s http://miapi.localhost:8099/; echo"
Enter
Sleep 3s
# CC: …pero /health falla: el .env del servidor aún tiene los datos de ejemplo de .env.example.
Sleep 0.3s
Type "curl -s http://miapi.localhost:8099/health; echo"
Enter
Sleep 5s

# ─── 4. El .env en el servidor ───────────────────────────────────────────────
# CC: Lo editamos en la vista Shared: producción y la conexión a PostgreSQL.
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
Wait+Screen@30s /DATABASE_URL/
Show
Sleep 2s
Type "e"
Sleep 1s
Down
Ctrl+A
Ctrl+K
Type "APP_ENV=production"
Down
Ctrl+A
Ctrl+K
Type "DATABASE_URL=postgresql://miapi_user:Sup3r-Secreta@127.0.0.1:5432/miapi"
Sleep 3s
# CC: Ctrl+S lo guarda en el servidor.
Ctrl+S
Sleep 3s
Escape
Sleep 500ms
Type "q"
Sleep 500ms

# CC: El servicio lee el .env al arrancar: lo reiniciamos.
Sleep 0.9s
Ctrl+L
Type "versa service production restart"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s
# CC: Ahora /health conecta con PostgreSQL.
Type "curl -s http://miapi.localhost:8099/health; echo"
Enter
Sleep 3s
Type "curl -s http://miapi.localhost:8099/; echo"
Enter
Sleep 4s

# ─── 5. Un cambio de código ──────────────────────────────────────────────────
# CC: Agreguemos un endpoint nuevo a la API.
Ctrl+L
Type "cat >> main.py <<'EOF'"
Enter
Enter
Type `@app.get("/saludo/{nombre}")`
Enter
Type "def saludo(nombre: str):"
Enter
Type `    return {"mensaje": f"Hola, {nombre}"}`
Enter
Type "EOF"
Enter
Sleep 2s
# CC: Commit y deploy. Es todo.
Type "git commit -qam 'Endpoint /saludo' && versa deploy production"
Enter
Sleep 5s
Hide
Wait@600s /❯\s*$/
Show
Sleep 2s
# CC: Detectó 1 archivo Python cambiado y reutilizó el virtualenv: requirements.txt no cambió.
Sleep 6s
Ctrl+L
Type "curl -s http://miapi.localhost:8099/saludo/Mundo; echo"
Enter
Sleep 3s
Type "versa history production"
Enter
Sleep 6s

# CC: Si algo sale mal: versa rollback production vuelve a la versión anterior al instante.
Sleep 0.7s
Sleep 5s
# CC: En el próximo episodio: Nuxt con SSR. ¡Nos vemos!
Sleep 1.2s
Sleep 4s
