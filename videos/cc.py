#!/usr/bin/env python3
"""Closed captions for a VHS tape: `# CC: text` comments become an .srt.

Everything visible in the video is timed by the tape (Type, keys, Sleep), so each
caption's time is known exactly. A Wait between Hide and Show is cut from the video;
a visible Wait (a deploy running) is timed from the recording's command log
(setup.sh writes it: when each command started and ended), and `# FAST x4` before
it plays it 4 times faster (render.sh). `# WEB: N` marks where episode.py puts the
browser shot that `web N URL` took.
A caption lasts until the next one (at most MAX_S seconds); `# CC:` with no text
ends the current one.

  cc.py episode.tape          the recording's computed length (stderr)
  cc.py episode.tape SCALE    the .srt in the final video's time (+ out/NAME.timing.json)
  cc.py episode.tape SCALE --filter   ffmpeg's filter for the fast parts ("" if none)
"""
import json
import os
import re
import subprocess
import sys

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "out")

MAX_S = 8.0
KEY_COST = 0.0048  # what VHS adds to every keystroke (measured: 4.7-4.8 ms on long tapes)
KEYS = {"Enter", "Backspace", "Delete", "Insert", "Down", "Up", "Left", "Right", "Tab",
        "Escape", "Space", "PageUp", "PageDown", "ScrollUp", "ScrollDown", "Home", "End"}


def seconds(v):
    m = re.fullmatch(r"([\d.]+)(ms|s|m)?", v)
    if not m:
        raise ValueError(f"bad time {v!r}")
    n = float(m[1])
    return {"ms": n / 1000, "m": n * 60}.get(m[2], n)


def load_log(tape):
    """(start, end, command) of every command the recording ran, or None."""
    path = os.path.join(OUT, os.path.splitext(os.path.basename(tape))[0] + ".cmdlog")
    if not os.path.exists(path):
        return None
    rows = [l.rstrip("\n").split("\t", 2) for l in open(path, encoding="utf-8")]
    num = lambda v: float(v.replace(",", "."))  # $EPOCHREALTIME follows the locale
    return [(num(a), num(b), c.strip()) for a, b, c in rows]


def captions(lines, sections=None, webs=None, fast=None, log=None, strict=True):
    """Caption (time, text) pairs and the recording's length. Chapter markers
    ("# ─── 1. Title ───") go into sections as (time, number, title), "# WEB: N" into
    webs as (time, N), and visible Waits into fast as (start, length, factor).
    A visible Wait needs the command log; strict=False counts it as 0 s (voice.py
    fits the voice before there is a recording)."""
    speed, t, hidden, out = 0.05, 0.0, False, []
    typed, cmd, factor, li = "", None, 1, 0
    for no, raw in enumerate(lines, 1):
        line = raw.strip()
        m = re.match(r"# ─+ (\d+)\. (.+?) ─", line)
        if m and sections is not None:
            sections.append((t, int(m[1]), m[2]))
            continue
        if line.startswith("# CC:"):
            out.append((t, line[5:].strip()))
            continue
        m = re.match(r"# FAST x(\d+)", line)
        if m:
            factor = int(m[1])
            continue
        m = re.match(r"# WEB: (\d+)", line)
        if m:
            if webs is not None:
                webs.append((t, int(m[1])))
            continue
        if not line or line.startswith("#"):
            continue
        cmd_, _, rest = line.partition(" ")
        base, _, at = cmd_.partition("@")
        if base == "Set" and rest.startswith("TypingSpeed"):
            speed = seconds(rest.split()[1])
        elif base == "Hide":
            hidden = True
        elif base == "Show":
            hidden = False
        elif hidden:
            continue
        elif base == "Sleep":
            t += seconds(rest)
        elif base == "Type":
            text = re.fullmatch(r'\s*(["`\'])(.*)\1\s*', rest, re.S)
            typed += text[2] if text else rest
            t += len(text[2] if text else rest) * ((seconds(at) if at else speed) + KEY_COST)
        elif base in KEYS or re.fullmatch(r"(Ctrl|Alt|Shift)\+.+", base):
            n = int(rest) if rest.strip().isdigit() else 1
            t += n * ((seconds(at) if at else speed) + KEY_COST)
            if base == "Enter" and typed:
                cmd, typed = (typed.strip(), t), ""
        elif base.startswith("Wait"):
            if log is None or cmd is None:
                if strict:
                    sys.exit(f"line {no}: a visible Wait needs the recording's command log (render.sh)")
                continue
            while li < len(log) and log[li][2] != cmd[0]:
                li += 1
            if li == len(log):
                sys.exit(f"line {no}: {cmd[0]!r} is not in the recording's command log")
            # the command ran end - start; the tape already counted what it did since Enter
            d = max(0.0, log[li][1] - log[li][0] - (t - cmd[1]))
            li += 1
            if fast is not None:
                fast.append((t, d, factor))
            t += d
            factor = 1
    return out, t


