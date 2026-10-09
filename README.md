# Sgound

**Sgound** es un servidor de música moderno y ligero escrito en **Go** con una interfaz web ultrarrápida construida en **Svelte 5**. Diseñado para ser portátil, potente y estéticamente impecable.

## Características principales

- **Arquitectura Zero-CGO:** Binario estático y portátil gracias al uso de SQLite en Go puro.
- **UI Moderna:** Interfaz fluida con Svelte 5, Runes y diseño inspirado en Material Design 3 (Expressive).
- **Escaneo de Biblioteca:** Escaneo recursivo con progreso en vivo, estado consultable y arranque automático al iniciar el servidor.
- **Transcodificación al Vuelo:** Streaming con soporte de `Range Requests` y transcodificación a MP3 vía FFmpeg (bitrate configurable: original, 320 kbps, 128 kbps), con normalización ReplayGain aplicada en directo.
- **Gestión Completa de Tags:** Editor de metadatos individual y masivo con escritura física para MP3, FLAC, M4A, OGG, Opus y WAV (vía FFmpeg).
- **Letras Sincronizadas:** Archivos `.lrc` sidecar con prioridad y búsqueda automática en proveedores externos (LRCLib y Lyrics.ovh), con guardado en el propio archivo.
- **Bandeja de Entrada (Inbox):** Los archivos nuevos entran como pendientes; incluye priorización de escaneo, análisis de metadatos con sugerencias aplicables, aprobación individual o masiva y rechazo.
- **Puntuación:** Valoración por estrellas por usuario, persistida también en los tags del archivo.
- **Mantenimiento de Biblioteca:** Duplicados, huérfanos, baja calidad, tags faltantes y salud de la biblioteca; limpieza, resincronización del índice y reparación de playlists.
- **Organizador de Archivos:** Renombrado y reorganización por patrón configurable, con vista previa y progreso en segundo plano.
- **Portadas:** Extracción automática, subida por archivo o URL (aplicable al álbum completo), rescaneo y normalización; portadas de artistas con caché y metadatos externos.
- **Metadatos Externos:** Sugerencias desde MusicBrainz (álbumes, releases y detalle de pistas), búsqueda en iTunes, autocompletado de campos y biografías/imágenes de artistas (Wikipedia y Deezer).
- **Playlists Inteligentes:** Colecciones automáticas por década, género, calidad (Hi-Res), recién añadidas y actividad reciente.
- **Personalización Total:** Motor de temas dinámico basado en una semilla de color e idioma de metadatos configurable.
- **Acceso Seguro:** Setup inicial guiado, multiusuario con permisos de lectura/escritura (`r`/`rw`), claves de API (`X-API-Key`) y sesión JWT en cookie.
- **Dashboard y Logs:** Estadísticas de la biblioteca con colecciones rápidas (joyas olvidadas, más escuchadas) y visor de logs en tiempo real (SSE).
- **Watcher en Tiempo Real:** Altas y bajas de archivos detectadas con `fsnotify` sin necesidad de reescaneo manual.
- **Documentación de API:** Swagger UI en `/api/docs` generada desde `openapi.yaml`.
- **OpenSubsonic:** navegación (`getArtists/getArtist/getAlbum/getSong/search3`), streaming, descarga y portadas; auth por API key o contraseña (token `t/s` no soportado contra hashes argon2, devuelve error 41). Playlists y scrobble, pendientes (`internal/gateway`).

## Inicio rápido

### Requisitos previos
- Go 1.25 o superior (para el backend)
- Node.js y npm (para compilar el frontend)
- **FFmpeg** instalado en el sistema (opcional; necesario para transcodificación, ReplayGain en directo, edición de tags en M4A/OGG/Opus/WAV y metadatos vía `ffprobe` en formatos distintos de MP3/FLAC)

### Instalación y Ejecución

1. **Compilar el Frontend:**
   ```bash
   cd web
   npm install
   npm run build
   cd ..
   ```

2. **Ejecutar el Servidor:**
   ```bash
   go run cmd/sgound/main.go
   ```

