# Examples

A ready `deploy.yml` for each framework, tested end to end by the e2e test on Ubuntu 24.04 (and, except FrankenPHP, on AlmaLinux 9 too). Copy it to your project and change what's marked `# change`; [Getting started](start.md) walks through it.

```bash
curl -fsSL -o deploy.yml https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/deploy.laravel.yml
```

Every option is in the [configuration reference](reference/DEPLOY.md). A GitHub Actions workflow for CI is at the end.

## Laravel

PHP-FPM behind nginx, PostgreSQL, Redis, migrations, cron and storage. [Video](videos.md#laravel).

??? example "deploy.laravel.yml"

    ```yaml
    --8<-- "examples/deploy.laravel.yml"
    ```

## Laravel con FrankenPHP

FrankenPHP as a service instead of PHP-FPM; nginx still serves the static files.

??? example "deploy.laravel-frankenphp.yml"

    ```yaml
    --8<-- "examples/deploy.laravel-frankenphp.yml"
    ```

## Next.js

`npm start` as a service; `node_modules` is installed on the server.

??? example "deploy.nextjs.yml"

    ```yaml
    --8<-- "examples/deploy.nextjs.yml"
    ```

## Nuxt

`.output/server` run by Node as a service. [Video](videos.md#nuxt).

??? example "deploy.nuxt.yml"

    ```yaml
    --8<-- "examples/deploy.nuxt.yml"
    ```

## FastAPI

uvicorn as a service, virtualenv on the server, PostgreSQL. [Video](videos.md#fastapi).

??? example "deploy.fastapi.yml"

    ```yaml
    --8<-- "examples/deploy.fastapi.yml"
    ```

## Flask

gunicorn as a service, virtualenv on the server.

??? example "deploy.flask.yml"

    ```yaml
    --8<-- "examples/deploy.flask.yml"
    ```

## Go (Gin)

A static binary built on your machine, run as a service.

??? example "deploy.gin.yml"

    ```yaml
    --8<-- "examples/deploy.gin.yml"
    ```

## GitHub Actions

Staging on every push to `main`, production on every `v*` tag.

??? example "github-actions.yml"

    ```yaml
    --8<-- "examples/github-actions.yml"
    ```
