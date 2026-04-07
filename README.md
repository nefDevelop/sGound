# Sgound 🎵

**Sgound** es un servidor de música moderno y ligero escrito en **Go** con una interfaz web ultrarrápida construida en **Svelte 5**. Diseñado para ser portátil, potente y estéticamente impecable.

## ✨ Características Principales

- **Arquitectura Zero-CGO:** Binario estático y portátil gracias al uso de SQLite en Go puro.
- **UI Moderna:** Interfaz fluida con Svelte 5, Runes y diseño inspirado en Material Design 3 (Expressive).
- **Transcodificación al Vuelo:** Streaming adaptativo usando FFmpeg (MP3/Opus) para ahorrar datos en móviles.
- **Gestión Completa de Tags:** Editor de metadatos integrado para MP3, FLAC, OGG y Opus.
- **Letras Sincronizadas:** Soporte para archivos `.lrc` y búsqueda automática en proveedores externos (LRCLib).
- **Playlists Inteligentes:** Colecciones automáticas por década, género, calidad (Hi-Res) y actividad reciente.
- **Personalización Total:** Motor de temas dinámico basado en una semilla de color.
- **Compatibilidad:** Capa de retrocompatibilidad con el protocolo **OpenSubsonic**.

## 🚀 Inicio Rápido

### Requisitos previos
- Go 1.21 o superior (para el backend)
- Node.js y npm (para compilar el frontend)
- **FFmpeg** instalado en el sistema (opcional, necesario para transcodificación)

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

El servidor estará disponible en `http://localhost:8080` por defecto.

## 🛠️ Configuración

Sgound se puede configurar mediante variables de entorno:

| Variable | Descripción | Defecto |
| :--- | :--- | :--- |
| `SGOUND_PORT` | Puerto del servidor HTTP | `8080` |
| `SGOUND_MUSIC_PATH` | Ruta a tu biblioteca de música | `./music` |
| `SGOUND_DB_PATH` | Ruta al archivo de base de datos | `sgound.db` |
| `SGOUND_THEME_COLOR` | Color semilla para el tema (Hex) | `#ff3366` |

## 📖 Documentación de API

Puedes encontrar el detalle de todos los endpoints disponibles en [API_DOCS.md](./API_DOCS.md).

## 📄 Licencia

Este proyecto está bajo la Licencia MIT.
