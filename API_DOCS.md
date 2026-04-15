# Sgound - Documentación de API (v1.2)

Este documento detalla los endpoints RESTful implementados para la orquestación del backend en Golang hacia nuestra UI en Svelte.

Base URL en Desarrollo Front: `http://localhost:5177` (Proxied via Vite)  
Base URL Nativa: `http://localhost:8088`

---

## 🎵 1. Artistas (Artists)

### Listar todos los artistas
**Método:** `GET`  
**Ruta:** `/api/artists`  
Devuelve una lista de todos los artistas únicos indexados en la base de datos.

### Obtener detalles de un artista
**Método:** `GET`  
**Ruta:** `/api/artists/{artist_id}`  
Devuelve información enriquecida del artista, incluyendo estadísticas de la biblioteca y metadatos externos.
**Respuesta (JSON):** 
```json
{
  "id": "...",
  "name": "Nombre del Artista",
  "albumCount": 5,
  "trackCount": 42,
  "biography": "Resumen biográfico de Wikipedia...",
  "externalImage": "https://url-imagen-alta-resolucion.jpg"
}
```

### Obtener álbumes de un artista
**Método:** `GET`  
**Ruta:** `/api/artists/{artist_id}/albums`  
Devuelve los álbumes asociados a un identificador de artista, ordenados por año descendente.

---

## 💿 2. Álbumes (Albums)

### Listar todos los álbumes
**Método:** `GET`  
**Ruta:** `/api/albums`  
Incluye información básica, año y un `coverId` (ID de una pista de ejemplo para descargar la carátula).

### Obtener canciones de un álbum
**Método:** `GET`  
**Ruta:** `/api/albums/{album_id}/tracks`  
Devuelve la lista de pistas del álbum ordenadas por número de track.

---

## 🎶 3. Canciones (Tracks)

### Listar todas las canciones
**Método:** `GET`  
**Ruta:** `/api/tracks`

### Editar Metadatos (Tags)
**Método:** `PUT`  
**Ruta:** `/api/tracks/{track_id}/tags`  
Actualiza los metadatos físicos del archivo (ID3/FLAC) y la base de datos.
**Carga útil (JSON):** `{"title": "...", "artist": "...", "album": "...", "year": 2024, ...}`

### Borrar canción
**Método:** `DELETE`  
**Ruta:** `/api/tracks/{track_id}`  
Elimina la canción de la base de datos (Nota: actualmente no borra el archivo físico).

---

## 🎧 4. Multimedia y Streaming

### Stream de Audio
**Método:** `GET`  
**Ruta:** `/api/stream/{track_id}`  
**Parámetros Query:**
- `bitrate` (Opcional): ej. `128k`, `64k`. Si se especifica, se activa la transcodificación al vuelo vía FFmpeg a formato MP3.
- `id` (Alternativo): se puede pasar como query param si no está en la ruta.

Streaming binario con soporte para `HTTP Range Request` (Partial Content), permitiendo adelantado y búsqueda (seeking).

### Obtener Carátula (Cover Art)
**Método:** `GET`  
**Ruta:** `/api/cover/{id}`  
Recupera la imagen de portada. `{id}` puede ser un ID de track o de álbum. Intenta en orden:
1. Extraer imagen embebida en el archivo de audio.
2. Buscar archivos `cover.jpg`, `folder.png`, etc., en el mismo directorio.
3. Fallback a cualquier imagen en la carpeta.

---

## ⚙️ 5. Gestión y Sistema

### Escaneo de Biblioteca
**Método:** `POST`  
**Ruta:** `/api/scan`  
Inicia el escaneo recursivo del directorio `MusicPath`.

### Configuración del Servidor
**Método:** `GET` | `PUT`  
**Ruta:** `/api/config`  
**Carga útil (PUT):** `{"musicPath": "...", "themeColor": "#hex", "autoRename": bool, "renamePattern": "..."}`  
Consulta o actualiza la configuración.

### Salud de la Biblioteca
**Método:** `GET`  
**Ruta:** `/api/library/health`  
Estadísticas sobre archivos con metadatos faltantes o sin letras.

### Metadatos de la Biblioteca
**Método:** `GET`  
**Ruta:** `/api/library/metadata`  
Devuelve una lista de décadas y géneros **existentes** en la base de datos para filtrado dinámico.

### Dashboard (Estadísticas)
**Método:** `GET`  
**Ruta:** `/api/dashboard`  
Devuelve contadores globales y colecciones rápidas (Joyas olvidadas, Más escuchadas).

---

## 🔍 6. Búsqueda y Metadatos

### Búsqueda Global
**Método:** `GET`  
**Ruta:** `/api/search?q={query}`  
Busca en pistas, artistas y álbumes simultáneamente.

### Sugerencias de iTunes (Búsqueda Externa)
**Método:** `GET`  
**Ruta:** `/api/metadata/search?artist={artist}&title={title}`  
Consulta la API oficial de iTunes para sugerir etiquetas correctas.

---

## 📜 7. Letras (Lyrics)

### Gestionar Letras
**Método:** `GET` | `PUT`  
**Ruta:** `/api/lyrics/{track_id}`
- **GET**: Intenta descargar letras de proveedores externos si no están embebidas.
- **PUT**: Guarda las letras permanentemente en el bloque USLT/Lyrics del archivo físico.

---

## 📂 8. Listas de Reproducción (Playlists)

### Listar y Crear Playlists
**Método:** `GET` | `POST`  
**Ruta:** `/api/playlists`
- **GET**: Lista todas las playlists creadas por el usuario.
- **POST**: Crea una nueva playlist. Carga útil: `{"name": "Nombre"}`.

### Obtener canciones de una Playlist
**Método:** `GET`  
**Ruta:** `/api/playlists/{playlist_id}/tracks`  
Devuelve la lista de pistas asociadas a la playlist.

### Playlists Inteligentes (Smart Playlists)
**Método:** `GET`  
**Ruta:** `/api/smart-playlists/{type}`  
**Tipos:** `recently-played`, `high-quality`, `recently-added`, `decade`, `genre`.
**Parámetros Query:**
- `value`: Para tipos `decade` (ej. `1990`) o `genre` (ej. `Rock`).
