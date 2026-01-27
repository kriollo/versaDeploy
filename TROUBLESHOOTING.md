# versaDeploy Troubleshooting Guide

This guide covers common issues and their solutions when using versaDeploy.

## Table of Contents

- [Configuration Issues](#configuration-issues)
- [SSH Connection Problems](#ssh-connection-problems)
- [Build Failures](#build-failures)
- [Deployment Errors](#deployment-errors)
- [Rollback Issues](#rollback-issues)

## Configuration Issues

### Error: "Project name is missing in config"

**Cause**: The `project` field is not defined in `deploy.yml`.

**Solution**:

```yaml
project: "my-app-name" # Add this at the top of deploy.yml
environments:
  # ...
```

### Error: "SSH key has insecure permissions"

**Cause**: SSH private key has permissions that are too open (e.g., 0644).

**Solution** (Linux/macOS):

```bash
chmod 600 ~/.ssh/deploy_key
```

**Solution** (Windows):

1. Right-click the key file → Properties
2. Security tab → Advanced
3. Disable inheritance
4. Remove all users except your account
5. Ensure your account has "Full control"

### Error: "remote_path must be an absolute path"

**Cause**: The `remote_path` in config is relative (e.g., `www/app`).

**Solution**:

```yaml
remote_path: "/var/www/app" # Must start with /
```

## SSH Connection Problems

### Error: "SSH Connection timed out"

**Cause**: Remote host is unreachable or firewall blocking port 22.

**Solution**:

1. Test SSH connection manually:
   ```bash
   ssh -i ~/.ssh/deploy_key user@host
   ```
2. Check firewall rules on remote server
3. Verify host is correct in `deploy.yml`
4. Try specifying custom port:
   ```yaml
   ssh:
     port: 2222 # If using non-standard port
   ```

### Error: "SSH Authentication failed"

**Cause**: SSH key not authorized on remote server.

**Solution**:

1. Copy public key to remote server:
   ```bash
   ssh-copy-id -i ~/.ssh/deploy_key.pub user@host
   ```
2. Or manually add to `~/.ssh/authorized_keys` on remote server
3. Ensure correct user in config:
   ```yaml
   ssh:
     user: "deploy" # Must match remote user
   ```

### Error: "SSH key not found"

**Cause**: Path to SSH key is incorrect or file doesn't exist.

**Solution**:

1. Verify key exists:
   ```bash
   ls -la ~/.ssh/deploy_key
   ```
2. Use absolute path in config:
   ```yaml
   ssh:
     key_path: "/home/user/.ssh/deploy_key"
   ```
3. Or use environment variable:
   ```yaml
   ssh:
     key_path: "${SSH_KEY_PATH}"
   ```

## Build Failures

### Error: "Composer command failed"

**Cause**: Composer dependencies can't be resolved or composer not installed.

**Solution**:

1. Test composer locally:
   ```bash
   composer install --no-dev --optimize-autoloader
   ```
2. Check `composer.json` for syntax errors
3. Ensure all dependencies are available
4. Try clearing composer cache:
   ```bash
   composer clear-cache
   ```

### Error: "Go build failed"

**Cause**: Go compilation errors or missing dependencies.

**Solution**:

1. Test build locally:
   ```bash
   GOOS=linux GOARCH=amd64 go build -o bin/app
   ```
2. Run `go mod tidy` to sync dependencies
3. Check for compilation errors in Go code
4. Verify target OS/ARCH in config:
   ```yaml
   builds:
     go:
       target_os: "linux" # Must match remote server
       target_arch: "amd64"
   ```

### Error: "NPM command failed"

**Cause**: npm dependencies can't be installed or package.json errors.

**Solution**:

1. Test npm locally:
   ```bash
   npm ci --only=production
   ```
2. Delete `node_modules` and `package-lock.json`, then retry
3. Check `package.json` for syntax errors
4. Ensure Node.js version compatibility

### Error: "Compile failed for [file]"

**Cause**: Custom compiler script failed.

**Solution**:

1. Test compiler manually:
   ```bash
   ./compiler.sh src/app.js
   ```
2. Check compiler script has execute permissions:
   ```bash
   chmod +x compiler.sh
   ```
3. Verify `{file}` placeholder in config:
   ```yaml
   builds:
     frontend:
       compile_command: "./compiler.sh {file}" # {file} is required
   ```

## Deployment Errors

### Error: "deploy.lock not found on remote server"

**Cause**: First deployment without `--initial-deploy` flag.

**Solution**:

```bash
versa deploy production --initial-deploy
```

### Error: "Working directory has uncommitted changes"

**Cause**: Local git repository has uncommitted files.

**Solution**:

1. Commit changes:
   ```bash
   git add .
   git commit -m "Deploy changes"
   ```
2. Or stash changes:
   ```bash
   git stash
   ```

### Error: "failed to upload artifact"

**Cause**: Network issues or insufficient disk space on remote server.

**Solution**:

1. Check remote disk space:
   ```bash
   ssh user@host "df -h /var/www/app"
   ```
2. Check network connection
3. Retry deployment
4. Check SFTP permissions on remote server

### Error: "Post-deploy hook failed (rolled back)"

**Cause**: Post-deploy script returned non-zero exit code.

**Solution**:

1. Test hook manually on remote server:
   ```bash
   ssh user@host "cd /var/www/app/current && php artisan cache:clear"
   ```
2. Check hook script for errors
3. Verify paths in hook commands
4. Add error handling to hooks:
   ```yaml
   post_deploy:
     - "php artisan cache:clear || true" # Continue on error
   ```

## Rollback Issues

### Error: "No previous release to rollback to"

**Cause**: Only one release exists on remote server.

**Solution**:

- This is expected for first deployment
- Deploy at least twice before rollback is possible
- Check available releases:
  ```bash
  versa status production
  ```

### Error: "Could not determine previous release"

**Cause**: Release directory structure is corrupted.

**Solution**:

1. Check releases on remote server:
   ```bash
   ssh user@host "ls -la /var/www/app/releases/"
   ```
2. Verify `current` symlink:
   ```bash
   ssh user@host "readlink /var/www/app/current"
   ```
3. Manually fix symlink if needed:
   ```bash
   ssh user@host "ln -sfn /var/www/app/releases/20260127-120000 /var/www/app/current"
   ```

## General Tips

### Enable Debug Mode

For detailed output:

```bash
versa deploy production --debug --log-file deploy.log
```

### Dry Run

Preview changes without deploying:

```bash
versa deploy production --dry-run
```

### Check Status

View current deployment state:

```bash
versa status production
```

### Manual Cleanup

If deployment gets stuck, clean up manually:

```bash
# Remove staging directories
ssh user@host "rm -rf /var/www/app/releases/*.staging"

# Remove old releases (keep last 5)
ssh user@host "cd /var/www/app/releases && ls -t | tail -n +6 | xargs rm -rf"
```

## Getting Help

If you encounter an issue not covered here:

1. Run with `--debug` flag for detailed logs
2. Check the [GitHub Issues](https://github.com/user/versaDeploy/issues)
3. Include error message, config (sanitized), and debug logs when reporting

## Common Patterns

### Deploying from CI/CD

```yaml
# .github/workflows/deploy.yml
- name: Deploy to Production
  run: |
    versa deploy production --log-file deploy.log
  env:
    SSH_KEY_PATH: ${{ secrets.SSH_KEY_PATH }}
```

### Multiple Environments

```bash
# Deploy to staging first
versa deploy staging

# Test staging
curl https://staging.example.com/health

# Deploy to production
versa deploy production
```

### Emergency Rollback

```bash
# Quick rollback
versa rollback production

# Verify rollback
versa status production
```
