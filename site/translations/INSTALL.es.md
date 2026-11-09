# 📦 Instalación { #installation }

versa es un solo binario, sin dependencias. Descárgalo del [último release](https://github.com/kriollo/versaDeploy/releases/latest):

| Sistema | Arquitectura          | Archivo                    |
| :------ | :-------------------- | :------------------------- |
| Windows | x64                   | `versa_windows_amd64.exe`  |
| Windows | ARM64                 | `versa_windows_arm64.exe`  |
| Linux   | x64                   | `versa_linux_amd64`        |
| Linux   | ARM64                 | `versa_linux_arm64`        |
| macOS   | Intel                 | `versa_darwin_amd64`       |
| macOS   | Apple Silicon (M1–M4) | `versa_darwin_arm64`       |

## 🪟 Windows { #windows }

1. Descarga `versa_windows_amd64.exe` y renómbralo a `versa.exe`.
2. Déjalo en una carpeta, por ejemplo `C:\versa`.
3. Agrega esa carpeta al `PATH`: busca **"Editar las variables de entorno del sistema"** → **Variables de entorno** → selecciona `Path` → **Editar** → **Nuevo** → `C:\versa`.
4. Abre una terminal **nueva** y comprueba:

   ```powershell
   versa version
   ```

> [!TIP]
> Si usas hooks `pre_deploy_local`, agrega también el `sh` de Git al `PATH` (`C:\Program Files\Git\bin`).

## 🐧 Linux y 🍎 macOS: una línea { #linux-and-macos-one-line }

```bash
curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh
```

Descarga el último release para tu sistema y procesador, **comprueba su sha256** contra el release y lo instala en `/usr/local/bin` (como root o si se puede escribir ahí) o en `~/.local/bin`. `VERSA_VERSION=v1.15.0` elige una versión y `VERSA_INSTALL_DIR=…` la carpeta. O a mano:

## 🐧 Linux { #linux }

```bash
curl -L -o versa https://github.com/kriollo/versaDeploy/releases/latest/download/versa_linux_amd64
chmod +x versa
sudo mv versa /usr/local/bin/
versa version
```

## 🍎 macOS { #macos }

```bash
# Apple Silicon; en Mac con Intel usa versa_darwin_amd64
curl -L -o versa https://github.com/kriollo/versaDeploy/releases/latest/download/versa_darwin_arm64
chmod +x versa
sudo mv versa /usr/local/bin/
versa version
```

Si macOS dice que no puede verificar al desarrollador, permítelo en **Configuración del Sistema → Privacidad y seguridad**, o ejecuta `xattr -d com.apple.quarantine /usr/local/bin/versa`.

## 🔄 Actualizar { #updating }

```bash
versa self-update
```

Descarga el último release para tu plataforma y reemplaza el binario. Necesita permiso de escritura en la carpeta donde está instalado `versa`.

## 🛠 Compilar desde el código { #building-from-source }

Necesita Go (la versión está en `go.mod`).

```bash
git clone https://github.com/kriollo/versaDeploy.git
cd versaDeploy
go build -o versa ./cmd/versa          # versa.exe en Windows
```

## Siguiente paso { #next-step }

Sigue la [Guía completa](GETTING_STARTED.md) para configurar tu primer deploy.
