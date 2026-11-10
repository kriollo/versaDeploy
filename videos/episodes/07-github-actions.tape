# Ep. 7 — De git push a producción: GitHub Actions + versaDeploy en tu propio VPS
# Antes de grabar: ./prep.sh 07 (kriollo/versadeploy-demo-ci en ep7-start, sin workflow ni
# secretos, desplegado desde aquí; el workflow de ci-runner/deploy.yml listo sin commit;
# el runner de ci-runner/ en marcha). Sigue al panel de los episodios 5 y 6.
# Esperas: los comandos esperan el prompt ❯ (ver README). Los deploys se ven en cámara
# rápida (# FAST x4); "web N URL" toma la página para "# WEB: N".
Source settings.tape
Set TypingSpeed 35ms

Hide
Type `source setup.sh proyectos/mi-panel || exit; export GH_CONFIG_DIR="$REAL_HOME/.config/gh" GIT_CONFIG_GLOBAL="$REAL_HOME/.gitconfig"; rm -f /tmp/codigos.txt /tmp/ci_deploy /tmp/ci_deploy.pub` Enter
Sleep 1s
Type "web 1 http://mipanel.localhost:8099/admin/login" Enter
Wait@60s /❯\s*$/
Ctrl+L
Show

# CC: Este es el panel versaWYS de los episodios cinco y seis, en producción. El título del login:
Sleep 1s
Type "curl -s http://mipanel.localhost:8099/admin/login | grep -o 'Ingresar a la plataforma'"
Enter
Sleep 3s
# WEB: 1
# CC: Hasta hoy lo desplegamos desde el computador. Al final de este video, cada git push lo despliega solo.
Sleep 3.5s
Sleep 3s

# ─── 1. El servidor y la llave SSH ───────────────────────────────────────────
# CC: Partimos de lo que ya funciona: el usuario deploy entra al VPS por SSH.
Ctrl+L
Type "versa ssh-test production"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 6s
# CC: El runner de GitHub Actions ya está registrado. En este video corre en el mismo PC que el VPS de Docker.
Ctrl+L
Type `gh api repos/kriollo/versadeploy-demo-ci/actions/runners -q '.runners[] | .name + "  " + .status'`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 4s
# CC: CI necesita su propia llave SSH. Nunca uses la tuya: esta se puede revocar sin tocar nada más.
Sleep 1.5s
Ctrl+L
Type `ssh-keygen -q -t ed25519 -N "" -C github-actions -f /tmp/ci_deploy`
Enter
Sleep 3s
# CC: La autorizamos para el usuario deploy del servidor…
Type `versa exec production "echo '$(cat /tmp/ci_deploy.pub)' >> ~/.ssh/authorized_keys"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s
# CC: …y la guardamos como secreto del repositorio. También la llave pública del servidor, para que CI lo reconozca.
Type "gh secret set DEPLOY_SSH_KEY < /tmp/ci_deploy"
Enter
Hide
Wait@60s /❯\s*$/
Show
Type "ssh-keyscan -p 2299 -t ed25519 127.0.0.1 2>/dev/null | gh secret set DEPLOY_KNOWN_HOSTS"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 2s
# CC: La llave privada queda solo en GitHub, cifrada. La borramos de aquí.
Type "rm /tmp/ci_deploy /tmp/ci_deploy.pub && gh secret list"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 5s

# ─── 2. deploy.yml, completo ─────────────────────────────────────────────────
# CC: Este es el deploy.yml del proyecto, el mismo de los episodios cinco y seis. Lo vemos entero, con nano.
Sleep 4.5s
Ctrl+L
Type "nano deploy.yml"
Enter
Sleep 2s
# CC: Arriba, cómo conectarse: host, puerto y usuario. No son secretos, así que van junto al código.
Sleep 1.4s
Sleep 5s
# CC: strict_host_key: true. Desde CI, versa solo se conecta a un servidor que ya conoce.
Sleep 1.7s
Sleep 4s
PageDown
# CC: provision describe el servidor: nginx, FrankenPHP, PostgreSQL, Redis y el cron del proyecto.
Sleep 1.4s
Sleep 5s
# CC: ignored_paths: lo que no se sube. Por ejemplo .github: el workflow no hace falta en el servidor.
Sleep 1.4s
Sleep 5s
PageDown
# CC: shared_paths: la configuración, los logs y las cachés viven fuera de cada versión.
Sleep 0.3s
Sleep 5s
# CC: El build: Composer sin dependencias de desarrollo, y el frontend compilado con pnpm.
Sleep 1.0s
Sleep 5s
# CC: FrankenPHP corre como servicio. Un deploy no lo reinicia: solo limpia su caché, sin cortar peticiones.
Sleep 2.3s
Sleep 5s
PageDown
# CC: Después del cambio de versión: app:deploy:finish, con las migraciones, las cachés y una revisión.
Sleep 1.2s
Sleep 5s
# CC: Y el health check. Si /up no responde, versa vuelve solo a la versión anterior.
Sleep 5s
Ctrl+X
Sleep 1s

# ─── 3. El workflow, paso a paso ─────────────────────────────────────────────
# CC: El workflow va en .github/workflows/deploy.yml. Lo escribimos antes de grabar; ahora lo recorremos completo.
Sleep 4.3s
Ctrl+L
Type "nano .github/workflows/deploy.yml"
Enter
Sleep 2s
# CC: Arriba, los dos secretos que acabamos de crear.
Sleep 0.4s
Sleep 3s
# CC: on push, branches main: corre con cada push a main. Y concurrency: un deploy a la vez.
Sleep 0.6s
Sleep 5s
# CC: runs-on: self-hosted, nuestro runner. Con un VPS público, sirve ubuntu-latest.
Sleep 0.6s
Sleep 5s
PageDown
# CC: El checkout trae todo el historial, porque versa despliega desde git.
Sleep 0.6s
Sleep 4s
# CC: Instala las dependencias PHP y revisa el código. Si la revisión falla, aquí se detiene todo.
Sleep 1.6s
Sleep 5s
# CC: Instala versa, con su instalador que verifica el sha256, y escribe la llave desde los secretos.
Sleep 1.8s
Sleep 5s
PageDown
# CC: Despliega con versa deploy production. Y al final borra la llave, aunque algo haya fallado.
Sleep 5s
Ctrl+X
Sleep 1s
# CC: Commit y push del workflow. Este push ya lo pone a correr.
Sleep 0.5s
Ctrl+L
Type "git add .github && git commit -qm 'Deploy automático con GitHub Actions' && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
# CC: gh run watch lo sigue desde la terminal. En cámara rápida, cuatro veces más rápido.
Sleep 0.3s
Type "gh run watch"
Enter
Sleep 3s
Enter
# FAST x4
Wait@300s /❯\s*$/
Sleep 2s
# CC: Cada paso en verde: revisó el código, instaló versa y desplegó.
Sleep 0.5s
Sleep 4s
Hide
Type `web 2 "$(gh run list -L1 --json url -q '.[0].url')"` Enter
Wait@60s /❯\s*$/
Show
# WEB: 2
# CC: Cada deploy es una versión nueva en releases, y current apunta a la activa.
Ctrl+L
Type `versa exec production "ls /var/www/mipanel/releases; readlink /var/www/mipanel/current"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 5s

