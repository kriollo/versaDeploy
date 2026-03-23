# 📚 Documentation Index - versaDeploy

## 🚀 Getting Started

1. 📖 **[QUICKSTART.md](QUICKSTART.md)** - Get up and running in 5 minutes.
   - Installation (Windows/Linux)
   - First deployment flow
   - Common patterns

2. 📋 **[README.md](README.md)** - High-level overview.
   - Key features
   - Architecture overview
   - Platform support

## ⚙️ Configuration & Usage

3. ⚙️ **[DEPLOY.md](DEPLOY.md)** - **The Configuration Bible**.
   - Detailed field reference for `deploy.yml`.
   - Build engine settings (PHP, Go, Frontend).
   - Hook configuration.

4. 🔧 **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Solutions for common errors.
   - SSH permission issues.
   - Windows shell execution tips.
   - Build failure patterns.

5. ⚙️ **[deploy.example.yml](deploy.example.yml)** - A complete, commented example configuration.

## 📁 Project Structure

```
versaDeploy/
├── 📚 DOCUMENTATION
│   ├── README.md          # High-level entry
│   ├── QUICKSTART.md      # Installation & First deploy
│   ├── DEPLOY.md          # Full config reference
│   ├── TROUBLESHOOTING.md # Fixes for common issues
│   └── INDEX.md           # This file
│
├── ⚙️ CONFIGURATION
│   ├── deploy.example.yml # Template for deploy.yml
│   └── .gitignore         # exclusions
│
├── 🔧 SOURCE CODE
│   ├── cmd/versa/         # CLI Entry point
│   └── internal/          # Core logic (Config, Builder, SSH, etc.)
│
└── 🧪 TESTING
    └── ...                # Unit and integration tests
```

---

**Built with ❤️ for deterministic deployments**
