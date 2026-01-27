# versaDeploy Quick Start Guide

## 🚀 Get Started in 5 Minutes

### Step 1: Build versaDeploy

```bash
git clone <this-repo>
cd versaDeploy
go build -o versa ./cmd/versa/main.go
sudo mv versa /usr/local/bin/  # or add to PATH
```

### Step 2: Create deploy.yml

Copy the example and customize:

```bash
cp deploy.example.yml deploy.yml
nano deploy.yml
```

**Minimal configuration:**

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
```

### Step 3: Prepare Remote Server

SSH into your server and create the base directory:

```bash
ssh deploy@your-server.com
mkdir -p /var/www/app/releases
exit
```

### Step 4: First Deploy

From your local project directory:

```bash
versa deploy production --initial-deploy
```

**What happens:**
1. ✅ Clones your repo to temp directory
2. ✅ Calculates SHA256 hashes for all files
3. ✅ Runs builds (composer, go build, frontend compiler)
4. ✅ Creates release artifact
5. ✅ Uploads to server via SSH
6. ✅ Creates symlink: `/var/www/app/current` → `releases/YYYYMMDD-HHMMSS`
7. ✅ Saves deploy.lock for next time

### Step 5: Subsequent Deploys

```bash
# Make changes to your code
git commit -am "Updated feature"

# Deploy (detects only changed files)
versa deploy production
```

### Step 6: Rollback (if needed)

```bash
versa rollback production
```

Instantly switches symlink to previous release.

## 📋 Common Scenarios

### PHP + Composer Project

```yaml
builds:
  php:
    enabled: true
    composer_command: "composer install --no-dev --optimize-autoloader"
```

### Go API Server

```yaml
builds:
  go:
    enabled: true
    target_os: "linux"
    target_arch: "amd64"
    binary_name: "api"
```

Server runs: `/var/www/app/current/bin/api`

### Vue.js Frontend with Custom Compiler

```yaml
builds:
  frontend:
    enabled: true
    compile_command: "./compiler.sh {file}"
    npm_command: "npm ci --only=production"
```

Create `compiler.sh`:

```bash
#!/bin/bash
FILE=$1
OUTPUT="public/$(basename $FILE .vue).js"
# Your custom compilation logic here
vue-compiler "$FILE" > "$OUTPUT"
```

### Full Stack (PHP + Go + Vue)

```yaml
builds:
  php:
    enabled: true
  go:
    enabled: true
    target_os: "linux"
    target_arch: "amd64"
    binary_name: "api"
  frontend:
    enabled: true
    compile_command: "./compiler.sh {file}"
```

## 🔧 Troubleshooting

### "deploy.lock not found" Error

**Solution:** Add `--initial-deploy` flag on first deploy:
```bash
versa deploy production --initial-deploy
```

### "SSH key has insecure permissions"

**Solution:** Fix key permissions:
```bash
chmod 600 ~/.ssh/id_rsa
```

### "composer install failed"

**Solution:** Test locally first:
```bash
composer install --no-dev
```

### Dry Run (Test Without Deploying)

```bash
versa deploy production --dry-run
```

Shows what would be deployed without actually deploying.

## 📊 Understanding Releases

Remote server structure:

```
/var/www/app/
├── releases/
│   ├── 20260127-120000/  ← Release 1
│   ├── 20260127-130000/  ← Release 2
│   └── 20260127-140000/  ← Release 3 (current)
├── current → releases/20260127-140000/  ← Symlink
└── deploy.lock  ← State tracking
```

**Point your web server to:** `/var/www/app/current/public`

**Nginx example:**
```nginx
server {
    root /var/www/app/current/public;
    # ...
}
```

**Apache example:**
```apache
DocumentRoot /var/www/app/current/public
```

## 🎯 Pro Tips

1. **Always commit before deploying** - versaDeploy checks for uncommitted changes
2. **Use --dry-run first** - See what will change before deploying
3. **Test builds locally** - Run composer/go build/npm commands manually first
4. **Keep logs** - Use `--log-file deploy.log` for debugging
5. **Multiple environments** - Define staging + production in same deploy.yml

## 📚 Next Steps

- Read full [README.md](README.md) for advanced features
- Set up CI/CD to invoke `versa deploy production`
- Configure post-deploy health checks
- Add custom route cache regeneration

## 🆘 Getting Help

```bash
versa --help
versa deploy --help
versa rollback --help
versa status --help
```

---

**You're ready to deploy! 🎉**
