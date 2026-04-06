.PHONY: build test clean dev-front dev-back dev all

BINARY_NAME=sgound-server

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
	@echo "=> Servidor UI de desarrollo en caliente (Vite)..."
	cd web && npm run dev

dev-back:
	@echo "=> Ejecutando backend en modo desarrollo..."
	go run ./cmd/sgound

dev:
	@echo "=> Iniciando frontend y backend en paralelo..."
	@make -j2 dev-front dev-back

clean:
	@echo "=> Limpiando archivos de build previos..."
	go clean
	rm -f $(BINARY_NAME)
	rm -rf web/dist
