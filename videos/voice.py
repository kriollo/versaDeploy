#!/usr/bin/env python3
"""Voice-over and music for an episode, from its captions.

  voice.py fit episodes/NN-name.tape   synthesize every caption and lengthen the tape's
                                       pauses where the spoken line is longer than the
                                       time its caption is on screen (re-render after)
  voice.py mix episodes/NN-name.tape   build out/NN-name-voice.mp4 and -voice-cc.mp4:
                                       each line at its caption's time (out/NN-name.srt)
                                       plus the music, ducked under the voice

Voice: Piper (offline), model in .work/voices; pronunciation.tsv says how to read
English tech words. Music: MUSIC (default out/audio_videos.mp4) from MUSIC_START s.
"""
import array
import hashlib
import os
import re
import subprocess
import sys
import wave

import cc

V = os.path.dirname(os.path.abspath(__file__))
VOICE = os.environ.get("VOICE", "es_MX-ald-medium")
MUSIC = os.environ.get("MUSIC", os.path.join(V, "out", "audio_videos.mp4"))
MUSIC_START = float(os.environ.get("MUSIC_START", "15"))
MUSIC_VOL = float(os.environ.get("MUSIC_VOL", "0.22"))
GAP = 0.35  # silence kept between one line and the next caption
RATE = 22050


def spoken(text):
    pairs = []
    with open(os.path.join(V, "pronunciation.tsv"), encoding="utf-8") as f:
        for line in f:
            if line.strip() and not line.startswith("#"):
                word, say = line.rstrip("\n").split("\t")
                pairs.append((word, say))
    pairs.sort(key=lambda p: -len(p[0]))
    # one pass, longest first, so "deploy.yml" wins over "deploy"
    rx = re.compile("|".join(r"(?<![\w.-])" + re.escape(w) + r"(?![\w-])" for w, _ in pairs))
    table = dict(pairs)
    text = rx.sub(lambda m: table[m.group(0)], text)
    return text.replace("…", ".").replace("(", "").replace(")", "")


def synth(text):
    """WAV of text (cached by voice + spoken text); returns (path, seconds)."""
    say = spoken(text)
    key = hashlib.sha1((VOICE + "\n" + say).encode()).hexdigest()[:16]
    out = os.path.join(V, ".work", "voice-cache", key + ".wav")
    if not os.path.exists(out):
        os.makedirs(os.path.dirname(out), exist_ok=True)
        model = os.path.join(V, ".work", "voices", VOICE + ".onnx")
        subprocess.run([os.path.join(V, ".work", "tts", "bin", "piper"), "-m", model, "-f", out,
                        "--sentence-silence", "0.15"], input=say.encode(), check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    with wave.open(out) as w:
        return out, w.getnframes() / w.getframerate()


def fit(tape):
    with open(tape, encoding="utf-8") as f:
        lines = f.readlines()
    caps, total = cc.captions(lines)
    # line numbers of the captions, in order
    idx = [i for i, l in enumerate(lines) if l.strip().startswith("# CC:")]
    extra = {}
    for n, ((start, text), i) in enumerate(zip(caps, idx)):
        if not text:
            continue
        _, dur = synth(text)
        end = caps[n + 1][0] if n + 1 < len(caps) else total
        need = dur + GAP - (end - start)
        if need > 0.05:
            extra[i] = round(need + 0.05, 1)
            print(f"+{extra[i]:>4}s  {text}")
    if not extra:
        print("every line fits")
        return
    out = []
    for i, l in enumerate(lines):
        out.append(l)
        if i in extra:
            out.append(f"Sleep {extra[i]}s\n")  # room for the spoken line
    with open(tape, "w", encoding="utf-8") as f:
        f.writelines(out)
    print(f"{len(extra)} pauses lengthened in {tape}: render it again")


def srt_times(path):
    out = []
    for m in re.finditer(r"(\d+):(\d+):(\d+),(\d+) --> .*\n(.+)", open(path, encoding="utf-8").read()):
        out.append((int(m[1]) * 3600 + int(m[2]) * 60 + int(m[3]) + int(m[4]) / 1000, m[5]))
    return out


def mix(tape):
    name = os.path.splitext(os.path.basename(tape))[0]
    base = os.path.join(V, "out", name)
    dur = float(subprocess.check_output(["ffprobe", "-v", "error", "-show_entries", "format=duration",
                                         "-of", "csv=p=0", base + ".mp4"]))
    track = array.array("h", bytes(int(dur * RATE) * 2))
    late = []
    for start, text in srt_times(base + ".srt"):
        path, secs = synth(text)
        with wave.open(path) as w:
            pcm = array.array("h", w.readframes(w.getnframes()))
        at = int((start + 0.15) * RATE)
        for k, s in enumerate(pcm):
            if at + k < len(track):
                track[at + k] = max(-32768, min(32767, track[at + k] + s))
        late.append((start, secs, text))
    for (s, secs, text), (s2, _, _) in zip(late, late[1:]):
        if s + 0.15 + secs > s2:
            print(f"overlaps the next line by {s + 0.15 + secs - s2:.1f}s: {text}  (run fit and render again)")
    voice = base + "-voice.wav"
    with wave.open(voice, "w") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(track.tobytes())
    # music from MUSIC_START, looped, faded in and out, ducked under the voice
    graph = (f"[1:a]atrim=start={MUSIC_START},asetpts=PTS-STARTPTS,aloop=loop=-1:size=2e9,atrim=0:{dur:.2f},"
             f"afade=t=in:d=2,afade=t=out:st={max(dur - 3, 0):.2f}:d=3,volume={MUSIC_VOL},aformat=channel_layouts=stereo[m];"
             f"[2:a]aformat=channel_layouts=stereo,asplit=2[v][sc];"
             f"[m][sc]sidechaincompress=threshold=0.015:ratio=10:attack=15:release=450[duck];"
             f"[duck][v]amix=inputs=2:normalize=0,alimiter=limit=0.95[a]")
    for suffix in ("", "-cc"):
        subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", base + suffix + ".mp4", "-i", MUSIC, "-i", voice,
                        "-filter_complex", graph, "-map", "0:v", "-map", "[a]", "-c:v", "copy",
                        "-c:a", "aac", "-b:a", "192k", "-shortest", f"{base}-voice{suffix}.mp4"], check=True)
    print(f"{base}-voice.mp4, {base}-voice-cc.mp4")


if __name__ == "__main__":
    {"fit": fit, "mix": mix}[sys.argv[1]](sys.argv[2])
