# Roadmap: Sgound - Music Server (Golang + Svelte)

## ✅ Fase 1: Arquitectura y Configuración Inicial (Backend)
- [x] Definir estructura del proyecto en Go.
- [x] Configurar SQLite (modernc.org/sqlite - No CGO).
- [x] Sistema de configuración (Variables de entorno).
- [x] Logger estructurado (slog).

## ✅ Fase 2: Control de Biblioteca (Scanner & Extractor)
- [x] Escaneo recursivo de directorios.
- [x] Extracción de metadata extendida (bitrate, samplerate, format, size).
- [x] Soporte para MP3, FLAC, OGG, M4A, MP4, OPUS, WAV.
- [x] Lógica de sincronización en DB.

## ✅ Fase 3: Transmisión y API RESTful
- [x] Servidor HTTP con soporte para HTTP Range (Seek).
- [x] Endpoints para Artistas, Álbumes y Canciones.
- [x] Búsqueda Global (Like SQL en múltiples campos).
- [x] Endpoint de Portadas (Embebidas + Archivos externos como cover.jpg).

## ✅ Fase 4: Edición de Tags (Backend)
- [x] Edición física de tags en archivos (id3v2 para MP3).
- [x] Sincronización inmediata con la Base de Datos.
- [x] Soporte para edición masiva (Bulk update).

## ✅ Fase 5: Interfaz Web (Frontend en Svelte)
- [x] UI Moderna con Grid Layout y Glassmorphism.
- [x] Componente de tabla de canciones con selección múltiple.
- [x] MiniPlayer con barra de progreso, volumen y metadatos técnicos.
- [x] Editores de Tags (Individual y Masivo) integrados.

## ✅ Fase 6: Integración y Empaquetado
- [x] Embeber la UI (SPA) dentro del binario Go usando `//go:embed`.
- [x] Single binary deployment (15MB aprox).

## ✅ Fase 7: Funcionalidades Avanzadas (Implementadas)
- [x] **Smart Playlists**: Motor de filtrado dinámico por década, género, calidad y actividad reciente.
- [x] **Letras Sincronizadas**: Integración con LRCLib y soporte para tags USLT/Lyrics.
- [x] **Transcodificación al Vuelo**: Streaming adaptativo con FFmpeg para dispositivos móviles.
- [x] **Motor de Temas Dinámico**: Personalización de la interfaz basada en Material Design 3 (Expressive).
- [x] **Vista de Rejilla de Álbumes**: Navegación visual optimizada por carátulas.

## 🚀 Fase 8: Próximas Funcionalidades
- [ ] **Multi-usuario**: Sistema de autenticación y perfiles.
- [ ] **App Móvil (PWA)**: Optimización para instalación como aplicación nativa en móviles.
- [ ] **Mejoras de Escaneo**: Notificaciones en tiempo real vía WebSockets del progreso del escáner.
- [ ] **Renombrado Físico de Archivos**: Opción para organizar archivos en carpetas basadas en Artist/Album (Ej: `Artist/Album/01 - Title.mp3`).
- [ ] **Radio/Mezcla Automática**: Generación de colas de reproducción infinitas basadas en gustos.

