# Ep. 7 — De git push a producción: GitHub Actions + versaDeploy en tu propio VPS
# Antes de grabar: ./prep.sh 07 (el repo kriollo/versadeploy-demo-ci en su commit base,
# desplegado, y el runner de ci-runner/ en marcha). Sigue al panel de los episodios 5 y 6.
# Esperas: los comandos esperan el prompt ❯ (ver README).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type `source setup.sh proyectos/mi-panel || exit; export GH_CONFIG_DIR="$REAL_HOME/.config/gh" GIT_CONFIG_GLOBAL="$REAL_HOME/.gitconfig"; rm -f /tmp/codigos.txt /tmp/ci_deploy /tmp/ci_deploy.pub` Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio: haces git push, y tu app se despliega sola en tu VPS. Con GitHub Actions y versaDeploy.
Sleep 3.4s
Sleep 4s
# CC: Primero, el resultado. Este es el título del login, en producción:
Type "curl -s http://mipanel.localhost:8099/admin/login | grep -o 'Ingresar[^<]*'"
Enter
Sleep 4s

# ─── 1. Un push, y está en producción ────────────────────────────────────────
# CC: Mientras tanto, pedimos la página dos veces por segundo: si el sitio se cae, lo veremos.
Ctrl+L
Type `(for i in $(seq 120); do curl -s -o /dev/null -w '%{http_code}\n' http://mipanel.localhost:8099/admin/login; sleep 0.5; done > /tmp/codigos.txt &)`
Enter
Sleep 3s
# CC: Cambiamos el título, commit y push. Nada más.
Type "sed -i 's/Ingresar a la plataforma/Ingresa a tu panel/' app/templates/dashboard/login/login.twig"
Enter
Type "git commit -qam 'Nuevo título del login' && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
# CC: GitHub Actions ya está trabajando. gh run watch lo sigue desde la terminal.
Type "gh run watch"
Enter
Sleep 3s
Enter
Hide
Wait@300s /❯\s*$/
Show
Sleep 2s
# CC: Revisó el código, instaló versa y desplegó. En unos veinte segundos.
Sleep 6s
# CC: El login ya tiene el título nuevo…
Ctrl+L
Type "curl -s http://mipanel.localhost:8099/admin/login | grep -o 'Ingresa a[^<]*'"
Enter
Sleep 3s
# CC: …y mientras desplegaba, todas las respuestas fueron 200: ninguna caída.
Type "sort /tmp/codigos.txt | uniq -c"
Enter
Sleep 6s

# ─── 2. El servidor y los secretos ───────────────────────────────────────────
# CC: ¿Qué hace falta? Un VPS con un usuario deploy que entre por SSH. versa ssh-test lo confirma.
Ctrl+L
Type "versa ssh-test production"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 7s
# CC: Y un runner de GitHub Actions. En este video corre en el mismo PC que el VPS de Docker.
Ctrl+L
Type `gh api repos/kriollo/versadeploy-demo-ci/actions/runners -q '.runners[] | .name + "  " + .status'`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 5s
# CC: CI necesita su propia llave SSH. Nunca uses la tuya: esta se puede revocar sola.
Sleep 0.7s
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
# CC: …y la guardamos como secreto del repositorio. Junto con la llave del servidor, para que CI lo reconozca.
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
# CC: La llave privada solo queda en GitHub, cifrada. La borramos de aquí.
Type "rm /tmp/ci_deploy /tmp/ci_deploy.pub && gh secret list"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 5s

# ─── 3. El archivo deploy.yml ────────────────────────────────────────────────
# CC: El host, el puerto y el usuario no son secretos: están en deploy.yml, junto al código.
Ctrl+L
Type "grep -nA6 '    ssh:' deploy.yml"
Enter
Sleep 7s
# CC: strict_host_key: true. CI solo se conecta a un servidor que conoce.
Sleep 0.8s
Sleep 4s
# CC: Cada deploy es una versión nueva en releases/, y current apunta a la activa.
Ctrl+L
Type `versa exec production "ls -l /var/www/mipanel | grep -E 'current|releases|shared'; ls /var/www/mipanel/releases"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 7s
# CC: Después del cambio: app:deploy:finish, y el health check. Si /up no responde, vuelve a la versión anterior.
Ctrl+L
Type "grep -nE 'post_deploy|deploy:finish|health_check|url:' deploy.yml"
Enter
Sleep 7s

# ─── 4. El workflow de GitHub Actions ────────────────────────────────────────
# CC: El workflow está en .github/workflows/deploy.yml. Corre con cada push a main, de a un deploy por vez.
Ctrl+L
Type "sed -n '6,25p' .github/workflows/deploy.yml"
Enter
Sleep 8s
# CC: Primero el checkout, las dependencias y la revisión del código. Si falla, no se despliega nada.
Ctrl+L
Type "sed -n '26,39p' .github/workflows/deploy.yml"
Enter
Sleep 8s
# CC: Después instala versa, escribe la llave desde los secretos y despliega. Al final, la llave se borra.
Ctrl+L
Type "sed -n '40,62p' .github/workflows/deploy.yml"
Enter
Sleep 9s

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
Hide
Wait@300s /❯\s*$/
Show
Sleep 2s
# CC: La revisión falló y el pipeline se detuvo: versa ni siquiera se conectó al servidor.
Sleep 7s
# CC: Producción sigue con la versión que funcionaba:
Ctrl+L
Type "curl -s http://mipanel.localhost:8099/admin/login | grep -o 'Ingresa a[^<]*'"
Enter
Sleep 4s
# CC: Y si algo fallara ya en el servidor, versa vuelve solo a la versión anterior. Lo vimos en el episodio 6.
Sleep 2.0s
Sleep 5s
# CC: Lo corregimos con un revert, y el pipeline vuelve a desplegar.
Type "git revert --no-edit HEAD -q && git push -q"
Enter
Hide
Wait@60s /❯\s*$/
Sleep 8s
Show
Type "gh run watch"
Enter
Sleep 3s
Enter
Hide
Wait@300s /❯\s*$/
Show
Sleep 4s

# CC: Recapitulando: tu servidor, tus reglas, sin agentes ni plataformas de por medio. Y gratis.
Sleep 1.1s
Sleep 5s
# CC: ¿Qué framework quieres ver en el próximo video? Déjalo en los comentarios. ¡Nos vemos!
Sleep 1.3s
Sleep 4s