# ─── 4. Un cambio, y está en producción ──────────────────────────────────────
# CC: Ahora, el día a día. Mientras desplegamos, pedimos /up dos veces por segundo: si el sitio se cae, lo veremos.
Ctrl+L
Type `(for i in $(seq 120); do curl -s -o /dev/null -w '%{http_code}\n' http://mipanel.localhost:8099/up; sleep 0.5; done > /tmp/codigos.txt &)`
Enter
Sleep 2s
# CC: Cambiamos el título del login con nano: buscar y reemplazar.
Type "nano app/templates/dashboard/login/login.twig"
Enter
Sleep 2s
Alt+r
Type "Ingresar a la plataforma"
Enter
Sleep 0.5s
Type "Ingresa a tu panel"
Enter
Sleep 1.5s
Type "s"
Sleep 1.5s
Ctrl+O
Enter
Sleep 1s
Ctrl+X
# CC: Commit y push. Nada más.
Ctrl+L
Type "git commit -qam 'Nuevo título del login' && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
Type "gh run watch"
Enter
Sleep 3s
Enter
# FAST x4
Wait@300s /❯\s*$/
Sleep 2s
# CC: Listo, en unos veinte segundos. Así se ve ahora el login:
Sleep 1.1s
Sleep 3s
Hide
Type "web 3 http://mipanel.localhost:8099/admin/login" Enter
Wait@60s /❯\s*$/
Show
# WEB: 3
# CC: Y mientras desplegaba, todas las respuestas fueron 200: ninguna caída.
Ctrl+L
Type "sort /tmp/codigos.txt | uniq -c"
Enter
Sleep 5s

# ─── 5. Cuando algo falla ────────────────────────────────────────────────────
# CC: Ahora, un error: subimos un archivo PHP que no compila.
Ctrl+L
Type `echo '<?php function total( {' > app/funciones.php`
Enter
Type "git add -A && git commit -qm 'Agrega funciones' && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
Type "gh run watch"
Enter
Sleep 3s
Enter
# FAST x4
Wait@300s /❯\s*$/
Sleep 2s
# CC: La revisión falló y el pipeline se detuvo: versa ni siquiera se conectó al servidor.
Sleep 0.8s
Sleep 5s
Hide
Type `web 4 "$(gh run list -L1 --json url -q '.[0].url')"` Enter
Wait@60s /❯\s*$/
Show
# WEB: 4
# CC: Producción sigue con la versión que funcionaba:
Ctrl+L
Type "curl -s http://mipanel.localhost:8099/admin/login | grep -o 'Ingresa a tu panel'"
Enter
Sleep 3s
# CC: Y si algo fallara ya en el servidor, versa vuelve solo a la versión anterior. Lo vimos en el episodio seis.
Sleep 3.0s
Sleep 4s
# CC: Lo corregimos con un revert, y el pipeline vuelve a desplegar.
Type "git revert --no-edit HEAD && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
Type "gh run watch"
Enter
Sleep 3s
Enter
# FAST x4
Wait@300s /❯\s*$/
Sleep 4s

# CC: Recapitulando: tu servidor, tus reglas, sin agentes ni plataformas de por medio. Y gratis.
Sleep 2.0s
Sleep 4s
# CC: ¿Qué framework quieres ver en el próximo video? Déjalo en los comentarios. ¡Nos vemos!
Sleep 1.3s
Sleep 4s
