# Sgound - Documentación de API (v1.1)

Este documento detalla los endpoints RESTful implementados para la orquestación del backend en Golang hacia nuestra UI en Svelte.

Base URL en Desarrollo Front: `http://localhost:5177` (Proxied via Vite)  
Base URL Nativa: `http://localhost:8088`

---

## 🎵 1. Artistas (Artists)

### Listar todos los artistas
**Método:** `GET`  
**Ruta:** `/api/artists`  
Devuelve una lista de todos los artistas únicos indexados en la base de datos.

### Obtener álbumes de un artista
**Método:** `GET`  
**Ruta:** `/api/artists/{artist_id}/albums`  
Devuelve los álbumes asociados a un identificador de artista.

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
Streaming binario con soporte para `HTTP Range Request` (Partial Content), permitiendo adelantado y búsqueda (seeking).

### Obtener Carátula (Cover Art)
**Método:** `GET`  
**Ruta:** `/api/cover/{track_id}`  
Recupera la imagen de portada. Intenta en orden:
1. Extraer imagen embebida en el archivo de audio.
2. Buscar archivos `cover.jpg`, `folder.png`, etc., en el mismo directorio.
3. Fallback a cualquier imagen en la carpeta (insensible a mayúsculas).

---

## ⚙️ 5. Gestión y Sistema

### Escaneo de Biblioteca
**Método:** `POST`  
**Ruta:** `/api/scan`  
Inicia el escaneo recursivo del directorio `MusicPath`.

### Configuración del Servidor
**Método:** `GET` | `PUT`  
**Ruta:** `/api/config`  
Consulta o actualiza la configuración (ruta de música, patrones de renombrado, auto-organización).

### Salud de la Biblioteca
**Método:** `GET`  
**Ruta:** `/api/library/health`  
Estadísticas sobre archivos con metadatos faltantes o sin letras.

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

### Autocompletado Local
**Método:** `GET`  
**Ruta:** `/api/autocomplete?field={field}&q={query}`  
Sugiere valores basados en lo que ya tienes en tu biblioteca.

---

## 📜 7. Letras (Lyrics)

### Gestionar Letras
**Método:** `GET` | `PUT`  
**Ruta:** `/api/lyrics/{track_id}`
- **GET**: Intenta descargar letras de proveedores externos si no están embebidas.
- **PUT**: Guarda las letras permanentemente en el bloque USLT/Lyrics del archivo físico.

---

## 📂 8. Listas de Reproducción (Playlists)

### Listar Playlists
**Método:** `GET`  
**Ruta:** `/api/playlists`  
Recupera las listas de reproducción del sistema.
