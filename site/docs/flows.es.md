# Flujos

Cómo funciona versaDeploy por dentro, en diagramas. Para cada paso hay más detalle en la [referencia](reference/DEPLOY.md).

## Qué hace `versa deploy`

Todo lo pesado ocurre en **tu computador** (o en el CI); el servidor solo recibe el resultado y lo activa.

```mermaid
flowchart TD
    A[versa deploy production] --> B{¿Cambió algo desde<br>el último deploy?<br><small>SHA256 de cada archivo</small>}
    B -- No --> Z0[Nada que hacer]
    B -- Sí --> C[Compila en tu computador<br><small>composer · npm · go build</small>]
    C --> D[Comprime y sube por SFTP<br><small>solo lo que cambió, en paralelo</small>]
    D --> E[Crea releases/NUEVA en el servidor<br><small>reutiliza vendor, node_modules, .venv</small>]
    E --> F[Enlaza shared/<br><small>.env, storage, uploads…</small>]
    F --> G[Hooks pre_deploy_server]
    G --> H[Cambia current → releases/NUEVA<br><small>un symlink: cero downtime</small>]
    H --> I[services_reload<br><small>PHP-FPM, nginx</small>]
    I --> J[Hooks post_deploy<br><small>migraciones, caches</small>]
    J --> K[Reinicia los services<br><small>Go, Python, Node</small>]
    K --> L{Health check}
    L -- OK --> M[✅ Registra el deploy<br>limpia releases viejas<br>avisa a Slack/Discord/Telegram]
    J -- falla --> R
    K -- falla --> R
    L -- falla --> R[↩️ Rollback automático<br><small>current vuelve a la versión anterior</small>]
    R --> N[❌ Deploy fallido: queda en el historial<br>con su log, y se avisa]
```

## Primer deploy en un servidor nuevo

```mermaid
flowchart LR
    A[VPS nuevo<br><small>acceso root por SSH</small>] --> B[deploy.yml<br><small>de examples/</small>]
    B --> C[versa → Services → I<br><small>instala el stack, crea el usuario<br>deploy y la base de datos</small>]
    C --> D[versa deploy production<br>--initial-deploy]
    D --> E[versa → Shared → .env<br><small>datos de producción</small>]
    E --> F[versa deploy production<br>--force<br><small>migraciones y caches</small>]
    F --> G[🚀 App en línea]
```

Paso a paso en [Primeros pasos](start.md), y en video en [Videos](videos.md).

## El día a día

```mermaid
sequenceDiagram
    actor Dev as Tú
    participant Git
    participant V as versa
    participant S as Servidor
    Dev->>Git: git commit
    Dev->>V: versa deploy production
    V->>V: detecta cambios y compila
    V->>S: sube solo lo que cambió
    V->>S: activa la nueva versión
    S-->>V: health check OK
    V-->>Dev: ✅ listo (y queda en versa history)
    Note over Dev,S: ¿Algo salió mal en producción?
    Dev->>V: versa rollback production
    V->>S: current → la versión anterior (1 segundo)
```

## Dónde vive cada cosa en el servidor

```mermaid
flowchart LR
    current["current<br><small>symlink</small>"] --> R3
    subgraph releases["releases/"]
        R1["20261101-090000/app"]
        R2["20261105-153000/app"]
        R3["20261109-120000/app<br><small>la versión en línea</small>"]
    end
    subgraph shared["shared/ (sobrevive a todos los deploys)"]
        ENV[".env"]
        ST["storage/"]
    end
    R1 -. ".env, storage" .-> shared
    R2 -. ".env, storage" .-> shared
    R3 -. ".env, storage" .-> shared
```

- Tu **`.env` de producción** vive solo en `shared/`: nunca pasa por git ni por tu computador. Se crea en el primer deploy a partir de `.env.example` y se edita desde la vista **Shared** de la TUI.
- Cada versión enlaza el mismo `.env`; volver atrás no lo cambia.
- Se guardan las últimas `releases_to_keep` versiones (5 por defecto): son las que puedes recuperar con un rollback.

## Rollback

```mermaid
flowchart LR
    subgraph Antes
        C1[current] --> V3a[v3 ❌]
    end
    subgraph Después
        C2[current] --> V2b[v2 ✅]
    end
    Antes -- "versa rollback production" --> Después
```

No se vuelve a compilar ni a subir nada: el código de la versión anterior sigue en el servidor, y versa solo mueve el enlace, recarga PHP-FPM/nginx y reinicia los services. Lo mismo ocurre solo cuando un deploy falla después de activarse.

## Staging, producción y CI

```mermaid
flowchart LR
    P[git push a main] --> S[GitHub Actions<br>versa deploy staging]
    T[git tag v1.2.0<br>git push --tags] --> PR[GitHub Actions<br>versa deploy production]
    PR --> TG{¿El commit tiene<br>un tag v*?}
    TG -- Sí --> OK[✅ Deploy]
    TG -- No --> NO[⛔ Rechazado<br><small>deploy_tags: v*</small>]
```

Cada entorno tiene su propio servidor (o su propio `remote_path`) y su propio `.env`. Con `deploy_tags: "v*"`, producción solo acepta versiones con tag, aunque alguien lo intente desde su computador. El workflow de ejemplo está en [examples/github-actions.yml](https://github.com/kriollo/versaDeploy/blob/main/examples/github-actions.yml). El [video de GitHub Actions](videos.md#github-actions) lo arma paso a paso.

## La vista Services (provisioning)

```mermaid
flowchart TD
    A[versa → Services] --> B[Detecta el servidor<br><small>distro, lo instalado, versiones</small>]
    B --> C[Marca lo que pide tu app<br><small>según deploy.yml y el repo</small>]
    C --> D{¿Choca con algo<br>ya instalado?}
    D -- Sí --> X[⛔ No lo instala<br><small>nunca nginx sobre Apache,<br>ni un segundo PHP</small>]
    D -- No --> E[I: instala por SSH como root<br><small>sin agentes</small>]
    E --> F[Lo ya instalado no se toca<br><small>ni su configuración, ni se reinicia</small>]
```

Detalles en [Servicios del servidor](reference/SERVICES.md).
