# Sgound - Documentación de API

Este documento detalla los endpoints RESTful implementados para la orquestación del backend en Golang hacia nuestra UI en Svelte.

Base URL en Desarrollo Front: `http://localhost:5174` (Proxied)  
Base URL Nativa: `http://localhost:8080`

---

## 🎵 1. Listar Artistas
**Método:** `GET`  
**Ruta:** `/api/artists`

Devuelve una lista de los artistas detectados por el servidor durante el escaneo en directorios usando la base de datos SQLite.

**Respuesta Exitosa (`200 OK`)**:
```json
[
  {
    "id": "daft-punk",
    "name": "Daft Punk"
  },
  {
    "id": "the-weeknd",
    "name": "The Weeknd"
  }
]
```

---

## 🎧 2. Stream Directo (Audio)
**Método:** `GET`  
**Ruta:** `/api/stream/{track_id}`

Inicia el streaming HTTP del binario del audio local. Este endpoint está configurado para atender peticiones `HTTP Range Request / Partial Content` permitiendo a clientes y aplicaciones HTML5 adelantar y buscar intervalos exactos de la canción limpiamente sin recargar la pista entera.

**Parámetros URL**:
- `track_id` (string): El id único en la BD.

**Respuestas Posibles**:
- `200 OK / 206 Partial Content`: Binario multimedia enviado a través de socket HTTP.
- `404 Not Found`: La pista no se halló en la BD o en la ruta física indexada.
- `400 Bad Request`: Faltan parámetros en la URL.

---

## 📂 4. Listar Álbumes
**Método:** `GET`  
**Ruta:** `/api/albums`

Devuelve todos los álbumes indexados, incluyendo el nombre del artista y el año.

**Respuesta**:
```json
[
  {
    "id": "daft-punk-discovery",
    "title": "Discovery",
    "artist": "Daft Punk",
    "year": 2001
  }
]
```

---

## 🗂️ 5. Álbumes por Artista
**Método:** `GET`  
**Ruta:** `/api/artists/{artist_id}/albums`

**Respuesta**: Lista de álbumes pertenecientes a ese artista específico.

---

## 🎵 6. Canciones por Álbum
**Método:** `GET`  
**Ruta:** `/api/albums/{album_id}/tracks`

**Respuesta**: Lista de canciones ordenadas por `track_num`.

---

## 🔍 7. Escaneo de Biblioteca (Trigger)
**Método:** `POST`  
**Ruta:** `/api/scan`

Dispara un escaneo asíncrono en el servidor. El servidor recorrerá la `MusicPath` configurada y actualizará la base de datos con los nuevos archivos encontrados o cambios en los existentes.

---

## ⚙️ 8. Configuración de Ruta
**Método:** `GET` | `PUT`  
**Ruta:** `/api/config`

Permite consultar o cambiar la ruta física donde Sgound busca la música (equivalente a la librería en MusicBee).

**Carga PUT**: 
```json
{
  "musicPath": "/storage/emulated/0/Music",
  "autoRename": true,
  "renamePattern": "{artist}/{year} - {album}/{track} - {title}"
}
```

---

## 🔎 9. Búsqueda Global
**Método:** `GET`  
**Ruta:** `/api/search?q={query}`

Realiza una búsqueda parcial en Títulos, Artistas, Álbumes y Géneros.

---

## ⚡ 10. Búsqueda de Metadatos (Externa)
**Método:** `GET`  
**Ruta:** `/api/metadata/search?artist={artist}&title={title}`

Consulta la API de iTunes para sugerir etiquetas correctas. Devuelve hasta 5 resultados con carátulas.

---

## 📝 11. Autocompletado Local
**Método:** `GET`  
**Ruta:** `/api/autocomplete?field={field}&q={query}`

Sugiere valores existentes en tu DB.  
**Campos:** `artist`, `album`, `genre`, `composer`.

---

## 📊 12. Dashboard y Estadísticas
**Método:** `GET`  
**Ruta:** `/api/dashboard`

Devuelve contadores totales y una lista de "Joyas Olvidadas" (10 canciones aleatorias).

---

## 🛡️ 13. Salud de la Biblioteca
**Método:** `GET`  
**Ruta:** `/api/library/health`

Detecta inconsistencias: canciones sin género, sin año o sin letras incrustadas.

---

## 📁 14. Organización Física
**Método:** `POST`  
**Ruta:** `/api/tracks/{track_id}/organize`

Mueve manualmente un archivo según el patrón configurado en Settings.

---

## 📜 15. Gestión de Letras (Lyrics)
**Método:** `GET` | `PUT`  
**Ruta:** `/api/lyrics/{track_id}`

- **GET:** Descarga letras de la nube (lyrics.ovh).
- **PUT:** Guarda letras permanentemente en el archivo físico (tag USLT).
