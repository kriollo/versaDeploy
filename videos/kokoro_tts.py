#!/usr/bin/env python3
"""Kokoro-82M (Apache-2.0) batch synthesis, run in its own venv (.work/tts2, Python 3.12):
reads [{"text": …, "out": …}, …] as JSON on stdin and writes 16-bit mono WAVs at 24 kHz.
The model is loaded once per batch. voice.py calls it; use the GPU when there is one."""
import json, os, sys, wave

import numpy as np
import torch
from kokoro import KPipeline

jobs = json.load(sys.stdin)
pipe = KPipeline(lang_code="e", device="cuda" if torch.cuda.is_available() else "cpu")
voice = os.environ.get("VOICE", "em_alex")
speed = float(os.environ.get("KOKORO_SPEED", "1.0"))
for job in jobs:
    audio = np.concatenate([a.cpu().numpy() if hasattr(a, "cpu") else a for _, _, a in pipe(job["text"], voice=voice, speed=speed)])
    pcm = (np.clip(audio, -1, 1) * 32767).astype("<i2")
    tmp = job["out"] + ".tmp"
    with wave.open(tmp, "w") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(24000)
        w.writeframes(pcm.tobytes())
    os.replace(tmp, job["out"])
