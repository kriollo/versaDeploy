# Security Policy

versaDeploy runs commands on your servers and handles SSH keys and secrets, so we take security reports seriously.

## Supported versions

Only the **latest release** gets security fixes. Update with `versa self-update` (it installs only a release whose sha256 matches).

## Reporting a vulnerability

**Please don't open a public issue.** Report it privately through GitHub: [Security → Report a vulnerability](https://github.com/kriollo/versaDeploy/security/advisories/new) in this repository.

Include what you can:

- the affected version (`versa version`) and component (deploy, a provisioning step, the TUI, self-update…);
- how to reproduce it, and what an attacker could do with it;
- a suggested fix, if you have one.

We aim to answer within a week. Once a fix is released, the advisory is published with credit to you, unless you prefer otherwise.

## Scope

In scope, for example: command injection through `deploy.yml` values or server output, secrets leaking (to logs, command lines, notifications or other users on the server), privilege escalation through the generated sudoers rules or services, host key verification bypasses, and anything that lets `versa self-update` or `install.sh` run an unverified binary.

Out of scope: problems that need someone who can already edit your `deploy.yml` or log in to your server as the deploy user, since they control what versaDeploy runs anyway.
