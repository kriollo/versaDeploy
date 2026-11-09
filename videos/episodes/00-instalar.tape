# Ep. 0 — Instalar versaDeploy
Source settings.tape
Set TypingSpeed 45ms

Hide
Type "source setup.sh mi-app fresh || exit" Enter
Sleep 1s
Ctrl+L
Show

# CC: versaDeploy despliega tus apps en un VPS Linux, por SSH y sin agentes.
Sleep 3s
# CC: Se instala con una sola línea, en Linux y macOS.
Sleep 1s
Type "curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh"
Sleep 500ms
Enter
# CC: El instalador descarga la última versión y comprueba su checksum sha256.
Hide
Wait+Screen@60s /Installed/
Show
Sleep 4s

# CC: Comprobemos que quedó instalado.
Type "versa version"
Sleep 300ms
Enter
Sleep 3s

# CC: Para actualizar más adelante basta con versa self-update.
Sleep 3s
# CC: En el próximo episodio preparamos el servidor. ¡Nos vemos!
Sleep 4s
