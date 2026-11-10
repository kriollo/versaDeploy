#!/bin/sh
# make.sh NN — an episode from scratch:
#   1. fresh server + project, room for the voice in the tape
#   2. the terminal recording (VHS) + its captions
#   3. the app in a browser, while the server still runs it
#   4. the final video (Remotion): opening, a card per chapter, the recording, the
#      browser, the outro, with voice (no music)
# → out/NN-name-final.mp4 (YouTube, + NN-name-full.srt) and NN-name-final-cc.mp4
set -eu
cd "$(dirname "$0")"
tape=$(ls episodes/"$1"-*.tape)
name=$(basename "$tape" .tape)
url=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['browser']['url'])" "episodes/$name.json")
./prep.sh "$1" >/dev/null
python3 voice.py fit "$tape"
./render.sh "$tape"
login=$(python3 -c "import json,sys; l=json.load(open(sys.argv[1]))['browser'].get('login'); print(l and l['url'] or '')" "episodes/$name.json")
if [ -n "$login" ]; then   # a page behind a login (versaWYS's panel)
    eval "$(python3 -c "import json,sys,shlex; l=json.load(open(sys.argv[1]))['browser']['login']; print('EMAIL='+shlex.quote(l['email']), 'PASS='+shlex.quote(l['password']))" "episodes/$name.json")"
    EMAIL=$EMAIL PASS=$PASS node shot-login.mjs "$login" "$url" "out/$name-browser.png"
else
    ./shot.sh "$url" "out/$name-browser.png"
fi
./motion/episode.sh "$1"
