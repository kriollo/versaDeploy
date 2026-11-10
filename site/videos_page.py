#!/usr/bin/env python3
"""Writes docs/videos.{es,en}.md from the videos' YouTube descriptions
(../videos/out/*-youtube.txt, made by videos/youtube.py): title, summary and chapters,
which become links that jump the player to that time. Run it after rendering videos."""
import os, re

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(os.path.dirname(HERE), "videos", "out")
VIDEOS = [("00-intro", "intro"), ("02-laravel", "laravel"), ("03-fastapi", "fastapi"), ("04-nuxt", "nuxt"),
          ("05-versawys", "versawys"), ("06-problemas", "problemas")]
EN = {
    "intro": ("What versaDeploy is and does", "What it does, how a deploy works, the services it installs, the TUI and its features for teams."),
    "laravel": ("Laravel: from an empty VPS to the app online", "deploy.yml, setting up the server from the Services view, the first deploy, the production .env and the app in the browser."),
    "fastapi": ("FastAPI + PostgreSQL", "A Python API as a systemd service, configured through the server's .env, and an incremental deploy."),
    "nuxt": ("Nuxt with SSR", "A Node server behind nginx, runtime config from the .env, and an instant rollback."),
    "versawys": ("versaWYS on FrankenPHP", "From composer create-project to the panel online: FrankenPHP instead of PHP-FPM, the versaWYS installer, app:deploy:finish, cron, Redis and maintenance mode."),
    "problemas": ("When a deploy fails", "The most common errors and how to fix them: uncommitted changes, SSH, a stuck lock, a failing hook, a failing health check and a manual rollback."),
}


def parse(name):
    text = open(os.path.join(OUT, name + "-youtube.txt"), encoding="utf-8").read()
    title = text.splitlines()[0].split(" | ")[0]
    summary = text.split("\n\n")[1]
    chapters = re.findall(r"^(\d+):(\d\d) (.+)$", text.split("📌")[1].split("\n\n")[0], re.M)
    return title, summary, [(int(m) * 60 + int(s), f"{m}:{s}", t) for m, s, t in chapters]


def page(lang):
    head = {
        "es": "# Videos\n\nLa serie completa, con voz y subtítulos (botón **CC** del reproductor). Cada capítulo es un enlace que salta a ese momento del video.\n",
        "en": "# Videos\n\nThe whole series, **in Spanish** with Spanish captions (**CC** button). Each chapter link jumps to that point of the video.\n",
    }[lang]
    out = [head]
    for name, vid in VIDEOS:
        title, summary, chapters = parse(name)
        if lang == "en":
            title, summary = EN[vid]
        links = " · ".join(f'<a href="#" data-video="v-{vid}" data-t="{t}">{ts} {c}</a>' for t, ts, c in chapters)
        out.append(f"""## {title} {{ #{vid} }}

{summary}

<video id="v-{vid}" controls preload="metadata" src="/media/{name}.mp4">
  <track kind="captions" src="/media/{name}.vtt" srclang="es" label="Español" default>
</video>

{links}
""")
    open(os.path.join(HERE, "docs", f"videos.{lang}.md"), "w", encoding="utf-8").write("\n".join(out))


if __name__ == "__main__":
    page("es")
    page("en")
