#!/bin/sh
# make.sh NN — an episode from scratch:
#   1. fresh server + project, room for the voice in the tape
#   2. the terminal recording (VHS) + its captions
#   3. the app in a browser, while the server still runs it
#   4. the final video (Remotion): opening, a card per chapter, the recording, the
#      browser, the outro, with voice and music
# → out/NN-name-final.mp4 (YouTube, + NN-name-full.srt) and NN-name-final-cc.mp4
set -eu
cd "$(dirname "$0")"
tape=$(ls episodes/"$1"-*.tape)
name=$(basename "$tape" .tape)
url=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['browser']['url'])" "episodes/$name.json")
./prep.sh "$1" >/dev/null
python3 voice.py fit "$tape"
./render.sh "$tape"
./shot.sh "$url" "out/$name-browser.png"
./motion/episode.sh "$1"
