#!/usr/bin/env python3
"""Timeline of the intro: each narration line is synthesized (the episodes' voice),
measured, and placed; the scenes last exactly as long as their lines.

Writes public/timeline.json (read by the Remotion composition), public/voice/*.wav,
public/music.m4a, public/shots/*.png (frames of the Laravel episode) and
../out/00-intro.srt.
"""
import json, os, re, shutil, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
import voice  # noqa: E402

FPS = 30
LEAD, GAP, TAIL = 0.6, 0.45, 0.9  # seconds: before a scene's first line, between lines, after the last
OUT = os.path.join(os.path.dirname(HERE), "out")
PUB = os.path.join(HERE, "public")


def srt_ts(s):
    ms = round(s * 1000)
    return f"{ms // 3600000:02}:{ms // 60000 % 60:02}:{ms // 1000 % 60:02},{ms % 1000:03}"


def shots():
    """Frames of the Laravel episode (terminal area only), found by their captions."""
    srt = open(os.path.join(OUT, "02-laravel.srt"), encoding="utf-8").read()
    want = {"services": "versa detectó Ubuntu", "env": "La base de datos: PostgreSQL", "history": "Y el historial"}
    os.makedirs(os.path.join(PUB, "shots"), exist_ok=True)
    for name, text in want.items():
        m = re.search(r"(\d+):(\d+):(\d+),(\d+) --> (\d+):(\d+):(\d+),(\d+)\n" + re.escape(text), srt)
        a = int(m[1]) * 3600 + int(m[2]) * 60 + int(m[3]) + int(m[4]) / 1000
        b = int(m[5]) * 3600 + int(m[6]) * 60 + int(m[7]) + int(m[8]) / 1000
        # the history fills only the top of the terminal: keep that part
        crop = "crop=1050:300:0:0" if name == "history" else "crop=1920:940:0:0"
        subprocess.run(["ffmpeg", "-y", "-v", "error", "-ss", str(a + (b - a) * 0.85), "-i", os.path.join(OUT, "02-laravel.mp4"),
                        "-frames:v", "1", "-vf", crop, os.path.join(PUB, "shots", name + ".png")], check=True)


def main():
    script = json.load(open(os.path.join(HERE, "script.json"), encoding="utf-8"))
    os.makedirs(os.path.join(PUB, "voice"), exist_ok=True)
    t, scenes, srt = 0.0, [], []
    for si, scene in enumerate(script["scenes"]):
        start = t
        t += LEAD
        lines = []
        for li, text in enumerate(scene["lines"]):
            path, dur = voice.synth(text)
            name = f"s{si:02}l{li}.wav"
            shutil.copy(path, os.path.join(PUB, "voice", name))
            lines.append({"text": text, "start": round(t - start, 3), "dur": round(dur, 3), "file": "voice/" + name})
            srt.append((t, t + dur, text))
            t += dur + GAP
        t += TAIL - GAP
        scenes.append({**{k: v for k, v in scene.items() if k != "lines"}, "start": round(start, 3),
                       "dur": round(t - start, 3), "lines": lines})
    timeline = {"fps": FPS, "total": round(t, 3), "scenes": scenes}
    json.dump(timeline, open(os.path.join(PUB, "timeline.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    with open(os.path.join(OUT, "00-intro.srt"), "w", encoding="utf-8") as f:
        for i, (a, b, text) in enumerate(srt, 1):
            f.write(f"{i}\n{srt_ts(a)} --> {srt_ts(b)}\n{text}\n\n")
    subprocess.run(["ffmpeg", "-y", "-v", "error", "-ss", str(voice.MUSIC_START), "-i", voice.MUSIC, "-vn",
                    "-c:a", "aac", "-b:a", "192k", os.path.join(PUB, "music.m4a")], check=True)
    shots()
    print(f"{len(scenes)} scenes, {len(srt)} lines, {t:.1f}s")


if __name__ == "__main__":
    main()
