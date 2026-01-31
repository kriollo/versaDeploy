# versaDeploy Troubleshooting Guide

This guide covers common issues and their solutions when using versaDeploy across different platforms.

## Configuration Issues

### Error: "SSH key has insecure permissions"

**Cause**: SSH private key has permissions that are too open (e.g., 0644).

**Solution (Linux/macOS)**:

```bash
chmod 600 ~/.ssh/deploy_key
```

**Solution (Windows)**:

1. Right-click the key file → **Properties**.
2. **Security** tab → **Advanced**.
3. **Disable inheritance**.
4. Remove all users except your account.
5. Ensure your account has **Full control**.

### Error: "remote_path must be an absolute path"

**Cause**: The `remote_path` in config is relative.

**Solution**:

```yaml
remote_path: "/var/www/app" # Must start with /
```

## Build Failures

### Error: "executable file not found in %PATH%" (Windows)

**Cause**: `cmd.exe` or the specified build command (e.g., `composer`, `npm`) is not in your system PATH.

**Solution**:

1. Ensure the tool is installed and its folder is in your Environment Variables.
2. If using `sh` scripts on Windows, you must have a bash environment (like Git Bash) in your PATH.

### Error: "Composer command failed"

**Cause**: Dependencies can't be resolved or `composer.json` is missing from the `root`.

**Solution**:

1. Verify the `root` field in `deploy.yml` points to the folder containing `composer.json`.
2. Test locally: `composer install`.

### Error: "Generic assets not copied"

**Cause**: The folder or file might be listed in `ignored_paths` or `.gitignore`.

**Solution**:

1. Check `deploy.yml` `ignored_paths`.
2. Ensure the files are tracked by Git (versaDeploy ignores untracked files by default).

## Deployment Errors

### Error: "deploy.lock not found on remote server"

**Cause**: This is the first deployment to this environment.

**Solution**: Use the `--initial-deploy` flag:

```bash
versa deploy production --initial-deploy
```

### Error: "Post-deploy hook failed"

**Cause**: The command on the remote server returned an error.

**Solution**:

1. Test the command manually on the server: `ssh user@host "cd /var/www/app/current && your-command"`.
2. Check if the remote user has permissions to execute the command.

---

## Getting Help

If your issue is not listed:

1. Run with `--debug` and check `deploy.log`.
2. Verify the configuration reference in **[DEPLOY.md](DEPLOY.md)**.
