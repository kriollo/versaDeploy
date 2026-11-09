#!/bin/sh
# render.sh — the intro (Remotion): voice + timeline, then the video with and without
# burned captions. Needs the Laravel episode rendered first (its frames are the
# screenshots) and `npm ci` once in this folder.
set -eu
cd "$(dirname "$0")"
python3 prepare.py
npx remotion render src/index.ts Intro ../out/00-intro-voice.mp4 --props='{"captions":false}' --log=error --concurrency=2
npx remotion render src/index.ts Intro ../out/00-intro-voice-cc.mp4 --props='{"captions":true}' --log=error --concurrency=2
echo "../out/00-intro-voice.mp4, ../out/00-intro-voice-cc.mp4, ../out/00-intro.srt"
