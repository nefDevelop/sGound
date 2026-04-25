.PHONY: build test clean dev-front dev-back dev all db-reset db-recreate dev-recreate

BINARY_NAME=sgound

# Puertos por defecto si no son pasados como comando (ej: make dev SGOUND_PORT=9000)
VITE_PORT ?= 5177
SGOUND_PORT ?= 8088

SGOUND_MUSIC_PATH ?= $(HOME)/Música/
SGOUND_USERS ?= user:rw

export SGOUND_PORT
export VITE_PORT
export SGOUND_MUSIC_PATH
export SGOUND_USERS

all: test build

build:
	@echo "=> Construyendo compilado SPA (Vite/Svelte)..."
	cd web && npm install && npm run build
	@echo "=> Empaquetando SPA dentro del binario Go con //go:embed..."
	go build -o $(BINARY_NAME) ./cmd/sgound
	@echo "=> Exito! Ejecutable '$(BINARY_NAME)' listo en la raiz."

test:
	@echo "=> Ejecutando set de pruebas unitarias locales..."
	go test -v ./...

dev-front:
	@echo "=> Servidor UI de desarrollo en caliente (Vite en puerto $(VITE_PORT))..."
	cd web && npx vite --port $(VITE_PORT) --strictPort --host

dev-back:
	@echo "=> Ejecutando backend en modo desarrollo (Puerto $(SGOUND_PORT))..."
	go run ./cmd/sgound

dev:
	@echo "=> Iniciando frontend y backend en paralelo..."
	@make -j2 dev-front dev-back

bench:
	@echo "=> Ejecutando benchmark de API (50 usuarios simultáneos)..."
	go run ./cmd/bench/main.go

clean:
	@echo "=> Limpiando archivos de build previos..."
	go clean
	rm -f $(BINARY_NAME)
	rm -rf web/dist

db-reset:
	@echo "=> Borrando TODAS las bases de datos (maestra y usuarios)..."
	rm -f sgound.db master.db
	rm -f library_*.db
	if [ ! -z "$(SGOUND_DB_PATH)" ]; then rm -f $(SGOUND_DB_PATH); fi
	@echo "=> Sistema totalmente reseteado con éxito."
	@echo "=> Ejecuta 'make dev' para arrancar de cero."

db-recreate: db-reset dev

dev-recreate: db-recreate
