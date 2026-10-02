# 📦 Installation

versa is a single binary with no dependencies. Download it from the [latest release](https://github.com/kriollo/versaDeploy/releases/latest):

| OS      | Architecture          | File                       |
| :------ | :-------------------- | :------------------------- |
| Windows | x64                   | `versa_windows_amd64.exe`  |
| Windows | ARM64                 | `versa_windows_arm64.exe`  |
| Linux   | x64                   | `versa_linux_amd64`        |
| Linux   | ARM64                 | `versa_linux_arm64`        |
| macOS   | Intel                 | `versa_darwin_amd64`       |
| macOS   | Apple Silicon (M1–M4) | `versa_darwin_arm64`       |

## 🪟 Windows

1. Download `versa_windows_amd64.exe` and rename it to `versa.exe`.
2. Put it in a folder, e.g. `C:\versa`.
3. Add that folder to `PATH`: search for **"Edit the system environment variables"** → **Environment Variables** → select `Path` → **Edit** → **New** → `C:\versa`.
4. Open a **new** terminal and check:

   ```powershell
   versa version
   ```

> [!TIP]
> If you use `pre_deploy_local` hooks, also add Git's `sh` to `PATH` (`C:\Program Files\Git\bin`).

## 🐧 Linux

```bash
curl -L -o versa https://github.com/kriollo/versaDeploy/releases/latest/download/versa_linux_amd64
chmod +x versa
sudo mv versa /usr/local/bin/
versa version
```

## 🍎 macOS

```bash
# Apple Silicon; use versa_darwin_amd64 on Intel Macs
curl -L -o versa https://github.com/kriollo/versaDeploy/releases/latest/download/versa_darwin_arm64
chmod +x versa
sudo mv versa /usr/local/bin/
versa version
```

If macOS says the developer cannot be verified, allow it in **System Settings → Privacy & Security**, or run `xattr -d com.apple.quarantine /usr/local/bin/versa`.

## 🔄 Updating

```bash
versa self-update
```

Downloads the latest release for your platform and replaces the binary. It needs write access to the folder where `versa` is installed.

## 🛠 Building from source

Requires Go (see `go.mod` for the version).

```bash
git clone https://github.com/kriollo/versaDeploy.git
cd versaDeploy
go build -o versa ./cmd/versa          # versa.exe on Windows
```

## Next step

Follow [Getting Started](GETTING_STARTED.md) to configure your first deploy.
