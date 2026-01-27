# versaDeploy

A production-grade deployment engine written in Go that deploys PHP, Go, and Vue.js projects with **zero compilation in production**.

## Features

✅ **Deterministic deployments** - SHA256 change detection, no heuristics  
✅ **Selective builds** - Only rebuild what changed (PHP/Go/Frontend)  
✅ **Atomic deployments** - Instant symlink switching, zero downtime  
✅ **Instant rollback** - Revert to previous release in <1 second  
✅ **No production compilation** - All builds happen locally/CI  
✅ **SSH-based** - Secure key-based authentication

## Installation

```bash
go build -o versa ./cmd/versa/main.go
sudo mv versa /usr/local/bin/
```

Or download pre-built binaries from releases.

## Quick Start

### 1. Create `deploy.yml` in your project root

```yaml
project: "my-app"

environments:
  production:
    ssh:
      host: "prod.example.com"
      user: "deploy"
      key_path: "~/.ssh/deploy_key"
      port: 22

    remote_path: "/var/www/app"

    builds:
      php:
        enabled: true
        composer_command: "composer install --no-dev --optimize-autoloader --classmap-authoritative"

      go:
        enabled: true
        target_os: "linux"
        target_arch: "amd64"
        binary_name: "api-server"
        build_flags: "-ldflags='-s -w'"

      frontend:
        enabled: true
        compile_command: "./compiler.sh {file}"
        npm_command: "npm ci --only=production"

    post_deploy:
      - "php /var/www/app/current/app/clear-cache.php"
      - "curl -f http://localhost/health || exit 1"

    route_files:
      - "app/routes.php"

    ignored_paths:
      - ".git"
      - "tests"
      - "node_modules/.cache"
      - "vendor/bin"
```

### 2. First Deployment

```bash
versa deploy production --initial-deploy
```

### 3. Subsequent Deployments

```bash
versa deploy production
```

### 4. Rollback

```bash
versa rollback production
```

### 5. Check Status

```bash
versa status production
```

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│ Local Machine (Build Environment)                          │
├─────────────────────────────────────────────────────────────┤
│ 1. Clone repo to clean temp directory                       │
│ 2. Fetch deploy.lock from remote server                     │
│ 3. Calculate SHA256 hashes → ChangeSet                      │
│ 4. Selective builds:                                        │
│    • PHP: composer install + copy vendor/                   │
│    • Go: GOOS=linux GOARCH=amd64 go build                   │
│    • Frontend: ./compiler.sh {file}                         │
│ 5. Generate release artifact with manifest.json             │
│ 6. Upload artifact via SFTP                                 │
│ 7. Atomic symlink switch on remote                          │
│ 8. Execute post-deploy hooks                                │
│ 9. Update deploy.lock on remote                             │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│ Remote Server (Production)                                  │
├─────────────────────────────────────────────────────────────┤
│ /var/www/app/                                               │
│ ├── releases/                                               │
│ │   ├── 20260127-120000/                                    │
│ │   ├── 20260127-130000/                                    │
│ │   └── 20260127-140000/  ← New release                     │
│ ├── current → releases/20260127-140000/  ← Atomic switch    │
│ └── deploy.lock                                             │
└─────────────────────────────────────────────────────────────┘
```

## Change Detection Rules

### PHP

- **composer.json changed** → Run `composer install`, copy vendor/
- **.php files changed** → Copy to artifact
- **.twig files changed** → Copy to artifact + mark Twig cache cleanup

### Go

- **go.mod or .go files changed** → Cross-compile binary for target OS/ARCH

### Frontend

- **package.json changed** → Run `npm ci`, copy node_modules/
- **.js/.vue/.ts files changed** → Execute custom compiler per file

## Configuration Reference

### SSH Configuration

```yaml
ssh:
  host: "example.com" # Required
  user: "deploy" # Required
  key_path: "~/.ssh/id_rsa" # Required (must be 0600 permissions)
  port: 22 # Optional (default: 22)
  known_hosts_file: "~/.ssh/known_hosts" # Optional (defaults to ~/.ssh/known_hosts)
```

### Build Configuration

#### PHP

```yaml
builds:
  php:
    enabled: true
    composer_command: "composer install --no-dev --optimize-autoloader"
```

#### Go

```yaml
builds:
  go:
    enabled: true
    target_os: "linux" # Required: linux, darwin, windows
    target_arch: "amd64" # Required: amd64, arm64, 386
    binary_name: "app" # Required: output binary name
    build_flags: "-tags prod" # Optional: additional go build flags
