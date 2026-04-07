# sGound Roadmap: Competitividad y Saneamiento Pro

Este documento detalla las carencias críticas de sGound frente a los estándares de la industria (Navidrome, Gonic, MusicBee) y el plan para superarlos.

## 1. Comparativa de Carencias Críticas y Coste de Cómputo

| Característica | Rival | Estado | Tipo de Carga | Impacto CPU 🔥 | Prioridad |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Transcodificación Auto** | Gonic | ⚠️ Manual | **Cómputo Directo** | **ALTO** (FFmpeg) | Alta |
| **ReplayGain (Normalización)** | MusicBee | ✅ Implementado | **Parsing de Tags** | Bajo (Solo Meta) | Alta |
| **Escaneo de Biblioteca** | Navidrome | ✅ Estable | **I/O Disco** | Medio (Peak) | Media |
| **Multi-usuario / Auth** | Navidrome | ✅ Implementado | **Criptografía** | Bajo (Argon2id) | Alta |
| **Inbox (Inbox Management)** | MusicBee | ✅ Implementado | **SQL Query** | Muy Bajo | Alta |
| **Rating (Stars 0-5)** | MusicBee | ✅ Implementado | **SQL + Tags** | Bajo | Media |
| **Scrobbling (Last.fm)** | Gonic | ❌ Inexistente | **Red (Latencia)** | Insignificante | Media |
| **Gapless Playback** | Navidrome | ⚠️ Parcial | **RAM / Buffer** | Bajo (Memoria) | Media |
| **Filtros Dinámicos** | MusicBee | ✅ Implementado | **Indexed SQL** | Bajo | Media |

---

## 2. Análisis del Gasto de Procesador (CPU)

Para dispositivos de bajos recursos (Pi Zero, VPS baratos), sGound optimiza el uso del núcleo:

### ⚡ Carga "Hot" (Uso Activo de Ciclos)
*   **Transcodificación (FFmpeg):** El mayor consumidor. Convertir un archivo FLAC de 30MB a MP3 de 3MB consume CPU en tiempo real. **Solución:** Cachear resultados para no procesar dos veces el mismo stream.
*   **Hash de Passwords (Argon2id):** Se ejecuta solo en el login. Extremadamente seguro, pero exige un pico de cómputo controlado (~500ms) para resistir ataques de fuerza bruta.

### 🧊 Carga "Cold" (Espera de Recursos)
*   **Sugerencias MusicBrainz:** El procesador no hace nada. Solo "espera" a Internet. El coste es de **latencia**, no de **procesador**.
*   **Limpieza de Duplicados (SQL):** Al usar SQLite, una vez que la base de datos está indexada, buscar entre 50,000 canciones consume menos de un 1% de CPU.

---

## 3. Plan de Implementación por Fases

### Fase 1: Robustez de Gestión (Coste: Bajo/I/O)
*   **Implementación del "Inbox":** Nuevos archivos escaneados entran en estado `pending`.
    *   *Impacto CPU:* Muy bajo, es puramente lógico.
*   **Lectura de ReplayGain:** ✅ Implementado. Extraer `REPLAYGAIN_TRACK_GAIN` durante el escaneo.
    *   *Impacto CPU:* Bajo. Se suma al proceso de escaneo ya existente.

### Fase 2: Eficiencia de Servidor (Coste: Alto/Cómputo)
*   **Transcodificación Inteligente:** Detectar ancho de banda y convertir FLAC a Opus/MP3.
    *   *Impacto CPU:* **ALTO**. Requiere optimizar el buffer de salida de FFmpeg.
*   **Autenticación JWT & Multi-usuario:** ✅ Implementado. Soporte basado en .env (GitOps) con bases de datos aisladas.
    *   *Impacto CPU:* Bajo en uso continuo, pico leve en inicio de sesión (Argon2id).

---

## 4. Notas Finales
*   sGound evitará librerías pesadas (como Electron) manteniendo su ligereza de < 20MB.
*   **Estrategia de Memoria:** Preferimos usar un poco más de RAM (caching) a gastar CPU (re-procesamiento).
