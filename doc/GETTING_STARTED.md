# 🚀 Getting Started with versaDeploy

Welcome! `versaDeploy` is a high-performance deployment tool designed to be fast, secure, and easy to use. This guide will help you set up your first deployment from scratch.

## 1. Prerequisites

### Local Machine (where you run `versa`)

- **Go**: Installed and in your PATH.
- **Git**: Installed and configured.
- **Tools**: `composer` (for PHP), `npm`/`pnpm` (for Frontend) if you plan to use build engines.

### Remote Server

- **SSH Access**: You must be able to connect via SSH key.
- **OS**: Linux (Ubuntu/Debian recommended).
- **Permissions**: The SSH user must have write access to the deployment directory.

### 2. Server Preparation

Before your first deploy, you need to prepare the remote directory structure.

1.  **Create the root folder**:

    ```bash
    mkdir -p /var/www/my-project
    chown -R my-user:my-group /var/www/my-project
    ```

2.  **Web Server Config**:

    #### Nginx

    Ensure your Nginx configuration uses `$realpath_root` to avoid OpCache issues with symlinks:

    ```nginx
    fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
    fastcgi_param DOCUMENT_ROOT $realpath_root;
    ```

    #### Apache

    In Apache, ensure that the `DocumentRoot` points to the `current` symlink and that `FollowSymLinks` is enabled. If you are using PHP-FPM, you should also use the absolute path to avoid caching issues:

    ```apache
    <VirtualHost *:80>
        DocumentRoot /var/www/my-project/current/public

        <Directory /var/www/my-project/current/public>
            Options +FollowSymLinks
            AllowOverride All
            Require all granted
        </Directory>

        # If using PHP-FPM, ensure paths are resolved correctly
        <FilesMatch \.php$>
            # Use ProxyPass Match to ensure real path is passed to FPM
            SetHandler "proxy:unix:/var/run/php/php8.2-fpm.sock|fcgi://localhost"
        </FilesMatch>
    </VirtualHost>
    ```

    > [!TIP]
    > Unlike Nginx, Apache generally handles symlink changes better by default, but having `Options +FollowSymLinks` is mandatory.

## 3. Initialize your Project

In your project root, run:

```bash
versa init
```

This will create a `deploy.yml` file.

## 4. Configure `deploy.yml`

Edit `deploy.yml` with your server details. Here is a minimal example for a Laravel/PHP project:

```yaml
project: "my-awesome-app"

environments:
  production:
    ssh:
      host: "1.2.3.4"
      user: "deploy"
      key_path: "~/.ssh/id_rsa"

    remote_path: "/var/www/my-project"

    builds:
      php:
        enabled: true
        composer_command: "composer install --no-dev --optimize-autoloader"

    shared_paths:
      - "storage"
      - ".env"

    post_deploy:
      - "php artisan migrate --force"
      - "php artisan cache:clear"
```

## 5. First Deployment

Since it's the first time, you must use the `--initial-deploy` flag to tell `versa` it's okay that there is no previous state on the server:

```bash
versa deploy production --initial-deploy
```

## 6. Normal Deployment

After the first one, just run:

```bash
versa deploy production
```

---

### Next Steps

- Read the [Full Configuration Guide](DEPLOY.md) for advanced options like Shared Paths and Build Engines.
- Check the [CLI Reference](CLI_REFERENCE.md) for all available commands.
