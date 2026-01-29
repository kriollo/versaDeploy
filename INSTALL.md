# 🚀 Installation Guide

`versaDeploy` is distributed as a single standalone binary. No external dependencies are required on your local machine other than the binary itself.

## 🪟 Windows

1. **Download**: Get the `versa-windows-amd64.exe` from the [Latest Release](https://github.com/your-repo/versaDeploy/releases).
2. **Rename**: Rename it to `versa.exe` for convenience.
3. **Add to PATH**:
   - Create a folder, e.g., `C:\tools\versa`.
   - Move `versa.exe` there.
   - Search for "Edit the system environment variables" in Windows.
   - Click "Environment Variables" -> Find `Path` in System variables -> Click "Edit".
   - Click "New" and add `C:\tools\versa`.
4. **Verify**: Open a new PowerShell/CMD and type:
   ```powershell
   versa version
   ```

## 🐧 Linux

1. **Download**:
   ```bash
   curl -L -o versa https://github.com/your-repo/versaDeploy/releases/download/v0.9.0-beta/versa-linux-amd64
   ```
2. **Make Executable**:
   ```bash
   chmod +x versa
   ```
3. **Move to PATH**:
   ```bash
   sudo mv versa /usr/local/bin/
   ```
4. **Verify**:
   ```bash
   versa version
   ```

## 🍎 macOS

### Intel (x86_64)

1. **Download**:
   ```bash
   curl -L -o versa https://github.com/your-repo/versaDeploy/releases/download/v0.9.0-beta/versa-darwin-amd64
   ```

### Apple Silicon (M1/M2/M3)

1. **Download**:

   ```bash
   curl -L -o versa https://github.com/your-repo/versaDeploy/releases/download/v0.9.0-beta/versa-darwin-arm64
   ```

2. **Setup**:
   ```bash
   chmod +x versa
   sudo mv versa /usr/local/bin/
   ```

> [!NOTE]
> On macOS, you might need to allow the binary in "System Settings" -> "Privacy & Security" if you see a "developer cannot be verified" warning after the first run.

---

## 🛠️ Global Configuration

Once installed, navigate to your project root and initialize your configuration:

1. **Initialize**:

   ```bash
   versa init
   ```

   (This will create a default `deploy.yml` based on our [Configuration Guide](./DEPLOY.md)).

2. **Check SSH**:
   Ensure you have your SSH keys ready and the remote server has your public key in `~/.ssh/authorized_keys`.

3. **Validate**:
   ```bash
   versa deploy production --check
   ```
