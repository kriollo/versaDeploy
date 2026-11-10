# Sourced (hidden) at the start of every episode: a sandboxed HOME (never the real
# one), a short prompt, and the episode's working dir.
# Usage: source setup.sh <workdir> [fresh]   (fresh: no versa installed yet)
V=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# the real HOME stays known: episode 7 uses its gh login and git config (never copied)
export REAL_HOME="${REAL_HOME:-$HOME}"
export HOME="$V/.work/home"
mkdir -p "$HOME/.local/bin" "$V/.work/$1"
case $HOME in "$V"/.work/*) ;; *) echo "setup.sh: HOME is not the sandbox"; return 1 ;; esac
[ "${2:-}" = fresh ] && rm -f "$HOME/.local/bin/versa"
export PATH="$HOME/.local/bin:$PATH"
export PS1='\[\e[38;5;111m\]\W\[\e[0m\] \[\e[38;5;141m\]❯\[\e[0m\] '
cd "$V/.work/$1" || return 1
unset HISTFILE PROMPT_COMMAND
export VERSA_DEPLOYER=dev@laptop
export GIT_PAGER=cat PAGER=cat GIT_AUTHOR_NAME=Dev GIT_AUTHOR_EMAIL=dev@example.com GIT_COMMITTER_NAME=Dev GIT_COMMITTER_EMAIL=dev@example.com
clear
# examples/ come from this checkout, not GitHub: a video can show an example before it is pushed
curl() {
    local url="${!#}" out="" prev="" a
    case $url in
    https://raw.githubusercontent.com/kriollo/versaDeploy/main/examples/*)
        for a; do [ "$prev" = -o ] && out=$a; prev=$a; done
        cp "$V/../examples/${url##*/}" "${out:-/dev/stdout}"
        ;;
    *) command curl "$@" ;;
    esac
}
