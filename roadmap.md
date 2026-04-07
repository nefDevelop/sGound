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

## 🚀 Fase 7: Próximas Funcionalidades (Roadmap Actualizado)
- [ ] **Renombrado Físico de Archivos**: Opción para organizar archivos en carpetas basadas en Artist/Album (Ej: `Artist/Album/01 - Title.mp3`).
- [ ] **Smart Playlists**: Implementar motor de reglas SQL para listas dinámicas (ej: "Añadidos recientemente", "Género: Rock").
- [ ] **Metadatos Externos**: Integración con APIs para Biografías, Letras (lyrics.ovh) y Fotos de artistas.
- [ ] **Gestión de Playlists en UI**: Crear, editar y borrar listas de reproducción desde la interfaz.
- [ ] **Vista de Rejilla de Álbumes**: Navegación visual por carátulas grandes.
- [ ] **Estado de Escaneo en Tiempo Real**: Notificaciones o barra de progreso durante el escaneo de biblioteca.
