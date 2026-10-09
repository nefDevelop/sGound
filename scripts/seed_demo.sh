#!/bin/sh
# seed_demo.sh — biblioteca de audio de ejemplo para el modo demo de sGound.
#
# Uso: scripts/seed_demo.sh [destino]
#
# Genera álbumes con pistas FLAC etiquetadas mediante ffmpeg (artista, álbum,
# título, año y número de pista) y les incrusta una portada generada, que es
# como sGound lee la carátula (imagen embebida, no un cover.jpg suelto).
#
# El contenido es musicalmente inocuo (tonos senoidales) pero la estructura y
# las etiquetas son realistas para capturar la interfaz con datos creíbles.
set -eu

DEST="${1:-./data/demo}"

if ! command -v ffmpeg >/dev/null 2>&1; then
	echo "seed_demo: se necesita ffmpeg para generar el audio de demo" >&2
	exit 1
fi

CON_COVERS=0
if command -v magick >/dev/null 2>&1; then
	CON_COVERS=1
fi

# Formato por álbum: artista|álbum|título1|título2|título3|título4
ALBUMES='Eco Lejano|Horizonte|Umbral|Deriva|Mareas|Cauce
Nocturna|Vidrio|Cristal|Reflejo|Fractura|Silencio
Costa Norte|Marea Baja|Arena|Salitre|Faro|Regreso'

echo "$ALBUMES" | while IFS='|' read -r artista album t1 t2 t3 t4; do
	[ -n "$artista" ] || continue
	dir="$DEST/$artista/$album"
	mkdir -p "$dir"

	# Portada del álbum (una por disco, se incrusta en cada pista).
	if [ "$CON_COVERS" -eq 1 ]; then
		magick -size 600x600 "xc:#1f6f5c" \
			-gravity center -fill '#f4f0e8' -pointsize 46 -annotate 0 "$album" \
			"$dir/.cover.png"
	fi

	n=1
	for titulo in "$t1" "$t2" "$t3" "$t4"; do
		[ -n "$titulo" ] || continue
		freq=$((170 + n * 35))
		base=$(printf '%s/%02d - %s' "$dir" "$n" "$titulo")

		ffmpeg -hide_banner -loglevel error -y \
			-f lavfi -i "sine=frequency=${freq}:duration=4" \
			-metadata artist="$artista" \
			-metadata album="$album" \
			-metadata title="$titulo" \
			-metadata track="$n" \
			-metadata date="2026" \
			"$base.flac"

		if [ "$CON_COVERS" -eq 1 ]; then
			# Incrusta la portada como imagen adjunta del contenedor FLAC.
			ffmpeg -hide_banner -loglevel error -y \
				-i "$base.flac" -i "$dir/.cover.png" \
				-map 0 -map 1 -c copy -disposition:v attached_pic \
				-metadata:s:v title="Cover" -metadata:s:v comment="Cover (front)" \
				"$base.tmp.flac"
			mv "$base.tmp.flac" "$base.flac"
		fi

		n=$((n + 1))
	done

	rm -f "$dir/.cover.png"
	echo "  · $artista — $album"
done

echo "seed_demo: biblioteca de ejemplo lista en $DEST"
