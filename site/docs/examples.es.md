# Ejemplos

Un `deploy.yml` listo para cada framework, probado de punta a punta por el test e2e en Ubuntu 24.04 (y, salvo FrankenPHP, también en AlmaLinux 9). Cópialo a tu proyecto y cambia lo marcado con `# change`; [Primeros pasos](start.md) explica cada paso.

```bash
curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.laravel.yml
```

Todas las opciones están en la [referencia de configuración](reference/DEPLOY.md). Para CI hay un workflow de GitHub Actions al final.

## Laravel

PHP-FPM detrás de nginx, PostgreSQL, Redis, migraciones, cron y storage. [Video](videos.md#laravel).

??? example "deploy.laravel.yml"

    ```yaml
    --8<-- "examples/deploy.laravel.yml"
    ```

## Laravel con FrankenPHP

FrankenPHP como servicio en vez de PHP-FPM; nginx sigue sirviendo los estáticos.

??? example "deploy.laravel-frankenphp.yml"

    ```yaml
    --8<-- "examples/deploy.laravel-frankenphp.yml"
    ```

## Next.js

`npm start` como servicio; `node_modules` se instala en el servidor.

??? example "deploy.nextjs.yml"

    ```yaml
    --8<-- "examples/deploy.nextjs.yml"
    ```

## Nuxt

`.output/server` con Node como servicio. [Video](videos.md#nuxt).

??? example "deploy.nuxt.yml"

    ```yaml
    --8<-- "examples/deploy.nuxt.yml"
    ```

## FastAPI

uvicorn como servicio, virtualenv en el servidor, PostgreSQL. [Video](videos.md#fastapi).

??? example "deploy.fastapi.yml"

    ```yaml
    --8<-- "examples/deploy.fastapi.yml"
    ```

## Flask

gunicorn como servicio, virtualenv en el servidor.

??? example "deploy.flask.yml"

    ```yaml
    --8<-- "examples/deploy.flask.yml"
    ```

## Go (Gin)

Binario estático compilado en tu computador, como servicio.

??? example "deploy.gin.yml"

    ```yaml
    --8<-- "examples/deploy.gin.yml"
    ```

## GitHub Actions

Staging en cada push a `main`, producción en cada tag `v*`.

??? example "github-actions.yml"

    ```yaml
    --8<-- "examples/github-actions.yml"
    ```
