#!/usr/bin/env python3
"""Builds versadeploy.websystem.cl (MkDocs Material, Spanish at / and English at /en/).

  build.py           assemble docs/ and run `mkdocs build` (→ build/)
  build.py serve     the same, then `mkdocs serve` for a live preview

- The reference pages come from ../doc (English, the single source) into
  docs/reference/X.en.md; their Spanish version is translations/X.es.md, or the
  English page with a note while it isn't translated.
- The videos are assets of the "media" pre-release (they don't belong in git): the
  Videos page plays them from there; only their captions (.vtt) go into the site.
"""
import os, re, shutil, subprocess, sys, urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
REPO = "https://github.com/kriollo/versaDeploy"
REF = ["INSTALL", "QUICKSTART", "GETTING_STARTED", "DEPLOY", "SERVICES", "CLI_REFERENCE", "TROUBLESHOOTING"]
VIDEOS = ["00-intro", "02-laravel", "03-fastapi", "04-nuxt", "05-versawys", "06-problemas"]
PENDING = """!!! note "Página en inglés"
    Esta página de referencia todavía no está traducida al español. Las guías (Primeros pasos, Flujos, Videos, Ejemplos) sí lo están.

"""


def admonitions(md):
    """GitHub alerts (> [!TIP]) → Material admonitions (!!! tip)."""
    def conv(m):
        body = "\n".join("    " + l[1:].lstrip() if l.startswith(">") else l for l in m[2].rstrip("\n").split("\n"))
        return f"!!! {m[1].lower()}\n{body}\n"
    return re.sub(r"^> \[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\n((?:>.*\n?)*)", conv, md, flags=re.M)


def fix_links(md):
    # files outside doc/ (deploy.example.yml, examples/, CHANGELOG.md…) → GitHub
    md = re.sub(r"\]\(\.\./([^)#]+)(#[^)]*)?\)", lambda m: f"]({REPO}/blob/main/{m[1]}{m[2] or ''})", md)
    # the old index page is the site's navigation now
    return admonitions(re.sub(r"\]\(INDEX\.md\)", "](../index.md)", md))


def reference():
    out = os.path.join(HERE, "docs", "reference")
    shutil.rmtree(out, ignore_errors=True)
    os.makedirs(out)
    for name in REF:
        md = fix_links(open(os.path.join(ROOT, "doc", name + ".md"), encoding="utf-8").read())
        open(os.path.join(out, name + ".en.md"), "w", encoding="utf-8").write(md)
        es = os.path.join(HERE, "translations", name + ".es.md")
        es_md = fix_links(open(es, encoding="utf-8").read()) if os.path.exists(es) else PENDING + md
        open(os.path.join(out, name + ".es.md"), "w", encoding="utf-8").write(es_md)


def media():
    out = os.path.join(HERE, "docs", "media")
    os.makedirs(out, exist_ok=True)
    local = os.path.join(ROOT, "videos", "out")
    # only the captions: the page plays each .mp4 straight from the release (with the
    # videos inside, the site was too big for Pages to publish), but a <track> must be
    # on the page's own origin
    for v in VIDEOS:
        srt = v + (".srt" if v == "00-intro" else "-full.srt")
        dst_vtt = os.path.join(out, v + ".vtt")
        if os.path.exists(os.path.join(local, srt)):
            srt_text = open(os.path.join(local, srt), encoding="utf-8").read()
            open(dst_vtt, "w", encoding="utf-8").write("WEBVTT\n\n" + re.sub(r"(\d\d:\d\d:\d\d),(\d\d\d)", r"\1.\2", srt_text))
            continue
        for f in (v + ".vtt",):
            dst = os.path.join(out, f)
            if not os.path.exists(dst):
                try:
                    urllib.request.urlretrieve(f"{REPO}/releases/download/media/{f}", dst)
                except Exception as e:
                    print(f"warning: {f} not available ({e})", file=sys.stderr)


if __name__ == "__main__":
    reference()
    media()
    mkdocs = os.path.join(HERE, ".venv", "bin", "mkdocs")
    mkdocs = mkdocs if os.path.exists(mkdocs) else "mkdocs"
    cmd = "serve" if sys.argv[1:] == ["serve"] else "build"
    subprocess.run([mkdocs, cmd] + (["-d", "build"] if cmd == "build" else []), cwd=HERE, check=True)