def squeeze(t, fast):
    """A time in the recording → the same moment once the fast parts are sped up."""
    out = t
    for a, d, k in fast:
        if t > a:
            out -= (min(t, a + d) - a) * (1 - 1 / k)
    return out


def timing(tape):
    """The recording's time → the final video's: render.sh's scale and fast parts."""
    name = os.path.splitext(os.path.basename(tape))[0]
    path = os.path.join(OUT, name + ".timing.json")
    if os.path.exists(path):
        tm = json.load(open(path, encoding="utf-8"))
    else:  # recorded before render.sh wrote it: no fast parts, the scale from the lengths
        _, total = captions(open(tape, encoding="utf-8").readlines())
        real = float(subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                                              "-of", "csv=p=0", os.path.join(OUT, name + ".mp4")]))
        tm = {"scale": real / total, "fast": []}
    return lambda t: squeeze(t * tm["scale"], tm["fast"])


def ffmpeg_filter(fast):
    """[0:v] → [v] with every fast part sped up and a "▶▶ x4" badge on it."""
    if not any(k > 1 for _, _, k in fast):
        return ""
    font = subprocess.check_output(["fc-match", "JetBrainsMono Nerd Font:bold", "-f", "%{file}"], text=True)
    parts, prev = [], 0.0
    for a, d, k in fast:
        if k > 1:
            parts += [(prev, a, 1), (a, a + d, k)]
            prev = a + d
    parts.append((prev, None, 1))
    chains = [f"[0:v]split={len(parts)}" + "".join(f"[s{i}]" for i in range(len(parts)))]
    for i, (a, b, k) in enumerate(parts):
        c = f"[s{i}]trim=start={a:.3f}" + (f":end={b:.3f}" if b is not None else "") + f",setpts=(PTS-STARTPTS)/{k}"
        if k > 1:
            c += (f",drawtext=fontfile={font}:text='▶▶ x{k}':x=w-tw-70:y=70:fontsize=40:fontcolor=0x7aa2f7"
                  ":box=1:boxcolor=0x1a1b26@0.9:boxborderw=16")
        chains.append(c + f"[p{i}]")
    chains.append("".join(f"[p{i}]" for i in range(len(parts))) + f"concat=n={len(parts)}:v=1:a=0,fps=24[v]")
    return ";".join(chains)


def srt(caps, total):
    def ts(s):
        ms = round(s * 1000)
        return f"{ms // 3600000:02}:{ms // 60000 % 60:02}:{ms // 1000 % 60:02},{ms % 1000:03}"
    blocks = []
    for i, (start, text) in enumerate(caps):
        end = min(caps[i + 1][0] if i + 1 < len(caps) else total, start + MAX_S)
        if text and end > start:
            blocks.append(f"{len(blocks) + 1}\n{ts(start)} --> {ts(end)}\n{text}\n")
    return "\n".join(blocks)


if __name__ == "__main__":
    tape = sys.argv[1]
    with open(tape, encoding="utf-8") as f:
        fast = []
        caps, total = captions(f.readlines(), fast=fast, log=load_log(tape))
    if len(sys.argv) > 2:  # real/computed duration
        scale = float(sys.argv[2])
        fast = [(a * scale, d * scale, k) for a, d, k in fast]
        if sys.argv[3:] == ["--filter"]:
            print(ffmpeg_filter(fast))
            sys.exit()
        name = os.path.splitext(os.path.basename(tape))[0]
        json.dump({"scale": scale, "fast": fast}, open(os.path.join(OUT, name + ".timing.json"), "w"))
        print(srt([(squeeze(s * scale, fast), c) for s, c in caps], squeeze(total * scale, fast)))
    print(f"computed duration {total:.2f}s", file=sys.stderr)
