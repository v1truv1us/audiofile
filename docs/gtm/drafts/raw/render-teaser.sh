#!/usr/bin/env bash
set -euo pipefail
# Renders both cuts of the AudioFile teaser (9:16 + 16:9) via Pillow frames + ffmpeg concat.
D="docs/gtm/drafts/raw"

python3 "$D/render_frames.py" 1080 1920 frame9_
python3 "$D/render_frames.py" 1920 1080 frame16_

render () {
  local prefix="$1" out="$2" w="$3" h="$4"
  ffmpeg -y \
    -framerate 30 -loop 1 -t 2.8 -i "$D/${prefix}1.png" \
    -framerate 30 -loop 1 -t 3.2 -i "$D/${prefix}2.png" \
    -framerate 30 -loop 1 -t 3.5 -i "$D/${prefix}3.png" \
    -framerate 30 -loop 1 -t 4.5 -i "$D/${prefix}4.png" \
    -framerate 30 -loop 1 -t 4.5 -i "$D/${prefix}5.png" \
    -framerate 30 -loop 1 -t 4.5 -i "$D/${prefix}6.png" \
    -filter_complex "[0:v][1:v][2:v][3:v][4:v][5:v]concat=n=6:v=1:a=0[v];[v]fade=t=in:st=0:d=0.4,fade=t=out:st=22.6:d=0.4,format=yuv420p[v2]" \
    -map "[v2]" -r 30 -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p -movflags +faststart \
    "$D/$out"
  echo "=== $out ==="
  ffprobe -v error -select_streams v:0 -show_entries stream=width,height,codec_name,r_frame_rate -show_entries format=duration,size -of default=noprint_wrappers=1 "$D/$out"
}

render frame9_  "audiofile-teaser-9x16.mp4"  1080 1920
render frame16_ "audiofile-teaser-16x9.mp4"  1920 1080
