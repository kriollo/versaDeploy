# 🚀 versaDeploy

A high-performance deployment tool for modern web applications. Fast, secure, and designed for simplicity.

[![Latest Release](https://img.shields.io/github/v/release/kriollo/versaDeploy?include_prereleases&style=flat-square)](https://github.com/kriollo/versaDeploy/releases)
[![Tests Status](https://img.shields.io/github/actions/workflow/status/kriollo/versaDeploy/test.yml?branch=main&label=tests&style=flat-square)](https://github.com/kriollo/versaDeploy/actions/workflows/test.yml)
[![License](https://img.shields.io/github/license/kriollo/versaDeploy?style=flat-square)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/kriollo/versaDeploy)

versaDeploy is designed for developers who want **deterministic, atomic deployments** from their local machines (Windows/Linux) or CI/CD environments to Linux servers.

## 📖 Documentation Hub

We have structured our documentation to be clear and accessible:

- [🚀 **Getting Started**](doc/GETTING_STARTED.md): **Start here!** A step-by-step guide for beginners.
- [⚙️ **Configuration Guide**](doc/DEPLOY.md): Full reference for all `deploy.yml` parameters.
- [📖 **CLI Reference**](doc/CLI_REFERENCE.md): Detailed documentation of commands and flags.
- [📝 **Changelog**](doc/CHANGELOG.md): History of all versions and improvements.
- [⚠️ **Troubleshooting**](doc/TROUBLESHOOTING.md): Solutions to common server and connection issues.

## ✨ Why versaDeploy?

- **Blazing Fast**: Concurrent builds and parallel chunked uploads reduce deployment time by up to 70%.
- **Smart Dependencies**: Intelligent reuse of `vendor` and `node_modules` via Linux hardlinks avoid redundant downloads.
- **Atomic & Secure**: Uses distributed locking and absolute symlinks to ensure zero-downtime and safe execution.
- **Cross-Platform**: Run deployments from Windows or Linux with equal ease.

## 🛠 Quick Installation

1. Download the latest binary for your OS from the [Releases](https://github.com/kriollo/versaDeploy/releases) page.
2. Move the binary to a folder in your PATH.
3. Verify by running:
   ```bash
   versa version
   ```

Check our [Getting Started](doc/GETTING_STARTED.md) guide to launch your first deploy in minutes!
