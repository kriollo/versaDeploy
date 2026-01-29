# versaDeploy

A production-grade deployment engine written in Go that deploys PHP, Go, and Frontend projects with **zero compilation in production**.

versaDeploy is designed for developers who want **deterministic, atomic deployments** from their local machines (Windows/Linux) or CI/CD environments to Linux servers.

## 🚀 Key Features

- ✅ **Deterministic deployments** - SHA256 change detection ensures only changed files are uploaded.
- ✅ **Selective builds** - Only rebuild what changed (PHP/Go/Frontend).
- ✅ **Atomic deployments** - Instant symlink switching for zero downtime.
- ✅ **Multi-Platform Support** - Run from Windows or Linux local dev environments.
- ✅ **No remote compilation** - Keep your production server clean; all builds happen locally or in CI.
- ✅ **Secure** - Built-in SSH/SFTP support with key-based authentication.

## 📖 Documentation

- 🚀 **[INSTALL.md](INSTALL.md)** - Installation guide for Windows, Linux, and macOS.
- ⚙️ **[DEPLOY.md](DEPLOY.md)** - Detailed reference for `deploy.yml` configuration.
- 🔧 **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Solutions for common errors.
- 📋 **[CHANGELOG.md](CHANGELOG.md)** - Recent changes and version history.
- 📚 **[QUICKSTART.md](QUICKSTART.md)** - 5-minute setup guide.

## 🛠️ Installation

```bash
# Build from source
go build -o versa ./cmd/versa/main.go

# Add to your PATH (Linux example)
sudo mv versa /usr/local/bin/
```

## 🏗️ How It Works

versaDeploy orchestrates the deployment from your **local machine** to the **remote server**:

1. **Detection**: Calculates SHA256 hashes of your local files.
2. **Comparison**: Compares local hashes with the `deploy.lock` from the remote server.
3. **Build**:
   - Executes `composer install` if PHP dependencies changed.
   - Cross-compiles Go binaries for the target OS/Arch.
   - Runs your frontend compiler (e.g., npm/vite) for modified assets.
4. **Upload**: Packages changed files and uploads them via SFTP to a new release directory.
5. **Switch**: Atomically updates the `current` symlink on the server.
6. **Cleanup**: Keeps a retention history of your last 5 releases.

## 💻 Environment Support

### Local (Developer Machine / CI)

- **Windows**: Full support via `cmd.exe` or PowerShell.
- **Linux / macOS**: Full support via standard shell.

### Remote (Production / Staging Server)

- **Linux**: Primary target for deployments and post-deploy hooks.

## 🧪 Testing

```bash
# Run all unit tests
go test ./... -cover
```

## ⚖️ License

MIT - See [LICENSE](LICENSE) for details.

---

**Built with ❤️ for deterministic deployments**
