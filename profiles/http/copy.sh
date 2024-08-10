#!/bin/sh

exec /run/current-system/sw/bin/ffmpeg -hide_banner -loglevel warning \
  -i "${1}" \
  -c:a copy \
  -c:v copy \
  -f mpegts -