También puedes usar el `Makefile`: `make build` genera el binario `sgound` con la SPA embebida, y `make dev` arranca frontend y backend en paralelo (ver [Desarrollo](#desarrollo)).

El servidor estará disponible en `http://localhost:8088` por defecto.

## Configuración

Sgound se puede configurar mediante variables de entorno:

| Variable | Descripción | Defecto |
| :--- | :--- | :--- |
| `SGOUND_PORT` | Puerto del servidor HTTP | `8088` |
| `SGOUND_MUSIC_PATH` | Ruta a tu biblioteca de música | `~/Música` |
| `SGOUND_DB_PATH` | Ruta al archivo de base de datos | `sgound.db` |
| `SGOUND_CACHE_PATH` | Ruta de la caché (portadas e imágenes de artistas) | `./.cache` |
| `SGOUND_USERS` | Lista de usuarios autorizados (`user:rw,amigo:r`) | (Vacío) |
| `SGOUND_THEME_COLOR` | Color semilla para el tema (Hex) | `#ff3366` |
| `SGOUND_LANGUAGE` | Idioma de metadatos externos (`es`/`en`) | `es` |
| `SGOUND_REPLAYGAIN` | Activar normalización de volumen (`true`/`false`) | `false` |
| `SGOUND_AUTO_RENAME` | Renombrar archivos según el patrón durante el escaneo (`true`/`false`) | `false` |
| `SGOUND_RENAME_PATTERN` | Patrón de renombrado (`{artist}/{album}/{track} - {title}`) | `{artist}/{album}/{track} - {title}` |
| `SGOUND_LOG_LEVEL` | Nivel de log (`debug`/`info`/`warn`/`error`) | `debug` |
| `SGOUND_SCANNER_WORKERS` | Número de workers del escaneo | `1` |
| `COOKIE_SECURE` | Activa la flag `Secure` en la cookie de sesión (solo tras proxy TLS; con HTTP interno déjalo sin activar) | (Desactivado; `1` para activar) |

## Documentación de API

Puedes encontrar el detalle de los endpoints en [API_DOCS.md](./API_DOCS.md), y la especificación OpenAPI interactiva (Swagger UI) en `http://localhost:8088/api/docs`, generada desde [openapi.yaml](./openapi.yaml).

## Desarrollo

| Comando | Descripción |
| :--- | :--- |
| `make build` | Compila la SPA (Vite) y la embebe en el binario Go |
| `make dev` | Frontend (puerto `5177`) y backend (puerto `8088`) en paralelo |
| `make test` | `svelte-check` + `go test ./...` |
| `make demo` | Biblioteca de ejemplo en `./data/demo` con su propia base de datos |
| `make bench` | Benchmark de API (50 usuarios simultáneos) |
| `make db-reset` | Borra todas las bases de datos |

## Despliegue con Docker

```bash
cp .env.example .env
# Rellena SGOUND_MUSIC_PATH_HOST y SGOUND_USERS en .env
docker compose up -d --build
```

Variables disponibles en `.env.example`: `SGOUND_PORT_HOST` (8088), `SGOUND_MUSIC_PATH_HOST` (obligatoria), `SGOUND_USERS` y, opcionales, `SGOUND_THEME_COLOR`, `SGOUND_REPLAYGAIN`, `SGOUND_LANGUAGE` y `COOKIE_SECURE=1` si el servicio queda detrás de un proxy TLS.

Los datos viven fuera del contenedor: `./data:/app/data` (`sgound.db`, con usuarios, playlists y claves API) y tu carpeta de música en `/music`.

## Copias de seguridad

```bash
sqlite3 data/sgound.db ".backup 'data/sgound-$(date +%F).db'"
```

- Todo vive en un único fichero `sgound.db` (`SGOUND_DB_PATH`): usuarios, biblioteca, playlists, puntuaciones y claves API.
- Si usas Docker, respalda también tu `.env`.
- La caché (`.cache`) se regenera sola, no hace falta copiarla.

## Licencia

Este proyecto está bajo la Licencia MIT.
