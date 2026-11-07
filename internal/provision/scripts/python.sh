if [ "$PKG_MGR" != apt ]; then
    sys_ver="$(python3 -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null || true)"
    ver="${PYTHON_VERSION:-$sys_ver}"
    if [ -z "$ver" ] || [ "$ver" = "$sys_ver" ]; then
        pkg_install python3 python3-pip python3-devel
        ok "Python $(python3 -V 2>&1 | awk '{print $2}') (system) ready"
        return 0
    fi
    # requested version next to system
    py="python${ver}"
    if command -v "$py" >/dev/null 2>&1 && "$py" -c 'import ensurepip' >/dev/null 2>&1; then
        ok "Python ${ver} already installed"
        return 0
    fi
    if pkg_available "$py"; then
        pkg_install "$py" "${py}-devel"
        ok "Python ${ver} installed next to the system Python ${sys_ver}"
        return 0
    else
        die "Python ${ver} is not packaged for ${OS_ID}: the system has ${sys_ver:-none} — set python_version: \"${sys_ver}\""
    fi
fi
sys_ver="$(python3 -c 'import sys; print("%d.%d" % sys.version_info[:2])' 2>/dev/null || true)"
ver="${PYTHON_VERSION:-$sys_ver}"
if [ -z "$ver" ] || [ "$ver" = "$sys_ver" ]; then
    apt_install python3 python3-pip python3-venv python3-dev
    ok "Python $(python3 -V 2>&1 | awk '{print $2}') (system) ready"
    return 0
fi
if command -v "python${ver}" >/dev/null && "python${ver}" -c 'import ensurepip' >/dev/null 2>&1; then
    ok "Python ${ver} already installed"
    return 0
fi
# Other versions install next to the system one (never replacing it): deadsnakes, Ubuntu only
[ "$OS_ID" = ubuntu ] || die "Python ${ver} is not packaged for ${OS_ID}: the system has ${sys_ver:-none} — set python_version: \"${sys_ver}\""
if ! grep -rqs deadsnakes /etc/apt/sources.list.d/; then
    apt_install software-properties-common
    info "Adding PPA deadsnakes/ppa..."
    add-apt-repository -y ppa:deadsnakes/ppa >/dev/null
    apt-get update -qq
fi
apt-cache show "python${ver}" >/dev/null 2>&1 || die "Python ${ver} is not available in deadsnakes for ${OS_CODENAME}"
apt_install "python${ver}" "python${ver}-venv" "python${ver}-dev"
ok "Python ${ver} installed next to the system Python ${sys_ver} — use python_command: python${ver}"
