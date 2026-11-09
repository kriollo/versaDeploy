# Comparison

There are great tools to deploy to a VPS. This page explains how versaDeploy is alike and different, so you can pick the one that suits you. If something is out of date, [let us know](https://github.com/kriollo/versaDeploy/issues).

| | **versaDeploy** | Laravel Forge + Envoyer | Deployer (PHP) | Kamal | Coolify / Dokploy |
| --- | --- | --- | --- | --- | --- |
| **Price** | Free, MIT | Monthly subscription | Free, MIT | Free, MIT | Free self-hosted; paid cloud |
| **Runs from** | Your machine or your CI | Web dashboard (SaaS) | Your machine or your CI | Your machine or your CI | A web dashboard installed on your server |
| **What the server needs** | Just SSH | SSH (managed by Forge) | Just SSH | Docker | Docker + the platform |
| **Agent on the server** | No | No | No | No | Yes (the platform) |
| **Where it builds** | On your machine; uploads only what changed | On the server | On the server | Builds a Docker image | On the server |
| **Atomic releases and rollback** | Yes, automatic on failure | Yes (Envoyer) | Yes | Yes (containers) | Yes (containers) |
| **Sets up the server** | Yes (Services view) | Yes (Forge) | Partly | Installs Docker | Yes |
| **Languages** | PHP, Node, Python, Go | Mostly PHP | Mostly PHP | Anything, in containers | Anything, in containers |
| **Interface** | CLI + TUI | Web | CLI | CLI | Web |

## When to pick versaDeploy

- You want to **deploy to your own VPS without Docker and without a dashboard** running on it: the server only needs SSH.
- Your server is **small or old**: building on your machine saves the server's CPU and RAM, and deploys work even on CentOS 5.
- You want something to **set up the server** (nginx, PHP, database, SSL, fail2ban, backups) **without a subscription**, respecting what's already installed.
- You have **apps in several languages** (Laravel, Nuxt, FastAPI, Go) and want the same workflow for all of them.

## When to pick another one

- **Forge + Envoyer**: you prefer a managed web dashboard with support, and the monthly cost isn't an issue.
- **Deployer**: your team is all PHP, you like PHP recipes, and you don't need the tool to set up the server.
- **Kamal**: you already work with Docker and want to deploy images to your servers.
- **Coolify / Dokploy**: you want a "Heroku-like platform" on your server, with a web dashboard, managed databases and git-push deploys, and don't mind running Docker and the platform on it.

!!! note
    This table summarizes each tool broadly; they all move fast. Check their docs for the details.
