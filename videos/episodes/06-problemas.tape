# Ep. 6 — Cuando un deploy falla: los errores más comunes y cómo resolverlos
# Antes de grabar: ./make.sh 05 (este episodio sigue con el panel versaWYS ya en línea)
# Esperas: los comandos esperan el prompt ❯ (ver README).
Source settings.tape
Set TypingSpeed 35ms

Hide
Type "source setup.sh proyectos/mi-panel || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: En este episodio provocamos a propósito los errores más comunes de un deploy, y los resolvemos.
Sleep 2.2s
Sleep 4s
# CC: Partimos del panel versaWYS del episodio anterior, ya en línea.
Type "versa history production --limit 3"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 4s

# ─── 1. Cambios sin commit ───────────────────────────────────────────────────
# CC: Editamos un archivo y, sin hacer commit, desplegamos.
Ctrl+L
Type `echo "// pendiente" >> app/schedule.php`
Enter
Type "versa deploy production"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s
# CC: versa se niega: solo despliega lo que está en git. Así cada versión se puede reproducir.
Sleep 1.0s
Sleep 5s
# CC: La solución: hacer commit, o descartar el cambio. Aquí lo descartamos.
Type "git checkout app/schedule.php && git status --short"
Enter
Sleep 3s

# ─── 2. No conecta por SSH ───────────────────────────────────────────────────
# CC: Ahora, un puerto equivocado en deploy.yml. Pasa al cambiar de servidor o de proveedor.
Sleep 0.5s
Ctrl+L
Type "sed -i 's/port: 2299/port: 2222/' deploy.yml"
Enter
Type "versa ssh-test production"
Enter
Hide
Wait@120s /❯\s*$/
Show
Sleep 3s
# CC: Connection refused: nadie escucha en ese puerto. Revisa el puerto y el firewall del servidor.
Sleep 0.9s
Sleep 5s
# CC: versa ssh-test prueba la conexión sin desplegar. Corregimos el puerto y conecta.
Ctrl+L
Type "git checkout deploy.yml && versa ssh-test production"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 7s

# ─── 3. Deploy bloqueado ─────────────────────────────────────────────────────
# CC: Si el computador se cuelga en medio de un deploy, el candado queda tomado en el servidor. Lo simulamos.
Sleep 0.2s
Ctrl+L
Type `versa exec production "mkdir /var/www/mipanel/.versa.lock"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Type "versa deploy production --force"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 3s
# CC: versa avisa desde cuándo está tomado y cómo liberarlo. Si nadie más está desplegando:
Sleep 4s
Type `versa exec production "rmdir /var/www/mipanel/.versa.lock"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 2s
# CC: Y si cortas un deploy con Ctrl+C, versa suelta el candado solo.
Sleep 0.8s
Sleep 4s

# ─── 4. Un hook falla ────────────────────────────────────────────────────────
# CC: Subimos un archivo PHP con un error de sintaxis y desplegamos.
Ctrl+L
Type `echo '<?php function total( {' > app/funciones.php`
Enter
Type "git add -A && git commit -qm 'Agrega funciones'"
Enter
Type "versa deploy production"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 3s
# CC: app:deploy:finish encontró el error, y versa volvió solo a la versión anterior.
Sleep 6s
# CC: El historial guarda cada intento, con la causa de la falla.
Ctrl+L
Type "versa history production --limit 2"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 6s
# CC: Mejor aún: atraparlo en tu computador, antes de subir nada. La misma revisión, como pre_deploy_local.
Ctrl+L
Type `printf '    pre_deploy_local:\n      - "php versaCLI app:check:deploy"\n' >> deploy.yml`
Enter
Type "git commit -qam 'Revisa antes de desplegar' && versa deploy production"
Enter
Hide
Wait@120s /❯\s*$/
Show
Sleep 3s
# CC: Ahora el error aparece en segundos, sin compilar ni subir. Lo corregimos y desplegamos.
Sleep 4s
Ctrl+L
Type "git rm -q app/funciones.php && git commit -qm 'Quita funciones' && versa deploy production"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 5s

# ─── 5. El health check falla ────────────────────────────────────────────────
# CC: Otro clásico: el sitio quedó en mantenimiento y desplegamos.
Ctrl+L
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:down --secret=equipo"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Type "versa deploy production --force"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 3s
# CC: El deploy termina, pero /up responde 503: el health check falla y versa vuelve atrás.
Sleep 0.6s
Sleep 6s
# CC: Una versión que no responde nunca se queda en línea. Abrimos el sitio y desplegamos otra vez.
Ctrl+L
Type `versa exec production "cd /var/www/mipanel/current/app && php versaCLI app:up"`
Enter
Hide
Wait@60s /❯\s*$/
Show
Type "versa deploy production --force"
Enter
Hide
Wait@600s /❯\s*$/
Show
Sleep 5s

# ─── 6. Volver atrás a mano ──────────────────────────────────────────────────
# CC: Y si nada lo detecta: un error que ves en producción. versa rollback vuelve a la versión anterior al instante.
Sleep 1.2s
Ctrl+L
Type "versa rollback production"
Enter
Hide
Wait@120s /❯\s*$/
Show
Sleep 5s
# CC: Solo cambia el enlace current: no recompila ni corre migraciones. Después corriges y despliegas.
Sleep 2.2s
Sleep 4s
# CC: Y el historial lo registra todo: deploys, fallas y rollbacks.
Ctrl+L
Type "versa history production --limit 6"
Enter
Hide
Wait@60s /❯\s*$/
Show
Sleep 7s

# CC: En resumen: versa ssh-test para la conexión, versa history para ver qué pasó, versa rollback para volver.
Sleep 2.1s
Sleep 5s
# CC: Y la guía de problemas frecuentes está en versadeploy.websystem.cl. ¡Nos vemos!
Sleep 2.5s
Sleep 4s
