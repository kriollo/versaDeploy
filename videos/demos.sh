#!/bin/sh
# demos.sh laravel|fastapi|nuxt — creates the episode's project in .work/<name>, the
# way a developer has it before deploying: fresh framework, dependencies, a git commit.
set -eu
V=$(cd "$(dirname "$0")" && pwd)
commit() { git -C "$1" init -q -b main && git -C "$1" add -A && git -C "$1" -c user.name=Dev -c user.email=dev@example.com commit -qm "Primer commit"; }
case $1 in
laravel)
    d=$V/.work/mitienda
    rm -rf "$d"
    composer create-project -q laravel/laravel "$d"
    (cd "$d" && npm install --no-audit --no-fund --silent >/dev/null)
    commit "$d"
    ;;
fastapi)
    # a small real API: reads its settings from the environment (the server's .env,
    # loaded by systemd) and checks PostgreSQL in /health
    d=$V/.work/mi-api
    rm -rf "$d" && mkdir -p "$d"
    cat > "$d/main.py" <<'PY'
import os

import psycopg
from fastapi import FastAPI

app = FastAPI(title=os.getenv("APP_NAME", "Mi API"))


@app.get("/")
def home():
    return {"app": app.title, "env": os.getenv("APP_ENV", "local")}


@app.get("/health")
def health():
    with psycopg.connect(os.environ["DATABASE_URL"]) as conn:
        conn.execute("select 1")
    return {"status": "ok", "db": "ok"}
PY
    printf 'fastapi==0.162.0\nuvicorn[standard]==0.54.0\npsycopg[binary]==3.3.6\n' > "$d/requirements.txt"
    printf 'APP_NAME="Mi API"\nAPP_ENV=local\nDATABASE_URL=postgresql://usuario:clave@127.0.0.1:5432/mi_api\n' > "$d/.env.example"
    printf '.venv/\n__pycache__/\n.env\n' > "$d/.gitignore"
    commit "$d"
    ;;
nuxt)
    d=$V/.work/mi-sitio
    rm -rf "$d"
    (cd "$V/.work" && npx -y nuxi@4.0.1 init mi-sitio --template minimal --packageManager npm --gitInit false --no-install --force >/dev/null)
    # a page that shows the site name from the server's .env (NUXT_PUBLIC_SITE_NAME)
    cat > "$d/nuxt.config.ts" <<'TS'
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: false },
  runtimeConfig: { public: { siteName: 'Mi sitio' } },
})
TS
    cat > "$d/app/app.vue" <<'VUE'
<script setup lang="ts">
const { siteName } = useRuntimeConfig().public
useHead({ title: siteName })
</script>

<template>
  <main>
    <h1>{{ siteName }}</h1>
    <p>Nuxt con SSR, desplegado con versaDeploy.</p>
  </main>
</template>

<style>
body { font-family: system-ui, sans-serif; display: grid; place-items: center; min-height: 100vh; margin: 0; background: #0f172a; color: #e2e8f0; }
h1 { font-size: 3rem; color: #00dc82; }
</style>
VUE
    (cd "$d" && npm install --no-audit --no-fund --silent >/dev/null)
    printf 'NUXT_PUBLIC_SITE_NAME="Mi sitio"\n' > "$d/.env.example"
    commit "$d"
    ;;
*) echo "usage: $0 laravel|fastapi|nuxt"; exit 2 ;;
esac
echo "ready: $d"
