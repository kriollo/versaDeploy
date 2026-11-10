#!/usr/bin/env python3
"""The trailer's soundtrack, synthesized: no third-party music (YouTube's Content ID
claimed the track the episodes used). 120 BPM in A minor: a tense intro with a riser,
the drop at 4 s, a driving beat, a whoosh + hit at every scene change, and the voice
(Kokoro em_alex, ../voice.py) on top with the music ducked under it.

  ../.work/tts2/bin/python soundtrack.py   → assets/soundtrack.wav
"""
import os, sys, wave

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))
import voice  # noqa: E402

SR = 48000
TOTAL = 32.0
BEAT = 0.5
DROP = 4.0
CUTS = [6, 10, 14, 18, 22, 26]  # scene changes after the drop
END = 31.0  # the beat stops; the last chord rings out
# (start, text): kept in sync with index.html
VOICE = [(0.25, "Tu código está listo."), (2.3, "¿Y el servidor?"), (4.3, "versaDeploy."),
         (6.3, "Un comando, y tu app está en línea."), (10.3, "Sin agentes. Solo SSH."),
         (18.3, "¿Algo falló? Vuelve atrás solo."), (22.3, "Y prepara tu servidor por ti."),
         (26.5, "versaDeploy. Gratis y open source.")]

rng = np.random.default_rng(7)  # deterministic: the same file on every run
N = int(TOTAL * SR)
music = np.zeros((N, 2))


def t(n):
    return np.arange(n) / SR


def add(sig, at, gain=1.0, pan=0.0, buf=None):
    buf = music if buf is None else buf
    i = int(at * SR)
    if i >= len(buf):
        return
    sig = sig[: len(buf) - i]
    if sig.ndim == 1:
        sig = np.stack([sig * (1 - pan) ** 0.5, sig * (1 + pan) ** 0.5], 1) / 2 ** 0.5
    buf[i:i + len(sig)] += sig * gain


def env(n, a=0.005, r=None):
    e = np.ones(n)
    na = max(1, int(a * SR))
    e[:na] = np.linspace(0, 1, na)
    if r:
        e *= np.exp(-t(n) / r)
    return e


def lowpass(x, cutoff):
    # one-pole low-pass; cutoff may be an array (a sweep)
    cutoff = np.broadcast_to(np.asarray(cutoff, float), x.shape)
    a = 1 - np.exp(-2 * np.pi * cutoff / SR)
    y = np.empty_like(x)
    s = 0.0
    for i in range(len(x)):
        s += a[i] * (x[i] - s)
        y[i] = s
    return y


def kick(gain=1.0):
    n = int(0.45 * SR)
    f = 45 + 110 * np.exp(-t(n) / 0.03)
    return np.sin(2 * np.pi * np.cumsum(f) / SR) * env(n, 0.001, 0.12) * gain


def snare():
    n = int(0.25 * SR)
    noise = rng.standard_normal(n) * env(n, 0.001, 0.05)
    tone = np.sin(2 * np.pi * 190 * t(n)) * env(n, 0.001, 0.04)
    return (noise * 0.6 + tone * 0.5)


def hat():
    n = int(0.06 * SR)
    x = rng.standard_normal(n)
    x = x - lowpass(x, 6000)  # high-pass
    return x * env(n, 0.001, 0.015)


def saw(freq, n):
    ph = (freq * t(n)) % 1.0
    return 2 * ph - 1


def note(freq, dur, cutoff=1800, detune=0.006):
    n = int(dur * SR)
    x = saw(freq, n) + saw(freq * (1 + detune), n) + saw(freq * (1 - detune), n)
    return lowpass(x / 3, cutoff)


def boom(dur=2.2):
    n = int(dur * SR)
    f = 30 + 60 * np.exp(-t(n) / 0.25)
    sub = np.sin(2 * np.pi * np.cumsum(f) / SR) * env(n, 0.002, 0.7)
    crack = lowpass(rng.standard_normal(n), 3000) * env(n, 0.001, 0.08)
    return sub * 1.1 + crack * 0.7


def whoosh(dur=0.5):
    n = int(dur * SR)
    x = rng.standard_normal(n)
    sweep = np.geomspace(300, 9000, n)
    shape = np.sin(np.pi * np.linspace(0, 1, n)) ** 2
    return lowpass(x, sweep) * shape


def riser(dur):
    n = int(dur * SR)
    x = lowpass(rng.standard_normal(n), np.geomspace(200, 12000, n)) * np.linspace(0, 1, n) ** 2
    tone = np.sin(2 * np.pi * np.cumsum(np.geomspace(110, 880, n)) / SR) * np.linspace(0, 0.4, n) ** 2
    return x + tone


def blip(freq):
    n = int(0.08 * SR)
    return np.sign(np.sin(2 * np.pi * freq * t(n))) * env(n, 0.001, 0.02) * 0.3


