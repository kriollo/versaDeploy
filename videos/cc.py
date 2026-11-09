#!/usr/bin/env python3
"""Closed captions for a VHS tape: `# CC: text` comments become an .srt.

Everything visible in the video is timed by the tape (Type, keys, Sleep), so each
caption's time is known exactly. Variable waits (Wait) must sit between Hide and
Show: hidden time isn't in the video. Usage: cc.py episode.tape > episode.srt
A caption lasts until the next one (at most MAX_S seconds); `# CC:` with no text
ends the current one.
"""
import re
import sys

MAX_S = 8.0
KEYS = {"Enter", "Backspace", "Delete", "Insert", "Down", "Up", "Left", "Right", "Tab",
        "Escape", "Space", "PageUp", "PageDown", "ScrollUp", "ScrollDown", "Home", "End"}


def seconds(v):
    m = re.fullmatch(r"([\d.]+)(ms|s|m)?", v)
    if not m:
        raise ValueError(f"bad time {v!r}")
    n = float(m[1])
    return {"ms": n / 1000, "m": n * 60}.get(m[2], n)


def captions(lines, sections=None):
    """Caption (time, text) pairs and the visible length. Chapter markers
    ("# ─── 1. Title ───") go into sections as (time, number, title) when given."""
    speed, t, hidden, out = 0.05, 0.0, False, []
    for no, raw in enumerate(lines, 1):
        line = raw.strip()
        m = re.match(r"# ─+ (\d+)\. (.+?) ─", line)
        if m and sections is not None:
            sections.append((t, int(m[1]), m[2]))
            continue
        if line.startswith("# CC:"):
            out.append((t, line[5:].strip()))
            continue
        if not line or line.startswith("#"):
            continue
        cmd, _, rest = line.partition(" ")
        base, _, at = cmd.partition("@")
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
            t += len(text[2] if text else rest) * (seconds(at) if at else speed)
        elif base in KEYS or re.fullmatch(r"(Ctrl|Alt|Shift)\+.+", base):
            n = int(rest) if rest.strip().isdigit() else 1
            t += n * (seconds(at) if at else speed)
        elif base.startswith("Wait"):
            sys.exit(f"line {no}: Wait outside Hide/Show would make the captions drift")
    return out, t


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
    with open(sys.argv[1], encoding="utf-8") as f:
        caps, total = captions(f.readlines())
    scale = float(sys.argv[2]) if len(sys.argv) > 2 else 1.0  # real/computed duration
    print(srt([(s * scale, c) for s, c in caps], total * scale))
    print(f"computed duration {total:.2f}s", file=sys.stderr)
