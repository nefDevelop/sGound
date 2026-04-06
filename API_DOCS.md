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

## 🏷️ 3. Edición Física de Tags
**Método:** `PUT`  
**Ruta:** `/api/tracks/{track_id}/tags`

Modifica permanentemente los `Metadatos ID3v2` escribiendo la carga entrante en el archivo acústico (.mp3) original usando `bogem/id3v2/v2`. Inmediatamente después, sincroniza el título editado a la base de datos SQLite como un espejo persistente.

**Body (`application/json`)**:
```json
{
  "title": "Starboy (Edit)",
  "artist": "The Weeknd",
  "album": "Starboy"
}
```

**Respuestas Posibles**:
- `200 OK`: `{"status":"success"}` indicando cambio y escritura correcta.
- `400 Bad Request`: Estructura JSON no parseable.
- `500 Internal Server Error`: Ojos fallos de I/O en disco (Ej: archivo sin permiso de escritura o no es formato .mp3).