```

#### Frontend

```yaml
builds:
  frontend:
    enabled: true
    compile_command: "./build.sh {file}" # {file} is replaced with relative path
    npm_command: "npm ci --only=production"
```

### Post-Deploy Hooks

Commands executed after successful deployment. **Automatic rollback on failure**.

```yaml
post_deploy:
  - "php artisan cache:clear"
  - "curl -f http://localhost/health"
  - "systemctl reload php-fpm"
```

### Route Files

Files that trigger route cache regeneration when changed:

```yaml
route_files:
  - "app/routes.php"
  - "config/routes.yml"
```

## CLI Commands

### Deploy

```bash
versa deploy <environment> [flags]

Flags:
  --dry-run          Show changes without deploying
  --initial-deploy   Required for first deployment
  --config PATH      Config file path (default: deploy.yml)
  --verbose          Verbose output
  --debug            Debug mode
  --log-file PATH    Write logs to file
```

### Rollback

```bash
versa rollback <environment> [flags]
```

### Status

```bash
versa status <environment> [flags]
```

## Security

- ✅ SSH key-based authentication only (no passwords)
- ✅ SSH key must have 0600 permissions
- ✅ No interactive prompts (BatchMode=yes)
- ✅ No secrets committed to source code

## Release Management

- **Naming**: Timestamp format `YYYYMMDD-HHMMSS` (e.g., `20260127-120000`)
- **Retention**: Keeps last 5 releases automatically
- **Atomic switching**: Two-step symlink creation prevents race conditions

## Error Handling

versaDeploy fails fast with clear error messages:

```bash
# No deploy.lock on first deploy
❌ deploy.lock not found - use --initial-deploy flag

# Uncommitted changes
❌ Working directory has uncommitted changes - commit or stash first

# SSH key permissions
❌ SSH key has insecure permissions 0644 (should be 0600)

# Build failure
❌ composer install failed: [output]

# Post-deploy hook failure
❌ Post-deploy hook failed (rolled back): healthcheck returned 1
```

## Troubleshooting

### Build fails locally

```bash
# Test build commands manually
composer install --no-dev
GOOS=linux GOARCH=amd64 go build -o bin/app
./compiler.sh src/app.js
```

### Upload fails

```bash
# Test SSH connection
ssh -i ~/.ssh/deploy_key deploy@prod.example.com

# Check permissions
ls -la ~/.ssh/deploy_key  # Should be -rw------- (0600)
```

### Post-deploy hook fails

```bash
# Test hooks manually on server
ssh deploy@prod.example.com "cd /var/www/app/current && php artisan cache:clear"
```

## Advanced Usage

### Multiple Environments

```yaml
environments:
  staging:
    ssh:
      host: "staging.example.com"
      user: "deploy"
      key_path: "~/.ssh/staging_key"
    remote_path: "/var/www/staging"
    # ... config ...

  production:
    ssh:
      host: "prod.example.com"
      user: "deploy"
      key_path: "~/.ssh/prod_key"
    remote_path: "/var/www/production"
    # ... config ...
```

```bash
versa deploy staging
versa deploy production
```

### Environment Variables in Config

```yaml
ssh:
  host: "${DEPLOY_HOST}"
  user: "${DEPLOY_USER}"
  key_path: "${SSH_KEY_PATH}"
```

### Custom Compiler Example

```bash
#!/bin/bash
# compiler.sh
FILE=$1
OUTPUT="public/$(basename $FILE .vue).js"

# Custom Vue compiler
vue-compiler "$FILE" > "$OUTPUT"

# Rewrite imports
sed -i 's|from "vue"|from "./node_modules/vue/dist/vue.esm.js"|g' "$OUTPUT"
```

## Non-Goals

versaDeploy is **NOT**:

- ❌ A CI/CD pipeline (invoke it FROM CI)
- ❌ A bundler/tree-shaker (use your custom tools)
- ❌ A container orchestrator
- ❌ A blue-green deployment system

## Testing

versaDeploy includes comprehensive unit tests for core components:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run tests verbosely
go test ./... -v

# Run specific package tests
go test ./internal/changeset/...
go test ./internal/config/...
go test ./internal/state/...
```

### Test Coverage

- ✅ **ChangeSet Detection** - SHA256 hashing, file categorization, ignore patterns
- ✅ **Config Validation** - YAML parsing, SSH key validation, environment variables
- ✅ **State Management** - deploy.lock parsing, serialization, version checking

## License

MIT

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

---

**Built with ❤️ for deterministic deployments**
