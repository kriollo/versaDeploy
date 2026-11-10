#!/bin/sh
# render.sh episodes/NN-name.tape → out/NN-name.mp4 (clean), .webm, .srt (CC for YouTube)
# and -cc.mp4 (captions burned in, for social media).
set -eu
cd "$(dirname "$0")"
tape=$1
name=$(basename "$tape" .tape)
mkdir -p out
# VHS records WebM (VP9); MP4 is re-encoded with whatever H.264 encoder ffmpeg has
# (Fedora's ffmpeg-free: openh264; with RPM Fusion's ffmpeg: libx264)
# VHS drops frames on a busy machine (the video runs fast and the captions drift):
# wait until it is idle, and refuse a recording that came out too short
while [ "$(cut -d' ' -f1 /proc/loadavg | cut -d. -f1)" -ge 2 ]; do
    echo "waiting for an idle machine (load $(cut -d' ' -f1 /proc/loadavg))..."; sleep 30
done
vhs -o "out/$name.webm" "$tape"
# setup.sh's command log and the web shots, under this episode's name
mv out/.cmdlog "out/$name.cmdlog" 2>/dev/null || true
for f in out/.web*; do [ -e "$f" ] && mv "$f" "out/$name-${f#out/.}"; done
h264=libx264
ffmpeg -hide_banner -encoders 2>/dev/null | grep -q ' libx264 ' || h264=libopenh264
# 1920x1080 with the caption band under the terminal (captions never cover it)
ffmpeg -y -v error -i "out/$name.webm" -vf "pad=1920:1080:0:0:color=0x16161e" -c:v "$h264" -b:v 6M -pix_fmt yuv420p "out/$name.mp4"
python3 cc.py "$tape" > /dev/null 2> out/.computed
computed=$(sed -n 's/computed duration \([0-9.]*\)s/\1/p' out/.computed)
real=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "out/$name.mp4")
# VHS adds a few ms per keystroke: stretch the captions to the real length
scale=$(python3 -c "print($real / $computed)")
if ! python3 -c "import sys; sys.exit(abs($scale - 1) > 0.03)"; then
    echo "recording is ${real}s, expected ${computed}s: frames were dropped (busy machine). Render again with the machine idle." >&2
    exit 1
fi
python3 cc.py "$tape" "$scale" > "out/$name.srt"
# the visible waits marked "# FAST xN" play N times faster
filter=$(python3 cc.py "$tape" "$scale" --filter)
if [ -n "$filter" ]; then
    ffmpeg -y -v error -i "out/$name.mp4" -filter_complex "$filter" -map "[v]" -c:v "$h264" -b:v 6M -pix_fmt yuv420p "out/$name.fast.mp4"
    mv "out/$name.fast.mp4" "out/$name.mp4"
fi
ffmpeg -y -v error -i "out/$name.mp4" -vf "subtitles=out/$name.srt:force_style='FontName=Noto Sans,FontSize=20,PrimaryColour=&H00FFFFFF,BackColour=&H99000000,BorderStyle=4,Outline=0,Shadow=0,MarginV=14'" -c:v "$h264" -b:v 6M -pix_fmt yuv420p "out/$name-cc.mp4"
echo "out/$name.mp4 (recorded ${real}s; captions scaled x$scale), out/$name.srt, out/$name-cc.mp4"