# ── intro: drone, heartbeat, glitch blips, riser into the drop ──
add(note(55, DROP, cutoff=400) * env(int(DROP * SR), 1.5), 0, 0.35)
add(note(82.4, DROP, cutoff=300) * env(int(DROP * SR), 2.0), 0, 0.2)
for b in (0, 1.0, 2.0, 3.0):
    add(kick(0.7), b, 0.6)
    add(kick(0.5), b + 0.25, 0.35)
for i, at in enumerate(np.arange(0.3, 1.9, 0.09)):  # the first line "types"
    add(blip(1800 + 300 * (i % 3)), at, 0.12, pan=0.4 * ((i % 2) * 2 - 1))
add(riser(1.6), DROP - 1.6, 0.5)
add(whoosh(0.35)[::-1], DROP - 0.35, 0.6)

# ── from the drop: the beat ──
add(boom(2.5), DROP, 1.0)
chords = [(55, 65.4, 82.4), (43.7, 55, 65.4), (65.4, 82.4, 98), (49, 61.7, 73.4)]  # Am F C G
bar = 0
for b in np.arange(DROP, END, BEAT):
    i = int(round((b - DROP) / BEAT))
    rolled = 18.0 <= b < 18.5  # the rollback: the beat drops out for a moment
    if rolled:
        continue
    add(kick(), b, 0.9)
    if i % 2 == 1:
        add(snare(), b, 0.45)
    for h in (0, 0.25):
        add(hat(), b + h, 0.18 if h else 0.1, pan=0.3)
    root = chords[(i // 4) % 4][0]
    for e in (0, 0.25):  # driving eighth-note bass
        add(note(root, 0.22, cutoff=700, detune=0.003) * env(int(0.22 * SR), 0.003, 0.12), b + e, 0.5)
    if i % 4 == 0:  # a chord stab on every bar
        stab = sum(note(f * 4, 1.6, cutoff=2500) for f in chords[(i // 4) % 4])
        add(stab * env(len(stab), 0.01, 0.5), b, 0.12, pan=-0.2)
        bar += 1

# the framework burst: a bright hit on every name
for k, b in enumerate(np.arange(14, 18, BEAT)):
    add(blip(880 * (1 + k % 4 * 0.25)) * 1.5, b, 0.25, pan=0.5 * ((k % 2) * 2 - 1))

# the rollback: a tape-stop on the beat before it, a reverse sweep back
n = int(0.5 * SR)
add(note(110, 0.5, cutoff=np.geomspace(4000, 200, n)) * np.linspace(1, 0, n), 17.75, 0.3)
add(whoosh(0.5)[::-1], 18.0, 0.5)

# a whoosh into every scene and a hit on it
for c in CUTS:
    add(whoosh(0.45), c - 0.45, 0.45, pan=-0.3)
    add(boom(1.2) * 0.5, c, 0.55)

# the end: the big hit, a held chord that rings out
add(boom(4.0), 26, 1.0)
held = sum(note(f * 2, TOTAL - END + 1.5, cutoff=1500) for f in chords[0])
add(held * env(len(held), 0.05, 1.6), END - 0.5, 0.25)

# ── voice on top, music ducked under it ──
vox = np.zeros((N, 2))
duck = np.ones(N)
for at, text in VOICE:
    path, _ = voice.synth(text)
    with wave.open(path) as w:
        sr, ch = w.getframerate(), w.getnchannels()
        x = np.frombuffer(w.readframes(w.getnframes()), np.int16).astype(float) / 32768
    if ch == 2:
        x = x.reshape(-1, 2).mean(1)
    if sr != SR:
        x = np.interp(np.arange(int(len(x) * SR / sr)) * sr / SR, np.arange(len(x)), x)
    add(x / (np.abs(x).max() + 1e-9), at, 0.9, buf=vox)
    i, j = int(at * SR), int((at + len(x) / SR) * SR)
    duck[max(0, i - 2400):j + 4800] = 0.45
duck = np.convolve(duck, np.ones(2400) / 2400, mode="same")  # smooth the ducking
mix = music * duck[:, None] * 0.55 + vox
# fade the very end, then soft-clip and normalize to -1 dBFS
mix *= np.clip((TOTAL - t(N)) / 1.0, 0, 1)[:, None]
mix = np.tanh(mix * 1.2)
mix *= 10 ** (-1 / 20) / np.abs(mix).max()

os.makedirs(os.path.join(HERE, "assets"), exist_ok=True)
out = os.path.join(HERE, "assets", "soundtrack.wav")
with wave.open(out, "wb") as w:
    w.setnchannels(2)
    w.setsampwidth(2)
    w.setframerate(SR)
    w.writeframes((mix * 32767).astype(np.int16).tobytes())
print(out, f"{TOTAL}s")
