# versaDeploy Quick Start Guide

## 🚀 Get Started in 5 Minutes

### Step 1: Install versaDeploy

versaDeploy is a single Go binary. You can build it from source on any platform.

#### Windows

```powershell
# Open PowerShell and run:
go build -o versa.exe ./cmd/versa/main.go
# Add the current directory to your PATH or move versa.exe to a folder in your PATH
```

#### Linux / macOS

```bash
go build -o versa ./cmd/versa/main.go
sudo mv versa /usr/local/bin/
```

### Step 2: Initialize your project

Run the following command in your project root:

```bash
versa init
```

This creates a `deploy.yml` template. Edit it to match your environment.

### Step 3: Configure `deploy.yml`

A minimal configuration for a PHP project looks like this:

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

> [!NOTE]
> For a full list of configuration options, see **[DEPLOY.md](DEPLOY.md)**.

### Step 4: First Deployment

Before running the first deploy, ensure you have SSH access to your Linux server and the `remote_path` exists (or the user has permissions to create it).

```bash
versa deploy production --initial-deploy
```

**What happens:**

1. ✅ **Detection**: Calculates SHA256 hashes for all local files.
2. ✅ **First Build**: Runs your configured builds (composer, go build, etc.).
3. ✅ **Package**: Creates a full release artifact (including all non-ignored files).
4. ✅ **Upload**: Transports the artifact to the server via SFTP.
5. ✅ **Atomic Switch**: Creates the `/var/www/app/current` symlink.
6. ✅ **State**: Saves the `deploy.lock` on the server for future diffing.

### Step 5: Subsequent Deploys

```bash
# Make changes to your code
versa deploy production
```

versaDeploy will now only upload **changed files**, making deployments incredibly fast.

### Step 6: Rollback (if needed)

```bash
versa rollback production
```

Instantly switches the symlink back to the previous successful release.

---

## 📋 Common Scenarios

### Framework in a Subdirectory

If your framework is in an `api/` folder:

```yaml
builds:
  php:
    enabled: true
    root: "api"
```

### Static Assets (Images, Storage)

versaDeploy automatically detects and copies images or PDFs if they change. Ensure your `public` folder is not ignored in `deploy.yml`.

### Post-Deploy Tasks

Run migrations or clear cache automatically:

```yaml
post_deploy:
  - "php artisan migrate --force"
  - "php artisan cache:clear"
```

---

## 🎯 Pro Tips

1. **Dry Run**: Use `versa deploy production --dry-run` to see exactly what will be uploaded without actually doing it.
2. **Local Pathing**: You can use `~/` or absolute paths for your SSH keys on both Windows and Linux.
3. **Log Files**: Use `--log-file deploy.log` to keep a history of your deployments.

---

**You're ready to deploy! 🎉**
