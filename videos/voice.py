#!/usr/bin/env python3
"""Voice-over for an episode, from its captions.

  voice.py fit episodes/NN-name.tape   synthesize every caption and lengthen the tape's
                                       pauses where the spoken line is longer than the
                                       time its caption is on screen (re-render after)
  voice.py mix episodes/NN-name.tape   build out/NN-name-voice.mp4 and -voice-cc.mp4:
                                       each line at its caption's time (out/NN-name.srt)
                                       (no music: Content ID claims it)

Voice: Piper (offline, default), model in .work/voices; pronunciation.tsv says how to
read English tech words. TTS=elevenlabs uses ElevenLabs instead (ELEVENLABS_API_KEY and
ELEVENLABS_VOICE, or .work/elevenlabs_api_key.txt and .work/elevenlabs_voice.txt).
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


TTS = os.environ.get("TTS", "kokoro")  # kokoro (local, GPU) | piper (local) | elevenlabs
VOICE = os.environ.get("VOICE", {"kokoro": "em_alex", "piper": "es_MX-ald-medium"}.get(TTS, ""))


def secret(name):
    """$NAME, else .work/<name>.txt (kept out of git)."""
    v = os.environ.get(name.upper(), "")
    path = os.path.join(V, ".work", name.lower() + ".txt")
    if not v and os.path.exists(path):
        v = open(path, encoding="utf-8").read().strip()
    if not v:
        sys.exit(f"{name.upper()} not set: export it or write it to {path}")
    return v


def elevenlabs(text, out):
    """ElevenLabs multilingual model: raw 16-bit PCM at RATE, wrapped as WAV."""
    import json
    import urllib.request
    req = urllib.request.Request(
        f"https://api.elevenlabs.io/v1/text-to-speech/{secret('elevenlabs_voice')}?output_format=pcm_{RATE}",
        data=json.dumps({"text": text, "model_id": os.environ.get("ELEVENLABS_MODEL", "eleven_multilingual_v2"),
                         "voice_settings": {"stability": 0.5, "similarity_boost": 0.75}}).encode(),
        headers={"xi-api-key": secret("elevenlabs_api_key"), "Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as r:
        pcm = r.read()
    with wave.open(out, "w") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(pcm)


def cache_path(text):
    """(spoken text, cached WAV path) of a line for the current engine and voice."""
    # Kokoro and Piper read with Spanish rules: the dictionary fixes English words
    say = text if TTS == "elevenlabs" else spoken(text)
    voice = "elevenlabs:" + secret("elevenlabs_voice") if TTS == "elevenlabs" else f"{TTS}:{VOICE}"
    key = hashlib.sha1((voice + "\n" + say).encode()).hexdigest()[:16]
    return say, os.path.join(V, ".work", "voice-cache", key + ".wav")


def prefetch(texts):
    """Synthesize every missing line at once (Kokoro loads its model once per batch)."""
    os.makedirs(os.path.join(V, ".work", "voice-cache"), exist_ok=True)
    jobs = []
    for text in texts:
        say, out = cache_path(text)
        if text and not os.path.exists(out) and all(j["out"] != out for j in jobs):
            jobs.append({"text": say, "out": out})
    if not jobs:
        return
    if TTS == "kokoro":
        import json
        print(f"kokoro: {len(jobs)} line(s) with {VOICE}", file=sys.stderr)
        subprocess.run([os.path.join(V, ".work", "tts2", "bin", "python"), os.path.join(V, "kokoro_tts.py")],
                       input=json.dumps(jobs).encode(), check=True, env={**os.environ, "VOICE": VOICE},
                       stderr=subprocess.DEVNULL)
    elif TTS == "elevenlabs":
        for j in jobs:
            elevenlabs(j["text"], j["out"] + ".tmp")
            os.replace(j["out"] + ".tmp", j["out"])
    else:
        for j in jobs:
            model = os.path.join(V, ".work", "voices", VOICE + ".onnx")
            subprocess.run([os.path.join(V, ".work", "tts", "bin", "piper"), "-m", model, "-f", j["out"],
                            "--sentence-silence", "0.15"], input=j["text"].encode(), check=True,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)


def synth(text):
    """WAV of a line (cached by engine, voice and text); returns (path, seconds)."""
    _, out = cache_path(text)
    if not os.path.exists(out):
        prefetch([text])
    with wave.open(out) as w:
        return out, w.getnframes() / w.getframerate()


def fit(tape):
    with open(tape, encoding="utf-8") as f:
        lines = f.readlines()
    caps, total = cc.captions(lines)
    prefetch([text for _, text in caps])
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
    lines = srt_times(base + ".srt")
    prefetch([text for _, text in lines])
    with wave.open(synth(lines[0][1])[0]) as w:
        rate = w.getframerate()  # the engine's rate (Kokoro 24 kHz, Piper 22.05 kHz)
    track = array.array("h", bytes(int(dur * rate) * 2))
    late = []
    for start, text in lines:
        path, secs = synth(text)
        with wave.open(path) as w:
            pcm = array.array("h", w.readframes(w.getnframes()))
        at = int((start + 0.15) * rate)
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
        w.setframerate(rate)
        w.writeframes(track.tobytes())
    for suffix in ("", "-cc"):
        subprocess.run(["ffmpeg", "-y", "-v", "error", "-i", base + suffix + ".mp4", "-i", voice,
                        "-map", "0:v", "-map", "1:a", "-c:v", "copy",
                        "-c:a", "aac", "-b:a", "192k", "-shortest", f"{base}-voice{suffix}.mp4"], check=True)
    print(f"{base}-voice.mp4, {base}-voice-cc.mp4")


if __name__ == "__main__":
    {"fit": fit, "mix": mix}[sys.argv[1]](sys.argv[2])
