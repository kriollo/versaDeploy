# How it works

What versaDeploy does under the hood, as diagrams. Every step is detailed in the [reference](reference/DEPLOY.md).

## What `versa deploy` does

The heavy lifting happens on **your machine** (or in CI); the server only receives the result and switches to it.

```mermaid
flowchart TD
    A[versa deploy production] --> B{Anything changed since<br>the last deploy?<br><small>SHA256 of every file</small>}
    B -- No --> Z0[Nothing to do]
    B -- Yes --> C[Build on your machine<br><small>composer · npm · go build</small>]
    C --> D[Compress and upload over SFTP<br><small>only what changed, in parallel</small>]
    D --> E[Create releases/NEW on the server<br><small>reuses vendor, node_modules, .venv</small>]
    E --> F[Link shared/<br><small>.env, storage, uploads…</small>]
    F --> G[pre_deploy_server hooks]
    G --> H[Switch current → releases/NEW<br><small>one symlink: zero downtime</small>]
    H --> I[services_reload<br><small>PHP-FPM, nginx</small>]
    I --> J[post_deploy hooks<br><small>migrations, caches</small>]
    J --> K[Restart services<br><small>Go, Python, Node</small>]
    K --> L{Health check}
    L -- OK --> M[✅ Record the deploy<br>clean old releases<br>notify Slack/Discord/Telegram]
    J -- fails --> R
    K -- fails --> R
    L -- fails --> R[↩️ Automatic rollback<br><small>current goes back to the previous release</small>]
    R --> N[❌ Failed deploy: kept in the history<br>with its log, and notified]
```

## First deploy to a new server

```mermaid
flowchart LR
    A[New VPS<br><small>root over SSH</small>] --> B[deploy.yml<br><small>from examples/</small>]
    B --> C[versa → Services → I<br><small>installs the stack, creates the<br>deploy user and the database</small>]
    C --> D[versa deploy production<br>--initial-deploy]
    D --> E[versa → Shared → .env<br><small>production settings</small>]
    E --> F[versa deploy production<br>--force<br><small>migrations and caches</small>]
    F --> G[🚀 App online]
```

Step by step in [Getting started](start.md), and on video in [Videos](videos.md).

## Every day

```mermaid
sequenceDiagram
    actor Dev as You
    participant Git
    participant V as versa
    participant S as Server
    Dev->>Git: git commit
    Dev->>V: versa deploy production
    V->>V: detects changes and builds
    V->>S: uploads only what changed
    V->>S: switches to the new release
    S-->>V: health check OK
    V-->>Dev: ✅ done (and recorded in versa history)
    Note over Dev,S: Something wrong in production?
    Dev->>V: versa rollback production
    V->>S: current → the previous release (1 second)
```

## Where things live on the server

```mermaid
flowchart LR
    current["current<br><small>symlink</small>"] --> R3
    subgraph releases["releases/"]
        R1["20261101-090000/app"]
        R2["20261105-153000/app"]
        R3["20261109-120000/app<br><small>the live release</small>"]
    end
    subgraph shared["shared/ (survives every deploy)"]
        ENV[".env"]
        ST["storage/"]
    end
    R1 -. ".env, storage" .-> shared
    R2 -. ".env, storage" .-> shared
    R3 -. ".env, storage" .-> shared
```

- Your **production `.env`** lives only in `shared/`: it never goes through git or your machine. The first deploy creates it from `.env.example`; you edit it in the TUI's **Shared** view.
- Every release links the same `.env`; rolling back doesn't change it.
- The last `releases_to_keep` releases (5 by default) are kept: those are the ones a rollback can go back to.

## Rollback

```mermaid
flowchart LR
    subgraph Before
        C1[current] --> V3a[v3 ❌]
    end
    subgraph After
        C2[current] --> V2b[v2 ✅]
    end
    Before -- "versa rollback production" --> After
```

Nothing is built or uploaded again: the previous release is still on the server, and versa only moves the link, reloads PHP-FPM/nginx and restarts the services. The same happens by itself when a deploy fails after switching.

## Staging, production and CI

```mermaid
flowchart LR
    P[git push to main] --> S[GitHub Actions<br>versa deploy staging]
    T[git tag v1.2.0<br>git push --tags] --> PR[GitHub Actions<br>versa deploy production]
    PR --> TG{Does the commit have<br>a v* tag?}
    TG -- Yes --> OK[✅ Deploy]
    TG -- No --> NO[⛔ Refused<br><small>deploy_tags: v*</small>]
```

Each environment has its own server (or its own `remote_path`) and its own `.env`. With `deploy_tags: "v*"`, production only accepts tagged versions, even from someone's laptop. The example workflow is [examples/github-actions.yml](https://github.com/kriollo/versaDeploy/blob/main/examples/github-actions.yml).

## The Services view (provisioning)

```mermaid
flowchart TD
    A[versa → Services] --> B[Detects the server<br><small>distro, what's installed, versions</small>]
    B --> C[Ticks what your app needs<br><small>from deploy.yml and the repo</small>]
    C --> D{Does it clash with<br>something installed?}
    D -- Yes --> X[⛔ Not installed<br><small>never nginx over Apache,<br>never a second PHP</small>]
    D -- No --> E[I: installs over SSH as root<br><small>no agent</small>]
    E --> F[What's installed is left alone<br><small>no config changes, no restarts</small>]
```

Details in [Server services](reference/SERVICES.md).
