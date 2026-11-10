# deploy.yml examples

Copy the one for your framework to your project as `deploy.yml`, change the values marked `change`, and validate it with `versa config validate production`.

| File | Framework | Runs as |
| --- | --- | --- |
| [deploy.laravel.yml](deploy.laravel.yml) | Laravel (+ Vite) | PHP-FPM behind nginx, `doc_root: current/app/public` |
| [deploy.laravel-frankenphp.yml](deploy.laravel-frankenphp.yml) | Laravel on FrankenPHP | FrankenPHP service behind nginx (no PHP-FPM) |
| [deploy.versawys-frankenphp.yml](deploy.versawys-frankenphp.yml) | versaWYS on FrankenPHP | FrankenPHP service behind nginx; `config.json` in `shared/versaWYS/config` |
| [deploy.nextjs.yml](deploy.nextjs.yml) | Next.js | `npm start` service; `node_modules` installed on the server |
| [deploy.nuxt.yml](deploy.nuxt.yml) | Nuxt | `node .output/server/index.mjs` service |
| [deploy.flask.yml](deploy.flask.yml) | Flask | gunicorn service, virtualenv built on the server |
| [deploy.fastapi.yml](deploy.fastapi.yml) | FastAPI | uvicorn service, virtualenv built on the server |
| [deploy.gin.yml](deploy.gin.yml) | Go + Gin | static binary built on your machine |

All of them keep the server's `.env` (versaWYS: `config.json`) in `shared/`: the first deploy creates it from `.env.example`, and you edit it from the TUI (`versa` → **Shared** → the file → `Enter` → `e` → `Ctrl+S`). Details: [`.env` and secrets](../doc/DEPLOY.md#environment-variables-and-secrets-env-configjson).

Each example is deployed with the real framework by the end-to-end test (`test/e2e`) on Ubuntu 24.04 and AlmaLinux 9 (FrankenPHP: Ubuntu 24.04, with versaTodo too). Every option is described in [doc/DEPLOY.md](../doc/DEPLOY.md); the `provision:` block, in [doc/SERVICES.md](../doc/SERVICES.md).

## CI

[github-actions.yml](github-actions.yml) deploys from GitHub Actions: staging on every push to `main`, production on every `v*` tag (with `deploy_tags: "v*"` in production, versa refuses untagged commits from anywhere). It installs versa with [install.sh](../install.sh), which checks the binary's sha256.
