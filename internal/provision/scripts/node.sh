# Node.js of this project's major, next to other projects' majors: the official build in
# /opt/versa/node-<major> (the deploy links the project's node/npm/npx into
# <remote_path>/.versa/bin). Never replaces the server's own node; the first one
# installed on a server without node becomes the server's (/usr/local/bin), as before.
major="${NODE_MAJOR:-22}"
dir="/opt/versa/node-${major}"
current="$(node -v 2>/dev/null | sed 's/^v//; s/\..*//' || true)"
if [ "$current" = "$major" ]; then
    ok "Node.js ${major} already installed ($(node -v), the server's): used as is"
    return 0
fi
[ -n "$current" ] && info "Node.js $(node -v) is the server's: Node ${major} goes next to it, in ${dir}"
# server_default: no node on the server → this one is its node, npm, npx (configs
# without node_major, hooks run by hand)
server_default() {
    [ -n "$current" ] && return 0
    for f in "${dir}"/bin/*; do ln -sfn "$f" "/usr/local/bin/$(basename "$f")"; done
    ok "node, npm, npx → ${dir} (the server's Node.js)"
}
if [ -x "${dir}/bin/node" ]; then
    ok "Node.js ${major} already installed ($("${dir}/bin/node" -v), ${dir})"
    server_default
    return 0
fi
case "$(uname -m)" in
    x86_64)  arch=x64 ;;
    aarch64) arch=arm64 ;;
    *) die "Node.js has no official build for $(uname -m)" ;;
esac
# The latest ${major}.x, checked against nodejs.org's SHASUMS256.txt (same HTTPS origin).
# ponytail: the GPG signature of SHASUMS256.txt.asc isn't checked; add it if nodejs.org's TLS isn't enough
base="https://nodejs.org/dist/latest-v${major}.x"
tmp="$(mktemp -d)"
curl -fsSL --proto '=https' -o "$tmp/SHASUMS256.txt" "$base/SHASUMS256.txt" \
    || { rm -rf "$tmp"; die "Node.js ${major} not found on nodejs.org (node_major: a release line like 22)"; }
f="$(awk -v want="-linux-${arch}.tar.gz" 'substr($2, length($2) - length(want) + 1) == want { print $2; exit }' "$tmp/SHASUMS256.txt")"
[ -n "$f" ] || { rm -rf "$tmp"; die "no linux-${arch} build of Node.js ${major}"; }
curl -fsSL --proto '=https' -o "$tmp/$f" "$base/$f" || { rm -rf "$tmp"; die "could not download $f"; }
(cd "$tmp" && grep "  ${f}\$" SHASUMS256.txt | sha256sum -c --quiet >/dev/null 2>&1) || { rm -rf "$tmp"; die "$f: sha256 mismatch — not installed"; }
mkdir -p /opt/versa
tar -xzf "$tmp/$f" -C "$tmp"
chown -R root:root "$tmp/${f%.tar.gz}"
mv "$tmp/${f%.tar.gz}" "$dir"
rm -rf "$tmp"
selinux_label bin_t "${dir}/bin(/.*)?" "${dir}/bin"
# pnpm and yarn for the server_install commands (corepack ships with Node up to 24)
[ -x "${dir}/bin/corepack" ] && "${dir}/bin/corepack" enable >/dev/null 2>&1 || true
ok "Node.js installed: $("${dir}/bin/node" -v) in ${dir}"
server_default
