# sGound Roadmap: Competitividad y Saneamiento Pro

Este documento detalla las carencias críticas de sGound frente a los estándares de la industria (Navidrome, Gonic, MusicBee) y el plan para superarlos, priorizando la **velocidad de ejecución** y la **ligereza del binario**.

## 1. Comparativa de Carencias Críticas

| Característica | Rival | Estado sGound | Coste Comp. | Tamaño | Prioridad |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Multi-usuario / Auth** | Navidrome | ❌ Inexistente | Bajo | Medio | Alta |
| **Scrobbling (Last.fm)** | Gonic | ❌ Inexistente | Muy Bajo | Pequeño | Media |
| **ReplayGain (Normalización)** | MusicBee | ❌ No se lee | Medio (Parsing) | Pequeño | Alta |
| **Inbox (Bandeja de Entrada)** | MusicBee | ❌ Entrada directa | Bajo | Medio | Alta |
| **Gapless Playback** | Navidrome | ⚠️ Parcial (Browser) | Medio (Buffer) | Medio | Media |
| **Transcodificación Auto** | Gonic | ⚠️ Manual | Alto (FFmpeg) | Pequeño | Alta |
| **Filtros Dinámicos** | MusicBee | ❌ Lineal | Bajo (SQL) | Grande (UI) | Media |
| **Calificación (1-5 Stars)** | MusicBee | ⚠️ Solo Like | Bajo | Pequeño | Media |

---

## 2. Plan de Implementación por Fases

### Fase 1: Robustez de Gestión (The MusicBee Way)
*   **Implementación del "Inbox":** Nuevos archivos escaneados entran en estado `pending`. No aparecen en la biblioteca principal hasta que el usuario valida etiquetas/carátula.
    *   *Coste:* Bajo. Requiere columna `status` en tabla `tracks`.
*   **Sistema de Rating Real:** Cambiar el "Like" por una escala de 0-5.
    *   *Coste:* Muy Bajo. Cambio de `bool` a `int` en DB.
*   **Lectura de ReplayGain:** El escáner debe extraer las etiquetas `REPLAYGAIN_TRACK_GAIN` para que el reproductor ajuste el volumen automáticamente.
    *   *Coste:* Bajo en lectura, evita que el usuario toque el volumen constantemente.

### Fase 2: Eficiencia de Servidor (The Navidrome/Gonic Way)
*   **Autenticación Simple (JWT):** Soporte para múltiples usuarios con contraseñas seguras (bcrypt).
    *   *Coste:* Medio en tamaño (dependencias de cifrado), bajo en CPU.
*   **Transcodificación Inteligente:** Detectar si el cliente está en móvil/web y ofrecer 128k MP3 automáticamente si el original es FLAC/WAV pesado.
    *   *Coste:* **Alto en CPU** durante el streaming, pero ahorra ancho de banda crítico.
*   **API de Scrobbling:** Enviar datos a Last.fm/ListenBrainz al terminar una canción (50% de progreso).
    *   *Coste:* Insignificante.

### Fase 3: Experiencia de Audio Avanzada
*   **Pre-buffering para Gapless:** Implementar una cola en el frontend que empiece a descargar la siguiente canción 10 segundos antes de que termine la actual.
    *   *Coste:* Medio en memoria RAM del navegador.
*   **Smart Playlists con SQL Real:** Permitir al usuario escribir sus propias reglas SQL para listas dinámicas (ej: "Top Rock de los 90 con > 4 estrellas").
    *   *Coste:* Bajo (ya tenemos la base en la tabla `playlists`).

---

## 3. Notas sobre Velocidad y Tamaño
*   **FFmpeg:** sGound seguirá dependiendo de FFmpeg externo para transcodificar, manteniendo el binario de Go por debajo de los 20MB.
*   **SQLite:** Se mantendrá como motor único. No se migrará a PostgreSQL para evitar complejidad de despliegue, usando índices optimizados para los nuevos filtros.
*   **Svelte:** Se evitarán librerías pesadas de componentes. El CSS seguirá siendo Vanilla para garantizar tiempos de carga < 1s en móviles antiguos.
