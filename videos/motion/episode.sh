#!/bin/sh
# episode.sh NN — the final episode video from its recording (out/NN-name.mp4/.srt),
# browser shot and ../episodes/NN-name.json.
set -eu
cd "$(dirname "$0")"
name=$(basename "$(ls ../episodes/"$1"-*.tape)" .tape)
python3 episode.py "$1"
npx remotion render src/index.ts Episode "../out/$name-final.mp4" --props="{\"ep\":\"$1\",\"captions\":false}" --log=error --timeout=120000 --concurrency=2
npx remotion render src/index.ts Episode "../out/$name-final-cc.mp4" --props="{\"ep\":\"$1\",\"captions\":true}" --log=error --timeout=120000 --concurrency=2
echo "../out/$name-final.mp4 (+ $name-full.srt), ../out/$name-final-cc.mp4"
