#!/usr/bin/env python3
"""YouTube description of each video, with chapters taken from its final timeline.

  youtube.py            → out/<video>-youtube.txt for the intro and every episode
                          whose final timeline exists (motion/public/ep/NN.json)

The text comes from episodes/NN-name.json (title, subtitle, opening lines) and
motion/script.json (the intro); chapters are where each scene / chapter card starts,
so they match the video exactly.
"""
import glob, json, os

V = os.path.dirname(os.path.abspath(__file__))
REPO = "https://github.com/kriollo/versaDeploy"
FOOTER = f"""🔗 versaDeploy (gratis y open source, MIT): {REPO}
📖 Documentación: {REPO}/tree/main/doc
🧩 Ejemplos de deploy.yml (Laravel, versaWYS, Next, Nuxt, FastAPI, Flask, Gin): {REPO}/tree/main/examples

⚡ Instalación (Linux y macOS):
curl -fsSL https://raw.githubusercontent.com/kriollo/versaDeploy/main/install.sh | sh
(Windows: el binario está en {REPO}/releases)

Todo lo que ves es versaDeploy funcionando de verdad contra un VPS Ubuntu 24.04 (en Docker): nada es simulado, solo se recortan las esperas largas.
¿Dudas o ideas? Déjalas en los comentarios o abre un issue en GitHub.

#versaDeploy #deploy #devops #vps #linux #ssh"""


def lower_first(s):
    # "Los errores…" → "los errores…", but names (GitHub, VPS) keep their case
    w = s.split(' ', 1)[0]
    return s if any(c.isupper() for c in w[1:]) else s[:1].lower() + s[1:]


def ts(sec):
    sec = int(sec)
    return f"{sec // 60}:{sec % 60:02}" if sec < 3600 else f"{sec // 3600}:{sec // 60 % 60:02}:{sec % 60:02}"


def chapters(points):
    # YouTube: the first at 0:00, at least 3, each at least 10 s long
    out = []
    for t, title in points:
        if out and t - out[-1][0] < 10:
            continue
        out.append((t, title))
    out[0] = (0, out[0][1])
    return "\n".join(f"{ts(t)} {title}" for t, title in out)


def write(name, text):
    path = os.path.join(V, "out", name + "-youtube.txt")
    with open(path, "w", encoding="utf-8") as f:
        f.write(text.strip() + "\n")
    print(path)


def intro():
    script = json.load(open(os.path.join(V, "motion", "script.json"), encoding="utf-8"))
    tl = json.load(open(os.path.join(V, "motion", "public", "timeline.json"), encoding="utf-8"))
    names = {"title": "Qué es versaDeploy", "outro": "Próximos videos"}
    points = [(s["start"], names.get(s["id"], s.get("title", s["id"]))) for s in tl["scenes"]]
    lines = [l for s in script["scenes"] for l in s["lines"]]
    write("00-intro", f"""versaDeploy: deploys a tu VPS Linux por SSH, sin agentes | Qué es y qué hace

{lines[0]} {lines[1]} En este video: cómo funciona un deploy, el rollback automático, los frameworks que soporta, cómo prepara el servidor (nginx, PHP o FrankenPHP, PostgreSQL, Redis, Node, Python, SSL, fail2ban, backups…), su interfaz en la terminal y sus funciones para equipos.

📌 Capítulos
{chapters(points)}

{FOOTER} #laravel #nuxt #fastapi""")


def episode(path):
    nn = os.path.basename(path)[:2]
    name = os.path.splitext(os.path.basename(path))[0]
    tl_path = os.path.join(V, "motion", "public", "ep", nn + ".json")
    if not os.path.exists(tl_path):
        return
    ep = json.load(open(path, encoding="utf-8"))
    tl = json.load(open(tl_path, encoding="utf-8"))
    yt = ep.get("youtube", {})  # what is this episode's own: summary, example, tags, browser chapter
    points = []
    for s in tl["segments"]:
        if s["type"] == "opening":
            points.append((s["start"], "Introducción"))
        elif s["type"] == "chapter":
            points.append((s["start"], f"{s['num']}. {s['title']}"))
        elif s["type"] == "browser" and not s.get("web"):  # a web shot inside a chapter isn't one
            points.append((s["start"], yt.get("browser", "La app funcionando en el navegador")))
        elif s["type"] == "outro":
            points.append((s["start"], "Cierre"))
    example = yt.get("example", {"02": "deploy.laravel.yml", "03": "deploy.fastapi.yml", "04": "deploy.nuxt.yml"}.get(nn, ""))
    tag = yt.get("tags", {"02": "#laravel #php", "03": "#fastapi #python #postgresql", "04": "#nuxt #vue #nodejs"}.get(nn, ""))
    summary = yt.get("summary", "Paso a paso: el deploy.yml, preparar el servidor desde la vista Services, el primer deploy, el .env de producción y la app funcionando.")
    if example:
        summary += f" El deploy.yml de partida de este video: {REPO}/blob/main/examples/{example}"
    write(name, f"""{ep['title']}: {lower_first(ep['subtitle'])} | versaDeploy

{' '.join(ep['opening']['lines'])}

{summary}

📌 Capítulos
{chapters(points)}

{FOOTER} {tag}""")


if __name__ == "__main__":
    intro()
    for p in sorted(glob.glob(os.path.join(V, "episodes", "0*.json"))):
        episode(p)
