# 🚀 versaDeploy

Atomic, incremental deployments from your machine (Windows, Linux, macOS) or CI to Linux servers over SSH.

[![Latest Release](https://img.shields.io/github/v/release/kriollo/versaDeploy?include_prereleases&style=flat-square)](https://github.com/kriollo/versaDeploy/releases)
[![Tests Status](https://img.shields.io/github/actions/workflow/status/kriollo/versaDeploy/test.yml?branch=main&label=tests&style=flat-square)](https://github.com/kriollo/versaDeploy/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/kriollo/versaDeploy)

versaDeploy builds your project locally (PHP/Composer, Node frontends, Go, Python), uploads only what's needed, and switches the server to the new release in one atomic step. If something fails after the switch, it rolls back on its own.

```bash
versa deploy production
```

## ✨ Features

- **Only deploys what changed**: SHA256 change detection against the last deploy; builders run only for the languages that changed.
- **Atomic releases and instant rollback**: each deploy is a new release directory; `current` is switched with a single symlink swap. `versa rollback` goes back in a second.
- **Fast uploads**: parallel, pipelined SFTP uploads in chunks; optional incremental upload of changed files only; `vendor`/`node_modules`/`.venv` reused via hardlinks.
- **Safe by default**: deploys only committed code, one deploy at a time (remote lock), automatic rollback when a `post_deploy` hook or the health check fails, host key verification.
- **Go and Python services**: runs your binary or Python server through systemd (or OpenRC / SysV init on old servers), starts it at boot, restarts it on every deploy and rollback, and rolls back if it doesn't stay up. Python virtualenvs are built on the server.
- **Hooks** locally and on the server, service reloads (PHP-FPM, Nginx…), HTTP/command health checks, Slack/Teams/Discord webhooks.
- **Works with old servers**: POSIX-only remote commands, legacy SSH algorithms for OpenSSH < 7 (CentOS 5/6).
- **Local mode** for hosting without SSH: builds a complete release folder to upload by FTP.
- **Interactive TUI**: run `versa` to browse releases and server files, deploy, roll back and open a remote terminal.

## 🛠 Install

Download the binary for your OS from [Releases](https://github.com/kriollo/versaDeploy/releases/latest), put it in your `PATH`, and check:

```bash
versa version
```

Details for each OS: [INSTALL.md](doc/INSTALL.md). Update later with `versa self-update`.

## ⚡ Quick start

```bash
cd my-project
versa init                                  # creates deploy.yml
versa config validate production            # check the file
versa info production                       # check SSH and the server
versa deploy production --initial-deploy    # first deploy
versa deploy production                     # every deploy after that
```

## 📖 Documentation

- [**Getting Started**](doc/GETTING_STARTED.md): server preparation and first deploy, step by step.
- [**Quick Start**](doc/QUICKSTART.md): the five-minute version and a command cheat sheet.
- [**CLI Reference**](doc/CLI_REFERENCE.md): every command, flag and TUI shortcut.
- [**Configuration Reference**](doc/DEPLOY.md): every `deploy.yml` option.
- [**Troubleshooting**](doc/TROUBLESHOOTING.md): common errors and fixes.
- [**Changelog**](CHANGELOG.md)

## 📄 License

[MIT](LICENSE)
