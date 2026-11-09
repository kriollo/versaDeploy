# ⚡ Guía rápida { #quick-start }

La versión corta, para cuando ya tienes un servidor con acceso SSH. Para la configuración paso a paso (preparar el servidor, el web server, sudoers), mira la [Guía completa](GETTING_STARTED.md).

```bash
# 1. En la raíz de tu repositorio Git
versa init                                   # elige [1] servidor SSH

# 2. Edita deploy.yml (host, user, key_path, remote_path, builds) y haz commit
versa config validate production
versa info production                        # prueba SSH + SFTP y muestra datos del servidor

# 3. Primer deploy
versa deploy production --initial-deploy

# 4. Desde ahora
git commit -am "cambios"
versa deploy production
```

`deploy.yml` mínimo:

```yaml
project: "my-app"
environments:
  production:
    ssh:
      host: "your-server.com"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"
    remote_path: "/var/www/app"
    builds:
      php:
        enabled: true
    shared_paths:
      - ".env"                               # el primer deploy lo crea desde .env.example
    services_reload:
      - "sudo systemctl reload php8.2-fpm"
```

Apunta tu web server a `/var/www/app/current/app/public`.

> [!TIP]
> ¿Servidor nuevo? Agrega `provision: { ssh: { user: root }, domain: example.com }` al entorno, ejecuta `versa`, abre **Services** y presiona `I` para instalar el stack (o `P` para crear solo el usuario de deploy, sudoers y el directorio). Mira **[Servicios del servidor](SERVICES.md)**.

## Qué hace un deploy { #what-a-deploy-does }

1. Comprueba que todo esté commiteado y clona `HEAD` en un directorio temporal.
2. Compara los hashes de los archivos con el `deploy.lock` del servidor; si nada cambió, se detiene.
3. Compila en local (composer, npm, go, pip…), solo lo que cambió.
4. Sube el release en partes, en paralelo, a `releases/<fecha>/`.
5. Enlaza los `shared_paths`, cambia `current` de forma atómica, recarga los servicios, corre los hooks `post_deploy` y reinicia los `services`.
6. Vuelve atrás solo si falla un hook `post_deploy`, un servicio o el health check.

## Resumen de comandos { #cheat-sheet }

```bash
versa                                        # interfaz a pantalla completa (TUI)
versa diff production                        # qué se desplegaría
versa deploy production --dry-run            # lo mismo, por el flujo de deploy
versa deploy production --force              # volver a desplegar sin cambios
versa deploy staging,production              # varios entornos seguidos
versa status production                      # releases en el servidor
versa history production                     # historial de deploys y rollbacks
versa rollback production                    # release anterior
versa rollback production --to 20261001-183807
versa logs production                        # tail -f del log de Laravel
versa exec production "df -h"                # cualquier comando en el servidor
versa hooks production                       # volver a correr los hooks post_deploy
versa services-reload production             # volver a correr services_reload
versa service production                     # estado de los services (Go/Python/Node)
versa service production restart
versa service production logs --name api
versa deploy production --log-file deploy.log
```

Todos los comandos y opciones: [Comandos (CLI y TUI)](CLI_REFERENCE.md). Todas las opciones de configuración: [Configuración](DEPLOY.md).
