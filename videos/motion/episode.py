#!/usr/bin/env python3
"""Timeline of an episode in Remotion: the VHS recording cut by chapter, with an
animated card before each chapter, an opening, the app in a browser and an outro.

  episode.py NN   → public/ep/NN.json (+ the recording, voice lines and browser shot)
                    and ../out/NN-name-full.srt

The episode's narration (cards) is in ../episodes/NN-name.json; the recording's
captions come from its .srt and its chapters from the tape's "# ─── N. Title ───".
"""
import glob, json, os, re, shutil, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
V = os.path.dirname(HERE)
sys.path.insert(0, V)
import cc, voice  # noqa: E402

FPS = 30
LEAD, GAP, TAIL = 0.5, 0.45, 0.8


def srt(path):
    out = []
    for m in re.finditer(r"(\d+):(\d+):(\d+),(\d+) --> .*\n(.+)", open(path, encoding="utf-8").read()):
        out.append((int(m[1]) * 3600 + int(m[2]) * 60 + int(m[3]) + int(m[4]) / 1000, m[5]))
    return out


def ts(s):
    ms = round(s * 1000)
    return f"{ms // 3600000:02}:{ms // 60000 % 60:02}:{ms // 1000 % 60:02},{ms % 1000:03}"


def main(nn):
    tape = glob.glob(os.path.join(V, "episodes", nn + "-*.tape"))[0]
    name = os.path.splitext(os.path.basename(tape))[0]
    script = json.load(open(os.path.join(V, "episodes", name + ".json"), encoding="utf-8"))
    out = os.path.join(V, "out", name)
    pub = os.path.join(HERE, "public", "ep")
    os.makedirs(os.path.join(pub, nn), exist_ok=True)

    # chapters and web shots, at the final recording's times (render.sh's scale and fast parts)
    sections, webs = [], []
    cc.captions(open(tape, encoding="utf-8").readlines(), sections, webs, log=cc.load_log(tape))
    to_video = cc.timing(tape)
    caps = srt(out + ".srt")
    start = next(t for t, c in caps if c.startswith(script["start_caption"]))
    end = next(t for t, c in caps if c.startswith(script["end_caption"]))
    web_lines = [l for w in script.get("webs", {}).values() for l in w["lines"]]
    texts = [c for _, c in caps] + script["opening"]["lines"] + script["browser"]["lines"] + script["outro"]["lines"] + web_lines
    voice.prefetch(texts + [l for ch in script["chapters"].values() for l in ch["lines"]])

    n = 0

    def line(text, at):
        nonlocal n
        path, dur = voice.synth(text)
        f = f"ep/{nn}/l{n:03}.wav"
        n += 1
        shutil.copy(path, os.path.join(HERE, "public", f))
        return {"text": text, "start": round(at, 3), "dur": round(dur, 3), "file": f}

    def card(kind, lines, **extra):
        t, ls = LEAD, []
        for text in lines:
            ls.append(line(text, t))
            t += ls[-1]["dur"] + GAP
        return {"type": kind, "dur": round(t - GAP + TAIL, 3), "lines": ls, **extra}

    def clip(a, b):
        ls = [line(c, t - a) for t, c in caps if a <= t < b]
        return {"type": "video", "from": round(a, 3), "dur": round(b - a, 3), "lines": ls}

    segs = [card("opening", script["opening"]["lines"], title=script["title"], subtitle=script["subtitle"],
                 steps=[title for _, _, title in sections])]
    # the recording, cut where a chapter card or a web shot goes in
    events = sorted([(to_video(t), 0, ("chapter", num, title)) for t, num, title in sections] +
                    [(to_video(t), 1, ("web", n, "")) for t, n in webs])
    pos = start
    for at, _, (kind, num, title) in events:
        if at - pos > 0.05:
            segs.append(clip(pos, at))
            pos = at
        if kind == "chapter":
            ch = script["chapters"][str(num)]
            segs.append(card("chapter", ch["lines"], num=num, title=title, bullets=ch.get("bullets", []), visual=ch.get("visual", "")))
        else:
            shot = f"ep/{nn}-web{num}.png"
            shutil.copy(f"{out}-web{num}.png", os.path.join(HERE, "public", shot))
            url = open(f"{out}-web{num}.url", encoding="utf-8").read().strip()
            segs.append(card("browser", script["webs"][str(num)]["lines"], url=url, shot=shot, web=True))
    segs.append(clip(pos, end))
    shot = f"ep/{nn}-browser.png"
    shutil.copy(out + "-browser.png", os.path.join(HERE, "public", shot))
    segs.append(card("browser", script["browser"]["lines"], url=script["browser"]["url"], shot=shot))
    segs.append(card("outro", script["outro"]["lines"], next=script["outro"]["next"], title=script["title"]))

    t = 0.0
    subs = []
    for s in segs:
        s["start"] = round(t, 3)
        for l in s["lines"]:
            subs.append((t + l["start"], t + l["start"] + l["dur"], l["text"]))
        t += s["dur"]
    # a keyframe every second: Remotion seeks into the recording at every cut, and the
    # recording's sparse keyframes made each seek decode for a long time (render timeouts)
    h264 = "libx264" if b" libx264 " in subprocess.check_output(["ffmpeg", "-hide_banner", "-encoders"]) else "libopenh264"
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", out + ".mp4", "-c:v", h264, "-b:v", "6M", "-g", str(FPS),
                    "-pix_fmt", "yuv420p", "-an", os.path.join(pub, nn + ".mp4")], check=True)
    json.dump({"fps": FPS, "total": round(t, 3), "video": f"ep/{nn}.mp4", "segments": segs},
              open(os.path.join(pub, nn + ".json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    with open(out + "-full.srt", "w", encoding="utf-8") as f:
        for i, (a, b, text) in enumerate(subs, 1):
            f.write(f"{i}\n{ts(a)} --> {ts(b)}\n{text}\n\n")
    print(f"{name}: {len(segs)} segments, {len(subs)} lines, {t:.1f}s")


if __name__ == "__main__":
    main(sys.argv[1])
