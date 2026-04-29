#!/bin/bash
# Crear un MP3 de prueba
ffmpeg -f lavfi -i anullsrc=r=44100:cl=stereo -t 1 -y test_input.mp3
# Intentar actualizar tags con el mismo comando que usa Go
ffmpeg -i test_input.mp3 -metadata title="MP3 Title" -metadata artist="MP3 Artist" -codec copy -map_metadata 0 -y test_output.mp3
echo "Exit code: $?"
