# Roadmap: Clon de Gonic en Golang + Svelte (sgound)

Este documento detalla el plan paso a paso para construir un servidor de streaming de música moderno, estructurado en Golang para el backend y Svelte para el frontend, con edición completa de tags y control de biblioteca y soporte para SQLite nativo.

## Fase 1: Arquitectura y Configuración Inicial (Backend)
- [ ] Definir estructura del proyecto en Go.
- [ ] Configurar SQLite (usando proxy como modernc.org/sqlite para evitar CGO).
- [ ] Implementar un sistema de configuración y logger.
- **Test Unitario**: Comprobar la conexión correcta a la base de datos en memoria y la validación de configuraciones.

## Fase 2: Control de Biblioteca (Scanner & Extractor)
- [ ] Recorrer recursivamente directorios especificados buscando archivos de audio.
- [ ] Extraer metadata nativa usando github.com/dhowden/tag.
- [ ] Lógica de sincronización.
- **Test Unitario**: Pruebas con archivos de audio "mock" en una carpeta falsa.

## Fase 3: Transmisión (Streaming Backend) y API RESTful
- [ ] Implementar servidor HTTP en Go.
- [ ] Crear endpoints de lectura.
- [ ] Endpoint de streaming de audio.
- **Test Unitario**: Pruebas usando httptest simulando peticiones a los endpoints REST.

## Fase 4: Edición de Tags (Backend)
- [ ] Endpoints de actualización.
- [ ] Usar github.com/bogem/id3v2 para guardar cambios en el disco.
- [ ] Disparador de actualización en Base de Datos.
- **Test Unitario**: Crear copias temporales de canciones, inyectar cambio de Tag de prueba y leer archivo.

## Fase 5: Interfaz Web Base (Frontend en Svelte)
- [ ] Iniciar aplicación en SvelteKit.
- [ ] Crear stores (estado reactivo).
- [ ] Componentes fundamentales (Player Bar, Listados, etc).
- **Test Unitario**: Vitest + Svelte Testing Library.

## Fase 6: Funcionalidades Avanzadas UI (Svelte)
- [ ] Interfaz de "Settings/Admin" para disparar escaneo.
- [ ] Menús contextuales y modal de edición.
- **Test Unitario**: Simular submits falsos del formulario y asercionar API calls.

## Fase 7: Integración y Empaquetado
- [ ] Compilar el proyecto Svelte.
- [ ] Usar embed para embeber la SPA web compilada dentro del backend Go.
- **Test Unitario/Integración**: Comprobar que el Request Handler devuelva Svelte index.html.
